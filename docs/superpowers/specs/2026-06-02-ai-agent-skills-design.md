# AI Agent Skill 系统

## 动机

AI 角色 `wiki_maintainer` 当前的 system prompt 是一段超长中文 markdown（`buildWikiMaintainerPrompt`，400+ 行），里面塞了协作方式、目录命名规范、工具使用细则等所有内容。每次想加一种"做事方式"（如"帮我把这个页面讲给小白听"、扫一遍本周新增页面做周回顾），都要硬改这段 prompt + 重启服务 + 改代码。

**问题**：
1. **能力无法增量添加** — 新场景 = 改 prompt 代码、不能并行、不能独立 review
2. **不能复用 Claude Code skill 生态** — 项目里已有 `.agents/skills/` 目录（给 Claude Code 自己用的 openspec 工作流），但 wiki agent 完全接不上
3. **没有用户可见的"快捷动作"概念** — 用户想用一个固定模式做事时，每次都要在聊天里把意图说一遍让 AI 凑

**类比**：Claude Code 用 `SKILL.md` 描述能力——frontmatter 给元信息、body 是在被触发时注入到 system prompt 的额外上下文。**渐进式披露**——不是所有 skill 的 body 都常驻 context，触发时才装载。我们的 `/命令` 就是触发机制。

## 设计目标

- skill 以 `SKILL.md` 形式存放在 `backend/internal/ai/skills/`，frontmatter + body，格式与 Claude Code skill 完全兼容
- 用户在聊天输入框用 `/<name> [args]` 显式触发，AI 在该次请求的 system prompt 末尾追加 skill body
- **不**给 AI 暴露"可用 skill 列表"——触发完全由人控制
- skill 不重定义工具、不重定义权限——继承 `WikiTools()` 和现有 permission 闸门
- cron 模式不开放 skill

## 非目标

- 不做 AI 自动挑选 skill（人控制）
- 不做 skill 内 `allowed-tools` 白名单（方案 B，v1 砍）
- 不做 skill 内的少样本示例（方案 C，v1 砍）
- 不做 hot reload（改 skill 文件需重启服务）
- 不做 skill 的 DB 存储（用户不能在 UI 加 skill，全部代码 + git 管理）
- 不在 cron 任务里支持 skill
- 不做 skill 之间的组合 / 链式调用

## 核心改动概述

1. 新增 `internal/ai/skills/` 包：`Skill` 结构体 + `Registry` + 启动时 loader
2. 用 `embed.FS` 把 skills 目录编进二进制
3. 新增 `GET /api/skills`（仅返回 `name` + `description`）
4. 扩展 `POST /api/ai/chat` 请求体：可选 `skill` 字段
5. 新增 `BuildChatSystemPrompt(role, wikiContext, *Skill)`，现有 `BuildSystemPrompt` 保留（cron 还在用）
6. 前端 ChatPanel 加 `/` 触发浮层，调 `GET /api/skills` 拉列表
7. 启动期 fail fast：缺字段 / name 重复 / 目录不存在 → 服务起不来

## 文件格式

### `backend/internal/ai/skills/<name>.md`

```markdown
---
name: explain-page
description: 解释当前页面的核心概念，用通俗语言给非专家讲清楚
license: MIT                  # 可选，原样保留，loader 不读
compatibility: human          # 可选，原样保留，loader 不读
metadata:                     # 可选，原样保留，loader 不读
  author: your-name
  version: "1.0"
---

你正在用「通俗解释」模式。用户会给你一个页面 ID 和可选的章节标题。

要求：
- 假设读者没有相关背景
- 用类比和例子，不堆术语
- 控制在 200 字以内
- 不修改页面内容
```

**frontmatter 字段**：
- `name`（必填）—— `/命令` 名，小写字母开头，仅含 `[a-z0-9-]`，全局唯一
- `description`（必填）—— 1-2 句中文，给前端 `/` 面板展示
- 其他字段（`license` / `compatibility` / `metadata` / 自定义 key）—— 原样存在 `Skill.Extra map[string]any` 里，loader 不读

**body**：自由 markdown，原样作为 system prompt 追加内容。可含 `##` 标题、code block、列表——这些是给 AI 看的指令，**不**给前端展示。

**兼容性**：本格式与 Claude Code skill 100% 对齐。skill 文件可以从本项目拷到 Claude Code（或反之），frontmatter 和 body 都不需要改。两边唯一的差别是触发方式（人 vs `/命令`），不是文件本身。

## 加载机制

### 包结构

```
backend/internal/ai/skills/
├── loader.go         # 扫描目录、解析 frontmatter、构建 Registry
├── loader_test.go
├── skill.go          # Skill 结构体 + Registry 定义
└── *.md              # skill 文件本体
```

### `Skill` 结构体

