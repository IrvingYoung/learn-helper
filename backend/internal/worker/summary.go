package worker

import (
	"context"
)

// SummaryWorker asynchronously generates AI summaries for wiki pages.
// One goroutine, serial processing, channel-buffered queue.
type SummaryWorker struct {
	queue chan int64
}

// Enqueue requests a summary generation for the given page.
// Non-blocking: if the channel is full, the request is dropped.
// Dropped requests stay in DB with summary_status='pending'
// and will be retried on next write or server restart.
func (w *SummaryWorker) Enqueue(pageID int64) {
	select {
	case w.queue <- pageID:
	default:
		// queue full, drop. status='pending' in DB will trigger retry.
	}
}

// Run starts the worker loop. Blocks until ctx is cancelled.
func (w *SummaryWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case pageID := <-w.queue:
			_ = pageID // generateOne will be implemented in Task 1.6
		}
	}
}
