package ai

import (
	"bufio"
	"time"
	"encoding/json"
	"context"
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
			Name:        "propose_plan",
			Description: "提出对知识库的操作计划。当你需要创建、更新、删除页面或建立链接时，使用此工具一次性提出所有操作。系统会按依赖顺序执行。用户确认后才会真正执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reasoning": map[string]any{
						"type":        "string",
						"description": "为什么建议这些操作，向用户解释你的思路",
					},
					"outline": map[string]any{
						"type":        "array",
						"description": "知识大纲树（可选）。展示为可折叠树状结构，确认后自动创建所有骨架页面（内容为空）。适用于 3+ 页面的复杂体系建设",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{
									"type":        "string",
									"description": "节点标识符，可选，供后续 action 通过 {{action:id.page_id}} 引用",
								},
								"title": map[string]any{
									"type":        "string",
									"description": "页面标题",
								},
								"page_type": map[string]any{
									"type":        "string",
									"enum":        []string{"entity", "concept", "overview"},
									"description": "页面类型",
								},
								"children": map[string]any{
									"type":        "array",
									"items":       map[string]any{"$ref": "#"},
									"description": "子节点，递归结构",
								},
							},
						},
					},
					"phases": map[string]any{
						"type":        "array",
						"description": "整体路线图（可选）。首次 propose_plan 时让用户了解全貌。纯信息字段，不做系统级追踪",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"title":       map[string]any{"type": "string", "description": "阶段标题"},
								"description": map[string]any{"type": "string", "description": "阶段简述"},
							},
						},
					},
					"phase_index": map[string]any{
						"type":        "integer",
						"description": "当前阶段序号，从 0 开始。首次调用传 0",
					},
					"total_phases": map[string]any{
						"type":        "integer",
						"description": "总阶段数。首次调用时必填",
					},
					"actions": map[string]any{
						"type":        "array",
						"description": "要执行的操作列表",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{
									"type":        "string",
									"description": "操作的唯一标识符，用于依赖引用。例如 'a1', 'a2'",
								},
								"type": map[string]any{
									"type":        "string",
									"enum":        []string{"create_page", "update_page", "delete_page", "link_pages", "move_page"},
									"description": "操作类型",
								},
								"create_page_params": map[string]any{
									"type": "object",
									"description": "create_page 的参数（type 为 create_page 时使用）",
									"properties": map[string]any{
										"title":     map[string]any{"type": "string", "description": "页面标题"},
										"slug":      map[string]any{"type": "string", "description": "URL slug，可选，默认自动生成"},
										"parent_id": map[string]any{"type": "integer", "description": "父页面 ID，顶级页面不需要"},
										"content":   map[string]any{"type": "string", "description": "页面 Markdown 内容"},
										"page_type": map[string]any{"type": "string", "description": "页面类型，默认 wiki"},
									},
									"required": []string{"title"},
								},
								"update_page_params": map[string]any{
									"type": "object",
									"description": "update_page 的参数（type 为 update_page 时使用）",
									"properties": map[string]any{
										"page_id": map[string]any{"type": "integer", "description": "要更新的页面 ID"},
										"content": map[string]any{"type": "string", "description": "新的页面内容"},
										"title":   map[string]any{"type": "string", "description": "新标题，可选"},
									},
									"required": []string{"page_id", "content"},
								},
								"delete_page_params": map[string]any{
									"type": "object",
									"description": "delete_page 的参数（type 为 delete_page 时使用）",
									"properties": map[string]any{
										"page_id": map[string]any{"type": "integer", "description": "要删除的页面 ID"},
									},
									"required": []string{"page_id"},
								},
								"link_pages_params": map[string]any{
									"type": "object",
									"description": "link_pages 的参数（type 为 link_pages 时使用）",
									"properties": map[string]any{
										"source_page_id": map[string]any{"type": "integer", "description": "源页面 ID"},
										"target_page_id": map[string]any{"type": "integer", "description": "目标页面 ID"},
										"link_text":      map[string]any{"type": "string", "description": "链接显示文本，可选"},
									},
									"required": []string{"source_page_id", "target_page_id"},
								},
								"move_page_params": map[string]any{
									"type": "object",
									"description": "move_page 的参数（type 为 move_page 时使用）",
									"properties": map[string]any{
										"page_id":      map[string]any{"type": "integer", "description": "要移动的页面 ID"},
										"new_parent_id": map[string]any{"type": "integer", "description": "新的父页面 ID"},
									},
									"required": []string{"page_id", "new_parent_id"},
								},
								"depends_on": map[string]any{
									"type":        "array",
									"items":       map[string]any{"type": "string"},
									"description": "依赖的操作ID列表。例如创建子页面依赖父页面的创建。依赖的操作中生成的page_id可通过 {{action:ID.page_id}} 在参数中引用",
								},
							},
							"required": []string{"id", "type"},
						},
					},
				},
				"required": []string{"reasoning", "actions"},
			},
		},
		{
			Name:        "lookup_page",
			Description: "根据页面标题查询页面信息，返回页面 ID、标题等元数据。可自动执行，无需用户确认。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{"type": "string", "description": "要查询的页面标题（精确匹配）"},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "read_page",
			Description: "根据页面 ID 读取 Wiki 页面的完整 Markdown 内容。可自动执行，无需用户确认。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "要读取的页面 ID"},
				},
				"required": []string{"page_id"},
			},
		},
		{
			Name:        "search_pages",
			Description: "在知识库中搜索页面标题和内容，返回匹配的页面列表。可自动执行，无需用户确认。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索关键词"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "websearch",
			Description: "搜索网络获取相关信息，返回结构化结果列表（标题、URL、摘要）。可自动执行，无需用户确认。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":       map[string]any{"type": "string", "description": "搜索关键词"},
					"max_results": map[string]any{"type": "integer", "description": "返回结果数量（默认 5）"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "webfetch",
			Description: "获取指定 URL 的网页内容，提取正文文本返回。可自动执行，无需用户确认。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "要获取内容的网页 URL"},
				},
				"required": []string{"url"},
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
	treeContext := ""
	if wikiContext != "" {
		treeContext = wikiContext
	} else {
		treeContext = "（暂无页面）"
	}

	wikiMaintainerPrompt := `你是 LLM Wiki 的知识库维护者。你的职责是管理知识树，包括创建、更新、删除页面和建立页面间的链接。

## 工作方式

1. **快速调查，果断行动** — 最多用 1-2 次 search_pages/lookup_page 查看现有内容，然后立即调用 propose_plan 提出操作
2. **选择合适的操作方式** — 单页知识（1-2 个页面）直接 propose_plan(actions=[...])，一次确认执行；复杂知识体系（3+ 页面）先用 outline 从大纲开始
3. **一次性提出计划** — 当需要修改知识库时，调用 propose_plan 工具，一次性提出所有当前阶段需要的操作
4. **操作有依赖关系** — 如果创建子页面依赖父页面，在 depends_on 中声明，使用 {{action:ID.page_id}} 引用依赖结果

## 关键：何时调用 propose_plan

- 用户要求创建/修改/删除页面 → 立即调用 propose_plan
- 知识库为空时 → 直接创建首页和分类，不需要反复搜索
- 已经搜索过且没有重复 → 立即 propose_plan，不要再次搜索

## propose_plan 调用示例

当 type 为某种操作时，使用对应的 *_params 字段填入参数：
- type="create_page" → 使用 create_page_params: {"title": "...", "content": "..."}
- type="update_page" → 使用 update_page_params: {"page_id": 1, "content": "..."}
- type="delete_page" → 使用 delete_page_params: {"page_id": 1}
- type="link_pages" → 使用 link_pages_params: {"source_page_id": 1, "target_page_id": 2}
- type="move_page" → 使用 move_page_params: {"page_id": 1, "new_parent_id": 2}

## 规则

- 创建页面时提供有意义的内容，不要留空
- 修改内容时先 read_page 查看现有内容
- 如果用户只是提问或聊天，不需要调用 propose_plan
- 在页面内容中使用 [[页面标题]] 语法创建链接

` + treeContext

	dateStr := time.Now().Format("2006-01-02")
	wikiMaintainerPrompt += fmt.Sprintf("\n[Request Timestamp: %s]\n[Context Notice: The user's query was issued at the timestamp above. Ensure search results are current and relevant to the query date.]\n", dateStr)

	return wikiMaintainerPrompt
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
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
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
	Role       string              `json:"role"`
	Content    any                 `json:"content"`
	ToolCalls  []deepseekToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
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

		// On finish, flush tool calls and signal done
		if choice.FinishReason == "tool_calls" || choice.FinishReason == "stop" {
			for _, tc := range toolCalls {
				callback(ChatChunk{ToolCall: tc})
			}
			toolCalls = make(map[int]*ToolCall) // prevent double-flush on [DONE]
			callback(ChatChunk{Done: true})
		}
	}

	return scanner.Err()
}
