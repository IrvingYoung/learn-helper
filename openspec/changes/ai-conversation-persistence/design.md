## Context

当前 AI 对话系统存在持久化缺失：后端将消息存入 conversations/messages 表，但每次 AI 请求只发送当前用户消息，不加载历史。前端 AIChatPanel 用 useState 管理消息，页面刷新即丢失。用户无法继续之前的对话，AI 也无法理解上下文引用（如"它的遍历方式"中的"它"）。

现有数据库 schema 已包含 conversations 和 messages 表，但 conversations 表缺少 role 列，需要新增 migration。后端 AIHandler 已支持 conversation_id 字段（但前端从未发送）。AIProvider 接口（Chat/StreamChat）接收 Message 数组，天然支持多轮对话。

## Goals / Non-Goals

**Goals:**
- 用户可手动创建、切换、删除 AI 会话
- AI 请求携带历史消息，实现多轮上下文对话
- 页面刷新后会话和消息不丢失
- 上下文窗口管理，防止 token 超限

**Non-Goals:**
- 会话自动绑定页面上下文（用户手动管理）
- 历史消息摘要/压缩（MVP 用滑动窗口即可）
- 会话搜索/标签功能
- 跨设备同步

## Decisions

### D1: 会话管理 UI — 顶部下拉选择器

**选择**: AIChatPanel 顶部放置下拉式会话选择器 + 新建/删除按钮

**替代方案**:
- 可折叠侧栏：空间占用大，窄面板体验差
- Tab 式：会话多时 tab 溢出

**理由**: 侧边面板空间有限，下拉选择器最紧凑，且用户同时只关注一个会话。

### D2: 上下文窗口策略 — 滑动窗口（MVP 仅按条数）

**选择**: 加载最近 N 条消息（N=20），MVP 仅按条数截断。Token 预算截断作为后续增强。

**替代方案**:
- 条数 + token 预算双重限制：MVP 阶段增加复杂度，且学习对话通常消息不会太长
- 摘要 + 窗口：MVP 过于复杂
- 不截断：小模型上下文有限，会报错

**理由**: 学习对话通常不会很长，20 条消息足够覆盖大多数场景。Token 预算截断留给后续迭代。

### D3: 会话与上下文的关系 — 创建时关联，不随页面切换

**选择**: 新建会话时关联 context_type/topic_id/exercise_id，但切换页面不影响当前会话

**理由**: 用户可能在不同页面继续同一个对话（如从知识点切换到练习题讨论）。手动管理意味着用户自己决定何时新建。

### D4: 前端会话状态持久化 — localStorage + API 混合

**选择**: localStorage 存储当前 conversation_id，页面加载时用 API 获取会话列表和消息历史

**替代方案**:
- 纯 localStorage 缓存消息：与后端数据不一致风险
- 纯 API 无本地缓存：每次打开面板都要网络请求

**理由**: localStorage 只存 ID（极小数据），消息历史从 API 加载（单一数据源），兼顾速度和一致性。

### D5: API 设计

**选择**:
```
GET    /api/ai/conversations              → 列出会话（含最后消息摘要、消息数）
POST   /api/ai/conversations              → 新建会话（role, context_type, topic_id?, exercise_id?, title?）
PATCH  /api/ai/conversations/:id          → 更新会话标题
GET    /api/ai/conversations/:id/messages → 获取会话消息历史
DELETE /api/ai/conversations/:id          → 删除会话
POST   /api/ai/chat                       → 现有端点，改造：conversation_id 必填，加载历史
```

**理由**: RESTful 风格，与现有 API 一致。会话列表端点返回摘要信息避免 N+1 查询。PATCH 用于重命名标题。POST /api/ai/chat 要求 conversation_id 必填，与手动管理模式一致。

### D6: SSE 事件格式 — 使用 event 字段区分

**选择**: 使用 SSE 的 `event:` 字段区分 meta 事件和内容 chunk：
- Meta 事件: `event: meta\ndata: {"conversation_id":1}\n\n`
- 内容 chunk: `data: <text>\n\n`（无 event 字段，保持向后兼容）

**替代方案**:
- 用 JSON 格式区分（检查 data 是否为 JSON）：脆弱，AI 可能输出 JSON 格式内容
- 在 [DONE] 事件后返回 meta：前端需要先显示内容再获取 ID，时序不对

**理由**: SSE 标准的 `event:` 字段就是为此设计的。前端用 `EventSource` 或手动解析时可以按 event name 分流，不会把 meta 数据拼到消息内容里。

### D7: 废弃无 conversation_id 自动创建会话

**选择**: POST /api/ai/chat 要求 conversation_id 必填。无 conversation_id 时返回 400 错误。

**替代方案**:
- 保留自动创建：与手动管理模式矛盾，用户无法控制会话生命周期

**理由**: 手动管理意味着用户必须先创建会话再发消息。自动创建会导致不可控的空会话堆积。

## Risks / Trade-offs

- **[滑动窗口丢失早期上下文]** → MVP 可接受。长对话中 AI 可能遗忘早期内容，但学习对话通常聚焦当前话题。后续可加摘要功能。
- **[会话列表性能]** → 单用户 SQLite，数据量小，无性能风险。
- **[SSE 流式响应中 conversation_id 返回]** → 使用 SSE event 字段区分 meta 和 content 事件（见 D6），前端按 event name 分流解析，不会误拼到消息内容。
- **[前端 AIChatPanel 改造范围大]** → 分步实施：先后端 API + 历史加载，再前端 UI 改造。
- **[POST /api/ai/chat 废弃自动创建]** → 前端需要先调用 POST /api/ai/conversations 创建会话，再发送消息。这改变了现有前端流程，需要同步改造。
