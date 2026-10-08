package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Chunk struct {
	Idx         int    `json:"idx"`
	StartOffset int64  `json:"startOffset"`
	EndOffset   int64  `json:"endOffset"` // inclusive
	Downloaded  int64  `json:"downloaded"`
	Done        bool   `json:"done"`
	lastErr     string // not exported
}

type Task struct {
	ID          string
	URL         string
	FilePath    string
	TotalBytes  int64
	Connections int
	AcceptRange bool
	Headers     map[string]string
}

type Progress struct {
	Downloaded int64
	Total      int64
	Speed      float64 // bytes/sec
	Chunks     []Chunk
	Error      string // non-empty when all workers for the task failed
}

type RateLimiter interface {
	Wait(ctx context.Context, n int) error
}

type Engine struct {
	mu       sync.Mutex
	tasks    map[string]*taskState
	client   *http.Client
	limiter  RateLimiter
	OnUpdate func(TaskID string, p Progress)
}

type taskState struct {
	task       Task
	file       *os.File
	chunks     []Chunk
	cancel     context.CancelFunc
	downloaded atomic.Int64
	speed      atomic.Uint64
	paused     atomic.Bool
	wg         sync.WaitGroup
	lastErr    string
	errMu      sync.Mutex
}

func (e *Engine) SetLimiter(l RateLimiter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.limiter = l
}

func New() *Engine {
	return &Engine{
		tasks: map[string]*taskState{},
		client: &http.Client{
			Timeout: 0, // no global timeout; per-request handled by ctx
		},
	}
}

// Probe validates URL via HTTP HEAD: returns total size + Accept-Ranges.
func Probe(url string, headers map[string]string) (total int64, acceptRanges bool, filename string, err error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, false, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := http.DefaultClient.Do(req)

	// Fallback ke GET jika HEAD ditolak atau gagal
	if err != nil || resp.StatusCode >= 400 || resp.ContentLength <= 0 {
		if resp != nil {
			resp.Body.Close()
		}
		req, _ = http.NewRequest(http.MethodGet, url, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		}
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Range", "bytes=0-0")
		resp, err = http.DefaultClient.Do(req)
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
		// e.g. "bytes 0-0/12345678"
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
		filename = "video.mp4"
	}
	return total, acceptRanges, filename, nil
}

// Split chunks a byte range into n parts. If acceptRanges is false, n=1.
func Split(total int64, n int, acceptRanges bool) []Chunk {
	if total <= 0 || !acceptRanges {
		return []Chunk{{Idx: 0, StartOffset: 0, EndOffset: total - 1}}
	}
	if n < 4 {
		n = 4
	}
	if n > 32 {
		n = 32
	}
	if int64(n) > total {
		n = int(total)
	}
	if n < 1 {
		n = 1
	}
	chunks := make([]Chunk, n)
	size := total / int64(n)
	var off int64
	for i := 0; i < n; i++ {
		end := off + size - 1
		if i == n-1 {
			end = total - 1
		}
		chunks[i] = Chunk{Idx: i, StartOffset: off, EndOffset: end}
		off = end + 1
	}
	return chunks
}

// Start begins (or resumes) a multi-part download. Prior progress in chunks
// is honored: workers begin at StartOffset+Downloaded.
func (e *Engine) Start(ctx context.Context, task Task, chunks []Chunk) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.tasks[task.ID]; ok {
		return fmt.Errorf("task %s already running", task.ID)
	}

	f, err := openSparse(task.FilePath, task.TotalBytes)
	if err != nil {
		return err
	}

	cctx, cancel := context.WithCancel(ctx)
	ts := &taskState{task: task, file: f, chunks: chunks, cancel: cancel}
	for _, c := range chunks {
		ts.downloaded.Add(c.Downloaded)
	}
	e.tasks[task.ID] = ts

	for i := range ts.chunks {
		if ts.chunks[i].Done {
			continue
		}
		ts.wg.Add(1)
		go e.runChunk(cctx, ts, i)
	}
	go e.reportLoop(cctx, ts)
	go func() {
		ts.wg.Wait()
		f.Close()
		e.mu.Lock()
		delete(e.tasks, task.ID)
		e.mu.Unlock()
	}()
	return nil
}