```go
type Skill struct {
    Name        string         `yaml:"name"`
    Description string         `yaml:"description"`
    Body        string         `yaml:"-"`
    Extra       map[string]any `yaml:",inline"` // 收容其他 frontmatter 字段
    SourcePath  string         `yaml:"-"`        // 启动时填，调试用
}
```

### `Registry`

```go
type Registry struct {
    mu     sync.RWMutex
    skills map[string]*Skill
    order  []string // 按文件名排序的 name 列表，给 List() 用
}

func (r *Registry) Get(name string) (*Skill, bool)
func (r *Registry) List() []*Skill
```

### 加载时机

- 服务启动时一次性扫 `internal/ai/skills/` 目录
- 解析方式：`embed.FS` 把目录编进二进制
- dev 模式可选：环境变量 `LH_SKILLS_DIR` 存在时改用 `os.DirFS` 加载（便于改完重启即生效；hot reload 仍不做）
- Registry 是单例，注入到 handler 层

### 启动期校验（fail fast）

| 情况 | 行为 |
|---|---|
| 目录不存在 | 启动报错，提示建出 `internal/ai/skills/` |
| 文件 frontmatter 缺 `name` | 启动报错，错误信息含文件路径 |
| 文件 frontmatter 缺 `description` | 启动报错，错误信息含文件路径 |
| 两个文件 `name` 相同 | 启动报错，列两条路径 |
| body 为空 | **不**报错，`Body == ""`，运行时拼到 prompt 仍是空串 |
| 未知 frontmatter 字段 | **不**报错，收进 `Extra` 字段 |

## API

### `GET /api/skills`

```json
[
  {"name": "explain-page", "description": "解释当前页面的核心概念，用通俗语言给非专家讲清楚"},
  {"name": "weekly-review", "description": "扫描本周新建/改动的页面，生成学习回顾"}
]
```

- 排序按 `name` 字典序
- **不**返回 `body` / `Extra` / `SourcePath`——内部数据，外部不暴露
- 无缓存（registry 启动时静态，handler 内存里直接拿）

### `POST /api/ai/chat`（扩展）

请求体新增可选字段：

```json
{
  "messages": [...],
  "skill": "explain-page"
}
```

- `skill` 缺省 = 不装载（与今天完全一致）
- `skill == ""` = 当 nil 处理
- `skill` 是未知名字 → 400：

```json
{
  "error": "unknown skill: xxx",
  "available": ["explain-page", "weekly-review"]
}
```

- `skill` 存在 → 当次请求 system prompt = base + skill body
- **不**修改 `messages`——`/命令 args` 走原 `messages` 字段；`args` 就是用户那条 message

### System prompt 拼装

```go
// ai package
func BuildChatSystemPrompt(role, wikiContext string, skill *skills.Skill) string {
    base := BuildSystemPrompt(role, wikiContext)
    if skill == nil {
        return base
    }
    return base + "\n\n## 当前 Skill: " + skill.Name + "\n\n" + skill.Body
}
```

`BuildSystemPrompt`（无 skill 参数版本）保留——cron runner 继续用它。

### 调用方改动

- `internal/handler/ai.go` 调 `BuildSystemPrompt` 处改为 `BuildChatSystemPrompt`，从请求体读 `skill`，查 `Registry`
- `internal/handler/cron` 不动——cron 不接 skill

## ReAct 集成

ReAct 循环（`internal/handler/ai_react.go`）**不**改逻辑：
- `ChatRequest.SystemPrompt` 字段已经存在（`models.go:70`）
- handler 在调 `RunReAct` 前拼好 prompt 即可
- skill body 在拼装阶段注入，ReAct 循环本身不感知 skill 概念

## 前端 `/` 面板

### 触发与交互

- 输入框为空时按 `/` → 弹浮层，列 `GET /api/skills` 返回项
- 输入 `/ex` → 浮层过滤到 `name` 前缀匹配 `ex` 的项（不区分大小写）
- 方向键选、回车确认 → 输入框填 `/<name> `，光标在末尾等用户输 args
- 浮层点 X 或按 ESC → 关闭，input 清掉 `/`
- 选中项展示：`name`（带 `/` 前缀）+ `description`

### 发送逻辑

- 前端发请求前解析输入：
  - 正则 `^/([a-z][a-z0-9-]*)(\s+(.*))?$`
  - 命中 → 拆 `skill` 和 `args`
  - `args` 为空时用 `""` 作为 user message
  - 请求体加 `skill: "<name>"`
- 不命中 → 当普通消息，**不**弹错

### 视觉（v1 最小版）

- 浮层绝对定位在输入框上方
- 半透明背景、单列、当前选中高亮
- 不做 fuzzy search、不做分组、不做最近使用

### 降级

