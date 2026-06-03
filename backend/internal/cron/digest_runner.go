package cron

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"learn-helper/internal/handler"
	"learn-helper/internal/twitter"
)

// DigestConfig and DigestAI live in the handler package (see
// handler/digest_ai.go) — the AI step is a method on *handler.AIHandler,
// so defining the types in handler avoids an import cycle
// (cron/runner.go already imports handler). The aliases below let
// callers in this package keep using the unqualified names.
type (
	DigestConfig = handler.DigestConfig
	DigestAI     = handler.DigestAI
)

// DigestRunner orchestrates a single twitter-digest run: fetch tweets
// for each enabled account, persist them, then call the AI to generate
// the wiki page.
type DigestRunner struct {
	Store  *twitter.Store
	Client twitter.Client
	// AI is set by the main package after construction; the AI step
	// is a separate function (see runDigestAI) so tests can skip it.
	AI DigestAI
}

// retryBackoffs is the per-attempt wait between retries. Indexed by
// attempt-1, so retryBackoffs[0] is the wait after the 1st failure
// (before attempt 2), retryBackoffs[1] after the 2nd (before attempt
// 3). Package-level so tests can shrink it to milliseconds.
var retryBackoffs = []time.Duration{5 * time.Second, 10 * time.Second}

// fetchWithRetry wraps Client.FetchUserTweets with retry logic. Retries
// up to 3 times total, waiting between attempts per retryBackoffs.
// Respects context cancellation. Returns the first successful result,
// or the last error if all attempts fail.
func (d *DigestRunner) fetchWithRetry(ctx context.Context, handle string, since time.Time, limit int) ([]twitter.Tweet, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		tweets, err := d.Client.FetchUserTweets(ctx, handle, since, limit)
		if err == nil {
			if attempt > 1 {
				log.Printf("[digest] fetch %s succeeded on attempt %d", handle, attempt)
			}
			return tweets, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			backoff := retryBackoffs[attempt-1]
			log.Printf("[digest] fetch %s attempt %d/3 failed: %v (retrying in %v)", handle, attempt, err, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		} else {
			log.Printf("[digest] fetch %s failed after 3 attempts: %v", handle, err)
		}
	}
	return nil, lastErr
}

// fetchAndPersist fetches tweets for every enabled account, persists
// them to the tweets table (idempotent on tweet_id), and returns the
// number of rows newly inserted. Per-account failures are logged and
// skipped.
func (d *DigestRunner) fetchAndPersist(ctx context.Context, runID string, cfg DigestConfig) (int, error) {
	accounts, err := d.Store.ListEnabledAccounts(ctx)
	if err != nil {
		return 0, err
	}
	since := time.Now().Add(-time.Duration(cfg.SinceHours) * time.Hour)
	total := 0
	for _, a := range accounts {
		tweets, err := d.fetchWithRetry(ctx, a.Handle, since, cfg.MaxTweetsPerAccount)
		if err != nil {
			log.Printf("[digest] fetch %s: %v (skipping)", a.Handle, err)
			continue
		}
		for _, tw := range tweets {
			if err := d.Store.InsertTweet(ctx, tw, runID); err != nil {
				log.Printf("[digest] insert tweet %s: %v", tw.TweetID, err)
				continue
			}
			total++
			if total >= cfg.MaxTotalTweets {
				log.Printf("[digest] reached MaxTotalTweets=%d, stopping", cfg.MaxTotalTweets)
				return total, nil
			}
		}
		// Backfill display_name from first successful fetch
		if !a.DisplayName.Valid && len(tweets) > 0 && tweets[0].AuthorName != "" {
			_ = d.Store.UpdateAccountDisplayName(ctx, a.Handle, tweets[0].AuthorName)
		}
	}
	return total, nil
}

// Run executes the full digest: fetch → persist → AI. The cron
// scheduler calls this from a goroutine. Returns the run record so
// the caller can persist it.
func (d *DigestRunner) Run(ctx context.Context, cronRunID *int64, cfg DigestConfig) (runID string, fetched int, err error) {
	runID = newUUID()
	insertDigestRunSQL := `INSERT INTO twitter_digest_runs (id, cron_run_id, status) VALUES (?, ?, 'running')`
	if _, err := d.Store.DB().ExecContext(ctx, insertDigestRunSQL, runID, cronRunID); err != nil {
		return runID, 0, err
	}

	fetched, ferr := d.fetchAndPersist(ctx, runID, cfg)
	if ferr != nil {
		_ = d.markFailed(ctx, runID, ferr.Error())
		return runID, 0, ferr
	}
	if fetched == 0 {
		_ = d.markFailed(ctx, runID, "no_new_tweets")
		return runID, 0, nil
	}
	if d.AI == nil {
		// No AI wired up (test path) — stop here, mark fetched.
		_ = d.markFetched(ctx, runID, fetched)
		return runID, fetched, nil
	}

	if err := d.AI.GenerateDigestPage(ctx, runID, cfg); err != nil {
		_ = d.markFailed(ctx, runID, "ai: "+err.Error())
		return runID, fetched, err
	}
	_ = d.markAnalyzed(ctx, runID, fetched)
	return runID, fetched, nil
}

func (d *DigestRunner) markFetched(ctx context.Context, runID string, n int) error {
	_, err := d.Store.DB().ExecContext(ctx,
		`UPDATE twitter_digest_runs SET status='fetched', tweets_fetched=? WHERE id=?`,
		n, runID)
	return err
}

func (d *DigestRunner) markAnalyzed(ctx context.Context, runID string, n int) error {
	_, err := d.Store.DB().ExecContext(ctx,
		`UPDATE twitter_digest_runs SET status='analyzed', tweets_fetched=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`,
		n, runID)
	return err
}

func (d *DigestRunner) markFailed(ctx context.Context, runID, reason string) error {
	_, err := d.Store.DB().ExecContext(ctx,
		`UPDATE twitter_digest_runs SET status='failed', error=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`,
		reason, runID)
	return err
}

func newUUID() string { return uuid.NewString() }
