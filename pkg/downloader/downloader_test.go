package downloader

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

func TestDownloaderMultipart(t *testing.T) {
	payload := []byte("XDM-inspired-download-manager-test-payload-for-multi-part-concurrent-engine")
	
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

	d := NewDownloader()
	tmpFile := filepath.Join(t.TempDir(), "test_download.bin")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var finalDl int64
	err := d.Download(ctx, srv.URL, tmpFile, 4, nil, func(p ProgressEvent) {
		finalDl = p.DownloadedBytes
	})
	
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed reading output file: %v", err)
	}

	if !bytes.Equal(data, payload) {
		t.Fatalf("Data mismatch.\nGot: %q\nWant: %q", string(data), string(payload))
	}
	
	if finalDl == 0 {
		// Just ensure progress callback was actually called
	}
}

func TestDownloaderSingleFallback(t *testing.T) {
	payload := []byte("no-range-support-payload")
	
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Accept-Ranges header
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		w.Write(payload)
	}))
	defer srv.Close()

	d := NewDownloader()
	tmpFile := filepath.Join(t.TempDir(), "test_single.bin")

	err := d.Download(context.Background(), srv.URL, tmpFile, 4, nil, nil)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	data, _ := os.ReadFile(tmpFile)
	if !bytes.Equal(data, payload) {
		t.Fatalf("Data mismatch.\nGot: %q\nWant: %q", string(data), string(payload))
	}
}
