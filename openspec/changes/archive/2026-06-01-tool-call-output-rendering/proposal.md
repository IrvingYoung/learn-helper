## Why

当 AI 调用工具（lookup_page、websearch、webfetch 等）时，当前完全静默执行——用户看不到工具调用过程，也不知道 AI 在后台做了什么。这导致：(1) 长时间等待时用户不知道进度，(2) 工具的返回结果对用户不可见，(3) 对话历史中没有工具调用的记录，丢失了 AI 推理过程的透明度。需要将工具调用的执行过程和结果渲染到聊天界面中，让用户对 AI 的工作过程有清晰的可见性。

## What Changes

- **新增 SSE 事件 `tool_call_start` 和 `tool_result`**：后端在工具调用开始时发送 `tool_call_start`（含 tool ID、name、input），在工具执行完成后发送 `tool_result`（含 tool ID、name、output/error）
- **前端 SSE 客户端增加对新事件的处理**：`streamChat()` 增加 `tool_call_start` 和 `tool_result` 事件回调
- **前端新增 ToolCall 组件**：渲染工具调用状态——调用时显示加载动画（`正在搜索...`、`正在读取页面...`），完成后展示折叠式的结果卡片
- **前端 `ConversationMessage` 类型扩展**：支持 `tool_calls` 字段，存储该消息中关联的工具调用记录
- **消息持久化扩展**：更新 `messages` 表结构或 `content` 字段，持久化工具调用记录
- **工具结果折叠/展开交互**：用户可折叠/展开工具调用详情，保持聊天界面整洁

## Capabilities

### New Capabilities
- `tool-call-visibility`: AI 工具调用的可见性——后端 SSE 事件流 + 前端渲染组件

### Modified Capabilities

（无）

## Impact

- **Backend**: `backend/internal/handler/ai.go` — ReAct 循环中新增 SSE 事件 `tool_call_start` / `tool_result`
- **Frontend types**: `frontend/src/types/index.ts` — 新增 `ToolCallInfo` 接口，扩展 `ConversationMessage`
- **Frontend SSE client**: `frontend/src/api.ts` — 新增事件处理
- **Frontend component**: `frontend/src/components/` — 新增 `ToolCallCard.tsx` 组件
- **Frontend chat panel**: `frontend/src/components/ChatPanel.tsx` — 集成工具调用渲染
- **Schema**: `backend/internal/model/models.go` — 消息持久化扩展（工具调用信息）
