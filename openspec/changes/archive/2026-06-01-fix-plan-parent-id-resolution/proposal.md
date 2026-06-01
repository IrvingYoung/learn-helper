## Why

`propose_plan` 在执行多 action 计划时，如果 `create_page` 动作的 `parent_id` 引用 `{{action:X.page_id}}` 但未声明 `depends_on`，engine 静默吞掉占位符解析失败，导致子页面写到根节点而不是挂在预期的父页面下（见用户构建 Go 学习体系时的实际脏数据）。同时系统提示中"先建主页 → 生成目录 → 逐页填内容"的标准流程缺乏强制约束，AI 可以一次塞十几个 create_page + 完整 Markdown，绕过设计。

## What Changes

- **engine 层**：让 `replacePlaceholders` 在解析失败时返回 error 而不是默默返回原占位符字符串
- **engine 层**：让 `hasParentID` 把未解析的 `{{...}}` 占位符视为"无 parent_id"，让 `focusPageID` 兜底生效
- **engine 层**：执行 plan 前自动从 `parent_id` 占位符反推 `depends_on`，容错 AI 漏写依赖
- **engine 层**：plan 中 action 数量按"建主页/生成目录/填内容"三阶段分别限制
- **AI 提示词层**：把"建主页 → outline 建空子页 → 逐页填内容"的三阶段流程写成强约束，禁止单 plan 混合阶段
- **AI 提示词层**：`parent_id` 是 `{{action:X.page_id}}` 时必须把 X 加入 `depends_on`，违反时 engine 应报错
- 新增两个 capability：`plan-execution-semantics`（执行语义）、`ai-propose-plan-contract`（AI 行为契约）

## Capabilities

### New Capabilities

- `plan-execution-semantics`: plan 执行时的语义约束——placeholder 解析必须失败时报错、focusPageID 兜底、生效依赖反推、各阶段 action 数量上限。覆盖 engine 层所有防御逻辑
- `ai-propose-plan-contract`: AI 调用 `propose_plan` 的行为契约——强制三阶段分离（建主页/生成目录/填内容）、outline 优先、depends_on 强制

### Modified Capabilities

（无现有 spec 需要修改。已检查 `exercise-detail` / `tool-call-visibility` / `topic-content` / `wiki-patch-edit` 四个现有 capability，均不涉及 plan 执行或 AI 提案契约）

## Impact

- `backend/internal/engine/engine.go`
  - `replacePlaceholders`：增加 error 返回路径
  - `hasParentID`：增加占位符识别逻辑
  - `ExecutePlan`：增加依赖反推、阶段 action 数量校验
- `backend/internal/ai/provider.go`
  - `buildWikiMaintainerPrompt`：重写"建结构"章节，强制 outline 字段 + 3 阶段分离 + depends_on 规则
- `backend/internal/handler/ai.go`
  - `createPlanFromToolCall`：可能需要在保存前再做一次 placeholder 完整性预检
- `backend/internal/engine/engine_test.go`
  - 现有 `TestReplacePlaceholders` 需更新（"unresolved placeholder left as-is" 这条用例变成"must return error"）
  - 新增依赖反推、focusPageID 兜底、阶段数量限制的测试
- 现有脏数据：11 个挂在根的章节页（IDs 21-26, 27-31）需要一次性数据修复脚本
