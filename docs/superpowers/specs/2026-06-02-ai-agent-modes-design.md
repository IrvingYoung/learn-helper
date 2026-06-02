# AI Agent 多模式设计

**日期**: 2026-06-02
**状态**: Spec 待评审
**作者**: brainstorming session（与用户协作产出）

## 1. 目标

在现有 `wiki_maintainer` 单一角色之上，引入 4 个面向不同学习任务的 AI 模式：

| 模式 | 角色定位 | 主要场景 |
|---|---|---|
| 维护 (maintainer) | 构建、整理、修改知识树 | 现状 —— 用户说"记下来"，AI 写页面、调结构 |
| 讲解 (explainer) | 教学、深入解释概念 | 用户问"X 是什么/为什么/怎么做"，AI 在聊天里讲清楚 |
| 调研 (researcher) | 联网查资料、汇总产出 | 用户说"帮我查一下 X"，AI 先 websearch 多轮，最后产出 draft 页面 |
| 复习 (quizzer) | 出题、考核、给反馈 | 用户说"考考我 X"，AI 用 ask_user 出题，按回答给反馈和下一题 |

## 2. 当前状态与限制

`internal/ai/provider.go` 中只有一个角色常量 `RoleWikiMaintainer = "wiki_maintainer"`，对应一个 system prompt 构造函数 `buildWikiMaintainerPrompt` 和一套工具 `WikiTools()`。`BuildSystemPrompt(role, wikiContext)` 是分发入口，目前只有 maintainer 一个分支。

`conversations.role` 列已存在（TEXT，无 CHECK 约束），每条会话存一个角色值。当前所有会话此列都是 `wiki_maintainer`。

cron 路径已有"角色变体"的成熟先例：`BuildCronSystemPrompt` 在 maintainer prompt 前加 autonomous prefix；`WikiToolsForCron` 在工具集上过滤 `ask_user`。两者共同验证了"通过子集工具 + 自定义 prompt 构造一个新模式"是行得通的。

限制：当前 AI 行为是单一的"知识库维护者"。用户想让 AI 讲解时，AI 仍然倾向于动手写；用户想被考核时，AI 不知道要主动出题。要让 AI 在不同任务下做不同的事，需要把"模式"建模为一等公民。

## 3. 设计方案

### 3.1 架构选型：模式作为 role

把 4 个模式建模为 4 个 role 常量，复用现有的 `conversations.role` 列和 `BuildSystemPrompt` 分发入口。每个 role 对应：

- 一份 `WikiToolsForXxx()` 函数（声明该模式可用的工具子集 —— **严格隔离**，物理不暴露不该有的工具）
- 一份 `buildXxxPrompt(wikiContext)` 函数（生成该模式的 system prompt）
- 一个显示名（中文，用于 UI 渲染）

不引入"mode"作为独立维度。在当前业务里 role 和 mode 是同一个东西："AI 现在扮演什么角色"。

### 3.2 角色常量

```go
// internal/ai/provider.go

const (
    RoleMaintainer = "maintainer"  // 新名，等价于老的 wiki_maintainer
    RoleExplainer  = "explainer"
    RoleResearcher = "researcher"
    RoleQuizzer    = "quizzer"

    // 向后兼容：老库里 conversations.role = 'wiki_maintainer' 仍然合法
    RoleWikiMaintainer = "wiki_maintainer"
)

var RoleDisplayNames = map[string]string{
    RoleMaintainer:     "维护",
    RoleWikiMaintainer: "维护", // 老值的显示名
    RoleExplainer:      "讲解",
    RoleResearcher:     "调研",
    RoleQuizzer:        "复习",
}
```

`BuildSystemPrompt` 把 `wiki_maintainer` 和 `maintainer` 路由到同一个分支：

```go
func BuildSystemPrompt(role string, wikiContext string) string {
    switch role {
    case RoleMaintainer, RoleWikiMaintainer:
        return buildMaintainerPrompt(wikiContext)
    case RoleExplainer:
        return buildExplainerPrompt(wikiContext)
    case RoleResearcher:
        return buildResearcherPrompt(wikiContext)
    case RoleQuizzer:
        return buildQuizzerPrompt(wikiContext)
    default:
        return "You are a helpful assistant."
    }
}
```

