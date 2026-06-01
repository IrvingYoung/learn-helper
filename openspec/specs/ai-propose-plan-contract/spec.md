# Spec: ai-propose-plan-contract

`wiki_maintainer` AI 调用 `propose_plan` 时的行为契约——通过 system prompt 强制约束 AI 走"建主页 → outline 建空子页 → 逐页填内容"三阶段流程，避免一次性提交大量 action 绕过设计。

## ADDED Requirements

### Requirement: AI Uses outline Field for Outline Stage

When the AI's task is to create the structure of a knowledge system (skeleton pages with no content), it MUST use the `outline` field of `propose_plan` and MUST NOT use `actions: create_page` to batch-create empty pages. The reasoning text MUST explain the proposed structure before invoking the tool.

#### Scenario: User asks AI to generate a chapter outline

- **WHEN** the user says "生成目录" or "建子页面" after the main page has been created
- **THEN** the AI MUST invoke `propose_plan` with a non-empty `outline` array and empty `actions`
- **AND** the `outline` MUST include `title`, `page_type`, and nested `children` for each node
- **AND** no `outline` node MUST include `content` (pages are created empty)

#### Scenario: AI attempts to batch-create empty pages with actions

- **WHEN** the AI's response includes `propose_plan` with 11 `actions: create_page` items all with empty `content`
- **THEN** the engine MUST reject the plan (per `plan-execution-semantics` stage rules)
- **AND** the rejection error message MUST include the suggestion to use the `outline` field instead
- **AND** the AI MUST reformulate the plan using `outline` in a subsequent turn

### Requirement: AI Uses Single create_page for Main Page Stage

When the AI's task is to create the main entry page of a new knowledge area, it MUST submit a plan with exactly one `create_page` action whose `parent_id` points to the appropriate existing parent (looked up via `lookup_page` first).

#### Scenario: User asks AI to plan a new knowledge area

- **WHEN** the user says "我想学 X" or "帮我建一个关于 X 的知识体系" and the discussion phase has completed
- **THEN** the AI MUST first invoke `lookup_page` to find the appropriate parent (e.g., "Go" node under "程序员")
- **AND** MUST then invoke `propose_plan` with exactly one `create_page` action whose `parent_id` is the looked-up ID
- **AND** the `content` MAY include an overview/路线图 but MUST NOT include section-level content
- **AND** the reasoning MUST describe the planned sub-topics without creating them yet

#### Scenario: Main page with section content is rejected

- **WHEN** the AI submits a main-stage `create_page` with `content` containing full Markdown sections for sub-pages (e.g., `# 1.1 环境搭建\n...` `# 1.2 变量...`)
- **THEN** the AI prompt's "before write" checklist MUST warn against this
- **AND** the engine MAY warn but MUST still allow it (main stage permits content)
- **AND** the user reviewing the plan in the right panel MUST see a warning banner suggesting to split

### Requirement: AI Limits Content Stage to 1-2 Pages

When the AI's task is to fill content into existing pages, it MUST submit a plan with at most 2 `update_page` or `patch_page` actions, and MUST NOT include any `create_page` action that carries content.

#### Scenario: User asks AI to write detailed content for chapter 1.1

- **WHEN** the user says "开始写 1.1" or similar
- **THEN** the AI MUST invoke `propose_plan` with 1-2 `update_page` or `patch_page` actions targeting the existing page IDs
- **AND** the `actions` array MUST NOT contain any `create_page`

#### Scenario: User asks AI to write content for 3 chapters at once

- **WHEN** the user says "把 1.1 1.2 1.3 都写了"
- **THEN** the AI MUST propose 3 separate plans across multiple turns, not one plan with 3 update_page actions
- **AND** MUST explain in the chat that the limit is 2 per plan and offer to do them sequentially
- **AND** the system prompt's behavior rules MUST reinforce this limit

### Requirement: AI Declares depends_on for Placeholder References

When any action's `params` contains a `{{action:X.field}}` placeholder referencing another action `X` in the same plan, the action's `depends_on` array MUST include `X`. This prevents the placeholder from being unresolved due to out-of-order execution.

#### Scenario: create_page references earlier create_page for parent

- **WHEN** the AI's plan has actions `a1` (creates main page) and `a2` (creates child with `parent_id: "{{action:a1.page_id}}"` )
- **THEN** `a2.depends_on` MUST equal `["a1"]`
- **AND** the engine MUST verify this declaration (warn if missing, but proceed thanks to implicit dependency inference)

#### Scenario: AI forgets depends_on despite prompt warning

- **WHEN** the AI's plan has `a2` with `parent_id` placeholder but no `depends_on`
- **THEN** the engine's implicit dependency inference (per `plan-execution-semantics`) MUST add `a1` to in-memory `depends_on`
- **AND** execution MUST proceed in correct order without the AI needing to be re-prompted
- **AND** the rejection behavior in `ai-propose-plan-contract` exists only as a documentation contract, not as a hard engine gate

### Requirement: AI Submits Plans in Three Distinct Stages

The system prompt's workflow description MUST be rewritten to use imperative language explicitly naming the three stages and prohibiting cross-stage mixing within a single plan. The prompt MUST include a self-check checklist the AI verifies before submitting.

#### Scenario: System prompt contains stage enforcement language

- **WHEN** the AI reads the system prompt for `wiki_maintainer`
- **THEN** the prompt MUST contain three explicit stage sections, each with: prohibited action types, allowed action types, and a count limit
- **AND** MUST contain a pre-submit self-check list: "提交前确认: (1) 阶段正确 (2) 不混合阶段 (3) parent_id 占位符对应 depends_on"
- **AND** MUST use MUST / MUST NOT / 禁止 / 必须 for normative statements

#### Scenario: AI attempts to mix stages in one plan

- **WHEN** the AI submits a plan with 1 `create_page` (main) plus 5 `update_page` (content) in the same `actions` array
- **THEN** the engine MUST reject the plan with an "ambiguous stage" error
- **AND** the error message MUST list which action types cannot co-exist
- **AND** the AI MUST reformulate by splitting into two plans in a subsequent turn

### Requirement: System Prompt Documents Placeholder Failure as a Hard Error

The system prompt MUST document that `{{action:X.field}}` placeholders that fail to resolve will cause the entire plan to fail (not silently pass through). This sets the AI's expectations so it self-corrects rather than relying on silent fallback.

#### Scenario: AI references a non-existent action

- **WHEN** the AI's plan uses `{{action:wrong_id.page_id}}` and `wrong_id` is not in the plan
- **THEN** the system prompt MUST have warned the AI that this will fail the plan
- **AND** the plan MUST be rejected with a clear error naming the unresolved reference
- **AND** the AI MUST issue a follow-up correction in chat (or resubmit with the correct ID)
