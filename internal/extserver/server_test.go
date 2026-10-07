package extserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServerDownloadFlow(t *testing.T) {
	var received DownloadPayload
	done := make(chan struct{})

	s := New("127.0.0.1:0")
	s.OnDownload = func(p DownloadPayload) (string, error) {
		received = p
		close(done)
		return "task-123", nil
	}
	s.addr = "127.0.0.1:0"

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/download", s.handleDownload)
	s.server = &http.Server{Handler: corsMiddleware(mux)}
	ts := httptest.NewServer(s.server.Handler)
	defer ts.Close()

	payload := DownloadPayload{URL: "https://example.com/video.mp4", Filename: "video.mp4"}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(ts.URL+"/api/download", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("callback not invoked")
	}

	if received.URL != payload.URL {
		t.Fatalf("wrong url received: %s", received.URL)
	}
}
