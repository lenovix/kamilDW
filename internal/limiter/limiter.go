package limiter

import (
	"context"
	"sync"
	"time"
)

// Bucket implements token bucket rate limiter (bytes/second).
type Bucket struct {
	rate       int64 // bytes per sec; <=0 means unlimited
	capacity   int64
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

func New(bytesPerSec int64) *Bucket {
	cap := bytesPerSec
	if cap < 64*1024 {
		cap = 64 * 1024
	}
	return &Bucket{
		rate:       bytesPerSec,
		capacity:   cap,
		tokens:     float64(cap),
		lastUpdate: time.Now(),
	}
}

func (b *Bucket) SetRate(bytesPerSec int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rate = bytesPerSec
	if bytesPerSec > 0 && bytesPerSec > b.capacity {
		b.capacity = bytesPerSec
	}
}

// Wait blocks until n bytes are permitted or ctx is canceled.
func (b *Bucket) Wait(ctx context.Context, n int) error {
	for {
		b.mu.Lock()
		if b.rate <= 0 {
			b.mu.Unlock()
			return nil
		}

		now := time.Now()
		elapsed := now.Sub(b.lastUpdate).Seconds()
		b.lastUpdate = now
		b.tokens += elapsed * float64(b.rate)
		if b.tokens > float64(b.capacity) {
			b.tokens = float64(b.capacity)
		}

		needed := float64(n)
		if b.tokens >= needed {
			b.tokens -= needed
			b.mu.Unlock()
			return nil
		}

		missing := needed - b.tokens
		sleepSec := missing / float64(b.rate)
		b.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(sleepSec * float64(time.Second))):
		}
	}
}
