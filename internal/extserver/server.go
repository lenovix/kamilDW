package extserver

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
)

type DownloadPayload struct {
	URL      string            `json:"url"`
	Filename string            `json:"filename,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	IsHLS    bool              `json:"isHls,omitempty"`
}

type Server struct {
	addr       string
	server     *http.Server
	OnDownload func(payload DownloadPayload)
	mu         sync.Mutex
	listeners  map[chan DownloadPayload]struct{}
}

func New(addr string) *Server {
	if addr == "" {
		addr = "127.0.0.1:19890"
	}
	return &Server{
		addr:      addr,
		listeners: make(map[chan DownloadPayload]struct{}),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/download", s.handleDownload)
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
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
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

	if s.OnDownload != nil {
		s.OnDownload(p)
	}

	s.broadcast(p)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan DownloadPayload, 10)
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

func (s *Server) broadcast(p DownloadPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.listeners {
		select {
		case ch <- p:
		default:
		}
	}
}
