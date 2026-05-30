package ai

import (
	"encoding/json"
	"strings"
)

// ContentBlockType represents the type of content block in a structured message.
type ContentBlockType string

const (
	ContentTypeText        ContentBlockType = "text"
	ContentTypeToolUse     ContentBlockType = "tool_use"
	ContentTypeToolResult  ContentBlockType = "tool_result"
)

// ContentBlock represents a structured content block in AI messages.
type ContentBlock struct {
	Type     ContentBlockType `json:"type"`
	Text     string           `json:"text,omitempty"`      // for text blocks
	ID       string           `json:"id,omitempty"`         // tool_use_id / tool_call_id
	Name     string           `json:"name,omitempty"`       // tool name
	Input    json.RawMessage  `json:"input,omitempty"`      // for tool_use: JSON input
	Content  string           `json:"content,omitempty"`    // for tool_result: result content
	IsError  bool             `json:"is_error,omitempty"`   // for tool_result: error flag
}

// Message represents a message in AI conversation history.
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"` // plain text or JSON array of ContentBlock
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
}

// IsStructuredContent checks if the content string is a JSON array of ContentBlocks.
func IsStructuredContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	return len(trimmed) > 0 && trimmed[0] == '['
}

// ParseContentBlocks parses structured content from a JSON array string.
func ParseContentBlocks(content string) []ContentBlock {
	var blocks []ContentBlock
	if err := json.Unmarshal([]byte(content), &blocks); err != nil {
		return nil
	}
	return blocks
}

// ContentBlocksToJSON serializes ContentBlocks to a JSON string for DB storage.
func ContentBlocksToJSON(blocks []ContentBlock) (string, error) {
	data, err := json.Marshal(blocks)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToolCall represents a completed tool call from the AI.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // JSON string of tool input
}

// ChatRequest represents a request to an AI provider.
type ChatRequest struct {
	Messages     []Message
	SystemPrompt string
	Model        string
	MaxTokens    int
	Tools        []Tool
}

// ChatResponse represents a non-streaming response from an AI provider.
type ChatResponse struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	TokenCount int        `json:"token_count"`
}
