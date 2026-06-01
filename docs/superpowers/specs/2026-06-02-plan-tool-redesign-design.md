# Plan 工具重设计 — 复刻 Claude Code 权限模型

## 动机

当前 `propose_plan` 是一个"超级工具":6 种 action 类型、3 阶段(main / outline / content)互斥、1 / 30 / 2 限额、`{{action:X.field}}` 占位符 + `depends_on` 依赖图——把这些揉在一个 schema 里,系统 prompt 用大量 MUST / MUST NOT 教 AI 怎么用,后端还要做 `inferPlanStage` 推断+执行时再次 stage 校验,落库前/落库后双重防御。

**问题**:

1. **工具职责错位** — `propose_plan` 同时承担"提议方案"和"待用户批准"两件事,跟 AI 实际工作模式(读→想→问→写)对不齐
2. **契约过载** — 阶段互斥、限额、占位符、depends_on 一堆规则,AI 经常踩坑(校验失败错误又不回灌给 AI,踩完坑下次继续踩)
3. **不能迭代** — 用户批完一个 plan 后,所有 action 一次性执行,中途不能改主意;要改得 reject 整批重来
4. **用户没机会在执行前"看一眼"** — AI 想"我准备这样改"的时候,只能贴两段 markdown 让用户肉眼对比,没有 diff 视图

**类比**:Claude Code 把"动作"和"许可"解耦——工具是动作,permission gate 是执行前的闸,`AskUserQuestion` 是 AI 主动想澄清时调的工具。wiki 系统应该套用这个模型。

## 设计目标

- AI 调用原子写工具,每个调用走统一的权限闸门,一次响应里的多个写调用批量化让用户整体决定
- `ask_user` 工具承载"边看边问"的场景,4 种 context kind(outline / page / markdown / diff)
- 撤掉所有"防 AI 偷懒"的间接约束(阶段推断、限额、占位符),改用系统 prompt 直白地教
- 删 `plans` / `plan_actions` 表,撤 `propose_plan` 工具

## 非目标

- 不做多端实时协同(单机单用户,手机/桌面同步靠刷新,SSE 重连会重放事件)
- 不做精细的"每个 op 类型一套表单"的 nice 版 inline 编辑(只做 JSON 编辑器够用)
- 不做权限粒度到"按页面配置谁能改"
- 不迁移历史 plan 数据(用户只有一个,旧 pending 直接 drop)

## 核心改动概述

1. `propose_plan` 工具退役,`WikiTools()` 拆成 12 个工具:5 读 + 6 写 + 1 ask_user
2. 写工具走新的权限闸门:一次 LLM 响应里的所有写调用聚合成一个 `permission_required` 事件,用户在右下面板批/拒/编辑后批
3. `ask_user` 工具用于"方向不确定时问用户",4 种 context kind 渲染在右下面板
4. 撤 `plans` / `plan_actions` 表、`PlanHandler`、stage 推断/限额引擎
5. ReAct loop 改造,不再有 terminal iteration,所有 tool_call 按"读/写/问"三类分发
6. 前端右下面板改成两块独立卡片:`PermissionQueue`(待批准) + `AskUserContext`(AI 想让你看的东西)

## 工具集

### 读工具(自动执行,5 个)— schema 不变

- `lookup_page(title)` — 查页面元数据
- `read_page(page_id)` — 读完整 markdown
- `search_pages(query)` — 全文搜索
- `websearch(query, max_results?)` — 联网搜
- `webfetch(url)` — 抓网页

### 写工具(走权限闸门,6 个)— schema 扁平化

```typescript
create_page(title, parent_id?, content?, page_type?, slug?)
update_page(page_id, content, title?)
patch_page(page_id, operations: [{type: "replace"|"append", target?, content}])
delete_page(page_id)
link_pages(source_page_id, target_page_id, link_text?)
move_page(page_id, new_parent_id)
```

不再有嵌套的 `*_params` 分支,所有参数平铺在 schema 根上。

### 问工具(走单独 chan,1 个)— 新增

```typescript
ask_user(
  question: string,
  options: string[],            // 2-4 项
  context?: {
    kind: "outline" | "page" | "markdown" | "diff",
    data: ...                   // 见下
  },
  multi_select?: boolean,       // 默认 false
  allow_free_text?: boolean,    // 默认 true
  header?: string              // max 12 chars
)
```

`context.data` 形状:

