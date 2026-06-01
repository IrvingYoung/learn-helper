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
		// ── Write tools (gated by permission queue) ──
		{
			Name:        "create_page",
			Description: "在指定父页面下创建新页面。走权限闸门,需要用户批准。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":     map[string]any{"type": "string", "description": "页面标题(必填)"},
					"parent_id": map[string]any{"type": "integer", "description": "父页面 ID;顶级页面或留空走 focusPageID"},
					"content":   map[string]any{"type": "string", "description": "页面 markdown 内容,可选(空骨架页)"},
					"page_type": map[string]any{"type": "string", "enum": []string{"entity", "concept", "overview"}, "description": "页面类型,默认 entity"},
					"slug":      map[string]any{"type": "string", "description": "URL slug,可选,默认自动生成"},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "update_page",
			Description: "覆盖式更新页面内容。走权限闸门。改大段用这个。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "要更新的页面 ID(必填)"},
					"content": map[string]any{"type": "string", "description": "新 markdown 内容(必填)"},
					"title":   map[string]any{"type": "string", "description": "新标题,可选"},
				},
				"required": []string{"page_id", "content"},
			},
		},
		{
			Name:        "patch_page",
			Description: "增量编辑页面:按标题替换章节(replace)或在末尾追加(append)。走权限闸门。改小段用这个,避免重写整页。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "页面 ID(必填)"},
					"operations": map[string]any{
						"type":        "array",
						"description": "操作列表,按顺序执行",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"type":    map[string]any{"type": "string", "enum": []string{"replace", "append"}},
								"target":  map[string]any{"type": "string", "description": "replace 的目标标题(带 # 号,如 '## 核心概念')"},
								"content": map[string]any{"type": "string", "description": "markdown 内容"},
							},
							"required": []string{"type", "content"},
						},
					},
				},
				"required": []string{"page_id", "operations"},
			},
		},
		{
			Name:        "delete_page",
			Description: "删除页面。走权限闸门。慎用:能 move_page / update_page 解决的优先用那两个。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "页面 ID"},
				},
				"required": []string{"page_id"},
			},
		},
		{
			Name:        "link_pages",
			Description: "在 source 页面添加指向 target 的链接。走权限闸门。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_page_id": map[string]any{"type": "integer", "description": "源页 ID"},
					"target_page_id": map[string]any{"type": "integer", "description": "目标页 ID"},
					"link_text":      map[string]any{"type": "string", "description": "链接显示文本,可选(默认用目标页标题)"},
				},
				"required": []string{"source_page_id", "target_page_id"},
			},
		},
		{
			Name:        "move_page",
			Description: "把页面移到新父节点下。走权限闸门。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id":       map[string]any{"type": "integer", "description": "要移动的页面 ID"},
					"new_parent_id": map[string]any{"type": "integer", "description": "新父页 ID"},
				},
				"required": []string{"page_id", "new_parent_id"},
			},
		},

		// ── ask_user ──
		{
			Name:        "ask_user",
			Description: "向用户提一个澄清问题。可以附带 context(outline / page / markdown / diff)让用户看到具体物料。阻塞 ReAct loop 直到用户回答。不用于确认写操作(那是权限闸门的事)。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{"type": "string", "description": "问题正文"},
					"options": map[string]any{
						"type":        "array",
						"description": "2-4 个选项",
						"items":       map[string]any{"type": "string"},
						"minItems":    2,
						"maxItems":    4,
					},
					"context": map[string]any{
						"type":        "object",
						"description": "可选,context.kind 决定渲染",
						"properties": map[string]any{
							"kind": map[string]any{"type": "string", "enum": []string{"outline", "page", "markdown", "diff"}},
							"data": map[string]any{"description": "按 kind 决定形状:outline→OutlineNode[];page→{page_id};markdown→string;diff→[{page_id,before,after,label?}]"},
						},
						"required": []string{"kind", "data"},
					},
					"multi_select":    map[string]any{"type": "boolean", "description": "默认 false"},
					"allow_free_text": map[string]any{"type": "boolean", "description": "默认 true"},
					"header":          map[string]any{"type": "string", "description": "短标签,最多 12 字符"},
				},
				"required": []string{"question", "options"},
			},
		},

		// ── Read tools (unchanged) ──
		{
			Name:        "lookup_page",
			Description: "根据页面标题查询页面信息,返回页面 ID、标题等元数据。可自动执行,无需用户确认。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{"type": "string", "description": "要查询的页面标题(精确匹配)"},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "read_page",
			Description: "根据页面 ID 读取 Wiki 页面的完整 Markdown 内容。可自动执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "页面 ID"},
				},
				"required": []string{"page_id"},
			},
		},
		{
			Name:        "search_pages",
			Description: "在知识库中搜索页面标题和内容,返回匹配的页面列表。可自动执行。",
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
			Description: fmt.Sprintf("搜索网络获取相关信息。当前是 %d 年,注意搜索内容的时效性。", time.Now().Year()),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":       map[string]any{"type": "string"},
					"max_results": map[string]any{"type": "integer"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "webfetch",
			Description: "获取指定 URL 的网页内容,提取正文文本返回。可自动执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string"},
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

## 工作节奏（强制遵守 — 三阶段分离）

每个 propose_plan 只能属于一个阶段。**禁止混阶段**，后端会拒绝。

**阶段一：建主页（main）** — actions 恰好 1 个 create_page，必须设 parent_id（用 lookup_page 先查父页 ID）。允许带 overview content，禁止带子章节内容。

**阶段二：生成目录（outline）** — outline 非空，actions 必须为空。每个节点有 id/title/page_type/children，禁止带 content。节点数 ≤ 30。

**阶段三：填充内容（content）** — outline 为空，actions 只含 update_page/patch_page/link_pages/move_page，数量 ≤ 2。禁止 create_page。

**占位符规则** — 引用 {{action:X.field}} 时必须把 X 加入 depends_on。引用不存在的 action 会让 plan 失败报错，不会脏写。

**自检清单** — 提交前逐项确认：阶段正确 / actions 和 outline 互斥 / main 1 个 create_page + parent_id / outline ≤ 30 节点 / content ≤ 2 mutation 无 create_page / 占位符对应 depends_on。

**简单任务** — 「记这里」「改这段」单页操作：单个 update_page/patch_page，无需分阶段。

**用户协作流程** — 用户说"想学 X"时：讨论 → 建主页（阶段一）→ 用户说"生成目录"时建 outline（阶段二）→ 用户说"写 1.1"时逐页填（阶段三）。每个阶段独立 plan、独立 confirm。

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
