package twitter

import (
	"context"
	"encoding/json"
	"time"
)

// Tweet is the normalized representation of one tweet, regardless of
// the underlying source (RSSHub, future X API client, etc.).
type Tweet struct {
	TweetID    string
	Handle     string
	AuthorName string
	Text       string
	CreatedAt  time.Time
	URL        string
	Metrics    map[string]int
	Raw        json.RawMessage
}

// Client is the abstract interface for any X data source. Implementations
// fetch tweets for a single handle, filtered by `since`, capped at `limit`.
type Client interface {
	FetchUserTweets(ctx context.Context, handle string, since time.Time, limit int) ([]Tweet, error)
}
