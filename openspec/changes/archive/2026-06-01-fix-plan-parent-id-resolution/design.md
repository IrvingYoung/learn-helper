## Context

`propose_plan` 工具被 `wiki_maintainer` AI 用于一次性提交一批 wiki 操作。引擎（`engine.ExecutePlan`）按拓扑顺序执行这些 action，每执行完一个就把结果（如 `page_id`）存入 `actionResultMap`，供后续 action 通过 `{{action:X.page_id}}` 占位符引用。

当前实现中 `replacePlaceholders` 在目标 action 还没执行时**静默返回原占位符字符串**（有单元测试 `TestReplacePlaceholders/unresolved placeholder left as-is` 明确文档化此行为）。配合 `hasParentID` 只检查 JSON key 存在性、`toInt64` 不接受字符串这三层静默，AI 漏写 `depends_on` 时脏数据会毫无阻碍地写入数据库。

此外系统提示（`provider.go:404-409`）已规定"建主页 → outline 建空子页 → 逐页填内容"三阶段，但缺乏强制约束，AI 在实际运行中一次性提交 12 个 `create_page` 携带完整 Markdown，绕过设计。

约束：
- 已有 `ExecOutline` 递归建空页机制（`engine.go:336-378`），只需把 AI 引导到这条路径
- 已有 `injectContentStatus` 和 `focusPageID` 注入机制（`engine.go:67-74`, `1218-1230`），新逻辑需要与它们协作
- 不得破坏现有 4 个 capability 已文档化的契约（`exercise-detail` / `tool-call-visibility` / `topic-content` / `wiki-patch-edit`）

## Goals / Non-Goals

**Goals:**
- plan 执行时不允许任何脏数据写入（placeholder 失败必须显式失败）
- 三阶段流程（建主页/生成目录/填内容）在 plan 创建时按 action 类型自动推断并校验
- 即便 AI 漏写 `depends_on`，engine 也能从 `parent_id` 占位符自动反推
- 修复用户当前实际脏数据（11 个根节点章节页）
- 保持现有 `ExecOutline` 路径的行为不变（已正确工作）

**Non-Goals:**
- 不重写整个 propose_plan 工具的 schema
- 不修改 AI provider（OpenCode Go / DeepSeek）的协议层
- 不强制 UI 端在 plan 确认时做额外校验（焦点在 engine + 提示词）
- 不引入新表/新列，迁移走一次性 SQL

## Decisions

### D1: `replacePlaceholders` 改为返回 error（不再静默）

**选择**: 把 `engine.go:256-282` 的"找不到就返回原 match"改为"找不到就累积到 unresolved 列表，最后返回 `fmt.Errorf("unresolved placeholders: %v", unresolved)`"

**理由**: 这是 bug 的最直接源头。静默失败掩盖了所有下游错误。改 error 后整个 plan 会显式失败，用户看到错误信息就能定位问题。

**备选**:
- 保留静默 + 改用警告日志 → 否，警告没人看
- 解析失败时只对 `parent_id` 字段报错 → 否，逻辑不统一，bug 会以其他形式复发

### D2: 阶段（Stage）由 action 类型自动推断

**选择**: 不在 propose_plan schema 里加新字段。engine 根据 plan 里的 action 形状推断阶段：
- "main page" 阶段：`actions` 中只有 ≤1 个 `create_page`，可带 `content`（用作 overview 简介）
- "outline" 阶段：**只能**用 `outline` 字段，`actions` 必须为空
- "content" 阶段：`actions` 中只有 `update_page` / `patch_page` / `link_pages` / `move_page`，禁止 `create_page`（除非用于建关联子页，且父级是 outline 阶段创建的）

**理由**: AI 提交 plan 时不需要额外声明阶段——它提交的 action 形状本身就是阶段签名。验证在 `createPlanFromToolCall`（plan 持久化前）发生，错误即时反馈给 AI 端。

**备选**:
- 在 propose_plan input 加显式 `stage` 字段 → 否，引入新 schema 字段，所有 prompt / 测试都要改
- 让 AI 自由发挥，靠 action 数上限硬卡 → 否，会误伤合法的"建主页"plan

### D3: 依赖反推在 `ExecutePlan` 入口执行，不在 plan 创建时

**选择**: `ExecutePlan` 加载 actions 后、拓扑排序前，扫描每个 `create_page`/`update_page`/`patch_page` action 的 params 里的 `{{action:X}}` 模式，把 X 加入该 action 的 `depends_on`（如果不在的话）。

**理由**:
- 计划可能由多次对话轮次拼成，反推在执行时更鲁棒
- plan 持久化的 `params` 保留原始状态，便于审计
- 反推后的 `depends_on` 仅存在于内存中，不写回 DB

**备选**:
- 在 `createPlanFromToolCall` 反推并写回 DB → 否，污染了"AI 实际声明了什么"的真相

### D4: `hasParentID` 把 `{{...}}` 视为无 parent

**选择**: `hasParentID`（`engine.go:1233-1240`）增加判断：如果 `parent_id` 是字符串且含 `{{` 子串，返回 false。

**理由**: 让现有的 `focusPageID` 兜底机制（`engine.go:67-74`）在占位符未解析时也能生效。这是与 D1 互补的兜底——D1 修"D1 触发后 plan 失败"，D4 修"如果 D1 没触发，page 至少挂到当前聚焦页"。