| `kind` | `data` |
|---|---|
| `outline` | `OutlineNode[]` — 递归树 `{id, title, page_type, children}` |
| `page` | `{ page_id: number }` |
| `markdown` | `string` |
| `diff` | `Array<{ page_id, before, after, label? }>` |

`ask_user` 一次只能问一个问题(不像 Claude Code 的 `AskUserQuestion` 一次最多 4 个)。`before` 让 AI 自己带(它必须先 `read_page` 拿到,工具调用自包含)。

### 退役

- `propose_plan`(整个工具)
- 字段:`outline` / `phases` / `phase_index` / `total_phases` / `calibration_question`
- 概念:三阶段、stage 互斥、1 / 30 / 2 限额、`{{action:X.field}}` 占位符、`depends_on` 依赖图

## 权限闸门流程

### 一次写工具调用的链路

```
LLM 返回 tool_call (create_page / update_page / ...)
  ↓
后端识别为写工具,加入本批 writeBatch
  ↓
整批收集完,发 SSE: permission_required
  ↓
ReAct loop 阻塞在 chanPermission[requestID] 上
  ↓
前端右下面板渲染 PermissionQueue 卡片组(每条带复选框 + preview 摘要)
  ↓
用户操作(批 / 拒 / 编辑后批 / 一键全选 / 一键全拒)
  ↓
前端 POST /api/ai/permission_response { request_id, decisions: [...] }
  ↓
后端从 chan 拿到 decisions,按顺序执行已批准项
  ↓
每条执行完发 SSE: tool_result(success 或 error)
  ↓
失败时(用户拒 / 后端抛错)回灌 tool_result 给 LLM,ReAct loop 继续
  ↓
批全部处理完,继续下一轮 LLM 调用
```

### 关键语义

- **批量聚合**:一次 LLM 响应里 N 个写调用 → 1 个 `permission_required` 事件,N 个 item 同卡片组展示
- **同批不能链式**:写调用不能引用本批其他 op 的结果(如 `create_page(parent_id=<前面 create 的 page_id>)`)。后端拒绝这种 input,error 引导 AI 拆到下一轮
- **无服务端硬超时**:ReAct loop 一直阻塞,不设 deadline。SSE 流断开(SSE 断连回调)时清空当前 conversation 的 pending chan,应用用户配置默认动作
- **默认动作**:用户设置 `settings.user.default_pending_action` ∈ {`reject_all`(默认), `approve_all`, `keep_pending`}
- **拒绝回灌**:任何非批准路径(用户拒 / 编辑后拒 / 跳过 / 断连 / 超时兜底)统一回灌 `tool_result` with `error`:
  - `"rejected by user"`
  - `"user session ended before responding"`
  - `"auto-rejected after N hours of inactivity"`(如果开了 inactivity 兜底)
- **执行顺序**:已批准按 LLM 调用顺序串行。失败后续不再继续(快速失败),但失败的 tool_result 仍回灌给 LLM

## ask_user 流程

### 一次 ask_user 调用的链路

```
LLM 返回 tool_call: ask_user(question, options, context)
  ↓
发 SSE: ask_user_request (含 question / options / context)
  ↓
ReAct loop 阻塞在 chanAskUser[requestID] 上
  ↓
chat 区域渲染 ask_user 卡片(问题 + 选项 + free text 输入)
right panel 渲染 context 卡片(若有 context)
  ↓
用户操作(点选项 / 输入 free text / 跳过)
  ↓
前端 POST /api/ai/ask_user_response { request_id, answer }
  ↓
后端从 chan 拿 answer,封装成 tool_result 格式
  ↓
发 SSE: tool_result(answer JSON)
  ↓
ReAct loop 继续
```

### tool_result 格式

```json
{ "answer": "selected option text" }                     // 单选
{ "answer": ["opt A", "opt C"] }                         // multi_select
{ "answer": "free text from user" }                      // 自由文本
{ "answer": "no_answer" }                                // 用户跳过 / 断连
```

### ask_user 与权限闸门的关系

| 维度 | `ask_user` | 写工具权限闸门 |
|---|---|---|
| 触发原因 | AI 不知道答案 | AI 知道要做啥 |
| UI 位置 | chat(问题) + right panel(context) | right panel(待批准) |
| 响应内容 | 答案文本 | 批准 / 拒绝 / 编辑后批准 |
| 触发频率 | 应该少(关键决策) | 每次写都触发 |
| 阻塞 chan | `chanAskUser` | `chanPermission` |

