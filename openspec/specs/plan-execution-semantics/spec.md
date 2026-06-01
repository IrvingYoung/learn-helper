# Spec: plan-execution-semantics

plan 执行时的语义约束——engine 层防御逻辑，确保任何 propose_plan 提交后不会产生脏数据。

## ADDED Requirements

### Requirement: Placeholder Resolution Failure Surfaces as Error

The engine MUST return an error when any `{{action:X.field}}` placeholder in an action's params cannot be resolved against the `actionResultMap` at execution time. The error MUST list all unresolved placeholders.

#### Scenario: Unresolved placeholder in create_page parent_id

- **WHEN** a `create_page` action's `parent_id` is `"{{action:a1.page_id}}"` and action `a1` either has not been executed yet or has failed
- **THEN** the engine MUST mark the action as `failed` with an error message naming the unresolved placeholder
- **AND** MUST NOT insert the row with the literal placeholder string in `parent_id`

#### Scenario: Multiple unresolved placeholders across actions

- **WHEN** a plan contains three `create_page` actions each with `"parent_id": "{{action:a99.page_id}}"` and `a99` does not exist in the plan
- **THEN** the engine MUST fail all three actions and return a single aggregated error listing all three unresolved placeholder references

#### Scenario: Successfully resolved placeholder is silent

- **WHEN** a `create_page` action's `parent_id` is `"{{action:a1.page_id}}"` and `a1` has already been executed with a valid `page_id`
- **THEN** the engine MUST substitute the placeholder with the integer value and proceed normally without error

### Requirement: Focus Fallback for Unresolved Parent Placeholders

The engine MUST treat a `parent_id` whose value is a `{{...}}` placeholder string as "no parent_id" for the purpose of the `focusPageID` fallback injection. This ensures pages with unresolved parent references still attach to the user's current focus page rather than landing at root.

#### Scenario: create_page with placeholder parent_id and active focus

- **WHEN** a `create_page` action has `parent_id: "{{action:a1.page_id}}"` (unresolved) and the plan is being executed with `focusPageID = 16`
- **THEN** the engine MUST inject `parent_id: 16` into the action's params before execution
- **AND** MUST log a WARN-level message stating "fallback to focusPageID due to unresolved parent_id placeholder"

#### Scenario: create_page with placeholder parent_id and no focus

- **WHEN** a `create_page` action has `parent_id: "{{action:a1.page_id}}"` (unresolved) and no `focusPageID` is set
- **THEN** the engine MUST proceed to placeholder resolution
- **AND** MUST fail the action with an "unresolved placeholder" error (per the Placeholder Resolution requirement)

#### Scenario: create_page with literal integer parent_id is unaffected

- **WHEN** a `create_page` action has `parent_id: 42` (integer)
- **THEN** the `focusPageID` fallback MUST NOT inject anything
- **AND** execution proceeds with `parent_id: 42`

### Requirement: Implicit Dependency Inference from Placeholders

The engine MUST scan every action's params for `{{action:X.field}}` patterns before topological sort and add any referenced `X` to the action's in-memory `depends_on` set if not already present. The inference MUST be in-memory only; persisted `depends_on` in `plan_actions.depends_on` MUST remain unchanged for audit purposes.

#### Scenario: Action references parent but omits depends_on

- **WHEN** action `a5` has `parent_id: "{{action:a1.page_id}}"` but `depends_on = []`
- **THEN** the engine MUST add `a1` to `a5`'s in-memory dependency set before sorting
- **AND** MUST execute `a1` before `a5`

#### Scenario: Action already declares depends_on explicitly

- **WHEN** action `a5` has both `parent_id: "{{action:a1.page_id}}"` and `depends_on: ["a1"]`
- **THEN** the engine MUST NOT duplicate the dependency
- **AND** the in-memory set MUST be `{a1}` (not `{a1, a1}`)

#### Scenario: Action references non-existent action

