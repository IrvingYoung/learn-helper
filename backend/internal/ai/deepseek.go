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

type DeepSeekProvider struct {
	apiKey string
	model  string
}

func NewDeepSeekProvider(apiKey, model string) *DeepSeekProvider {
	if model == "" {
		model = "deepseek-chat"
	}
	return &DeepSeekProvider{apiKey: apiKey, model: model}
}

func (p *DeepSeekProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]deepseekMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, messageToDeepSeekMessage(m))
	}

	dsReq := deepseekRequest{
		Model:       model,
		Messages:    messages,
		Stream:      false,
		MaxTokens:   req.MaxTokens,
		Temperature: 0.7,
	}

	if len(req.Tools) > 0 {
		dsReq.Tools = make([]deepseekTool, len(req.Tools))
		for i, t := range req.Tools {
			dsReq.Tools[i] = deepseekTool{
				Type: "function",
				Function: deepseekFunctionDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}

	jsonData, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call DeepSeek API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("DeepSeek API error: %s", string(body))
	}

	var dsResp struct {
		Choices []struct {
			Message struct {
				Content          string             `json:"content"`
				ReasoningContent string             `json:"reasoning_content,omitempty"`
				ToolCalls        []deepseekToolCall `json:"tool_calls,omitempty"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &dsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var toolCalls []ToolCall
	content := ""
	reasoning := ""
	if len(dsResp.Choices) > 0 {
		msg := dsResp.Choices[0].Message
		content = msg.Content
		reasoning = msg.ReasoningContent
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			})
		}
	}

	return &ChatResponse{
		Content:          content,
		ReasoningContent: reasoning,
		ToolCalls:        toolCalls,
		TokenCount:       dsResp.Usage.TotalTokens,
	}, nil
}

func (p *DeepSeekProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]deepseekMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, messageToDeepSeekMessage(m))
	}

	dsReq := deepseekRequest{
		Model:       model,
		Messages:    messages,
		Stream:      true,
		MaxTokens:   req.MaxTokens,
		Temperature: 0.7,
	}

	if len(req.Tools) > 0 {
		dsReq.Tools = make([]deepseekTool, len(req.Tools))
		for i, t := range req.Tools {
			dsReq.Tools[i] = deepseekTool{
				Type: "function",
				Function: deepseekFunctionDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}

	jsonData, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call DeepSeek API: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("DeepSeek API error: %s", string(body))
	}

	ch := make(chan ChatChunk, 100)
	go func() {
		defer close(ch)
		ParseDeepSeekSSE(resp.Body, func(chunk ChatChunk) {
			ch <- chunk
		})
	}()

	return ch, nil
}

// messageToDeepSeekMessage converts a Message to DeepSeek (OpenAI-compatible) message format.
// It handles old-style (plain text), structured content, and tool role messages.
func messageToDeepSeekMessage(m Message) deepseekMessage {
	// Tool result: use role "tool" with tool_call_id
	if m.Role == "tool" {
		return deepseekMessage{
			Role:       "tool",
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
	}

	// Structured content: extract text for assistant messages, keep blocks for assistant tool_use
	if IsStructuredContent(m.Content) {
		parsed := ParseContentBlocks(m.Content)
		if m.Role == "assistant" {
			// For assistant messages, extract text and tool_calls from structured content
			var textBuilder strings.Builder
			var toolCalls []deepseekToolCall
			for _, b := range parsed {
				switch b.Type {
				case ContentTypeText:
					textBuilder.WriteString(b.Text)
				case ContentTypeToolUse:
					args := string(b.Input)
					if args == "" {
						args = "{}"
					}
					toolCalls = append(toolCalls, deepseekToolCall{
						ID:   b.ID,
						Type: "function",
						Function: deepseekFunction{
							Name:      b.Name,
							Arguments: args,
						},
					})
				}
			}
			return deepseekMessage{
				Role:             "assistant",
				Content:          textBuilder.String(),
				ToolCalls:        toolCalls,
				ReasoningContent: m.ReasoningContent, // pass back thinking-mode content
			}
		}
		// For user messages, just extract text
		var textBuilder strings.Builder
		for _, b := range parsed {
			if b.Type == ContentTypeText {
				textBuilder.WriteString(b.Text)
			}
		}
		return deepseekMessage{Role: m.Role, Content: textBuilder.String()}
	}

	// Old format: plain text. Assistant turns may carry reasoning_content
	// (DeepSeek thinking mode) — pass it through.
	return deepseekMessage{
		Role:             m.Role,
		Content:          m.Content,
		ReasoningContent: m.ReasoningContent,
	}
}

func (p *DeepSeekProvider) GetModel() string {
	return p.model
}