`ask_user` 不用于确认写操作,权限闸门不用于问问题。系统 prompt 要明示。

## ReAct loop 改造

### 新结构(伪代码)

```go
reactLoop:
for iteration := 0; iteration < maxIterations; iteration++ {
    // 1. LLM 流式调用
    streamCh, _ := provider.StreamChat(ctx, chatReq)
    var respContent string
    var respToolCalls []ai.ToolCall
    for chunk := range streamCh {
        if chunk.Content != "" { SSE "content" }
        if chunk.ToolCall != nil { respToolCalls = append(respToolCalls, *chunk.ToolCall) }
    }
    
    // 2. 无 tool_calls → AI 推理完毕,落库 assistant 消息,break
    if len(respToolCalls) == 0 {
        saveAssistantMessage(respContent, nil)
        break
    }
    
    // 3. assistant turn 入 aiMessages
    appendAssistantTurn(aiMessages, respContent, respToolCalls)
    
    // 4. 分类:readBatch / writeBatch / askUserBatch
    readBatch, writeBatch, askUserBatch := classify(respToolCalls)
    
    // 5. 执行 readBatch(自动)
    for _, tc := range readBatch {
        SSE "tool_call_start"
        result := h.executeReadTool(ctx, tc)
        SSE "tool_result"
        aiMessages = append(aiMessages, toolResultMsg(tc.ID, result, ""))
    }
    
    // 6. 整批 writeBatch 走权限闸门
    if len(writeBatch) > 0 {
        SSE "tool_call_start" × len(writeBatch)
        SSE "permission_required" (整批)
        decisions := <-chanPermission[requestID]   // 阻塞
        for i, dec := range decisions {
            input := writeBatch[i].Input
            if dec.action == "edit" { input = dec.editedInput }
            if dec.action == "approve" || dec.action == "edit" {
                SSE "tool_result"
                result := h.executeWriteTool(ctx, writeBatch[i].Name, input)
                aiMessages = append(aiMessages, toolResultMsg(writeBatch[i].ID, result, ""))
            } else {
                SSE "tool_result" with error
                aiMessages = append(aiMessages, toolResultMsg(writeBatch[i].ID, "", "rejected by user"))
            }
        }
    }
    
    // 7. askUserBatch(按出现顺序处理)
    for _, tc := range askUserBatch {
        SSE "tool_call_start"
        SSE "ask_user_request" (含 question / options / context)
        answer := <-chanAskUser[requestID]   // 阻塞
        SSE "tool_result" (answer JSON)
        aiMessages = append(aiMessages, toolResultMsg(tc.ID, answer, ""))
    }
}
```

### 落地细节

- **chan 表**:`chanPermission map[string]chan PermissionResponse`、`chanAskUser map[string]chan AskUserResponse`,key 是 request_id,用 `sync.Mutex` 保护
- **SSE 断连回调**:stream 关闭时清空当前 conversation 的所有 pending chan,触发"用户走了"分支
- **assistant 消息落库**:每 iter 结束存一次,支持断线恢复
- **focusPageID 行为保留**:`create_page` 缺 `parent_id` 时后端自动注入 `focusPageID`,并在 SSE 流里发一个 `tool_call_start` 修订事件,前端展示最终执行版本
- **错误传播**:工具执行失败 → `tool_result` 带 `error`,继续 loop;AI 永远能从 tool_result 看到发生了什么

### 取消的逻辑分支

- `createPlanFromToolCall` 整个函数
- `inferPlanStage` / `validatePersistedPlanStage` / `MaxMainStageCreatePages` / `MaxOutlineStageNodes` / `MaxContentStageMutations`
- `propose_plan` 工具定义
- 老的"terminal iteration"分支(L582-682)

## 持久化迁移

### 一次性 SQL migration

`backend/db/migrations/012_drop_plans_tables.sql`:

```sql
-- 单文件,部署时跑
DROP TABLE IF EXISTS plan_actions;
DROP TABLE IF EXISTS plans;
-- 索引自动跟着没
```

### Go 代码清理

- `model.Plan` / `model.PlanAction` 从 `models.go` 删
- `PlanHandler` 整个文件删
- `/api/plans/*` 路由从 chi router 拿掉
- `engine.go` 里 `inferPlanStage` / `validatePersistedPlanStage` / 三个 `Max*` 常量全删
- `propose_plan` 工具定义从 `WikiTools()` 拿掉

