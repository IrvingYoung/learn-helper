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

	"learn-helper/internal/ai/skills"
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

// ChatChunk represents a piece of streamed AI response.
type ChatChunk struct {
	Content          string    // text content delta
	ReasoningContent string    // reasoning_content delta (DeepSeek thinking mode)
	ToolCall         *ToolCall // completed tool call (only when Done=true and a tool was called)
	Done             bool
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
					"page_type": map[string]any{"type": "string", "enum": []string{"entity", "concept", "overview"}, "description": "页面类型,默认 entity。entity = 具体知识点/实体；concept = 抽象概念（与 entity 区别不大，按习惯选）；overview = 分类入口页（每主要分类应有一个）"},
					"slug":      map[string]any{"type": "string", "description": "URL slug,可选,默认自动生成"},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "update_page",
			Description: "覆盖式更新整页内容。走权限闸门。改大段或整页重写时用这个；改小段用 patch_page 避免重写整页。",
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

		// ── load_skill (progressive disclosure: catalog is in system prompt,
		//     body is loaded on demand via this tool) ──
		{
			Name:        "load_skill",
			Description: "按需加载一个 skill 的完整 markdown body 到当前 context。当你判断当前任务需要某个 skill 的专门指导时调用（例如用户说「用通俗方式讲」→ 加载 explain-page；用户说「帮我做学习大纲」→ 加载 study-outline）。返回的 body 是 markdown 格式，按它的指示回复。skill 名称不带前导 /。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "skill 名称（不带 / 前缀），如 explain-page"},
				},
				"required": []string{"name"},
			},
		},
	}
}

