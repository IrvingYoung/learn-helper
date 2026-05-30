package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// AIProvider defines the interface for AI model providers.
type AIProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// ProviderType defines supported AI provider types.
type ProviderType string

const (
	ProviderClaude   ProviderType = "claude"
	ProviderDeepSeek ProviderType = "deepseek"
)

// NewProvider creates an AI provider based on the provider type.
func NewProvider(providerType ProviderType, apiKey, model string) (AIProvider, error) {
	switch providerType {
	case ProviderClaude:
		return NewClaudeProvider(apiKey, model), nil
	case ProviderDeepSeek:
		return NewDeepSeekProvider(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s (supported: claude, deepseek)", providerType)
	}
}

// RoleDisplayNames maps role constants to display names.
var RoleDisplayNames = map[string]string{
	RoleWikiMaintainer: "Wiki 管理员",
}

// SystemPromptTemplates is kept for backward compatibility; wiki_maintainer uses BuildSystemPrompt instead.
var SystemPromptTemplates = map[string]string{
	RoleWikiMaintainer: "", // handled by BuildSystemPrompt
}

// ChatChunk represents a piece of streamed AI response.
type ChatChunk struct {
	Content  string    // text content delta
	ToolCall *ToolCall // completed tool call (only when Done=true and a tool was called)
	Done     bool
}

// ToolCall represents a completed tool call from the AI.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // JSON string of tool input
}

// Tool represents a tool definition for AI function calling.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// WikiTools returns the tool definitions for wiki_maintainer.
func WikiTools() []Tool {
	return []Tool{
		{
			Name:        "create_page",
			Description: "创建新的 Wiki 页面。用户确认后才会执行。如果只想创建页面占位符，内容可为空。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":     map[string]any{"type": "string", "description": "页面标题"},
					"slug":      map[string]any{"type": "string", "description": "URL 标识符，英文短横线格式如 job-hunting"},
					"parent_id": map[string]any{"type": "integer", "description": "父页面 ID，从知识库结构中获取对应的页面 ID"},
					"content":   map[string]any{"type": "string", "description": "页面内容，Markdown 格式（可为空）"},
					"page_type": map[string]any{"type": "string", "description": "页面类型：entity(实体) 或 concept(概念)"},
				},
				"required": []string{"title", "slug"},
			},
		},
		{
			Name:        "update_page",
			Description: "更新已有 Wiki 页面的内容。用户确认后才会执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "要更新的页面 ID"},
					"title":   map[string]any{"type": "string", "description": "新标题（可选）"},
					"content": map[string]any{"type": "string", "description": "新内容，Markdown 格式"},
				},
				"required": []string{"page_id", "content"},
			},
		},
		{
			Name:        "delete_page",
			Description: "删除 Wiki 页面。只能删除空页面（无内容或无子页面）。用户确认后才会执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "要删除的页面 ID"},
				},
				"required": []string{"page_id"},
			},
		},
	}
}

// Role constants
const (
	RoleWikiMaintainer = "wiki_maintainer"
)

// BuildSystemPrompt constructs the system prompt with wiki context.
func BuildSystemPrompt(role string, wikiContext string) string {
	switch role {
	case RoleWikiMaintainer:
		return buildWikiMaintainerPrompt(wikiContext)
	default:
		return "You are a helpful assistant."
	}
}

func buildWikiMaintainerPrompt(wikiContext string) string {
	prompt := `你是一位 Wiki 知识库管理员。你通过工具调用来管理知识库，所有写入操作需要用户确认后才会执行。

你的职责：
- 根据用户请求，使用提供的工具创建、更新或删除 Wiki 页面
- 确保知识库结构合理，内容准确
- 为空页面补充内容
- 维护页面之间的层级关系

重要规则：
1. 创建页面前，先检查知识库结构（如下）看是否已存在同名页面
2. 如果页面已存在，不要重复创建；直接使用该页面作为父页面
3. 如果用户说"在 XXX 目录下创建 YYY"，且 XXX 已存在，就创建 YYY 作为 XXX 的子页面
4. 创建子页面时必须设置正确的 parent_id（父页面的 ID）
5. 创建页面时 slug 使用英文短横线格式（如 job-hunting）
6. 内容使用 Markdown 格式

知识库结构：`

	if wikiContext != "" {
		prompt += "\n" + wikiContext
	} else {
		prompt += "\n（暂无页面）"
	}

	return prompt
}

