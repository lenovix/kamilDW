package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
)

const bufferSize = 64 * 1024 // 64 KB buffer (matching XDM FILL_BYTES)

// downloadChunk executes a Range request and streams data directly to disk via file.WriteAt.
func downloadChunk(
	ctx context.Context,
	client *http.Client,
	url string,
	headers map[string]string,
	file *os.File,
	chunk *Chunk,
	totalDownloaded *atomic.Int64,
	speedWindow *atomic.Int64,
) error {
	start := chunk.StartOffset + atomic.LoadInt64(&chunk.Downloaded)
	if chunk.EndOffset >= 0 && start > chunk.EndOffset {
		chunk.SetStatus(StatusFinished)
		return nil
	}

	chunk.SetStatus(StatusDownloading)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		chunk.SetStatus(StatusFailed)
		return fmt.Errorf("create worker request failed: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "kamilDW/1.0 (XDM-inspired Go Engine)")
	}

	if chunk.EndOffset >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, chunk.EndOffset))
	}

	resp, err := client.Do(req)
	if err != nil {
		chunk.SetStatus(StatusFailed)
		return fmt.Errorf("worker request failed: %w", err)
	}
	defer resp.Body.Close()

	if chunk.EndOffset >= 0 && resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		chunk.SetStatus(StatusFailed)
		return fmt.Errorf("worker HTTP %d %s", resp.StatusCode, resp.Status)
	}

	if chunk.EndOffset < 0 && resp.StatusCode != http.StatusOK {
		chunk.SetStatus(StatusFailed)
		return fmt.Errorf("worker HTTP %d %s", resp.StatusCode, resp.Status)
	}

	buf := make([]byte, bufferSize)
	var written int64

	for {
		select {
		case <-ctx.Done():
			chunk.SetStatus(StatusCancelled)
			return ctx.Err()
		default:
		}

		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			writeOffset := start + written
			if _, werr := file.WriteAt(buf[:n], writeOffset); werr != nil {
				chunk.SetStatus(StatusFailed)
				return fmt.Errorf("worker write at offset %d failed: %w", writeOffset, werr)
			}

			n64 := int64(n)
			written += n64
			atomic.AddInt64(&chunk.Downloaded, n64)
			totalDownloaded.Add(n64)
			speedWindow.Add(n64)

			curEnd := chunk.EndOffset
			if curEnd >= 0 && start+written > curEnd {
				chunk.SetStatus(StatusFinished)
				return nil
			}
		}

		if rerr != nil {
			if rerr == io.EOF {
				chunk.SetStatus(StatusFinished)
				return nil
			}
			chunk.SetStatus(StatusFailed)
			return fmt.Errorf("worker read stream failed: %w", rerr)
		}
	}
}
