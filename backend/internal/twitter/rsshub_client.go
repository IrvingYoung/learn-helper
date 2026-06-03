package twitter

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RSSHubClient fetches tweets by calling RSSHub's twitter/user/:handle
// route and parsing the returned RSS 2.0 feed.
type RSSHubClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewRSSHubClient returns a client with the given base URL and timeout.
// If timeout is 0, a 15s default is used.
func NewRSSHubClient(baseURL string, timeout time.Duration) *RSSHubClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &RSSHubClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

type rssFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Items   []rssItem `xml:"channel>item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

// FetchUserTweets calls GET {BaseURL}/twitter/user/{handle} and returns
// tweets newer than `since`, capped at `limit`. If `since` is zero, no
// time filter is applied.
func (c *RSSHubClient) FetchUserTweets(ctx context.Context, handle string, since time.Time, limit int) ([]Tweet, error) {
	url := fmt.Sprintf("%s/twitter/user/%s", c.BaseURL, handle)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "learn-helper/1.0")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("rss hub returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	out := make([]Tweet, 0, len(feed.Items))
	for _, it := range feed.Items {
		created, err := parseRSSDate(it.PubDate)
		if err != nil {
			continue // skip malformed dates
		}
		if !since.IsZero() && created.Before(since) {
			continue
		}
		if limit > 0 && len(out) >= limit {
			break
		}
		raw, _ := json.Marshal(it)
		out = append(out, Tweet{
			TweetID:   it.GUID,
			Handle:    handle,
			Text:      it.Description,
			CreatedAt: created,
			URL:       it.Link,
			Raw:       raw,
		})
	}
	return out, nil
}

func parseRSSDate(s string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized RSS date: %q", s)
}
