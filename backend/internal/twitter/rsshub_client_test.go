package twitter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const rssFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>@karpathy</title>
    <item>
      <title>karpathy: new post on LLM trends</title>
      <link>https://x.com/karpathy/status/1234</link>
      <guid isPermaLink="false">1234</guid>
      <pubDate>Mon, 03 Jun 2026 10:00:00 GMT</pubDate>
      <description>some long body text here</description>
    </item>
    <item>
      <title>karpathy: another take</title>
      <link>https://x.com/karpathy/status/1235</link>
      <guid isPermaLink="false">1235</guid>
      <pubDate>Sun, 02 Jun 2026 09:00:00 GMT</pubDate>
      <description>old post</description>
    </item>
  </channel>
</rss>`

func TestRSSHubClient_FetchUserTweets_FiltersBySince(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/twitter/user/karpathy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssFixture))
	}))
	defer srv.Close()

	c := NewRSSHubClient(srv.URL, 5*time.Second)
	since := parseDate(t, "2026-06-03T00:00:00Z")
	tweets, err := c.FetchUserTweets(context.Background(), "karpathy", since, 50)
	if err != nil {
		t.Fatalf("FetchUserTweets: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet (filtered by since), got %d", len(tweets))
	}
	got := tweets[0]
	if got.TweetID != "1234" {
		t.Errorf("TweetID: got %q want 1234", got.TweetID)
	}
	if got.Handle != "karpathy" {
		t.Errorf("Handle: got %q want karpathy", got.Handle)
	}
	if got.URL != "https://x.com/karpathy/status/1234" {
		t.Errorf("URL: got %q", got.URL)
	}
}

func TestRSSHubClient_FetchUserTweets_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rssFixture))
	}))
	defer srv.Close()

	c := NewRSSHubClient(srv.URL, 5*time.Second)
	tweets, err := c.FetchUserTweets(context.Background(), "karpathy", time.Time{}, 1)
	if err != nil {
		t.Fatalf("FetchUserTweets: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet (limit=1), got %d", len(tweets))
	}
}

func TestRSSHubClient_FetchUserTweets_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewRSSHubClient(srv.URL, 5*time.Second)
	_, err := c.FetchUserTweets(context.Background(), "karpathy", time.Time{}, 10)
	if err == nil {
		t.Fatal("expected error on HTTP 502, got nil")
	}
}

func parseDate(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tt
}
