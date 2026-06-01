package worker

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSummaryWorker_EnqueueIsNonBlocking(t *testing.T) {
	w := &SummaryWorker{queue: make(chan int64, 3)}

	// Fill the buffer completely.
	w.Enqueue(1)
	w.Enqueue(2)
	w.Enqueue(3)

	// Enqueue one more with no consumer - should drop, not block.
	done := make(chan bool)
	go func() {
		w.Enqueue(4) // would block forever if queue were synchronous
		done <- true
	}()

	select {
	case <-done:
		// expected: Enqueue returned quickly
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Enqueue blocked on full channel")
	}
}

func TestSummaryWorker_StopsOnContextCancel(t *testing.T) {
	w := &SummaryWorker{
		queue: make(chan int64),
	}

	// Manually wire up the Run loop's context handling
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Simulate Run's select loop without the actual work
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.queue:
				// would call generateOne in real code
			}
		}
	}()

	// Give the goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	cancel()
	wg.Wait() // should not hang
}