- **WHEN** action `a5` has `parent_id: "{{action:nonexistent.page_id}}"` and `nonexistent` is not a valid action in the plan
- **THEN** the engine MUST NOT add any dependency (the reference is invalid; it will be caught by placeholder resolution at execution time)

#### Scenario: Dependency inference introduces cycle

- **WHEN** actions `a1` and `a2` both reference each other via `{{action:a2.page_id}}` and `{{action:a1.page_id}}` in their params
- **THEN** the topological sort MUST detect the cycle and return an error
- **AND** no action MUST be executed

### Requirement: Plan Stage Inference from Action Shape

The engine MUST infer a plan's stage ("main", "outline", "content") from the shape of its input at plan-creation time (before persisting to `plans`):

- **"main" stage**: `outline` is empty/null AND `actions` contains exactly 1 `create_page` (any content allowed, may include overview intro)
- **"outline" stage**: `outline` is non-empty AND `actions` is empty
- **"content" stage**: `outline` is empty/null AND `actions` contains only `update_page`, `patch_page`, `link_pages`, `move_page` (no `create_page` with non-empty content)
- **Ambiguous** (mixed signals): inference fails, plan is rejected

#### Scenario: Plan with single create_page is main stage

- **WHEN** `createPlanFromToolCall` receives `actions: [{type: "create_page", ...}]` and no `outline`
- **THEN** the plan MUST be persisted with `stage = "main"` (new column) or equivalent marker
- **AND** the count of `create_page` actions MUST be ≤ 1 (rejection if > 1)

#### Scenario: Plan with outline only is outline stage

- **WHEN** `createPlanFromToolCall` receives `outline: [{title: "...", children: [...]}]` and empty `actions`
- **THEN** the plan MUST be persisted with `stage = "outline"`
- **AND** the total node count in the outline tree MUST be ≤ 30

#### Scenario: Plan with only update_page is content stage

- **WHEN** `createPlanFromToolCall` receives `actions: [{type: "update_page", ...}, {type: "patch_page", ...}]` and no `outline`
- **THEN** the plan MUST be persisted with `stage = "content"`
- **AND** the count of `update_page` + `patch_page` actions MUST be ≤ 2

#### Scenario: Mixed outline and actions is rejected

- **WHEN** `createPlanFromToolCall` receives both a non-empty `outline` and non-empty `actions`
- **THEN** the plan MUST be rejected with an error explaining that outline and actions are mutually exclusive

#### Scenario: create_page in content stage is rejected

- **WHEN** `createPlanFromToolCall` receives `actions: [{type: "update_page", ...}, {type: "create_page", content: "..."}]`
- **THEN** the plan MUST be rejected with an error stating that `create_page` is not allowed in the content stage

#### Scenario: main stage with multiple create_page rejected

- **WHEN** `createPlanFromToolCall` receives `actions: [{type: "create_page", ...}, {type: "create_page", ...}]` (two create_page actions)
- **THEN** the plan MUST be rejected with an error explaining that the main stage allows at most 1 create_page
- **AND** MUST suggest splitting into separate plans

#### Scenario: outline stage with too many nodes rejected

- **WHEN** `createPlanFromToolCall` receives an outline tree with 50 leaf nodes
- **THEN** the plan MUST be rejected with an error stating the node count limit
- **AND** MUST suggest splitting the outline

### Requirement: Stage Limits Enforced on Existing Plans

Existing plans in the database (created before this capability shipped) MUST be re-validated at execution time using the same stage rules. A plan that violates the new stage rules MUST be rejected at execution with the same error messages that would have been raised at creation.

#### Scenario: Legacy plan with 12 create_page actions executed after upgrade

- **WHEN** `ExecutePlan` is called for a pre-existing plan with `actions = [12 × create_page]`
- **THEN** the engine MUST reject the plan with a "main stage limit exceeded" error before any action runs
- **AND** MUST NOT partially execute (no inserts to `wiki_pages`)
