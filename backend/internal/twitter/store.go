package twitter

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// handleRE is duplicated in handler/twitter_account.go (a known minor
// DRY violation). Future cleanup: move to a shared twitter/validate.go
// or expose ValidHandle from the twitter package.
var handleRE = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)

func validHandle(h string) bool {
	return handleRE.MatchString(h)
}

// Account is one row of tracked_twitter_accounts.
type Account struct {
	ID          int64
	Handle      string
	DisplayName sql.NullString
	Enabled     bool
	Notes       sql.NullString
}

// Store wraps the DB operations needed by the twitter package.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InsertTweet inserts a tweet. Returns nil on duplicate (idempotent via
// tweet_id UNIQUE). digest_runID may be empty.
func (s *Store) InsertTweet(ctx context.Context, tw Tweet, digestRunID string) error {
	metricsJSON, err := json.Marshal(tw.Metrics)
	if err != nil {
		metricsJSON = []byte("{}")
	}
	var runID any
	if digestRunID != "" {
		runID = digestRunID
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO tweets
		  (tweet_id, handle, author_name, text, created_at, url, metrics_json, raw_json, digest_run_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		tw.TweetID, tw.Handle, tw.AuthorName, tw.Text,
		tw.CreatedAt.UTC().Format(time.RFC3339),
		tw.URL, string(metricsJSON), string(tw.Raw), runID,
	)
	return err
}

// CountTweetsByRun returns the number of tweets associated with a run.
func (s *Store) CountTweetsByRun(ctx context.Context, runID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tweets WHERE digest_run_id = ?`, runID,
	).Scan(&n)
	return n, err
}

// ListEnabledAccounts returns all enabled accounts.
func (s *Store) ListEnabledAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, handle, display_name, enabled, notes
		FROM tracked_twitter_accounts
		WHERE enabled = 1
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Handle, &a.DisplayName, &a.Enabled, &a.Notes); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAccountDisplayName sets display_name for the given handle.
// Used to backfill the author name from the first successful fetch.
func (s *Store) UpdateAccountDisplayName(ctx context.Context, handle, name string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tracked_twitter_accounts
		 SET display_name = ?
		 WHERE handle = ? AND (display_name IS NULL OR display_name = '')`,
		name, handle,
	)
	return err
}

// BulkInsertAccounts inserts each handle idempotently (UNIQUE on handle).
// Returns the number of rows actually inserted (not counting existing).
// Empty/duplicate handles are silently skipped. Invalid handles (not
// matching ^[A-Za-z0-9_]{1,15}$) are skipped (no error, just not inserted).
// A leading "@" is trimmed before validation.
func (s *Store) BulkInsertAccounts(ctx context.Context, handles []string) (int, error) {
	added := 0
	for _, raw := range handles {
		h := strings.TrimSpace(raw)
		h = strings.TrimPrefix(h, "@")
		if !validHandle(h) {
			continue
		}
		res, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO tracked_twitter_accounts (handle, enabled) VALUES (?, 1)`,
			h,
		)
		if err != nil {
			return added, err
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			added++
		}
	}
	return added, nil
}

// DB returns the underlying *sql.DB. Used by the digest runner to
// write to twitter_digest_runs without coupling the runner to the
// store's individual helpers.
func (s *Store) DB() *sql.DB { return s.db }