func (e *Engine) runChunk(ctx context.Context, ts *taskState, i int) {
	defer ts.wg.Done()
	c := &ts.chunks[i]
	lastErr := ""

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if ts.paused.Load() || ctx.Err() != nil {
			return
		}

		start := c.StartOffset + c.Downloaded
		if start > c.EndOffset && c.EndOffset >= 0 {
			c.Done = true
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.task.URL, nil)
		if err != nil {
			return
		}
		for k, v := range ts.task.Headers {
			req.Header.Set(k, v)
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		}
		if ts.task.TotalBytes > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, c.EndOffset))
		}

		resp, err := e.client.Do(req)
		if err != nil {
			lastErr = fmt.Sprintf("req err: %v", err)
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
				continue
			}
			ts.errMu.Lock()
			c.lastErr = lastErr
			ts.errMu.Unlock()
			return
		}

		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
				continue
			}
			ts.errMu.Lock()
			c.lastErr = lastErr
			ts.errMu.Unlock()
			return
		}

		buf := make([]byte, 128*1024)
		var written int64
		var readErr error
		for {
			if ts.paused.Load() {
				resp.Body.Close()
				return
			}
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if e.limiter != nil {
					if werr := e.limiter.Wait(ctx, n); werr != nil {
						resp.Body.Close()
						return
					}
				}
				if _, werr := ts.file.WriteAt(buf[:n], start+written); werr != nil {
					resp.Body.Close()
					lastErr = fmt.Sprintf("write err: %v", werr)
					ts.errMu.Lock()
					c.lastErr = lastErr
					ts.errMu.Unlock()
					return
				}
				written += int64(n)
				c.Downloaded += int64(n)
				ts.downloaded.Add(int64(n))
				ts.speed.Add(uint64(n))
				if start+written > c.EndOffset && c.EndOffset >= 0 {
					c.Done = true
					resp.Body.Close()
					return
				}
			}
			if rerr != nil {
				readErr = rerr
				break
			}
		}
		resp.Body.Close()

		if readErr == io.EOF || (c.EndOffset >= 0 && start+written > c.EndOffset) {
			c.Done = true
			e.stealWork(ctx, ts)
			return
		}

		time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
	}

	// Mark chunk done with error
	ts.errMu.Lock()
	c.lastErr = lastErr
	ts.errMu.Unlock()
	e.stealWork(ctx, ts)
}

// stealWork finds the slowest chunk and splits its remaining bytes to this worker.
func (e *Engine) stealWork(ctx context.Context, ts *taskState) {
	if ts.paused.Load() {
		return
	}
	e.mu.Lock()
	var slowest *Chunk
	var maxRemain int64
	for i := range ts.chunks {
		cc := &ts.chunks[i]
		if cc.Done {
			continue
		}
		remain := (cc.EndOffset - cc.StartOffset) - cc.Downloaded
		if remain > 1024*1024 && remain > maxRemain { // Only steal if > 1MB remaining
			maxRemain = remain
			slowest = cc
		}
	}

	if slowest == nil {
		e.mu.Unlock()
		return
	}

	// Calculate half
	stealSize := maxRemain / 2
	newEnd := slowest.EndOffset
	slowest.EndOffset = slowest.EndOffset - stealSize

	newChunk := Chunk{
		Idx:         len(ts.chunks),
		StartOffset: slowest.EndOffset + 1,
		EndOffset:   newEnd,
		Downloaded:  0,
		Done:        false,
	}
	ts.chunks = append(ts.chunks, newChunk)
	idx := len(ts.chunks) - 1
	ts.wg.Add(1)
	e.mu.Unlock()

	go e.runChunk(ctx, ts, idx)
}

// reportLoop pushes Progress snapshots until ctx done.
func (e *Engine) reportLoop(ctx context.Context, ts *taskState) {
	if e.OnUpdate == nil {
		return
	}
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if ts.paused.Load() {
				return
			}
			p := e.progressOf(ts)
			p.Speed = float64(ts.speed.Swap(0)) // reset bytes/sec
			e.OnUpdate(ts.task.ID, p)
		}
	}
}

func (e *Engine) progressOf(ts *taskState) Progress {
	e.mu.Lock()
	chunks := make([]Chunk, len(ts.chunks))
	copy(chunks, ts.chunks)
	e.mu.Unlock()

	ts.errMu.Lock()
	errStr := ""
	allFailed := true
	for _, c := range ts.chunks {
		if c.lastErr == "" {
			allFailed = false
		} else {
			errStr = c.lastErr
		}
	}
	ts.errMu.Unlock()

	if !allFailed {
		errStr = ""
	}

	return Progress{
		Downloaded: ts.downloaded.Load(),
		Total:      ts.task.TotalBytes,
		Speed:      0, // Speed diisi oleh reportLoop per detik
		Chunks:     chunks,
		Error:      errStr,
	}
}

func (e *Engine) Progress(taskID string) (Progress, bool) {
	e.mu.Lock()
	ts, ok := e.tasks[taskID]
	e.mu.Unlock()
	if !ok {
		return Progress{}, false
	}
	return e.progressOf(ts), true
}

// Pause stops workers; chunk state in memory preserves resume offsets.
func (e *Engine) Pause(taskID string) bool {
	e.mu.Lock()
	ts, ok := e.tasks[taskID]
	e.mu.Unlock()
	if !ok {
		return false
	}
	ts.paused.Store(true)
	ts.cancel()
	return true
}

// Cancel stops and removes the task (file kept on disk for inspection).
func (e *Engine) Cancel(taskID string) bool {
	e.mu.Lock()
	ts, ok := e.tasks[taskID]
	if ok {
		delete(e.tasks, taskID)
	}
	e.mu.Unlock()
	if !ok {
		return false
	}
	ts.cancel()
	return true
}

// openSparse pre-allocates total bytes with zero-merge semantics.
func openSparse(path string, total int64) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if total > 0 {
		if err := f.Truncate(total); err != nil {
			f.Close()
			return nil, err
		}
	}
	return f, nil
}
