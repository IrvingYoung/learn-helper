package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ClaudeConfig struct {
	APIKey string
	Model  string
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model         string           `json:"model"`
	MaxTokens     int              `json:"max_tokens"`
	SystemPrompt string           `json:"system,omitempty"`
	Messages     []claudeMessage `json:"messages"`
	Stream       bool             `json:"stream"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type claudeStreamResponse struct {
	Type string `json:"type"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

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
		messages = append(messages, claudeMessage{Role: m.Role, Content: m.Content})
	}

	claudeReq := claudeRequest{
		Model:         model,
		MaxTokens:     req.MaxTokens,
		SystemPrompt:  req.SystemPrompt,
		Messages:      messages,
		Stream:        false,
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

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	content := ""
	for _, c := range claudeResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &ChatResponse{
		Content:    content,
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
		messages = append(messages, claudeMessage{Role: m.Role, Content: m.Content})
	}

	claudeReq := claudeRequest{
		Model:         model,
		MaxTokens:     req.MaxTokens,
		SystemPrompt:  req.SystemPrompt,
		Messages:      messages,
		Stream:        true,
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
	go p.streamResponse(resp.Body, ch)
	return ch, nil
}

func (p *ClaudeProvider) streamResponse(body io.Reader, ch chan<- ChatChunk) {
	defer close(ch)
	decoder := json.NewDecoder(body)

	for decoder.More() {
		var event claudeStreamResponse
		if err := decoder.Decode(&event); err != nil {
			return
		}

		if event.Type == "content_block_delta" {
			ch <- ChatChunk{Content: event.Delta.Text, Done: false}
		} else if event.Type == "message_stop" {
			ch <- ChatChunk{Done: true}
			return
		}
	}
}

func (p *ClaudeProvider) GetModel() string {
	return p.model
}