### 不动的数据

- `wiki_pages` / `wiki_log` — 真相之源
- `messages.tool_calls` — 历史的 `propose_plan` tool call 记录保留,前端按 `created_at` 切,新消息走新组件,老消息按原样渲染(标 legacy 角标可选)
- `conversations` — 不动

### 不做

- 不分 A/B 步部署
- 不加 `cancelled` 状态枚举(直接 drop,旧 pending 数据无所谓)
- 不留 rollback 路径(只我一个人用,本地 sqlite dump 自己留一份)

## 系统 prompt 改写

### 新结构(总长比旧版短 40%)

```
[角色定位] (1-2 句)
你是 wiki_maintainer,管理用户的个人知识树。读操作自动执行,写操作需要用户批准。

[知识地图使用]  ← 沿用,不动
(现有的 knowledgeMapUsageGuide 字符串)

[工具集总览] (一段,≤ 15 行)
- 读工具 5 个,自动执行
- 写工具 6 个,走权限闸门
- ask_user:方向不确定时主动问
- 没有 propose_plan;不要试图调用它

[工作流规则] (5-7 条,核心)
1. 写前先读:用 lookup_page / search_pages / read_page 了解上下文
2. 方向不确定 → 调 ask_user(context 传具体物料),不调 ask_user 来确认写操作
3. 写操作一次可调多个(同一批权限闸门),但不要在同批里引用尚未执行的 op 结果——需要等的话拆下一轮
4. 页面内用 [页面标题] 语法做链接
5. 用户没让你做 → 不主动写
6. delete_page 慎用:能 move_page / update_page 解决的优先用那两个
7. 改大段用 update_page,改小段用 patch_page(避免重写整页)

[ask_user 用法] (一段)
context.kind 四种:outline / page / markdown / diff(数组)
2-4 个选项,默认单选 + 允许 free text
不用于确认写操作

[权限闸门行为] (一段)
写工具调用后 ReAct loop 暂停,等用户在右下面板批准 / 拒绝 / 编辑后批准
拒绝会回灌 error,你可以改方案重提

[内容质量] ← 沿用大部分,精简
[当前日期] ← 沿用
[知识地图 / 树上下文] ← 注入的运行时数据
```

### 关键删除

- 三阶段定义、阶段互斥
- `{{action:X.field}}` 占位符语法
- `depends_on` 声明规则
- "自检清单"
- `propose_plan` 整个章节
- `calibration_question` 字段说明
- "1-2 pages per plan" 限额

### 关键新增

- 权限闸门的存在和"会暂停等用户"行为
- ask_user 存在和"不用于确认写操作"边界
- "不要在同批里引用尚未执行的 op 结果"链式约束
- 写工具 [页面标题] 链接的提示

## 前端 UX

### 布局

```
┌─────────────────────────┬──────────────────────────┐
│  Chat                   │  Right panel             │
│                         │                          │
│  [user msg]             │  [Permission queue]      │
│  [AI text]              │  待批准 (3)              │
│  [tool card: read ✓]    │  □ create_page "Go 并发" │
│  [tool card: create ⏳]│  □ create_page "goro..." │
│                         │  □ link_pages A→B        │
│  ── ask_user card ──    │  [全选][全批][全拒]      │
│  你想侧重哪个方向?      │                          │
│  [底层原理][实际应用]   │  [Ask user context]      │
│  [对比 Python]          │  kind: outline           │
│  [free text:_____] [跳]│  ┌─ goroutine            │
│                         │  ├─ channel              │
│  [tool card: create ✓]│  └─ select                │
│  [AI text response]     │                          │
└─────────────────────────┴──────────────────────────┘
```

### Chat 区域

**ask_user 卡片**(新):
- 浅色背景 + 蓝色左边框(表示"等你回答")
- 标题:AI 的 `question` 字段
- 选项:按钮(可点击);`multi_select` 时复选框
- 下方 free text 输入框 + "发送"
- 右上"跳过"→ `no_answer`
- 用户回答后收起,显示"已回答: A" / "已跳过"

**tool call 卡片**(沿用现有):
- 写工具卡片新增 `pending` 状态(用户批前),半透明 + loading
- 批/拒后流转到 `done` / `error`

### Right panel 两块独立卡片

