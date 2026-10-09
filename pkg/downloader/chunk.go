package downloader

import (
	"sync/atomic"
)

type ChunkStatus int32

const (
	StatusReady ChunkStatus = iota
	StatusDownloading
	StatusFinished
	StatusFailed
	StatusCancelled
)

func (s ChunkStatus) String() string {
	switch s {
	case StatusReady:
		return "Ready"
	case StatusDownloading:
		return "Downloading"
	case StatusFinished:
		return "Finished"
	case StatusFailed:
		return "Failed"
	case StatusCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// Chunk represents a segment of the file downloaded in parallel.
type Chunk struct {
	Idx         int   `json:"idx"`
	StartOffset int64 `json:"startOffset"`
	EndOffset   int64 `json:"endOffset"` // inclusive; -1 if unknown/single-stream
	Downloaded  int64 `json:"downloaded"`
	status      int32 // atomic ChunkStatus
	Done        bool  `json:"done"`
}

// SetStatus updates chunk status atomically.
func (c *Chunk) SetStatus(s ChunkStatus) {
	atomic.StoreInt32(&c.status, int32(s))
	if s == StatusFinished {
		c.Done = true
	}
}

// GetStatus returns chunk status atomically.
func (c *Chunk) GetStatus() ChunkStatus {
	return ChunkStatus(atomic.LoadInt32(&c.status))
}

// Split divides total bytes into n chunks.
// If acceptRanges is false or total <= 0, it falls back to a single stream (EndOffset -1).
func Split(total int64, n int, acceptRanges bool) []*Chunk {
	if total <= 0 || !acceptRanges {
		c := &Chunk{Idx: 0, StartOffset: 0, EndOffset: -1}
		c.SetStatus(StatusReady)
		return []*Chunk{c}
	}
	if n < 1 {
		n = 1
	}
	if n > 32 {
		n = 32
	}
	if int64(n) > total {
		n = int(total)
	}

	chunks := make([]*Chunk, n)
	size := total / int64(n)
	var off int64
	for i := 0; i < n; i++ {
		end := off + size - 1
		if i == n-1 {
			end = total - 1
		}
		c := &Chunk{
			Idx:         i,
			StartOffset: off,
			EndOffset:   end,
			Downloaded:  0,
			Done:        false,
		}
		c.SetStatus(StatusReady)
		chunks[i] = c
		off = end + 1
	}
	return chunks
}
