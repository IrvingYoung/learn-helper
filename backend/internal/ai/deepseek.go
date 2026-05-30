package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		messages = append(messages, deepseekMessage{Role: m.Role, Content: m.Content})
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
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &dsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	content := ""
	if len(dsResp.Choices) > 0 {
		content = dsResp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Content:    content,
		TokenCount: dsResp.Usage.TotalTokens,
	}, nil
}

func (p *DeepSeekProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]deepseekMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, deepseekMessage{Role: m.Role, Content: m.Content})
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

func (p *DeepSeekProvider) GetModel() string {
	return p.model
}
