package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ClaudeProvider struct {
	apiKey string
	model  string
}

func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	if model == "" {
		model = "claude-sonnet-4-7-20250514"
	}
	return &ClaudeProvider{apiKey: apiKey, model: model}
}

func (p *ClaudeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]claudeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, messageToClaudeMessage(m))
	}

	claudeReq := claudeRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
		Stream:    false,
	}

	if len(req.Tools) > 0 {
		claudeReq.Tools = make([]claudeTool, len(req.Tools))
		for i, t := range req.Tools {
			claudeReq.Tools[i] = claudeTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
		}
	}

	jsonData, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Claude API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Claude API error: %s", string(body))
	}

	var claudeResp struct {
		Content []claudeContentBlock `json:"content"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var textContent strings.Builder
	var toolCalls []ToolCall

	for _, c := range claudeResp.Content {
		switch c.Type {
		case "text":
			textContent.WriteString(c.Text)
		case "tool_use":
			inputStr := ""
			if c.Input != nil {
				inputStr = string(c.Input)
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:    c.ID,
				Name:  c.Name,
				Input: inputStr,
			})
		}
	}

	return &ChatResponse{
		Content:    textContent.String(),
		ToolCalls:  toolCalls,
		TokenCount: claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
	}, nil
}

func (p *ClaudeProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]claudeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, messageToClaudeMessage(m))
	}

	claudeReq := claudeRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
		Stream:    true,
	}

	if len(req.Tools) > 0 {
		claudeReq.Tools = make([]claudeTool, len(req.Tools))
		for i, t := range req.Tools {
			claudeReq.Tools[i] = claudeTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
		}
	}

	jsonData, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Claude API: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Claude API error: %s", string(body))
	}

	ch := make(chan ChatChunk, 100)
	go func() {
		defer close(ch)
		ParseClaudeSSE(resp.Body, func(chunk ChatChunk) {
			ch <- chunk
		})
	}()

	return ch, nil
}

// messageToClaudeMessage converts a Message to Claude's message format.
// It handles both old-style (plain text) and new-style (structured content blocks) messages.
func messageToClaudeMessage(m Message) claudeMessage {
	// Tool result: use role "user" with tool_result content block
	if m.Role == "tool" {
		blocks := []claudeContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
				IsError:   false,
			},
		}
		return claudeMessage{Role: "user", Content: blocks}
	}

	// Structured content (JSON array of content blocks)
	if IsStructuredContent(m.Content) {
		parsed := ParseContentBlocks(m.Content)
		blocks := make([]claudeContentBlock, 0, len(parsed))
		for _, b := range parsed {
			switch b.Type {
			case ContentTypeText:
				blocks = append(blocks, claudeContentBlock{Type: "text", Text: b.Text})
			case ContentTypeToolUse:
				blocks = append(blocks, claudeContentBlock{
					Type:  "tool_use",
					ID:    b.ID,
					Name:  b.Name,
					Input: b.Input,
				})
			}
		}
		return claudeMessage{Role: m.Role, Content: blocks}
	}

	// Old format: plain text
	return claudeMessage{Role: m.Role, Content: m.Content}
}

func (p *ClaudeProvider) GetModel() string {
	return p.model
}
