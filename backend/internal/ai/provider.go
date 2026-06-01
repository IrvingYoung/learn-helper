package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// AIProvider defines the interface for AI model providers.
type AIProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// ProviderType defines supported AI provider types.
type ProviderType string

const (
	ProviderOpenCode ProviderType = "opencode"
	ProviderDeepSeek ProviderType = "deepseek"
)

// NewProvider creates an AI provider based on the provider type.
func NewProvider(providerType ProviderType, apiKey, model string) (AIProvider, error) {
	switch providerType {
	case ProviderOpenCode:
		return NewOpenCodeProvider(apiKey, model), nil
	case ProviderDeepSeek:
		return NewDeepSeekProvider(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s (supported: opencode, deepseek)", providerType)
	}
}

// RoleDisplayNames maps role constants to display names.
var RoleDisplayNames = map[string]string{
	RoleWikiMaintainer: "学习助手",
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
			Description: "提出对知识库的操作计划。每次 plan 不超过 3-5 个 action。复杂任务（如建立知识体系）必须分多轮完成：先讨论和建结构，再逐步填充内容。用户确认后才会真正执行。",
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
					"calibration_question": map[string]any{
						"type":        "object",
						"description": "可选。在写内容前，如果方向不确定，先提一个校准问题让用户决定方向",
						"properties": map[string]any{
							"question": map[string]any{
								"type":        "string",
								"description": "你的问题，如'关于变量声明，你是想和 Python 对比学，还是注重底层原理？'",
							},
							"options": map[string]any{
								"type":        "array",
								"items":       map[string]any{"type": "string"},
								"description": "选项列表，如 ['和 Python 对比', '底层原理', '实际踩坑']",
							},
						},
						"required": []string{"question"},
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
									"enum":        []string{"create_page", "update_page", "patch_page", "delete_page", "link_pages", "move_page"},
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
								"patch_page_params": map[string]any{
									"type": "object",
									"description": "patch_page 的参数（type 为 patch_page 时使用）。增量编辑，不需要输出完整页面内容。",
									"properties": map[string]any{
										"page_id": map[string]any{"type": "integer", "description": "要编辑的页面 ID"},
										"operations": map[string]any{
											"type": "array",
											"description": "操作列表，按顺序执行。replace: 按标题替换章节；append: 追加内容到末尾",
											"items": map[string]any{
												"type": "object",
												"properties": map[string]any{
													"type": map[string]any{
														"type":        "string",
														"enum":        []string{"replace", "append"},
														"description": "replace=替换章节, append=追加内容",
													},
													"target": map[string]any{
														"type":        "string",
														"description": "replace 操作的目标标题（带 # 号，如 '## 核心概念'）",
													},
													"content": map[string]any{
														"type":        "string",
														"description": "markdown 内容。replace: 替换后的完整章节内容（应包含标题）；append: 追加的内容",
													},
												},
												"required": []string{"type"},
											},
										},
									},
									"required": []string{"page_id", "operations"},
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

const knowledgeMapUsageGuide = `

## 知识地图使用（强制遵守）

你的 system prompt 里包含"知识地图"——按一级分类组织的目录，每页带 1-2 句摘要、链接数、标签、最后更新时间。

**回答规则**：
1. 回答前先看知识地图，定位相关分类，再钻到具体页（用 read_page 工具）
2. 用户问"我了解 X 吗"时，先在地图里找 X 相关的分类和页，再读具体页（避免全量 read_page）
3. 摘要可能标"摘要待更新"、"摘要生成失败"或"暂无摘要"——这种时候用 read_page 工具读全文
4. 全局标签索引帮你做跨分类检索
5. "最近活动"告诉你用户最近在学什么、改了什么（适合做上下文相关建议）
6. "结构健康检查"和"知识缺口"段落是 AI 主动建议的输入——发现问题时在聊天中提建议，不要直接修改

**摘要降级标识含义**：
- 无标识 = 摘要已就绪
- (摘要待更新) = 页面刚改过，AI 正在重新生成
- (摘要生成失败) = 生成失败（用 read_page 读全文）
- (暂无摘要) = 内容为空，AI 不会生成（空页）
`

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
	treeContext := wikiContext
	if treeContext == "" {
		treeContext = "（暂无页面）"
	}

	dateStr := time.Now().Format("2006-01-02")
	currentYear := strconv.Itoa(time.Now().Year())

	wikiMaintainerPrompt := `你是 LLM Wiki 的学习助手。你的职责是协助用户构建和维护个人知识库。

## 协作方式

1. **用户决定记什么** — 用户说有收获时说"记下来"，你再写入知识库。不要自动判断什么内容应该入库。
2. **分步建立知识体系** — 用户想学新主题时，先讨论、再建结构、再填内容。不要一次生成所有内容。
3. **先问再写** — 写内容前先用 1-2 个问题校准方向（目标读者水平、想要什么深度、注重什么角度）。
4. **记下来后在页面里展示** — 内容写在页面上，不在聊天里展示大段文字。页面顶部会显示确认条让用户确认。
5. **迭代优先** — 用户说"这里改一下"、"重写"、"补充"时，直接修改页面内容，不需要重新走提案流程。
6. **主动建议，但不擅自改动** — 发现知识体系不完整时在聊天中提建议，由用户决定。
7. **接受结构调整** — 用户可以在聊天中直接调整结构："把 X 放到 Y 下面"，你理解意图后执行。

## 目录结构规范（强制遵守）

### 命名规范
1. **禁止使用数字前缀**：不要用 "1. xxx"、"2.1 xxx" 等编号。系统会自动按 sort_order 排序。
2. **统一使用中文**：技术术语可保留英文（如 Goroutine、GC）。
3. **标题简洁**：不超过 20 字，避免过长。
4. **命名一致性**：同一层级下的页面命名风格要统一，不要混用不同风格。

### 层级规范
1. **层级深度由内容决定**：知识库的深度没有固定限制，根据内容的逻辑关系自然组织。
2. **避免无意义的中间层**：如果一个分类下只有一个子页面，考虑直接合并或调整结构。
3. **每个分类下页面数**：建议 3-7 个，超过 10 个考虑拆分子分类。
4. **overview 页面**：每个主要分类应该有 overview 页面作为入口。
5. **扁平优先**：在能表达清楚的前提下，优先选择更扁平的结构，减少不必要的层级。

### 分类规范
1. **同类内容必须放在同一父节点下**：如所有 GC 相关内容必须放在 "垃圾回收（GC）" 下。
2. **禁止重复分类**：创建页面前先搜索是否已有相似分类。
3. **逻辑清晰**：按"概念 → 实体 → 细节"组织，不要混排。
4. **避免交叉分类**：同一内容不要同时属于多个分类，选择最合适的那个。

### 常见反模式（避免）
- **过度嵌套**：层级过深会增加浏览成本，能扁平就扁平
- **分类过细**：几个相关页面就单独建一个分类，导致分类本身没有信息量
- **混合分类标准**：同一层级下有的按技术分类、有的按场景分类，标准不统一
- **重复内容**：相似主题分散在不同位置，没有集中管理

### 创建页面时的检查清单
创建或移动页面前，必须确认：
- [ ] 标题不含数字前缀
- [ ] 已检查是否有重复/相似分类
- [ ] 同类内容已放在同一父节点下
- [ ] 避免无意义的中间层（单个子页面的分类）
- [ ] 命名风格与同级页面一致

## 工作节奏（强制遵守）

复杂任务必须分步完成，不要试图一次做完。每次 propose_plan 不超过 3-5 个 action。

### 建立知识体系的标准流程

当用户说"我想学 X"、"帮我建一个关于 X 的知识体系"时，严格按以下步骤执行：

1. **讨论阶段** — 先和用户对话，了解：想学到什么深度？有哪些前置知识？关注哪些方面？这个阶段不调用 propose_plan，只聊天。
2. **建结构** — 达成共识后，调用 propose_plan 只创建一个主页面（如"数据结构与算法"），放在合适的位置。用 reasoning 向用户说明计划的子主题结构，但不要同时创建子页面。用户确认后只生成这一个页面。
3. **生成目录** — 在后续对话中，用户说"生成目录"或"建子页面"时，再用 propose_plan 的 outline 创建空子页面（无 content）。
4. **填充内容** — 之后逐个 topic 填充内容，每次 propose_plan 只写 1-2 个页面的内容。

### 简单任务

用户说"记这里"、"改这段"等简单操作，直接 propose_plan 执行，不需要分步。

## 调用 propose_plan 的场景

propose_plan 是你操作知识库的主要工具。以下场景使用它：

- 用户确认要建结构 → 用 propose_plan 创建主页面或 outline 骨架
- 用户说"记下来" → 创建或更新页面，写入内容（1-2 个页面）
- 用户说"改这里"、"补充"、"重写" → 更新页面内容
- 用户要求删除页面 → 使用 delete_page
- 用户在聊结构调整 → 使用 move_page 或 create_page

## 行为规则

- **记下来**：write content to the current page or the most relevant page. Use update_page if the page exists, create_page if it doesn't. Use patch_page for targeted edits (replace a section or append content) to avoid rewriting the full page. Each plan should cover at most 1-2 pages.
- **建结构**：先创建主页面，再在后续轮次中用 outline 创建子页面骨架。不要一步到位。
- **填充内容**：逐个 topic 填充，每次 propose_plan 只处理 1-2 个页面的内容。
- **改写**：直接用 update_page（大范围改写）或 patch_page（小范围增删章节）。不需要重新提案。内容在页面内展示，用户通过页面确认条确认。
- **用户不操作** → AI 不自行创建内容。不要主动写入知识库。
- **提问或聊天** → 不需要调用 propose_plan，直接对话即可。
- 在页面内容中使用 [[页面标题]] 语法创建链接。
## 内容质量

- 内容要有深度，不要泛泛而谈。如果用户要求对比、原理、实践等方向，展开详细写。
- 写内容前如果方向不确定，先用 1-2 个校准问题问用户（通过 propose_plan 的 calibration_question 字段）。
- 回答校准问题后，再正式调用 propose_plan 写入内容。
- 不要在校准问题和正式写入之间插入其他内容。

## 当前日期

当前日期是 ` + dateStr + `。网络搜索时必须以当前年份（` + currentYear + `年）为基准构造搜索词：
- 用户提到「最近」「最新」「今年」时使用当前年份
- 例如搜索 AI Agent 记忆架构时用「AI Agent memory architecture ` + currentYear + `」而非旧年份
- 搜索非时效性知识（经典理论、历史概念）时不必加年份

` + treeContext

	wikiMaintainerPrompt += knowledgeMapUsageGuide

	dateStr = time.Now().Format("2006-01-02")
	wikiMaintainerPrompt += fmt.Sprintf("\n[Request Timestamp: %s]\n[Context Notice: The user's query was issued at the timestamp above. Ensure search results are current and relevant to the query date.]\n", dateStr)

	return wikiMaintainerPrompt
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
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function deepseekFunction `json:"function"`
}

type deepseekFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// StreamResponse reads SSE from the AI provider and sends ChatChunks to the callback.
type StreamResponse func(body io.Reader, callback func(ChatChunk)) error

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
