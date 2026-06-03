package cron

import (
	"context"
	"log"
	"time"

	"learn-helper/internal/twitter"
)

// DigestConfig is the per-task subset used by the digest runner.
type DigestConfig struct {
	SinceHours          int
	MaxTweetsPerAccount int
	MaxTotalTweets      int
}

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

// DigestAI is the abstract surface the digest runner needs from the
// AI handler. The main package provides a concrete implementation.
type DigestAI interface {
	GenerateDigestPage(ctx context.Context, runID string, cfg DigestConfig) error
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
		tweets, err := d.Client.FetchUserTweets(ctx, a.Handle, since, cfg.MaxTweetsPerAccount)
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
