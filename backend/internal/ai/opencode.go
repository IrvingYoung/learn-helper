package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OpenCodeProvider struct {
	apiKey string
	model  string
}

func NewOpenCodeProvider(apiKey, model string) *OpenCodeProvider {
	if model == "" {
		model = "deepseek-v4-pro"
	}
	return &OpenCodeProvider{apiKey: apiKey, model: model}
}

func (p *OpenCodeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]deepseekMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, messageToDeepSeekMessage(m))
	}

	ocReq := deepseekRequest{
		Model:     model,
		Messages:  messages,
		Stream:    false,
		MaxTokens: req.MaxTokens,
	}

	if len(req.Tools) > 0 {
		ocReq.Tools = make([]deepseekTool, len(req.Tools))
		for i, t := range req.Tools {
			ocReq.Tools[i] = deepseekTool{
				Type: "function",
				Function: deepseekFunctionDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}

	jsonData, err := json.Marshal(ocReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://opencode.ai/zen/go/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenCode API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenCode API error: %s", string(body))
	}

	var ocResp struct {
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
	if err := json.Unmarshal(body, &ocResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var toolCalls []ToolCall
	content := ""
	reasoning := ""
	if len(ocResp.Choices) > 0 {
		msg := ocResp.Choices[0].Message
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
		TokenCount:       ocResp.Usage.TotalTokens,
	}, nil
}

func (p *OpenCodeProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]deepseekMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, messageToDeepSeekMessage(m))
	}

	ocReq := deepseekRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
		MaxTokens: req.MaxTokens,
	}

	if len(req.Tools) > 0 {
		ocReq.Tools = make([]deepseekTool, len(req.Tools))
		for i, t := range req.Tools {
			ocReq.Tools[i] = deepseekTool{
				Type: "function",
				Function: deepseekFunctionDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}

	jsonData, err := json.Marshal(ocReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://opencode.ai/zen/go/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenCode API: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("OpenCode API error: %s", string(body))
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

func (p *OpenCodeProvider) GetModel() string {
	return p.model
}
