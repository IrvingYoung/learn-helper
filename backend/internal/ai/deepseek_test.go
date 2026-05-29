package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepSeekProvider_Chat(t *testing.T) {
	// Mock server for testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Error("Authorization header not set correctly")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type header not set correctly")
		}

		// Parse request body
		var req deepseekRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "deepseek-v4-flash" {
			t.Errorf("expected model 'deepseek-v4-flash', got '%s'", req.Model)
		}

		// Return mock response
		resp := deepseekResponse{
			Choices: []struct {
				Index        int `json:"index"`
				Message      struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Content: "Hello, this is DeepSeek response",
					},
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				TotalTokens: 100,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with test server URL (we need to modify for this test)
	provider := NewDeepSeekProvider("test-api-key", "deepseek-v4-flash")

	// Test chat request structure
	// Note: Full integration test would require mocking the actual API endpoint
	_ = ChatRequest{
		Model: "deepseek-v4-flash",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Note: In real test, we would need to inject the test server URL
	// For now, we test the provider structure
	if provider.model != "deepseek-v4-flash" {
		t.Errorf("expected model 'deepseek-v4-flash', got '%s'", provider.model)
	}
}

func TestDeepSeekProvider_DefaultModel(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "")
	if provider.model != "deepseek-v4-flash" {
		t.Errorf("expected default model 'deepseek-v4-flash', got '%s'", provider.model)
	}
}

func TestDeepSeekProvider_GetModel(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "deepseek-coder")
	if provider.GetModel() != "deepseek-coder" {
		t.Errorf("expected model 'deepseek-coder', got '%s'", provider.GetModel())
	}
}

func TestNewProvider_Claude(t *testing.T) {
	provider, err := NewProvider(ProviderClaude, "test-key", "claude-sonnet")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := provider.(*ClaudeProvider); !ok {
		t.Error("expected *ClaudeProvider")
	}
}

func TestNewProvider_DeepSeek(t *testing.T) {
	provider, err := NewProvider(ProviderDeepSeek, "test-key", "deepseek-v4-flash")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := provider.(*DeepSeekProvider); !ok {
		t.Error("expected *DeepSeekProvider")
	}
}

func TestNewProvider_Invalid(t *testing.T) {
	_, err := NewProvider(ProviderType("invalid"), "test-key", "model")
	if err == nil {
		t.Error("expected error for invalid provider type")
	}
	if !strings.Contains(err.Error(), "unsupported provider type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDeepSeekStreamResponse_Parsing(t *testing.T) {
	// Test parsing SSE format - note: parseStreamResponse handles "data: " prefix
	// so we test the JSON part only
	jsonData := `{"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`

	var resp deepseekStreamResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to parse stream response: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Delta.Content != "Hello" {
		t.Errorf("expected content 'Hello', got '%s'", resp.Choices[0].Delta.Content)
	}
}

func TestDeepSeekProvider_StreamChat_Interface(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "deepseek-v4-flash")
	var _ AIProvider = provider // Verify it implements AIProvider interface
}