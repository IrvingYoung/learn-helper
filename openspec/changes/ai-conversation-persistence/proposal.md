## Why

AI 对话没有持久化记忆：每次请求只发送当前消息给 AI，历史消息虽存入数据库但从不加载。用户无法继续之前的对话，页面刷新后所有聊天记录丢失。作为面试学习助手，持续的对话上下文对深度学习至关重要。

## What Changes

- 新增会话管理 API：创建、列表、更新标题、删除会话，加载会话历史消息
- 后端 AI Chat 改造：conversation_id 必填，加载历史消息作为上下文发送给 AI Provider
- 前端 AIChatPanel 改造：顶部下拉式会话切换器，支持新建/切换/删除/重命名会话
- 前端会话持久化：页面刷新后恢复当前会话及历史消息
- 上下文窗口管理：滑动窗口策略（MVP 按条数，20 条上限）
- 修复 conversation_id 获取 bug（SQLite INSERT RETURNING）
- 数据库新增 conversations.role 列 migration

## Capabilities

### New Capabilities
- `conversation-management`: 会话的 CRUD 操作、历史消息加载、会话列表查询
- `conversation-context`: 后端为 AI 请求加载历史消息、滑动窗口上下文管理

### Modified Capabilities

## Impact

- **Backend API**: 新增 4 个端点（GET/POST/PATCH/DELETE conversations），修改 POST /api/ai/chat 逻辑（conversation_id 必填）
- **Backend handler/ai.go**: 主要改造文件，新增会话管理 handler，修改 Chat handler 加载历史
- **Backend repository**: 新增会话和消息的查询方法（sqlc 生成）
- **Frontend AIChatPanel.tsx**: 大幅改造，新增会话下拉选择器、历史加载逻辑
- **Frontend api.ts**: 新增会话相关 API 调用函数
- **Frontend types**: 新增 Conversation、Message 类型定义
- **Database**: 新增 migration — conversations 表增加 role 列
