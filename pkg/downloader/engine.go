package downloader

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProgressEvent is emitted periodically during download.
type ProgressEvent struct {
	DownloadedBytes  int64   `json:"downloadedBytes"`
	TotalBytes       int64   `json:"totalBytes"`
	SpeedBytesPerSec float64 `json:"speedBytesPerSec"`
	Percentage       float64 `json:"percentage"`
	ETA              int64   `json:"eta"`
}

// Engine is the high-performance multi-part segmented download engine.
type Engine struct {
	client *http.Client
}

// NewEngine initializes a new download Engine.
func NewEngine() *Engine {
	return &Engine{
		client: &http.Client{
			Timeout: 0,
		},
	}
}

// Preflight checks URL headers to validate range support and file size.
func (e *Engine) Preflight(url string, headers map[string]string) (total int64, acceptRanges bool, filename string, err error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, false, "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "kamilDW/1.0 (XDM-inspired Go Engine)")
	}
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := e.client.Do(req)
	// Fallback to GET if HEAD rejected or without Content-Length
	if err != nil || resp.StatusCode >= 400 || resp.ContentLength <= 0 {
		if resp != nil {
			resp.Body.Close()
		}

		req, _ = http.NewRequest(http.MethodGet, url, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "kamilDW/1.0 (XDM-inspired Go Engine)")
		}
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Range", "bytes=0-0")
		resp, err = e.client.Do(req)
	}

	if err != nil {
		return 0, false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return 0, false, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	total = resp.ContentLength
	cr := resp.Header.Get("Content-Range")
	if cr != "" {
		if idx := strings.Index(cr, "/"); idx != -1 {
			var parsedTotal int64
			if _, err := fmt.Sscanf(cr[idx+1:], "%d", &parsedTotal); err == nil && parsedTotal > 0 {
				total = parsedTotal
			}
		}
	}

	acceptRanges = resp.Header.Get("Accept-Ranges") == "bytes" || resp.StatusCode == http.StatusPartialContent || cr != ""
	filename = filepath.Base(resp.Request.URL.Path)
	if filename == "/" || filename == "." || len(filename) > 64 {
		filename = "download.bin"
	}

	return total, acceptRanges, filename, nil
}

// Download executes concurrent multi-part download using WriteAt.
func (e *Engine) Download(
	ctx context.Context,
	url string,
	outputPath string,
	connections int,
	headers map[string]string,
	onProgress func(ProgressEvent),
) error {
	total, acceptRanges, _, err := e.Preflight(url, headers)
	if err != nil {
		return fmt.Errorf("preflight failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create dir failed: %w", err)
	}

	// Zero-merge pre-allocation
	f, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	if total > 0 {
		if err := f.Truncate(total); err != nil {
			return fmt.Errorf("file pre-allocation truncate failed: %w", err)
		}
	}

	chunks := Split(total, connections, acceptRanges)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var totalDownloaded atomic.Int64
	var speedWindow atomic.Int64
	var wg sync.WaitGroup

	progressDone := make(chan struct{})
	if onProgress != nil {
		go func() {
			defer close(progressDone)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					dl := totalDownloaded.Load()
					spd := float64(speedWindow.Swap(0)) * 2
					pct := 0.0
					var eta int64
					if total > 0 {
						pct = float64(dl) / float64(total) * 100
						if spd > 0 {
							eta = int64(float64(total-dl) / spd)
						}
					}
					onProgress(ProgressEvent{
						DownloadedBytes:  dl,
						TotalBytes:       total,
						SpeedBytesPerSec: spd,
						Percentage:       pct,
						ETA:              eta,
					})
				}
			}
		}()
	}

	for i := range chunks {
		wg.Add(1)
		go func(c *Chunk) {
			defer wg.Done()
			downloadChunk(ctx, e.client, url, headers, f, c, &totalDownloaded, &speedWindow)
		}(chunks[i])
	}

	wg.Wait()
	cancel()
	if onProgress != nil {
		<-progressDone
	}

	for _, c := range chunks {
		if c.GetStatus() == StatusFailed {
			return fmt.Errorf("chunk %d failed", c.Idx)
		}
	}

	return nil
}