// WikiToolsForCron returns the same tool set as WikiTools but excludes
// ask_user and load_skill. Cron tasks run autonomously with no human in the
// loop, so the AI must not be able to call ask_user (which would block
// forever waiting for a response). load_skill is excluded because cron tasks
// have their own task prompts and don't go through the /command flow.
// All other read and write tools are retained so the AI can still fetch,
// search, read, and modify the wiki on its own.
func WikiToolsForCron() []Tool {
	all := WikiTools()
	out := make([]Tool, 0, len(all))
	for _, t := range all {
		if t.Name == "ask_user" || t.Name == "load_skill" {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Role constants
const (
	RoleWikiMaintainer = "wiki_maintainer"
)

const knowledgeMapUsageGuide = `## 知识地图使用

下面紧接着是 **"知识地图"**——按一级分类组织的目录，每页带摘要、链接数、最后更新时间。

**怎么读它**：
1. 回答 / 写入前先看地图定位相关分类，再用 read_page 钻具体页
2. 用户问"我了解 X 吗" → 先在地图里找 X 相关分类和页，再读具体页（不要全量 read_page）
3. "最近活动" 段告诉你用户最近在改什么，做上下文相关建议时参考
4. "结构健康检查" / "知识缺口" 段是主动建议的输入——发现问题在聊天里提，不直接动手

**摘要降级标识含义**：
- 无标识 = 摘要已就绪
- (摘要待更新) = 页面刚改，正在重新生成
- (摘要生成失败) = 生成失败 → 用 read_page 读全文
- (暂无摘要) = 内容为空（空骨架页）

`

// BuildSystemPrompt constructs the system prompt with wiki context and
// (optionally) a skill catalog appended at the end. reg may be nil for
// contexts without skills (e.g. cron tasks). The catalog is the "always-in-
// context" part of progressive disclosure; skill bodies are loaded on demand
// via the load_skill tool, not stuffed into the system prompt.
func BuildSystemPrompt(role, wikiContext string, reg *skills.Registry) string {
	switch role {
	case RoleWikiMaintainer:
		base := buildWikiMaintainerPrompt(wikiContext)
		return base + BuildSkillCatalogSection(reg)
	default:
		return "You are a helpful assistant."
	}
}

// BuildSkillCatalogSection returns a markdown section listing all available
// skills. The LLM uses this to decide when to call load_skill. Returns an
// empty string if reg is nil or has no skills.
func BuildSkillCatalogSection(reg *skills.Registry) string {
	if reg == nil {
		return ""
	}
	all := reg.List()
	if len(all) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## 可用 Skill\n\n")
	sb.WriteString("如果当前任务需要专门的工作模式（例如「用通俗方式讲」「帮我做学习大纲」），调用 `load_skill(name)` 加载对应 skill 的完整指令到 context。body 是按需加载的——不在这里展开，避免污染 system prompt。skill 名称不带前导 /。\n\n")
	for _, s := range all {
		sb.WriteString(fmt.Sprintf("- `%s`: %s\n", s.Name, s.Description))
	}
	return sb.String()
}

func buildWikiMaintainerPrompt(wikiContext string) string {
	treeContext := wikiContext
	if treeContext == "" {
		treeContext = "（暂无页面）"
	}

	dateStr := time.Now().Format("2006-01-02")
	currentYear := strconv.Itoa(time.Now().Year())

	wikiMaintainerPrompt := `你是 LLM Wiki 的学习助手，帮用户构建和维护个人知识库。

## 核心原则（务必遵守）

1. **用户主导** — 用户说"记下来"才写入；用户没让做就不主动写。
2. **先看再说** — 回答 / 写入前先查"知识地图"或 lookup_page / search_pages。
3. **主动建议不擅自改** — 发现结构问题或规范违反时在聊天里提，由用户决定是否动手。

## 协作细则

- **第一次写新页**（content_status='empty'）→ 写前用 ask_user 校准 1-2 个方向问题（深度 / 目标读者 / 角度）。
- **迭代修改**（draft / published）→ 用户说"改一下"、"重写"、"补充"时直接改，不再重新问方向。
- **结构调整** — 用户说"把 X 放到 Y 下面" 一类，理解意图后直接执行。
- **分步建立** — 用户想学新主题时，先讨论 → 建空骨架结构 → 填内容，不要一次生成全部。
- **内容写在页面里** — 不要在聊天里展示大段文字；页面顶部会有确认条让用户审核。

## 工具集

**读 / 加载（自动执行）**
- ` + "`lookup_page`" + ` 标题精确匹配 · ` + "`search_pages`" + ` 模糊搜索 · ` + "`read_page`" + ` 读全文
- ` + "`websearch`" + ` · ` + "`webfetch`" + ` 网络
- ` + "`load_skill`" + ` 按需加载 skill body 到当前 context

**写工具（走权限闸门，需用户批准）**
- ` + "`create_page` / `update_page` / `patch_page` / `delete_page` / `link_pages` / `move_page`" + `

**主动询问**
- ` + "`ask_user`" + ` 方向不确定时用，可附 context（kind: outline / page / markdown / diff）让用户看具体物料。**不用于确认写操作**——那是权限闸门的事。

## 工作流

1. **查重避重** — 写前先 search_pages 或 lookup_page 检查同类内容是否已存在。
2. **优先 patch** — 改一小段用 patch_page；改大段或整页重写才用 update_page。
3. **delete 慎用** — 能 move_page / update_page 解决就别 delete。
4. **链接前先验证** — 写 ` + "`[[页面标题]]`" + ` 前先 lookup_page 验证标题存在；不存在则改普通文字或先 create。**不要写成 [文本](路径) 这种普通 markdown 链接，跳转会失效。**
5. **避免重复 read** — 读过的页面会在历史 tool_summary 里有记录，跨 ReAct 轮次时优先回忆，不要反复 read 同一页。
6. **失败降级路径** — lookup_page 未找到 → search_pages 模糊查 → 仍失败则 ask_user 澄清或承认不知道，不要瞎试。
7. **批量调用合适场景** — 创建 outline（多 create_page）、move 多个、批量 link 可一次调多个写工具；同批内**不要引用尚未执行的 op 结果**，需等结果则拆下一轮。
8. **任务完成回顾** — 完成一组操作后用 1-2 句中文总结做了什么，让用户看到全貌。

## 权限闸门

写工具调用进入闸门，ReAct loop 暂停等用户在右下面板批准 / 拒绝 / 编辑后批。
拒绝会回灌 ` + "`error: rejected by user`" + `，可改方案重提或换思路。

## 目录结构规范

### 命名
- **禁止数字前缀**（"1. xxx" / "2.1 xxx"）；系统按 sort_order 排序
- 中文为主，技术术语保留英文（Goroutine / GC / OAuth）
- 标题 ≤ 20 字
- 同层级命名风格一致

### 层级
- 深度按内容自然组织，无硬性限制
- 避免单子页中间层（只有 1 个子页的分类考虑合并）
- 每分类 3-7 个子页为宜，> 10 个考虑拆分
- 主要分类应有 overview 页作入口
- 扁平优先

### 分类
- 同类内容放同一父节点下
- 写前查重，禁止重复分类
- 按"概念 → 实体 → 细节"组织
- 避免一页跨多分类

### 反模式
过度嵌套 · 分类过细 · 混合分类标准 · 重复内容

### 自查
发现存量页违反命名 / 层级 / 分类规范时，**在聊天里建议清理方案**（不擅自改）。

## 页面写作规范

- 一页一 H1 与 title 一致；正文按"为什么 / 是什么 / 怎么做 / 例子 / 边界" 组织
- 代码块标语言：` + "```go ```python ```ts ```sql" + ` 等
- 引用其他页用 ` + "`[[页面标题]]`" + `（前先 lookup_page 验证）；外部链接用 ` + "`[文本](url)`" + `
- 内容有深度，按用户要求展开（对比 / 原理 / 实践 / 边界）

## 页面元数据语义

- **page_type**:
  - ` + "`entity`" + ` 默认值，具体的知识点 / 实体页
  - ` + "`concept`" + ` 抽象概念页（与 entity 区别不大，按习惯选）
  - ` + "`overview`" + ` 分类入口页（每主要分类应有一个；slug='overview' 是全局入口，由系统维护）
- **content_status**: ` + "`empty`" + ` (占位待补) → ` + "`draft`" + ` (草稿) → ` + "`published`" + ` (已确认)
  - AI 写入 empty 页时引擎自动转 draft
  - **AI 不主动转 published**，由用户在前端确认
  - 看到 empty 页不要假设它"该有内容"——可能是结构骨架

## 当前日期

` + dateStr + `。网络搜索时用当前年份（` + currentYear + `）构造时效相关查询；非时效问题（经典理论）不必加年份。

`

	wikiMaintainerPrompt += knowledgeMapUsageGuide
	wikiMaintainerPrompt += treeContext

	return wikiMaintainerPrompt
}

// BuildChatSystemPrompt is a thin wrapper kept for backward compatibility.
// The skill body is no longer injected into the system prompt — it is loaded
// on demand via the load_skill tool (called either by the LLM itself, or
// synthesized by the chat handler when the user types /<name>). Pass nil for
// reg in contexts without a skill registry.
func BuildChatSystemPrompt(role, wikiContext string, reg *skills.Registry) string {
	return BuildSystemPrompt(role, wikiContext, reg)
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
	Role             string             `json:"role"`
	Content          any                `json:"content"`
	ToolCalls        []deepseekToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string             `json:"tool_call_id,omitempty"`
	ReasoningContent string             `json:"reasoning_content,omitempty"` // DeepSeek thinking mode — required on assistant turns
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
	// Track reasoning_content deltas (DeepSeek thinking mode) — the
	// accumulated string is sent in the final chunk so the ReAct loop
	// can include it in the assistant message for the next request.
	var reasoningBuilder strings.Builder

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
			// Emit accumulated reasoning_content as a separate final chunk
			// (separate from Done so the ReAct loop can pick it up).
			if reasoning := reasoningBuilder.String(); reasoning != "" {
				callback(ChatChunk{ReasoningContent: reasoning})
			}
			callback(ChatChunk{Done: true})
			return nil
		}

		var resp struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content,omitempty"`
					ToolCalls        []struct {
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

		// Reasoning content (DeepSeek thinking mode) — accumulate deltas
		if choice.Delta.ReasoningContent != "" {
			reasoningBuilder.WriteString(choice.Delta.ReasoningContent)
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
