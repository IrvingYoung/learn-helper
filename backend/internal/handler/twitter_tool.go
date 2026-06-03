package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"learn-helper/internal/ai"
)

func (h *AIHandler) executeListRecentTweets(ctx context.Context, tc ai.ToolCall) string {
	var args struct {
		Since  string `json:"since"`
		Handle string `json:"handle"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &args); err != nil {
		return "[系统] list_recent_tweets 执行失败：参数解析错误"
	}
	if args.Limit <= 0 {
		args.Limit = 50
	}
	if args.Limit > 200 {
		args.Limit = 200
	}

	q := `SELECT tweet_id, handle, author_name, text, created_at, url, metrics_json
	      FROM tweets WHERE 1=1`
	var params []any
	if args.Since != "" {
		q += " AND created_at >= ?"
		params = append(params, args.Since)
	}
	if args.Handle != "" {
		q += " AND handle = ?"
		params = append(params, args.Handle)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	params = append(params, args.Limit)

	rows, err := h.db.QueryContext(ctx, q, params...)
	if err != nil {
		return fmt.Sprintf("[系统] list_recent_tweets 查询失败：%v", err)
	}
	defer rows.Close()

	type item struct {
		TweetID    string         `json:"tweet_id"`
		Handle     string         `json:"handle"`
		AuthorName sql.NullString `json:"-"`
		Author     string         `json:"author,omitempty"`
		Text       string         `json:"text"`
		CreatedAt  string         `json:"created_at"`
		URL        string         `json:"url"`
		Metrics    string         `json:"metrics,omitempty"`
	}
	var out []item
	for rows.Next() {
		var it item
		var mj sql.NullString
		if err := rows.Scan(&it.TweetID, &it.Handle, &it.AuthorName, &it.Text, &it.CreatedAt, &it.URL, &mj); err != nil {
			return fmt.Sprintf("[系统] list_recent_tweets 扫描失败：%v", err)
		}
		if it.AuthorName.Valid {
			it.Author = it.AuthorName.String
		}
		if mj.Valid && mj.String != "" {
			it.Metrics = mj.String
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Sprintf("[系统] list_recent_tweets 读取失败：%v", err)
	}

	body, _ := json.Marshal(out)
	return fmt.Sprintf("[系统] list_recent_tweets 返回 %d 条推文：%s", len(out), string(body))
}