### 3.3 模式定义表

| 维度 | 维护 | 讲解 | 调研 | 复习 |
|---|---|---|---|---|
| **底色** | 现状 wiki_maintainer 保留全部 prompt | 知识库讲师，深入解释 | 主动外网调研，最后产出 draft | 出题人 + 评卷人 |
| **读工具** | lookup / read / search / websearch / webfetch | lookup / read / search / websearch / webfetch | lookup / read / search / **websearch / webfetch（重点）** | lookup / read / search |
| **写工具** | create / update / patch / delete / link / move | **无** | **仅 create_page / update_page** | **无** |
| **ask_user** | ✓ | ✓ | ✓ | ✓（**主要交互方式**） |
| **走权限闸门** | 是 | 是（不会触发，因没写工具） | 是（最后那次 create/update） | 是（不会触发） |
| **入口标识** | `RoleMaintainer` | `RoleExplainer` | `RoleResearcher` | `RoleQuizzer` |
| **显示名** | 维护 | 讲解 | 调研 | 复习 |

### 3.4 各模式 prompt 核心改写点

**维护模式**：不变，沿用现有 `buildWikiMaintainerPrompt` 全部内容，仅把函数重命名为 `buildMaintainerPrompt`。

**讲解模式**：

- 角色定位："你是 LLM Wiki 的讲师。用户已有知识库，你的职责是把里面的概念讲清楚。"
- 不主动写：不调用 create_page / update_page / patch_page，没有这些工具
- 引用知识库：用 `[[页面标题]]` 引用已有页
- 鼓励澄清：方向不确定时调 ask_user 问"你想懂哪个角度"
- 表达手法：举例 / 类比 / 拆解 / 画图（mermaid）
- 可建议"要不要记下来"，但只是文字建议；用户点头后让他自己切到维护模式

**调研模式**（双阶段流程）：

- 角色定位："你是调研员。用户给你一个待查的主题，你先用 websearch / webfetch 多轮收集，理出要点，再问用户是否要整理成 wiki 页。"
- 阶段一（默认）：lookup_page 查库内是否已有 → websearch 多轮 → 在聊天里给要点总结 → 调 ask_user 问"是否要整理成 wiki 页面（标题、父节点等）"
- 阶段二（用户确认后）：create_page 或 update_page 提交 draft
- 时效性强制：搜索必须用当前年份构造关键词
- 引用来源：要点段落要带 source URL

**复习模式**（卡片式出题）：

- 角色定位："你是出题人。基于知识地图选定一个页或一组相关页，连续向用户出题、评卷、给反馈。"
- 出题方式：每次调一次 ask_user，options 即题目选项（2-4 个）；多选题用 `multi_select=true`；问答题用 `allow_free_text=true`
- 难度梯度：从识记 → 理解 → 应用 → 综合，逐题递进
- 反馈：用户回答后下一轮先点评对错 + 解释，再出下一题
- 终止条件：用户说"够了 / 停 / 切模式"或答对 5 题以上
- 不写库：物理无写工具

### 3.5 共享 prompt fragment

下列内容在 4 个 build*Prompt 函数里复用（提取为独立小函数）：

- 知识地图段（buildKnowledgeMap 的输出，由 handler 注入到 wikiContext 字段）
- 知识地图使用指南（现有 `knowledgeMapUsageGuide` 常量）
- 当前日期 + 时效性提醒

### 3.6 模式切换：UI 选择器（无斜杠命令）

**新 endpoint**

```
POST /api/ai/conversations/:id/role
Body: { "role": "researcher" }
Response: { "ok": true, "role": "researcher" }
```

行为：

