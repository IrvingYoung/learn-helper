## ADDED Requirements

### Requirement: agent_status 事件
SSE 流 SHALL 提供 `event: agent_status` 事件类型，报告 Agent 循环的当前步骤状态。

#### Scenario: 发送 agent_status 事件
- **WHEN** Agent 循环开始或每步完成时
- **THEN** 系统发送 agent_status 事件，data 为 `{"step": 1, "max_steps": 20, "status": "running"}`

#### Scenario: 向后兼容
- **WHEN** 现有 content、meta、done 事件被发送
- **THEN** 其格式不变，agent_status 仅为新增事件类型

### Requirement: 事件顺序
Agent 循环的 SSE 事件流 MUST 按以下顺序发送。

#### Scenario: 完整事件流
- **WHEN** Agent 循环完整运行（含自动工具和写工具）
- **THEN** 事件顺序为：agent_status → content（多段）→ agent_status → ... → meta(pending_actions) → done

#### Scenario: 仅文本输出
- **WHEN** Agent 循环中 AI 未调用任何工具
- **THEN** 事件顺序为：agent_status → content → done

### Requirement: 前端 SSE 解析
前端 SSE 解析器 MUST 正确处理 `agent_status` 事件，并更新界面步骤指示器。

#### Scenario: 前端接收 agent_status
- **WHEN** 前端收到 `event: agent_status` 
- **THEN** 解析器读取 data 中的 step/max_steps 字段，更新 ChatPanel 中的进度条显示
