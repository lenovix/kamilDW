package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Chunk struct {
	Idx         int   `json:"idx"`
	StartOffset int64 `json:"startOffset"`
	EndOffset   int64 `json:"endOffset"` // inclusive
	Downloaded  int64 `json:"downloaded"`
	Done        bool  `json:"done"`
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
func Probe(url string) (total int64, acceptRanges bool, filename string, err error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, false, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, false, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return 0, false, "", fmt.Errorf("HEAD %s: %s", url, resp.Status)
	}
	total = resp.ContentLength
	acceptRanges = resp.Header.Get("Accept-Ranges") == "bytes"
	filename = filepath.Base(resp.Request.URL.Path)
	if filename == "/" || filename == "." {
		filename = "download.bin"
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
		if ts.task.TotalBytes > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, c.EndOffset))
		}

		resp, err := e.client.Do(req)
		if err != nil {
			time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
			continue
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
			e.OnUpdate(ts.task.ID, p)
		}
	}
}

func (e *Engine) progressOf(ts *taskState) Progress {
	e.mu.Lock()
	chunks := make([]Chunk, len(ts.chunks))
	copy(chunks, ts.chunks)
	e.mu.Unlock()
	return Progress{
		Downloaded: ts.downloaded.Load(),
		Total:      ts.task.TotalBytes,
		Speed:      float64(ts.speed.Load()),
		Chunks:     chunks,
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
