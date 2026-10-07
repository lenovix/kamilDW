package extserver

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

type DownloadPayload struct {
	URL         string            `json:"url"`
	Filename    string            `json:"filename,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	IsHLS       bool              `json:"isHls,omitempty"`
	Connections int               `json:"connections,omitempty"`
}

type Server struct {
	addr          string
	server        *http.Server
	OnDownload    func(payload DownloadPayload) (string, error)
	OnList        func(category string) (any, error)
	OnPause       func(id string) bool
	OnResume      func(id string) bool
	OnDelete      func(id string) bool
	OnSetSpeed    func(bytesPerSec int64)
	mu            sync.Mutex
	listeners     map[chan any]struct{}
}

func New(addr string) *Server {
	if addr == "" {
		addr = "127.0.0.1:19890"
	}
	return &Server{
		addr:      addr,
		listeners: make(map[chan any]struct{}),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskAction)
	mux.HandleFunc("/api/limiter", s.handleLimiter)
	mux.HandleFunc("/api/events", s.handleEvents)

	fs := http.FileServer(http.Dir("./frontend/dist"))
	mux.Handle("/", fs)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: corsMiddleware(mux),
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to bind %s: %w", s.addr, err)
	}

	go s.server.Serve(ln)
	return nil
}

func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "app": "kamilDW"})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.OnList == nil {
		json.NewEncoder(w).Encode([]any{})
		return
	}
	cat := r.URL.Query().Get("category")
	list, err := s.OnList(cat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "task id required", http.StatusBadRequest)
		return
	}
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	w.Header().Set("Content-Type", "application/json")
	switch action {
	case "pause":
		ok := s.OnPause != nil && s.OnPause(id)
		json.NewEncoder(w).Encode(map[string]any{"ok": ok})
	case "resume":
		ok := s.OnResume != nil && s.OnResume(id)
		json.NewEncoder(w).Encode(map[string]any{"ok": ok})
	default:
		if r.Method == http.MethodDelete {
			ok := s.OnDelete != nil && s.OnDelete(id)
			json.NewEncoder(w).Encode(map[string]any{"ok": ok})
			return
		}
		http.Error(w, "unknown action", http.StatusNotFound)
	}
}

func (s *Server) handleLimiter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		BytesPerSec int64 `json:"bytesPerSec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.OnSetSpeed != nil {
		s.OnSetSpeed(req.BytesPerSec)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var p DownloadPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if p.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	var taskID string
	var err error
	if s.OnDownload != nil {
		taskID, err = s.OnDownload(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	s.Broadcast(map[string]any{"type": "new_task", "id": taskID, "url": p.URL, "filename": p.Filename})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "id": taskID})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan any, 20)
	s.mu.Lock()
	s.listeners[ch] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.listeners, ch)
		s.mu.Unlock()
		close(ch)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) Broadcast(msg any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.listeners {
		select {
		case ch <- msg:
		default:
		}
	}
}