**Permission queue(顶)**:
- 标题:`待批准 (N)`
- 每条:复选框 + 工具名 + preview 摘要
- 底部:`全部批准` / `全部拒绝` / `全不选`
- 单条"编辑":inline 展开成 JSON textarea,改完"保存并批准"或"取消"
- 用户操作后该条立刻从 queue 消失
- 空状态:`当前没有待批准的操作`

**Ask user context(底,可选)**:
- 仅当 `ask_user.context` 非空时渲染
- 按 `kind` 渲染:outline 折叠树 / page 卡片 / markdown / diff(顶部 tab 切换,inline diff)
- 跟 chat 卡片联动:用户选完答案后收起
- 空状态:不渲染

### 编辑 inline 表单(简单版)

点"编辑"→ item 展开,raw input JSON 出现在 textarea,用户改完"保存并批准"。不做按 op 类型定制表单(只我一个人用,JSON 编辑器够用)。

## API 表面

### 新增 SSE 事件

| 事件 | 触发时机 | payload |
|---|---|---|
| `tool_call_start` | 每个 tool_call 即将执行 | `{id, name, input}` |
| `tool_result` | 每个 tool_call 执行完 | `{id, name, output, error}` |
| `permission_required` | 写工具批开始 | `{request_id, items: [{id, tool, input, preview}]}` |
| `permission_resolved` | 单条 op 被批/拒 | `{request_id, id, action}` |
| `ask_user_request` | ask_user 工具被调 | `{request_id, question, options, context, multi_select, allow_free_text, header}` |
| `ask_user_resolved` | 用户回答 | `{request_id, answer}` |

### 新增 HTTP 端点

| 方法 | 路径 | 用途 |
|---|---|---|
| `POST` | `/api/ai/permission_response` | 提交权限闸门 decisions |
| `POST` | `/api/ai/ask_user_response` | 提交 ask_user 答案 |

请求体:

```jsonc
// /api/ai/permission_response
{
  "request_id": "perm-123",
  "decisions": [
    { "id": "toolu_abc", "action": "approve" },
    { "id": "toolu_def", "action": "edit", "edited_input": {...} },
    { "id": "toolu_ghi", "action": "reject" }
  ]
}

// /api/ai/ask_user_response
{
  "request_id": "ask-456",
  "answer": "底层原理"   // 或 "no_answer"
}
```

### 删除 HTTP 端点

| 方法 | 路径 |
|---|---|
| `POST` | `/api/plans/confirm` |
| `POST` | `/api/plans/reject` |
| `POST` | `/api/plans` |
| `GET` | `/api/plans/{id}` |
| 其他 | `/api/plans/*` |

## 数据模型变化

```sql
-- 新 migration: 012_drop_plans_tables.sql
DROP TABLE IF EXISTS plan_actions;
DROP TABLE IF EXISTS plans;
```

`messages` 表不动。`conversations` / `wiki_pages` / `wiki_log` 不动。

## 涉及的组件和改动文件

| 文件 | 改动 |
|---|---|
| `backend/internal/ai/provider.go` | `WikiTools()` 重写:加 ask_user、扁平化写工具、删 propose_plan |
| `backend/internal/ai/provider.go` | `BuildSystemPrompt()` 改写:删阶段规则,加权限闸门 + ask_user 教学 |
| `backend/internal/handler/ai.go` | ReAct loop 改造:分类 dispatch + 权限闸门 + ask_user chan |
| `backend/internal/handler/ai.go` | 删 `createPlanFromToolCall` / `cleanProposalJSON` / 阶段相关 |
| `backend/internal/handler/plan.go` | **整个文件删** |
| `backend/internal/model/models.go` | 删 `Plan` / `PlanAction` 结构体 |
| `backend/internal/engine/engine.go` | 删 `inferPlanStage` / `validatePersistedPlanStage` / 三个 `Max*` 常量 |
| `backend/internal/engine/engine.go` | **无变化**(`ask_user` context 由 AI 工具调用自带,后端透传给前端) |
| `backend/cmd/server/main.go` | 删 `/api/plans/*` 路由;新增 `/api/ai/permission_response` `/api/ai/ask_user_response` |
| `backend/cmd/server/main.go` | 加载 `012_drop_plans_tables.sql` migration |
| `backend/db/migrations/012_drop_plans_tables.sql` | 新文件 |
| `frontend/src/components/PlanPreview.tsx` | **重写或退役** |
| `frontend/src/components/PermissionQueue.tsx` | **新文件** |
| `frontend/src/components/AskUserCard.tsx` | **新文件** |
| `frontend/src/components/AskUserContext.tsx` | **新文件**(含 outline / page / markdown / diff 四种渲染) |
| `frontend/src/components/ToolCallCard.tsx` | 加 `pending` 状态(写工具批前) |
| `frontend/src/hooks/useChatStream.ts` | 处理 `permission_required` / `ask_user_request` 事件,管 pending 状态 |
| `frontend/src/types/index.ts` | 删 `Plan` / `PlanAction` 类型;加新事件类型 |
| `openspec/specs/ai-propose-plan-contract/spec.md` | **ARCHIVED** |
| `openspec/specs/plan-execution-semantics/spec.md` | **ARCHIVED** |
| `openspec/specs/wiki-write-permission-gate/spec.md` | **新文件**(实施 spec,设计阶段先不写) |

