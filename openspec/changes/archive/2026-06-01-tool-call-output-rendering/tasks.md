## 1. Backend: 消息表增加 tool_calls 字段

- [x] 1.1 创建新 migration 文件 `007_add_tool_calls.sql`，在 `messages` 表增加 `tool_calls TEXT` 列
- [x] 1.2 更新 `backend/internal/model/models.go` 中 `Message` 结构体，增加 `ToolCalls sql.NullString`
- [x] 1.3 更新 `GetConversationMessages` 的 SQL 查询，包含 `tool_calls` 列
- [x] 1.4 更新 SSE 流结束后保存 assistant 消息的 INSERT，写入 `tool_calls` 字段

## 2. Backend: 工具调用 SSE 事件流

- [x] 2.1 在后端定义工具调用结果结构体（用于 SSE 序列化和消息表存储）
- [x] 2.2 在 `executeAutoTool` 调用前发送 `tool_call_start` SSE 事件（含 id、name、input 摘要）
- [x] 2.3 在 `executeAutoTool` 调用后发送 `tool_result` SSE 事件（含 id、name、output、error）
- [x] 2.4 在 plan 分支（line 557-562）和 auto-tools-only 分支（line 609-614）均加入上述 SSE 事件
- [x] 2.5 在 ReAct 循环中累积工具调用结果，最终保存到 `tool_calls` 列

## 3. Frontend: 类型和 SSE 事件处理

- [x] 3.1 在 `frontend/src/types/index.ts` 新增 `ToolCallInfo` 接口（含 id、name、input、output、error），扩展 `ConversationMessage.tool_calls`
- [x] 3.2 在 `frontend/src/lib/api.ts` 的 `streamChat` 函数中，增加 `tool_call_start` 和 `tool_result` 事件解析
- [x] 3.3 新增 `onToolCall` 回调参数，用于将工具调用状态传递给 ChatPanel

## 4. Frontend: ToolCallCard 组件

- [x] 4.1 创建 `frontend/src/components/ToolCallCard.tsx` 组件
- [x] 4.2 实现加载态：显示工具名称、参数摘要、旋转加载指示器
- [x] 4.3 实现完成态：显示工具名称、结果摘要、可展开详情区域
- [x] 4.4 实现错误态：显示错误信息（红色标记）
- [x] 4.5 实现展开/折叠交互：默认最新工具结果展开，历史结果折叠

## 5. Frontend: ChatPanel 集成

- [x] 5.1 在 `ChatPanel.tsx` 的 `streamChat` 调用中传入 `onToolCall` 回调
- [x] 5.2 在 ChatPanel 中维护活跃的工具调用列表状态（用于流式显示）
- [x] 5.3 在消息渲染循环中（`renderedMessages`），对包含 tool_calls 的消息渲染 ToolCallCard
- [x] 5.4 支持边流式边显示工具调用状态（非完整消息，流式期间的临时工具调用渲染）