1. 校验 role ∈ {maintainer, explainer, researcher, quizzer}（不接受老的 wiki_maintainer 作为新写入值）
2. UPDATE `conversations SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
3. INSERT `messages (conversation_id, role, content) VALUES (?, 'system', '[已切换到: 调研]')`
4. 返回 ok

**conversation.role 是单一事实源**：所有 ReAct loop 入口（HTTP / cron）查这一列拿 role；切换后下一轮 chat 自动用新模式。

**create conversation 时设置初始模式**：`POST /api/ai/conversations` 已有 `role` 字段（默认 `wiki_maintainer`），改默认为 `maintainer`，并接受 4 个新 role 值。

## 4. 前端 UI

### 4.1 ModeSelector 组件

位置：ChatPanel 顶栏左上，会话标题前。

默认态：`[● 维护 ▾]`

- 圆点是模式色：维护=蓝、讲解=绿、调研=橙、复习=紫
- 中文显示名 + 下拉箭头

点击展开下拉面板：

```
┌─────────────────────────────────────────┐
│ ● 维护    构建知识树、写页面、整理结构    │
│ ● 讲解    深入解释，不主动写              │
│ ● 调研    联网查资料，最后产出 draft     │
│ ● 复习    出题测验，AI 给反馈            │
└─────────────────────────────────────────┘
```

选中后立刻 POST endpoint → SWR mutate 刷消息列表 → chip 颜色和文字立刻更新。

ReAct loop in-flight 时禁用切换按钮（loading 态），避免流式中途切换导致语义错乱。

### 4.2 输入框 placeholder

按模式切换文案：

- 维护："告诉 AI 你想加什么、改什么..."
- 讲解："让 AI 讲一个概念，或问问题..."
- 调研："给 AI 一个调研任务..."
- 复习："让 AI 选题给你考..."

### 4.3 系统消息样式

会话流里出现 `[已切换到: 调研]` 时，渲染为细灰色分隔线 + 居中文字，不占用 assistant/user 气泡样式：

```
─────── [已切换到: 调研] ───────
```

`GetConversationMessages`（`handler/ai.go:154`）当前 SELECT 已返回所有 role 的消息，前端不需要改后端 SELECT；只需 ChatPanel 渲染时按 role 分支识别 `system`。

**但** AI 历史加载循环（`handler/ai.go:387` 附近）会把 DB 里所有消息塞进 `aiMessages` 数组发给 LLM。如果 role='system' 的切换标记进了这数组，Claude / DeepSeek 接口的 `messages` 字段不接受 role=system（system 走独立的 SystemPrompt 字段），会报错。

**修复**：在 AI 历史加载循环里加一行过滤 —— `if role == "system" { continue }`。系统切换标记仅供前端 UI 渲染使用，不进 AI 上下文。

副作用：AI 看不到"我之前在维护模式做了 X"这个边界信息。这是有意的 —— 历史消息本身已带工具调用记录和 assistant 文本，足够 AI 推断上下文；引入"模式切换"作为 AI 认知概念反而会让它在边界处过度自我修正。

### 4.4 新会话表单

当前 NewConversation 表单只有标题输入。改成：标题 + Mode picker（同款下拉）。默认维护。

## 5. API 变更

| 端点 | 变更 |
|---|---|
| `POST /api/ai/conversations` | `role` 字段接受 4 个新 role 值（向后兼容老 `wiki_maintainer`） |
| `POST /api/ai/conversations/:id/role` | **新增**，切换 role + 写系统消息 |
| `GET /api/ai/conversations/:id/messages` | 行为不变，但前端 ChatPanel 需要识别 role='system' 分支渲染。**AI 历史加载循环（`ai.go:387` 附近）需新增过滤：跳过 role='system' 消息**（这些消息只供 UI，不进 LLM 上下文） |
| `POST /api/ai/chat` | 无签名变更；内部按 conversations.role 取对应工具集和 prompt |

## 6. 失败语义

| 场景 | 行为 |
|---|---|
| 前端切换模式但 endpoint 500 | toast 错误 + 模式不变（前端等 response 再 mutate） |
| 用户切到讲解模式后,AI 仍尝试调 create_page | 不可能 —— 工具集物理不暴露，provider 层拒绝；如真发生（bug）返 tool not found 错误，AI 见错改口 |
| 模式切换时 ReAct loop 正在跑 | 端点不并发于流；前端 chat in-flight 时禁用切换按钮 |
| 老会话 role='wiki_maintainer' | BuildSystemPrompt 把 'wiki_maintainer' 当 'maintainer' 处理；UI 显示"维护" |
| 调研模式 AI 最后产出 draft 时用户拒绝写入 | 现有权限闸门拒绝逻辑接住，AI 改方案重提 |
| POST /role 传非法值 | 400 + 错误消息列出合法值 |

## 7. 测试

### 7.1 Backend 单测

- `ai/prompts_test.go`
  - 每个 buildXxxPrompt 输出包含其模式标志关键词（explainer 含"不主动写"，researcher 含"websearch"，quizzer 含"出题"）
  - explainer prompt 不含 "create_page" 字串（确保不诱导写）
- `ai/tools_test.go`
  - WikiToolsForMaintainer 返回工具名集合 == 现有 WikiTools()
  - WikiToolsForExplainer 不含写工具名
  - WikiToolsForResearcher 包含 create_page、update_page，但不含 delete/move/patch/link
  - WikiToolsForQuizzer 不含写工具，含 ask_user
- `handler/ai_mode_test.go`
  - POST /conversations/:id/role 成功路径：DB 更新 + 系统消息插入
  - 不存在的 conversation id：404
  - 非法 role 值：400
- `ai/provider_test.go`
  - `BuildSystemPrompt("wiki_maintainer", ...)` == `BuildSystemPrompt("maintainer", ...)`
- `handler/ai_test.go`（如已有则扩展）
  - 历史加载循环过滤 role='system' 消息：插一条 system 消息进 DB，验证 chat 上下文里不出现

### 7.2 Frontend 手动 smoke

- 4 个模式各切一次，确认 chip 颜色和 placeholder 变
- 新会话表单默认值显示"维护"
- 系统消息分隔条渲染样式

不引入新 e2e 测试框架。

## 8. 迁移与向后兼容

- DB schema 不变；不 ALTER 表；不需要数据迁移脚本
- 老 `conversations.role = 'wiki_maintainer'` 数据保留原值，通过 BuildSystemPrompt 兼容映射工作
- cron 路径继续用 `RoleWikiMaintainer` 常量（runner.go 不动），cron 模式不受新增 role 影响
- `RoleWikiMaintainer` 常量保留为别名（不删除），避免破坏 cron 引用
- 前端 SWR cache key 不变；conversation 对象本来就有 role 字段

## 9. 不在范围（YAGNI）

- 模式自动检测：AI 不会自动建议切模式，全部由用户主动点
- 复习模式的状态存储：本期纯无状态出题，未来如做间隔重复再拆 quiz_records 表
- 模式间状态隔离：当前所有模式共享同一条 conversation 的历史消息；切模式不清空上下文，AI 看得到之前对话
- 模式权限差异：不引入"哪些模式只对某些 wiki 子树生效"之类的概念
- 新模式注册机制：4 个模式是写死的常量，不做插件化

## 10. 文件改动清单

**Backend 新增**：

- `internal/ai/prompts.go` —— 4 个 build*Prompt 函数 + 共享 fragment 函数
- `internal/ai/tools.go` —— WikiTools / WikiToolsForCron / WikiToolsForXxx 集中放这
- `internal/handler/ai_role.go` —— POST /conversations/:id/role 处理函数 + role 合法性校验
- 各文件配套 `_test.go`

**Backend 修改**：

- `internal/ai/provider.go` —— 添加新 role 常量；BuildSystemPrompt 扩展分发；老的 `WikiTools` 函数移到 tools.go 后在此保留薄包装/别名（cron 路径 `WikiToolsForCron` 仍引用 `WikiTools()`，避免破坏性改动）
- `internal/handler/ai.go` —— AI 历史加载循环加 `if role == "system" { continue }` 过滤
- `cmd/server/main.go` —— 在 `/api/ai` 路由组里注册 `POST /conversations/{id}/role`

**Frontend 新增**：

- `src/components/ModeSelector.tsx`

**Frontend 修改**：

- `src/components/ChatPanel.tsx` —— 顶栏挂 ModeSelector；输入框 placeholder 跟模式联动；新增系统消息渲染分支
- `src/types/conversation.ts` —— role 类型扩展

**DB / 数据迁移**：无