## 不涉及改动的部分

- [x] Wiki 页面数据模型(`wiki_pages` / `wiki_log`)
- [x] 知识地图构建与渲染(现有的 `buildKnowledgeMap`)
- [x] 5 个读工具的 schema 和实现
- [x] SSE 流式传输框架(只新增事件类型)
- [x] AI provider 抽象(OpenCode Go / Claude 都不动)
- [x] 工具调用卡片基础组件(`tool-call-visibility` spec)

## 验收标准

### 功能

- [ ] `WikiTools()` 返回 12 个工具:`lookup_page` / `read_page` / `search_pages` / `websearch` / `webfetch` / `create_page` / `update_page` / `patch_page` / `delete_page` / `link_pages` / `move_page` / `ask_user`
- [ ] `propose_plan` 不再出现在工具列表
- [ ] 一次 LLM 响应里 N 个写调用 → 1 个 `permission_required` SSE 事件
- [ ] 用户批/拒/编辑后批,正确回灌 `tool_result` 给 LLM
- [ ] 拒绝路径统一 error 格式:`rejected by user` / `user session ended before responding`
- [ ] 同批链式引用被拒,error 信息明确
- [ ] `ask_user` 阻塞后正常恢复,answer 以 `tool_result` 格式回灌
- [ ] 4 种 `context.kind` 渲染正确(outline 折叠树 / page 预览 / markdown / diff tab 切换)
- [ ] `messages.tool_calls` 历史的 `propose_plan` 记录渲染不崩

### 持久化

- [ ] migration 跑完,`plans` / `plan_actions` 表已 drop
- [ ] 旧 pending plan 全部不见(直接 drop,不做 cancelled 标记)
- [ ] `/api/plans/*` 路由全部 404
- [ ] `model.Plan` / `model.PlanAction` Go 代码引用全清(grep 无残留)

### UX

- [ ] ask_user 卡片在 chat 显示,选项点击立刻发送响应
- [ ] Permission queue 在 right panel,全选/全批/全拒工作
- [ ] 编辑 inline JSON 后批准,后端用 edited_input 执行
- [ ] 浏览器关 tab → SSE 断开 → 后端应用用户默认动作

### 行为

- [ ] 写工具在批前在 chat 显示为 `pending` 状态卡片(半透明 + loading),批后才转 `done` / `error`;拒绝的写工具在 chat 显示为 `error` 状态卡片并带 reason
- [ ] LLM 看到拒绝 error 后能改方案重提(测一个 "想写 Go 并发但被拒" 的对话)
- [ ] `ask_user` 跳过时 `no_answer` 正确回灌
- [ ] 知识地图渲染、读工具、wiki 页面浏览完全不受影响

## 风险

- **删表不可逆**:本地 sqlite dump 自留一份;如需回滚,恢复 dump + 还原旧后端
- **同批链式约束靠 prompt**:LLM 不一定每次都遵守,后端要严格校验(已经设计了 reject + 明确 error)
- **SSE 断连检测**:`http.Flusher.CloseNotify()` 在不同 Go 版本 / 反代下行为不一,需实测;若不可靠,fallback 用客户端 30s 心跳
- **写工具 input 校验**:每个 op 类型独立校验(create_page 必填 title、update_page 必填 page_id + content 等),要把现在的 engine 校验逻辑搬到 tool handler 层
- **AI 行为迁移**:旧 prompt 教的"三阶段"在 AI 训练数据里没有,新 prompt 要写清楚"读 → 问 → 写"的节奏,系统 prompt 长度合理(~150-200 行)