- `GET /api/skills` 失败 → 静默不弹，`/` 当普通字符输入
- URL query 参数（`?skill=foo`）**不**支持

## 错误处理与边界

### 启动期

| 情况 | 行为 |
|---|---|
| skills 目录不存在 | 启动报错，提示建目录 |
| 任一文件 frontmatter 缺 `name` 或 `description` | 启动报错，列所有坏文件 |
| `name` 冲突 | 启动报错，列冲突的两条 |
| body 为空 | 允许，no-op skill |

### 请求期

| 情况 | 行为 |
|---|---|
| 未知 `skill` | 400 + `available` 列表 |
| `skill == ""` | 当 nil |
| 运行时改了 skill 文件 | 不感知，用启动时版本 |

### Cron 模式

- `BuildCronSystemPrompt` 签名不变
- cron runner 不读 `skill` 字段
- 即使 cron 任务里塞了 `skill` 也忽略

### 权限

- skill 不改写工具集、不改写 permission 闸门
- skill body 里写"不要写"是 prompt 约束，不是系统强制
- 硬约束留待方案 B（`allowed-tools`）

### Context 注入

- 触发了 skill 之后，base prompt 里的"知识库结构 / 全局标签 / 摘要降级"段依然在
- skill body 是 wiki context 之**上**的补充，不是替代

### 重复触发

- 同一会话连续 `/explain-page` → 每次都重拼，无残留
- 不做"已激活 skill 状态机"，保持 stateless

## 测试

### 单元（`internal/ai/skills/loader_test.go`）

- 正常加载：N 个 .md → registry 有 N 个 skill
- 缺 `name` → 报错，错误信息含文件路径
- 缺 `description` → 报错
- `name` 重复 → 报错，列两条
- body 含 `---`（frontmatter 边界）→ 正确切分
- 空 body → 不报错，`Body == ""`
- 未知字段（`license`, `metadata`）→ 不报错，`Extra` 里有
- 目录不存在 → 报错

### 集成（handler 层）

- `POST /api/ai/chat` 带 `skill: "explain-page"` → system prompt 含 skill body
- `POST /api/ai/chat` 不带 `skill` → 行为与今天完全一致（system prompt 字符串快照）
- `POST /api/ai/chat` 带未知 skill → 400 + `available`
- `GET /api/skills` → 列表按 name 排序，`body` / `Extra` / `SourcePath` 字段不出现

### 端到端

- 启动后输入 `/` → 浮层出现
- 选 `explain-page`、输 args、发送 → AI 行为符合 skill 定义
- 输入 `/nonsense` → 弹"未知 skill"

### 回归保护

- 留一个"空 skill"用例：chat 不带 skill 时，system prompt 与不含 skill 分支的字面量完全相等（防 skill 拼装逻辑偷偷改了 base 行为）

## 实施要点

### 改动文件清单

**新增**：
- `backend/internal/ai/skills/loader.go`
- `backend/internal/ai/skills/loader_test.go`
- `backend/internal/ai/skills/skill.go`
- `backend/internal/ai/skills/<至少一个示例>.md`（如 `explain-page.md`）
- `backend/internal/ai/skills/embed.go`（`//go:embed *.md`）

**修改**：
- `backend/internal/ai/provider.go` —— 新增 `BuildChatSystemPrompt`
- `backend/internal/handler/ai.go` —— 改 `BuildSystemPrompt` 调用点，读 `skill` 字段
- `backend/internal/handler/skills.go`（新）—— `GET /api/skills` handler
- `backend/cmd/server/main.go` —— 装配 `Registry`、注册路由
- `frontend/src/components/ChatPanel.*` —— `/` 面板
- `frontend/src/lib/api.ts` —— 调 `/api/skills`

**不改**：
- `internal/handler/ai_react.go`（ReAct 循环不感知 skill）
- `internal/handler/cron/*`（cron 模式不接 skill）
- `internal/ai/cron_prompt.go`（签名不变）
- `WikiTools()`（skill 不改工具集）

### 依赖

- `gopkg.in/yaml.v3`（如果项目里没有则 go get）
- 前端 0 依赖（已有 React 19 + SWR）

## 风险与权衡

| 风险 | 缓解 |
|---|---|
| skill body 太长撑爆 context | loader 不限制 body 长度，但 prompt 拼装处加注释提醒"控制在 1k 字以内" |
| skill body 跟 base prompt 矛盾 | 不解决——base + body 谁覆盖谁靠 prompt 自然的优先级（后追加的更近）；未来如果出问题，方案 B 的 `allowed-tools` 是真隔离 |
| dev 模式改 skill 文件需要重启 | 可接受，YAGNI；hot reload 留 future |
| 多个 skill 想组合 | 不支持，v1 一个 skill 一次 |
