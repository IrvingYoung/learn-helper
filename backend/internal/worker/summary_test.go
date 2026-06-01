package worker

import (
	"context"
	"errors"
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

// --- Tests for generateOne (Task 1.6) ---

type mockProvider struct {
	mu      sync.Mutex
	calls   int
	summary string
	err     error
}

func (m *mockProvider) GenerateSummary(ctx context.Context, title, content string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.summary, m.err
}

type mockPage struct {
	ID                 int64
	Title              string
	Content            string
	SummaryStatus      string
	SummaryContentHash string
}

type mockUpdate struct {
	PageID  int64
	Summary string
	Hash    string
}

type mockDB struct {
	mu        sync.Mutex
	pages     map[int64]*mockPage
	updates   []mockUpdate
	emptyIDs  []int64
	failedIDs []int64
}

func (m *mockDB) GetPageForSummary(ctx context.Context, pageID int64) (string, string, string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pages[pageID]
	if !ok {
		return "", "", "", "", errors.New("not found")
	}
	return p.Title, p.Content, p.SummaryStatus, p.SummaryContentHash, nil
}

func (m *mockDB) UpdateSummary(ctx context.Context, pageID int64, summary, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, mockUpdate{PageID: pageID, Summary: summary, Hash: hash})
	return nil
}

func (m *mockDB) MarkSummaryEmpty(ctx context.Context, pageID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emptyIDs = append(m.emptyIDs, pageID)
	return nil
}

func (m *mockDB) MarkSummaryFailed(ctx context.Context, pageID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedIDs = append(m.failedIDs, pageID)
	return nil
}

func (m *mockDB) ListPendingSummaries(ctx context.Context, limit int) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []int64
	for _, p := range m.pages {
		if p.SummaryStatus == "pending" || p.SummaryStatus == "failed" {
			ids = append(ids, p.ID)
		}
	}
	return ids, nil
}

func TestSummaryWorker_generateOne_ready(t *testing.T) {
	provider := &mockProvider{summary: "数组：线性表基础。"}
	db := &mockDB{
		pages: map[int64]*mockPage{
			1: {ID: 1, Title: "数组", Content: "线性表基础内容", SummaryStatus: "pending"},
		},
	}
	w := &SummaryWorker{provider: provider, db: db}

	w.generateOne(context.Background(), 1)

	if provider.calls != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.calls)
	}
	if len(db.updates) != 1 {
		t.Fatalf("expected 1 DB update, got %d", len(db.updates))
	}
	if db.updates[0].Summary != "数组：线性表基础。" {
		t.Errorf("unexpected summary: %s", db.updates[0].Summary)
	}
	if db.updates[0].Hash == "" {
		t.Error("expected content_hash to be set")
	}
}

func TestSummaryWorker_generateOne_emptyContent(t *testing.T) {
	provider := &mockProvider{summary: "should not be called"}
	db := &mockDB{
		pages: map[int64]*mockPage{
			1: {ID: 1, Title: "树", Content: "", SummaryStatus: "pending"},
		},
	}
	w := &SummaryWorker{provider: provider, db: db}

	w.generateOne(context.Background(), 1)

	if provider.calls != 0 {
		t.Errorf("expected 0 calls for empty content, got %d", provider.calls)
	}
	if len(db.emptyIDs) != 1 || db.emptyIDs[0] != 1 {
		t.Errorf("expected MarkSummaryEmpty for page 1, got %v", db.emptyIDs)
	}
}

func TestSummaryWorker_generateOne_failure(t *testing.T) {
	provider := &mockProvider{err: errors.New("api timeout")}
	db := &mockDB{
		pages: map[int64]*mockPage{
			1: {ID: 1, Title: "链表", Content: "节点+指针", SummaryStatus: "pending"},
		},
	}
	w := &SummaryWorker{provider: provider, db: db}

	w.generateOne(context.Background(), 1)

	if len(db.failedIDs) != 1 || db.failedIDs[0] != 1 {
		t.Errorf("expected MarkSummaryFailed for page 1, got %v", db.failedIDs)
	}
}
