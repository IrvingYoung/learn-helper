## ADDED Requirements

### Requirement: 工具分类
系统 SHALL 将工具分为两类：自动执行工具和待确认工具。

#### Scenario: 自动执行工具
- **WHEN** AI 调用 `lookup_page`、`read_page` 或 `search_pages`
- **THEN** 系统自动执行该工具，跳过用户确认环节，结果直接反馈给 Agent 循环

#### Scenario: 待确认工具
- **WHEN** AI 调用 `create_page`、`update_page` 或 `delete_page`
- **THEN** 系统不执行该工具，将其收集到 pending_actions 列表，Agent 循环结束后统一返回前端

### Requirement: read_page 工具
系统 SHALL 提供 `read_page` 工具，按页面 ID 读取页面完整内容。

#### Scenario: 成功读取页面
- **WHEN** AI 调用 read_page 传入有效的 page_id
- **THEN** 系统返回该页面的完整 Markdown 内容，包括 ID、标题、slug、内容和元数据

#### Scenario: 读取不存在的页面
- **WHEN** AI 调用 read_page 传入不存在的 page_id
- **THEN** 系统返回错误信息提示页面不存在

### Requirement: search_pages 工具
系统 SHALL 提供 `search_pages` 工具，在知识库中全文搜索页面标题和内容。

#### Scenario: 搜索到匹配页面
- **WHEN** AI 调用 search_pages 传入关键词
- **THEN** 系统返回匹配页面的列表，每条包含 ID、标题、slug、内容片段预览

#### Scenario: 无匹配结果
- **WHEN** AI 调用 search_pages 传入无匹配的关键词
- **THEN** 系统返回空列表

### Requirement: 批量确认执行
系统 SHALL 在 Agent 循环结束后统一发送 pending_actions，用户一次确认后批量执行所有写操作。

#### Scenario: 批量确认
- **WHEN** Agent 循环结束，pending_actions 中包含 2 个写操作
- **THEN** 前端显示"2 个待确认操作"和[全部确认]按钮

#### Scenario: 全部执行
- **WHEN** 用户点击[全部确认]
- **THEN** 后端按顺序执行所有 pending_actions，通过 SSE 返回执行完成消息

### Requirement: 写工具排序
批量执行时 create_page 操作必须先于依赖它的 update_page 操作。

#### Scenario: 保持创建顺序
- **WHEN** pending_actions 列表中同时包含 create_page 和后续的 update_page
- **THEN** 执行时保持原列表顺序不变
