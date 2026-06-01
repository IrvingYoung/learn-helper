## Context

当前 AI 聊天中，工具调用过程完全对用户不可见。AI 调用 `websearch`、`webfetch`、`lookup_page`、`search_pages`、`read_page` 等工具时，后端 ReAct 循环在 `executeAutoTool` 中静默执行，结果仅注入 `aiMessages` 上下文，前端只看到最终的文本响应。

### 当前 SSE 事件流

```
text streaming ... → done
```

### 目标 SSE 事件流

```
text streaming ... → tool_call_start → text streaming ... → tool_result → text streaming ... → done
```

## Goals / Non-Goals

### Goals

- 后端在工具调用开始和结束时发送 SSE 事件
- 前端实时显示工具调用状态（加载 → 完成）
- 工具调用结果可在聊天中折叠/展开查看
- 工具调用记录随消息持久化，刷新后仍可见

### Non-Goals

- 非工具调用的 agent 内部状态不暴露（如 ReAct 推理过程、prompt 组装）
- 不对 auto-tool 的执行结果做 UI 内操作（如重新执行、编辑参数）
- 不改变 `propose_plan` 工具的现有行为（它已有独立的 `meta` 事件流）

## Decisions

### 1. SSE 事件设计：`tool_call_start` + `tool_result`

**方案**：新增两个 SSE 事件，而非将工具调用嵌入现有 `content` 事件中。

- `event: tool_call_start` — `{"id": "toolu_xxx", "name": "websearch", "input": {...}}`
- `event: tool_result` — `{"id": "toolu_xxx", "name": "websearch", "output": "...", "error": ""}`

**理由**：工具调用是一个有独立生命周期的异步过程（start → 执行 → result），用两个事件表达比嵌入 content 更清晰。前端可以独立更新 UI 状态，不影响文本流渲染。

### 2. 前端状态管理：流式累加 + 消息绑定

**方案**：在 SSE 流式处理中维护一个工具调用映射表：
- 收到 `tool_call_start` → 插入到映射表，触发 UI 渲染（加载态）
- 收到 `tool_result` → 更新映射表对应条目，触发 UI 渲染（完成态）
- 流结束后，将工具调用列表绑定到当前消息的 `tool_calls` 字段

**理由**：工具调用与文本流是并发的（AI 可能在工具调用前后都有文本输出），单独管理比耦合到文本内容中更灵活。

### 3. ToolCallCard 组件：折叠式卡片

**方案**：每个工具调用渲染为一个折叠式卡片：
- **加载态**：显示工具名称 + 输入参数摘要 + 旋转加载指示器
- **完成态**：工具名称 + 结果摘要 + 可展开的详情区域（原始输出/错误）
- 交互：点击卡片标题展开/折叠详情，默认最新工具结果展开，历史结果折叠

**理由**：工具结果可能很长（完整网页内容），默认折叠保持聊天可读性，展开可查看详情。

### 4. 消息持久化：`tool_calls` JSON 字段

**方案**：在 `messages` 表中新增 `tool_calls TEXT` 列，存储 JSON 数组：
```json
[{"id": "toolu_xxx", "name": "websearch", "input": {...}, "output": "...", "error": ""}]
```

**理由**：无需建新表，读取时单次查询即可加载。工具调用与消息一一对应。

## Risks / Trade-offs

- **[Tool call 顺序] 工具调用可能与文本交错** → 前端 SSE handler 不假定顺序，独立处理每个事件，最终按时间线合并显示
- **[长输出] 工具结果可能极大** → 后端对 `tool_result` 的 output 做截断（如 5000 字符），前端也限制默认展开长度
- **[向后兼容] 旧消息没有 tool_calls** → 前端对 `tool_calls` 做空值处理，旧消息正常渲染不受影响
