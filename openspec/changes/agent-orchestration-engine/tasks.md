## 1. 消息模型升级

- [x] 1.1 扩展 `ai.Message` 结构体增加 `ToolCallID`、`ToolName` 字段；新增 `ContentBlock` 类型支持 text/tool_use/tool_result
- [x] 1.2 DB migration：`ALTER TABLE messages ADD COLUMN tool_call_id TEXT; ADD COLUMN tool_name TEXT;`
- [x] 1.3 更新 `GetConversationMessages` handler 返回新字段
- [x] 1.4 实现旧格式消息兼容检测（content 是否 JSON 数组，否则按纯文本处理）
- [x] 1.5 更新 `ai.Message` 的序列化逻辑：新格式输出 content block 数组，旧格式保持纯文本

## 2. 新增自动执行工具

- [x] 2.1 在 `provider.go` 的 `WikiTools()` 中添加 `read_page` 工具定义（参数：page_id integer）
- [x] 2.2 在 `provider.go` 的 `WikiTools()` 中添加 `search_pages` 工具定义（参数：query string）
- [x] 2.3 在 `handler/ai.go` 中实现 `executeReadPage(ctx, tc)` 方法（按 ID 查询 wiki_pages，返回完整内容）
- [x] 2.4 在 `handler/ai.go` 中实现 `executeSearchPages(ctx, tc)` 方法（LIKE 搜索 title 和 content，返回匹配列表含片段）

## 3. 后端 Agent Loop 引擎

- [x] 3.1 新增 `runAgentLoop(ctx, provider, messages, tools, onEvent)` 函数，封装 20 步循环核心逻辑
- [x] 3.2 实现自动/写工具分类判断函数 `isWriteTool(name)`
- [x] 3.3 实现 tool_result 结构化消息追加逻辑（构建 `{type: "tool_result", tool_use_id, content}`）
- [x] 3.4 实现 tool_use 内容块提取：AI 响应中的 ToolCalls 转换为结构化 assistant content 数组
- [x] 3.5 实现写工具暂存逻辑（收集到 pendingActions，不停止循环，但循环不再继续调用 AI）
- [x] 3.6 循环终止条件：无工具调用、只剩写工具、达到 20 步、AI 调用错误
- [x] 3.7 Agent 循环 SSE 输出：累加的文本内容 + 最终 pending_actions + done

## 4. 重写 AIChat 处理器

- [x] 4.1 将 `AIChat` 中现有单次/二次调用逻辑替换为 `runAgentLoop` 调用
- [x] 4.2 保留 confirmed_actions 路径不变（用户确认后批量执行）
- [x] 4.3 保留 lookup_page、overview_page 等辅助函数
- [x] 4.4 消息保存逻辑适配新格式：assistant 消息 content 存为 JSON 数组，tool 消息写入 tool_call_id/tool_name
- [x] 4.5 SSE 事件发送：每步开始时发送 agent_status 事件

## 5. SSE 流扩展

- [x] 5.1 在 `sseWrite` 基础上确保 agent_status 事件的正确格式和 flush
- [x] 5.2 更新 `streamChat` 前端解析器识别 `event: agent_status` 行，提取 step/max_steps
- [x] 5.3 保持现有 content/meta/done 事件的格式不变

## 6. 前端 UI 增强

- [x] 6.1 ChatPanel 添加 `agentStatus` state（step/maxSteps/running/stepHistory）
- [x] 6.2 实现 Agent 步骤进度条组件（显示"步骤 3/20"）
- [x] 6.3 改进 pending_actions 展示：合并为"X 个待确认操作" + [全部确认] 按钮
- [x] 6.4 批量确认请求发送逻辑：将累积的 pending_actions 通过 confirmed_actions 发送到后端

## 7. 集成与验证

- [ ] 7.1 `openspec verify` 验证 artifacts 完整性
- [ ] 7.2 编译检查：`cd backend && go build ./...`
- [ ] 7.3 前后端集成测试：启动服务，发送一条要求多步推理的消息，验证 Agent 循环正常执行
- [ ] 7.4 测试边界：20 步上限、AI 错误、旧格式历史兼容
