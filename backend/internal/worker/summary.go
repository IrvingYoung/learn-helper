package worker

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"learn-helper/internal/model"
)

// SummaryProvider is the minimal interface SummaryWorker needs from an AI provider.
type SummaryProvider interface {
	GenerateSummary(ctx context.Context, title, content string) (string, error)
}

// SummaryDB is the minimal interface for the DB operations the worker needs.
type SummaryDB interface {
	GetPageForSummary(ctx context.Context, pageID int64) (title, content, status, hash string, err error)
	UpdateSummary(ctx context.Context, pageID int64, summary, hash string) error
	MarkSummaryEmpty(ctx context.Context, pageID int64) error
	MarkSummaryFailed(ctx context.Context, pageID int64) error
	ListPendingSummaries(ctx context.Context, limit int) ([]int64, error)
}

// SummaryWorker asynchronously generates AI summaries for wiki pages.
// One goroutine, serial processing, channel-buffered queue.
type SummaryWorker struct {
	provider SummaryProvider
	db       SummaryDB
	queue    chan int64
}

const summaryQueueSize = 100

// NewSummaryWorker creates a worker with the given dependencies.
func NewSummaryWorker(provider SummaryProvider, db SummaryDB) *SummaryWorker {
	return &SummaryWorker{
		provider: provider,
		db:       db,
		queue:    make(chan int64, summaryQueueSize),
	}
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
			w.generateOne(ctx, pageID)
		}
	}
}

// generateOne is the core: fetch page, ask AI for summary, write back.
func (w *SummaryWorker) generateOne(ctx context.Context, pageID int64) {
	title, content, status, _, err := w.db.GetPageForSummary(ctx, pageID)
	if err != nil {
		// Page was deleted between enqueue and processing. Skip.
		return
	}

	// Skip empty content - nothing to summarize.
	if strings.TrimSpace(content) == "" {
		_ = w.db.MarkSummaryEmpty(ctx, pageID)
		return
	}

	// Skip if already 'empty' status (avoid regenerating).
	if status == "empty" {
		return
	}

	// Rate limit: 200ms between AI calls to avoid provider limits.
	time.Sleep(200 * time.Millisecond)

	summary, err := w.provider.GenerateSummary(ctx, title, content)
	if err != nil {
		_ = w.db.MarkSummaryFailed(ctx, pageID)
		return
	}

	hash := contentHash(title, content)
	if err := w.db.UpdateSummary(ctx, pageID, summary, hash); err != nil {
		_ = w.db.MarkSummaryFailed(ctx, pageID)
	}
}

// contentHash returns MD5(title + content) for staleness detection.
func contentHash(title, content string) string {
	h := md5.New()
	h.Write([]byte(title))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// BackfillOnce processes all pages with status='pending' or 'failed'.
// Called at server startup to catch up after crashes.
func (w *SummaryWorker) BackfillOnce(ctx context.Context, batchSize int) error {
	ids, err := w.db.ListPendingSummaries(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("list pending summaries: %w", err)
	}
	for _, id := range ids {
		w.generateOne(ctx, id)
	}
	return nil
}

// Compile-time check that sqlDBAdapter satisfies SummaryDB.
var _ SummaryDB = (*sqlDBAdapter)(nil)

// sqlDBAdapter adapts *model.Queries to the SummaryDB interface.
type sqlDBAdapter struct {
	q *model.Queries
}

// NewSQLDBAdapter wraps a *model.Queries so it can be used as a SummaryDB.
func NewSQLDBAdapter(q *model.Queries) *sqlDBAdapter {
	return &sqlDBAdapter{q: q}
}

func (a *sqlDBAdapter) GetPageForSummary(ctx context.Context, pageID int64) (string, string, string, string, error) {
	page, err := a.q.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return "", "", "", "", err
	}
	return page.Title, page.Content, page.SummaryStatus, page.SummaryContentHash.String, nil
}

func (a *sqlDBAdapter) UpdateSummary(ctx context.Context, pageID int64, summary, hash string) error {
	return a.q.UpdatePageSummary(ctx, model.UpdatePageSummaryParams{
		Summary:            summary,
		SummaryContentHash: sql.NullString{String: hash, Valid: true},
		ID:                 pageID,
	})
}

func (a *sqlDBAdapter) MarkSummaryEmpty(ctx context.Context, pageID int64) error {
	return a.q.MarkSummaryEmpty(ctx, pageID)
}

func (a *sqlDBAdapter) MarkSummaryFailed(ctx context.Context, pageID int64) error {
	return a.q.MarkSummaryFailed(ctx, pageID)
}

func (a *sqlDBAdapter) ListPendingSummaries(ctx context.Context, limit int) ([]int64, error) {
	rows, err := a.q.ListPendingSummaries(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, nil
}
