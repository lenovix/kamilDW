package engine

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEngineMultipartWriteAt(t *testing.T) {
	payload := []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_!@#$%^&*")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		rg := r.Header.Get("Range")
		if rg == "" {
			w.Write(payload)
			return
		}

		// Parse simple bytes=start-end
		rg = strings.TrimPrefix(rg, "bytes=")
		parts := strings.Split(rg, "-")
		start, _ := strconv.Atoi(parts[0])
		end, _ := strconv.Atoi(parts[1])
		if end >= len(payload) {
			end = len(payload) - 1
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(payload[start : end+1])
	}))
	defer srv.Close()

	total, acceptRanges, _, err := Probe(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if total != int64(len(payload)) || !acceptRanges {
		t.Fatalf("probe failed: total=%d, accept=%v", total, acceptRanges)
	}

	chunks := Split(total, 4, acceptRanges)
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}

	tmpFile := filepath.Join(t.TempDir(), "out.bin")
	eng := New()
	task := Task{
		ID:          "task-1",
		URL:         srv.URL,
		FilePath:    tmpFile,
		TotalBytes:  total,
		Connections: 4,
		AcceptRange: true,
	}

	if err := eng.Start(context.Background(), task, chunks); err != nil {
		t.Fatal(err)
	}

	// Wait task finish
	for {
		p, ok := eng.Progress(task.ID)
		if !ok || p.Downloaded == total {
			break
		}
	}

	got, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("download mismatch: got %q, want %q", got, payload)
	}
}