// Claude API types

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
	Tools     []claudeTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []claudeContentBlock
}

type claudeContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type claudeTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// DeepSeek API types

type deepseekRequest struct {
	Model       string             `json:"model"`
	Messages    []deepseekMessage  `json:"messages"`
	Tools       []deepseekTool     `json:"tools,omitempty"`
	Stream      bool               `json:"stream"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
}

type deepseekMessage struct {
	Role      string              `json:"role"`
	Content   any                 `json:"content"`
	ToolCalls []deepseekToolCall  `json:"tool_calls,omitempty"`
}

type deepseekTool struct {
	Type     string              `json:"type"`
	Function deepseekFunctionDef `json:"function"`
}

type deepseekFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type deepseekToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function deepseekFunction   `json:"function"`
}

type deepseekFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// StreamResponse reads SSE from the AI provider and sends ChatChunks to the callback.
type StreamResponse func(body io.Reader, callback func(ChatChunk)) error

// ParseClaudeSSE parses Claude's SSE stream.
func ParseClaudeSSE(body io.Reader, callback func(ChatChunk)) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventType string
	var currentToolID string
	var currentToolName string
	var toolInputParts []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		switch eventType {
		case "content_block_delta":
			var delta struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &delta); err == nil && delta.Delta.Text != "" {
				callback(ChatChunk{Content: delta.Delta.Text})
			}

		case "content_block_start":
			var block struct {
				ContentBlock struct {
					Type  string          `json:"type"`
					ID    string          `json:"id"`
					Name  string          `json:"name"`
					Text  string          `json:"text"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(data), &block); err == nil {
				if block.ContentBlock.Type == "tool_use" {
					currentToolID = block.ContentBlock.ID
					currentToolName = block.ContentBlock.Name
					toolInputParts = nil
				} else if block.ContentBlock.Text != "" {
					callback(ChatChunk{Content: block.ContentBlock.Text})
				}
			}

		case "input_json_delta":
			var delta struct {
				PartialJSON string `json:"partial_json"`
			}
			if err := json.Unmarshal([]byte(data), &delta); err == nil {
				toolInputParts = append(toolInputParts, delta.PartialJSON)
			}

		case "content_block_stop":
			if currentToolID != "" {
				input := strings.Join(toolInputParts, "")
				callback(ChatChunk{
					Done: false,
					ToolCall: &ToolCall{
						ID:    currentToolID,
						Name:  currentToolName,
						Input: input,
					},
				})
				currentToolID = ""
				currentToolName = ""
				toolInputParts = nil
			}

		case "message_stop":
			callback(ChatChunk{Done: true})

		case "error":
			var errEvent struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &errEvent); err == nil {
				callback(ChatChunk{Content: fmt.Sprintf("\n[AI Error: %s]", errEvent.Error.Message), Done: true})
				return nil
			}
		}
	}

	return scanner.Err()
}

// ParseDeepSeekSSE parses DeepSeek's SSE stream (OpenAI-compatible format).
func ParseDeepSeekSSE(body io.Reader, callback func(ChatChunk)) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Track tool calls by index since they arrive in deltas
	toolCalls := make(map[int]*ToolCall)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// Flush any remaining tool calls
			for _, tc := range toolCalls {
				callback(ChatChunk{ToolCall: tc})
			}
			callback(ChatChunk{Done: true})
			return nil
		}

		var resp struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		if len(resp.Choices) == 0 {
			continue
		}

		choice := resp.Choices[0]

		// Text content
		if choice.Delta.Content != "" {
			callback(ChatChunk{Content: choice.Delta.Content})
		}

		// Tool calls (accumulate across deltas)
		for _, tc := range choice.Delta.ToolCalls {
			existing, ok := toolCalls[tc.Index]
			if !ok {
				existing = &ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				toolCalls[tc.Index] = existing
			}
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Function.Name != "" {
				existing.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				existing.Input += tc.Function.Arguments
			}
		}

		// On finish, flush tool calls
		if choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" {
			for _, tc := range toolCalls {
				callback(ChatChunk{ToolCall: tc})
			}
			toolCalls = make(map[int]*ToolCall)
			if choice.FinishReason == "stop" {
				callback(ChatChunk{Done: true})
			}
		}
	}

	return scanner.Err()
}
