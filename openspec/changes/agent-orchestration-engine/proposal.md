## Why

当前 AI 聊天仅支持单次调用：用户发送消息 → AI 响应（含工具建议）→ 用户手动确认 → 完成。缺少多步推理能力，AI 无法在执行工具后基于结果继续推理。这限制了 AI 作为知识库管理员的自主性——比如"帮我整理所有 React 相关页面"就需要先搜索、再读取、再决定更新哪些页面，目前需要用户多次交互才能完成。

## What Changes

- **后端 Agent Loop 引擎**：在 `AIChat` 处理器中实现最多 20 步的自动推理循环，每次 AI 调用后自动执行非写工具，将结构化结果反馈给 AI 继续推理
- **新增自动执行工具**：增加 `read_page`（按 ID 读取页面全文）和 `search_pages`（全文搜索标题和内容），自动执行无需确认
- **消息模型升级**：`ai.Message` 支持结构化 `tool_use`/`tool_result` 内容块，与 Claude 原生 API 格式对齐
- **数据库迁移**：`messages` 表增加 `tool_call_id` 和 `tool_name` 列
- **写操作批量确认**：Agent 循环中写工具暂存，结束后统一作为 `pending_actions` 返回，用户一键批量确认执行
- **SSE 事件扩展**：增加 `event: agent_status` 事件类型报告步骤进度（向后兼容）
- **前端轻量增强**：ChatPanel 增加步骤进度条（"步骤 3/20"），写操作合并为单个批量确认按钮
- **历史兼容**：旧格式消息不迁移，加载时自动兼容判断

## Capabilities

### New Capabilities
- `agent-loop`: 后端 Agent 自动推理循环引擎，支持最多 20 步连续工具调用与结果反馈
- `tool-execution`: 工具执行系统，支持自动执行（read/search）和待确认（create/update/delete）的分类管控
- `sse-streaming`: SSE 流式事件扩展，增加 agent_status 事件类型

### Modified Capabilities

（无 — 现有 spec（exercise-detail、topic-content）的 requirements 不变）

## Impact

| 影响范围 | 说明 |
|---------|------|
| `backend/internal/ai/models.go` | `Message` 结构体增加 ToolCallID、ToolName 字段 |
| `backend/internal/ai/provider.go` | 新增 read_page、search_pages 工具定义 |
| `backend/internal/ai/claude.go` | 支持 tool_result 序列化为结构化内容块 |
| `backend/internal/handler/ai.go` | 重写 AIChat 为核心 Agent Loop 逻辑 |
| DB `messages` 表 | 新增 tool_call_id、tool_name 列 |
| `frontend/src/components/ChatPanel.tsx` | 添加 Agent 进度条和批量确认按钮 |
| `frontend/src/utils/api.ts` | SSE 解析支持 agent_status 事件 |
