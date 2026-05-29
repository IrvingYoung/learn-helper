package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DeepSeek API types
type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepseekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream"`
	Temperature float64           `json:"temperature,omitempty"`
}

type deepseekResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int `json:"index"`
		Message      struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type deepseekStreamChoice struct {
	Index        int `json:"index"`
	Delta        struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type deepseekStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Choices []deepseekStreamChoice `json:"choices"`
}

// DeepSeekProvider implements AIProvider for DeepSeek API
type DeepSeekProvider struct {
	apiKey string
	model  string
}

// NewDeepSeekProvider creates a new DeepSeek provider
func NewDeepSeekProvider(apiKey, model string) *DeepSeekProvider {
	if model == "" {
		model = "deepseek-v4-flash"
	}
	return &DeepSeekProvider{apiKey: apiKey, model: model}
}

// Chat implements AIProvider.Chat for DeepSeek
func (p *DeepSeekProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]deepseekMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, deepseekMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, deepseekMessage{Role: m.Role, Content: m.Content})
	}

	deepseekReq := deepseekRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	if req.MaxTokens > 0 {
		deepseekReq.MaxTokens = req.MaxTokens
	}

	jsonData, err := json.Marshal(deepseekReq)
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

	var dsResp deepseekResponse
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

// StreamChat implements AIProvider.StreamChat for DeepSeek
func (p *DeepSeekProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]deepseekMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, deepseekMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, deepseekMessage{Role: m.Role, Content: m.Content})
	}

	deepseekReq := deepseekRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	if req.MaxTokens > 0 {
		deepseekReq.MaxTokens = req.MaxTokens
	}

	jsonData, err := json.Marshal(deepseekReq)
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
	go p.parseStreamResponse(resp.Body, ch)
	return ch, nil
}

func (p *DeepSeekProvider) parseStreamResponse(body io.Reader, ch chan<- ChatChunk) {
	defer close(ch)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 6 {
			continue
		}

		if line[:6] != "data: " {
			continue
		}

		data := line[6:]
		if data == "[DONE]" {
			ch <- ChatChunk{Done: true}
			return
		}

		var streamResp deepseekStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) > 0 {
			content := streamResp.Choices[0].Delta.Content
			if content != "" {
				ch <- ChatChunk{Content: content, Done: false}
			}

			if streamResp.Choices[0].FinishReason != "" {
				ch <- ChatChunk{Done: true}
				return
			}
		}
	}
}

// GetModel returns the current model name
func (p *DeepSeekProvider) GetModel() string {
	return p.model
}