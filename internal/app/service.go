package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"kamildw/internal/clipboard"
	"kamildw/internal/engine"
	"kamildw/internal/extserver"
	"kamildw/internal/limiter"
	"kamildw/internal/store"
)

type Service struct {
	db        *store.DB
	eng       *engine.Engine
	ext       *extserver.Server
	limiter   *limiter.Bucket
	clip      *clipboard.Listener
	downloads string
	mu        sync.Mutex
}

func NewService(dbPath string, downloadDir string) (*Service, error) {
	if downloadDir == "" {
		home, _ := os.UserHomeDir()
		downloadDir = filepath.Join(home, "Downloads", "kamilDW")
	}
	_ = os.MkdirAll(downloadDir, 0o755)

	db, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}

	eng := engine.New()
	srv := &Service{
		db:        db,
		eng:       eng,
		limiter:   limiter.New(0),
		downloads: downloadDir,
	}

	eng.OnUpdate = func(taskID string, p engine.Progress) {
		status := "downloading"
		if p.Downloaded >= p.Total && p.Total > 0 {
			status = "completed"
		}
		_ = db.UpsertTask(store.TaskRecord{
			ID:         taskID,
			Downloaded: p.Downloaded,
			Status:     status,
		})
	}

	srv.ext = extserver.New("127.0.0.1:19890")
	srv.ext.OnDownload = func(payload extserver.DownloadPayload) {
		_, _ = srv.AddDownload(payload.URL, payload.Filename, 8)
	}

	srv.clip = clipboard.New(func(u string) {
		srv.ext.OnDownload(extserver.DownloadPayload{URL: u})
	})
	_ = srv.clip.Start(context.Background())

	eng.SetLimiter(srv.limiter)

	return srv, nil
}

func (s *Service) StartServer() error {
	return s.ext.Start()
}

func (s *Service) AddDownload(rawURL, filename string, connections int) (string, error) {
	total, acceptRanges, detectedName, err := engine.Probe(rawURL)
	if err != nil {
		return "", fmt.Errorf("probe failed: %w", err)
	}

	if filename == "" {
		filename = detectedName
	}
	cat := store.Categorize(filename)
	destDir := filepath.Join(s.downloads, cat)
	_ = os.MkdirAll(destDir, 0o755)
	destPath := filepath.Join(destDir, filename)

	b := make([]byte, 8)
	rand.Read(b)
	id := hex.EncodeToString(b)

	if connections <= 0 {
		connections = 8
	}

	chunks := engine.Split(total, connections, acceptRanges)

	taskRec := store.TaskRecord{
		ID:           id,
		URL:          rawURL,
		Filename:     filename,
		FilePath:     destPath,
		TotalBytes:   total,
		Downloaded:   0,
		Status:       "queued",
		Connections:  len(chunks),
		Category:     cat,
		AcceptRanges: acceptRanges,
	}

	if err := s.db.UpsertTask(taskRec); err != nil {
		return "", err
	}

	var dbChunks []store.ChunkRecord
	for _, c := range chunks {
		dbChunks = append(dbChunks, store.ChunkRecord{
			TaskID:      id,
			Idx:         c.Idx,
			StartOffset: c.StartOffset,
			EndOffset:   c.EndOffset,
			Downloaded:  0,
			Status:      "pending",
		})
	}
	_ = s.db.SaveChunks(id, dbChunks)

	// Start engine download
	t := engine.Task{
		ID:          id,
		URL:         rawURL,
		FilePath:    destPath,
		TotalBytes:  total,
		Connections: len(chunks),
		AcceptRange: acceptRanges,
	}

	if err := s.eng.Start(context.Background(), t, chunks); err != nil {
		return "", err
	}

	return id, nil
}

func (s *Service) Pause(id string) bool {
	ok := s.eng.Pause(id)
	if ok {
		_ = s.db.UpsertTask(store.TaskRecord{ID: id, Status: "paused"})
	}
	return ok
}

func (s *Service) List(category string) ([]store.TaskRecord, error) {
	return s.db.ListTasks(category)
}

func (s *Service) Progress(id string) (engine.Progress, bool) {
	return s.eng.Progress(id)
}