**注意**: D4 是临时兜底，D1 是主防线。两者都要做。

### D5: 阶段限制的具体数字

**选择**:
- main page: ≤1 个 `create_page`（**禁止 outline 字段同时使用**）
- outline: outline 节点数 ≤30（覆盖一个大型知识体系），**禁止 `actions: create_page`**
- content: ≤2 个 `update_page`/`patch_page`，禁止 `create_page` 携带非空 content

**理由**: 数字基于用户实际案例（Go 学习体系 11 章节页）和已有 `CreateConversation` 等接口的规模惯例。30 个 outline 节点约等于"基础/进阶/高级/实战"四大主题 × 平均 7-8 章节。

**备选**:
- 把数字做成可配置（环境变量）→ 否，YAGNI，硬编码 + 在 spec 里写明即可

### D6: AI 提示词改写为"三阶段强制"措辞

**选择**: 重写 `provider.go:398-413` 的"工作节奏"章节：
- 用大写"必须"/"禁止"措辞
- 三阶段每段单独编号并各加示例
- 末尾加 self-check 清单（参考 GitHub Copilot 提示词模式）

**理由**: 当前提示词用了"分步完成"、"先 X 再 Y"等软性措辞，AI 会忽略。改成 imperative mood 显著降低违规概率。

**注意**: 即使提示词改了，**也不能替代** D1-D4 的 engine 防线。提示词只是减少概率，防线保证正确性。

### D7: 脏数据修复策略

**选择**: 一次性 SQL 脚本（`backend/db/migrations/010_reparent_root_chapters.sql`），针对每个 ID 21-26、27-31 的根节点章节页，扫描其原始 plan_actions.params 中 `parent_id` 占位符，解析出真正应该挂的父页 ID（统一为 32，"Go 语言从基础到精通"），执行 `UPDATE wiki_pages SET parent_id = 32 WHERE id IN (...)` 并重新计算 path。

**理由**: 
- 这些页面的 plan_actions 历史记录都在 DB 里可追溯
- 全部 11 页都引用了同一个 `{{action:plan-1780318840946982000-a1.page_id}}`，解析后统一指向 32
- 重新计算 path 用现有 `UpdateWikiPagePath` query

**备选**:
- 让用户手动拖拽 → 否，11 个拖拽工作量大
- 删了让 AI 重建 → 否，丢失已写的学习内容

## Risks / Trade-offs

**[R1] D1 破坏现有调用方** → 任何依赖"未解析占位符静默通过"的代码路径会突然报错。Mitigation: 全仓搜索 `replacePlaceholders` 调用方，逐一评估；现有 `TestReplacePlaceholders` 的 "unresolved placeholder left as-is" 用例要更新为 "must return error"。

**[R2] D2 阶段推断误判** → 罕见 plan 形状可能同时匹配多个阶段。Mitigation: 推断不唯一时报错让 AI 调整，而不是猜。

**[R3] D3 依赖反推引入循环依赖** → AI 错误地让 a1 引用 a2 同时 a2 引用 a1。Mitigation: 现有 `topoSort` 已有 cycle detection，会报"circular dependency detected"。

**[R4] D4 掩盖真实 bug** → 兜底把页面挂到错误位置比"失败"更难发现。Mitigation: focusPageID 兜底时在 log 里打印 WARN 级别日志，标注"fallback to focusPageID due to unresolved placeholder"。

**[R5] D5 数字太严导致合法 plan 被拒** → 未来用户想一次建 40 页 outline 怎么办。Mitigation: 数字写进 spec 后可通过新 change 调整，spec-driven workflow 支持此模式。

**[R6] D6 提示词改了让 AI 行为退化** → 强制三阶段后，AI 可能变得过度啰嗦、每条都问确认。Mitigation: 提示词里明确"三阶段是结构化操作上的分离，不是必须用户每次都确认"; 同时 D1-D4 防线让"违规"代价是 plan 失败而非脏数据，AI 自我修正成本可控。

**[R7] D7 迁移脚本出错破坏现有数据** → SQL 写错或 ID 错位。Mitigation: 脚本加 `BEGIN TRANSACTION` + 手动 rollback 注释; 先在备份 db 上跑通; 迁移前后用 `SELECT count(*) WHERE parent_id IS NULL` 验证。

## Migration Plan

部署顺序：

1. **代码层**（无破坏性）：合并 engine + AI 提示词改动到 main，CI 跑通
2. **观察期**（1-2 天）：新会话触发时观察日志，确认 D1-D6 工作正常，无误伤
3. **数据迁移**：跑 D7 脚本，验证 root 章节页全部 reparent 到 32
4. **回滚策略**：迁移脚本用事务包住，失败回滚；engine 改动可通过 revert commit 回滚到上一个 release tag

不需要 feature flag——所有改动要么是 prompt 文本（可热更新），要么是 plan 创建/执行路径（plan 失败不会破坏既有数据）。

## Open Questions

- **Q1**: 阶段限制的精确数字（main ≤1、outline ≤30、content ≤2）是否合理？需要在 spec review 时敲定。
- **Q2**: D6 改写提示词后，是否需要同步给 DeepSeek 模型做一次行为回归测试？模型表现可能与 OpenCode Go 不同。
- **Q3**: D7 迁移脚本要不要也修复"home"标签显示？那些章节页的 `page_type` 是 `overview`（这就是"home"标签来源），用户视角下应该改成 `entity` 或保持原样？
