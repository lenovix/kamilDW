package engine

import "sync/atomic"

func (c *Chunk) downloadedBytes() int64 { return atomic.LoadInt64(&c.Downloaded) }

func (c *Chunk) addDownloaded(n int64) { atomic.AddInt64(&c.Downloaded, n) }

func (c *Chunk) endOffset() int64 { return atomic.LoadInt64(&c.EndOffset) }

func (c *Chunk) setEndOffset(v int64) { atomic.StoreInt64(&c.EndOffset, v) }

func (e *Engine) markDone(c *Chunk) {
	e.mu.Lock()
	c.Done = true
	e.mu.Unlock()
}
