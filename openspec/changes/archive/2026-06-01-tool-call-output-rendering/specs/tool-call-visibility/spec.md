# Spec: tool-call-visibility

AI 工具调用可见性——后端 SSE 事件 + 前端渲染组件

## Overview

当 AI 调用 auto-tool（websearch、webfetch、lookup_page、search_pages、read_page）时，系统通过 SSE 事件将调用过程和结果实时推送到前端，并在聊天界面中以可折叠卡片形式展示。工具调用记录随消息持久化，刷新后仍可见。

## Data Model

### Messages 表扩展

| 字段 | 类型 | 格式 | 说明 |
|------|------|------|------|
| `tool_calls` | TEXT | JSON | 工具调用记录数组 |

### tool_calls JSON 格式

```json
[
  {
    "id": "toolu_xxx",
    "name": "websearch",
    "input": {"query": "Go 并发 实践 最佳实践"},
    "output": "[系统] 工具 websearch 已执行完毕，返回了 xx 条结果",
    "error": ""
  }
]
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 工具调用唯一 ID（AI provider 生成或后端生成） |
| `name` | string | 工具名称 |
| `input` | object | 调用参数 |
| `output` | string | 执行结果文本 |
| `error` | string | 错误信息（空字符串表示成功） |

## Behavior

### ADDED Requirements

### Requirement: 后端在工具调用开始时发送 tool_call_start SSE 事件

当 AI 返回的工具调用即将执行时，后端 SHALL 在工具执行前向 SSE 流发送 `tool_call_start` 事件。

#### Scenario: websearch 工具调用开始
- **WHEN** AI 返回包含 `websearch` 的 tool_use
- **THEN** 后端在调用 Tavily API 前发送 SSE 事件 `event: tool_call_start`，data 包含工具 id、name、input
- **AND** SSE 事件格式为 `data: {"id":"toolu_xxx","name":"websearch","input":{"query":"xxx"}}`

#### Scenario: lookup_page 工具调用开始
- **WHEN** AI 返回包含 `lookup_page` 的 tool_use
- **THEN** 后端在查询数据库前发送 SSE 事件 `event: tool_call_start`
- **AND** data 包含工具 id、name、input（含 title 参数）

### Requirement: 后端在工具调用完成后发送 tool_result SSE 事件

当 auto-tool 执行完毕（成功或失败）后，后端 SHALL 向 SSE 流发送 `tool_result` 事件。

#### Scenario: 工具执行成功
- **WHEN** auto-tool 执行成功并返回结果
- **THEN** 后端发送 SSE 事件 `event: tool_result`
- **AND** data 包含工具 id、name、output（执行结果文本）、error（空字符串）

#### Scenario: 工具执行失败
- **WHEN** auto-tool 执行失败（如网络错误、数据库查询失败）
- **THEN** 后端发送 SSE 事件 `event: tool_result`
- **AND** data 包含工具 id、name、output 为空字符串、error 为错误描述

#### Scenario: 工具结果输出截断
- **WHEN** 工具返回的结果文本超过 8000 字符
- **THEN** 后端将 output 截断至 8000 字符，并在末尾添加 `...(截断)`

### Requirement: 前端接收并处理 tool_call_start / tool_result 事件

前端 SSE 客户端 SHALL 处理 `tool_call_start` 和 `tool_result` 两种新事件类型。

#### Scenario: SSE 客户端接收 tool_call_start
- **WHEN** SSE 流收到 `event: tool_call_start` 事件
- **THEN** 前端解析 data 中的 id、name、input
- **AND** 创建或更新工具调用 UI 状态（加载态）

#### Scenario: SSE 客户端接收 tool_result
- **WHEN** SSE 流收到 `event: tool_result` 事件
- **THEN** 前端解析 data 中的 id、name、output、error
- **AND** 更新对应 id 的工具调用 UI 状态（完成态）

#### Scenario: 未知事件类型
- **WHEN** SSE 流收到无法识别的事件类型
- **THEN** 前端 SHALL 忽略该事件，不影响其他事件的处理

### Requirement: 前端显示工具调用状态卡片

前端 SHALL 为每个工具调用渲染为独立的折叠式状态卡片，展示调用过程和结果。

#### Scenario: 工具调用加载态
- **WHEN** 工具调用开始（收到 tool_call_start）
- **THEN** 聊天界面显示工具调用卡片，包含工具名称、输入参数摘要、旋转加载指示器

#### Scenario: 工具调用完成态
- **WHEN** 工具调用完成（收到 tool_result）
- **THEN** 工具调用卡片更新为完成态，加载指示器消失
- **AND** 卡片显示工具名称、结果摘要（输出前 100 字符）
- **AND** 用户可点击展开查看完整输出

#### Scenario: 工具调用失败态
- **WHEN** 工具调用失败（tool_result 的 error 非空）
- **THEN** 工具调用卡片显示错误状态（红色标记）
- **AND** 卡片显示错误信息

#### Scenario: 多个工具调用渲染
- **WHEN** 同一次 AI 响应中包含多个工具调用
- **THEN** 每个工具调用独立渲染为一张卡片
- **AND** 卡片按调用发生的时间顺序排列
- **AND** 最新完成的工具调用默认展开，其余默认折叠

#### Scenario: 无工具调用的消息
- **WHEN** 消息没有关联工具调用（tool_calls 为空或 null）
- **THEN** 正常渲染文本内容，不展示工具调用区域

### Requirement: 工具调用记录持久化

系统 SHALL 将工具调用记录随消息持久化到数据库。

#### Scenario: SSE 流结束保存
- **WHEN** SSE 流结束（收到 done 事件）
- **THEN** 前端将当前消息的 tool_calls 数组发送到后端
- **AND** 后端将 tool_calls JSON 保存到 messages 表的 tool_calls 字段

#### Scenario: 页面刷新后恢复
- **WHEN** 用户刷新页面并打开同一会话
- **THEN** 前端从 API 加载消息时包含 tool_calls 字段
- **AND** 工具调用卡片按持久化数据恢复渲染

### Requirement: 工具调用卡片折叠/展开交互

工具调用卡片 SHALL 支持折叠和展开交互。

#### Scenario: 点击标题折叠/展开
- **WHEN** 用户点击工具调用卡片的标题区域
- **THEN** 卡片在折叠/展开之间切换

#### Scenario: 默认展开最新的
- **WHEN** 批量渲染消息中的多个工具调用
- **THEN** 每条消息中最新（最后完成）的工具调用卡片默认展开
- **AND** 其余卡片默认折叠
