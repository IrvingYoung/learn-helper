## ADDED Requirements

### Requirement: Agent Loop Engine
后端 Agent Loop 引擎 SHALL 支持最多 20 步的连续自动推理，每次 AI 调用后解析工具调用并决定是否继续循环。

#### Scenario: 连续的自动工具调用
- **WHEN** AI 响应中包含可自动执行的工具调用（如 search_pages）
- **THEN** 系统自动执行该工具，将结构化 result 追加到消息历史，并继续调用 AI 进行下一步推理

#### Scenario: 达到最大步数
- **WHEN** Agent 循环达到第 20 步且仍有工具调用
- **THEN** 系统停止循环，输出已收集的 pending_actions 和文本内容，不抛出错误

#### Scenario: AI 无工具调用时结束循环
- **WHEN** AI 响应中既无 write 工具也无 read/search 工具
- **THEN** 系统结束 Agent 循环，输出最终文本内容

#### Scenario: 只剩下写工具时提前结束
- **WHEN** AI 本轮只调用了 write 工具（create/update/delete），无 read/search 调用
- **THEN** 系统收集这些 write 工具为 pending_actions，结束循环，不再继续调用 AI

#### Scenario: 循环内部错误处理
- **WHEN** 某次 AI 调用失败（网络错误、API 错误）
- **THEN** 系统通过 SSE 发送 error 事件，终止 Agent 循环，已收集的文本和 pending_actions 正常返回

### Requirement: 消息历史构建
Agent 循环向 AI 发送的消息历史 MUST 包含结构化 tool_use 和 tool_result 内容块，而非纯文本序列化。

#### Scenario: 构造 tool_use 内容块
- **WHEN** AI 的 assistant 消息包含工具调用
- **THEN** 该消息内容被解析为 `{type: "tool_use", id, name, input}` 结构体数组存储到 DB，并在后续请求中以此格式发送

#### Scenario: 构造 tool_result 内容块
- **WHEN** 自动工具执行完成
- **THEN** 系统创建 `{type: "tool_result", tool_use_id, content}` 消息追加到历史，而非纯文本描述

#### Scenario: 旧格式消息兼容
- **WHEN** 加载的历史消息中 content 不含结构化内容块
- **THEN** 系统将其作为纯文本 `{role, content}` 发送给 AI，不做转换
