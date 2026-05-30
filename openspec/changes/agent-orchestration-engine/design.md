## Context

当前 `AIChat` 处理器在 `backend/internal/handler/ai.go` 中处理 AI 聊天的全部逻辑。每次用户消息触发一次非流式 AI 调用（`provider.Chat`），仅对 `lookup_page` 工具做自动执行后二次调用。其他工具调用（create/update/delete）需要用户通过独立的 `confirmed_actions` 请求确认。消息历史中工具调用以纯文本 `\n[操作建议]` 形式存储，而非结构化格式，导致后续 AI 调用无法正确解析工具调用上下文。

## Goals / Non-Goals

**Goals:**
- 后端 Agent Loop 引擎：最多 20 步自动推理，自动执行非写工具，结构化反馈
- 消息模型升级：`ai.Message` 支持 `tool_use`/`tool_result` 内容块
- 新增 `read_page`、`search_pages` 工具（自动执行）
- DB schema 扩展：`messages` 表加 `tool_call_id`、`tool_name` 列
- SSE 增加 `agent_status` 事件
- 前端步骤进度条 + 批量确认按钮
- 旧消息兼容

**Non-Goals:**
- 新增流式推理模式（Agent 循环仍用非流式 `provider.Chat`）
- 模型切换或混合提供者支持
- 消息编辑/重试功能
- 用户可见的 thinking/思考过程

## Decisions

### D1: Agent 循环使用非流式 Chat，而非 StreamChat
**选择**: 循环内部使用 `provider.Chat()`（非流式），整个循环结束后统一 SSE 输出。

**理由**:
- 当前架构中第一次调用就是非流式的（为了检测 lookup_page）
- 不流式内部循环避免复杂的状态管理（多轮 SSE 交错）
- 用户感受上仍然是"发一条消息等一回响应"

**放弃的方案**: 每步都用 StreamChat 实时推流。被放弃因为：
- SSE 连接可能超时（20 步可能耗时 30-60s）
- 需要精细的流合并逻辑

### D2: 消息历史使用结构化内容块
**选择**: `ai.Message` 扩展为支持两种模式：
- 旧模式：`Role + Content`（纯文本）
- 新模式：`Role + ContentBlock[]`（结构化数组，含 text/tool_use/tool_result）

DB 存储时将结构化块序列化为 JSON 字符串存入 `content` 字段，读取时根据格式自动反序列化。

**理由**:
- Claude API 原生支持 content block 数组
- DeepSeek 使用 tool role 消息
- 结构化格式让 AI 正确理解工具调用历史

### D3: SQLite 迁移使用 ALTER TABLE ADD COLUMN
**选择**: 直接使用 `ALTER TABLE messages ADD COLUMN`，不引入迁移框架。

**理由**: SQLite 支持 ADD COLUMN（非 NOT NULL 即可），零风险向后兼容。旧行新列的值为 NULL。

### D4: Agent 内部错误不中断前端 SSE
**选择**: 中间某步失败时发送错误事件，但不停止 SSE 流（已积累的文本和 pending_actions 正常发送）。

**理由**: 用户应看到"在第 X 步遇到了问题，但已完成的工作如下：..."，而不是突然断连。

### D5: 前端步骤进度条为纯展示，不参与交互
**选择**: 进度条只读，显示当前步骤/最大步数，不带暂停/取消按钮。

**理由**: 最小 MVP。后续可加取消功能。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| 某些场景下 20 步耗时长（10-30s），用户等待感明显 | 前端进度条展示步骤进度；考虑后续加 SSE 中间推流 |
| tool_result 内容较大（长页面全文），token 消耗增加 | 每次循环的完整消息历史包含累积的 tool_result，需关注 token 用量 |
| DeepSeek 的 tool role 消息格式与 Claude 不同 | 提供者各自处理序列化/反序列化，AIProvider 接口保持统一 |
| 旧消息中的 `\n[操作建议]\n` 文本被发送给 AI，可能干扰推理 | 加载旧消息时检测 content 中是否含结构化 JSON 前缀；仅向后兼容，不做清洗 |

## Migration Plan

1. **DB 迁移**: `ALTER TABLE messages ADD COLUMN tool_call_id TEXT; ALTER TABLE messages ADD COLUMN tool_name TEXT;`
2. **新消息写入**: 从 migration 完成后，新消息使用结构化格式存储
3. **旧消息**: 不做回填迁移，运行时按格式兼容

## Open Questions

1. `search_pages` 的搜索范围 — 只搜 title 还是 title + content 都搜？建议都搜（LIKE 或 FTS5），但 FTS5 需要额外配置。
2. 20 步是否足够？目前按保守值设，可后续通过配置调整。
