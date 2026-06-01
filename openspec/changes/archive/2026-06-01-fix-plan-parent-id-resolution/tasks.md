## 1. Failing Tests First (TDD)

- [x] 1.1 Update `TestReplacePlaceholders` in `engine_test.go`: flip the "unresolved placeholder left as-is" case to "must return error and list unresolved placeholders"
- [x] 1.2 Add `TestHasParentID_RejectsPlaceholder` verifying `{{...}}` string value returns false
- [x] 1.3 Add `TestHasParentID_AcceptsInteger` regression test for the integer case
- [x] 1.4 Add `TestInferDependencyFromPlaceholder` covering: missing depends_on, already declared, non-existent reference, cycle detection
- [x] 1.5 Add `TestInferPlanStage_MainPage` (1 create_page, no outline → stage=main)
- [x] 1.6 Add `TestInferPlanStage_Outline` (non-empty outline, empty actions → stage=outline)
- [x] 1.7 Add `TestInferPlanStage_Content` (only update/patch/link/move → stage=content)
- [x] 1.8 Add `TestRejectPlan_MainStageTooManyCreate` (2 create_pages in main → error)
- [x] 1.9 Add `TestRejectPlan_OutlineTooManyNodes` (50 nodes → error)
- [x] 1.10 Add `TestRejectPlan_ContentStageHasCreate` (update + create → error)
- [x] 1.11 Add `TestRejectPlan_OutlineAndActionsMixed` (both set → error)
- [x] 1.12 Add `TestFocusFallback_PlaceholderAndFocus` (placeholder parent + focusPageID → parent_id injected, WARN logged)
- [x] 1.13 Run `go test ./internal/engine/...` to confirm all 12 new tests fail (red phase)

## 2. Engine: Placeholder Resolution Error Path (D1)

- [x] 2.1 Modify `replacePlaceholders` in `engine.go:256-282`: collect all unresolved matches into a slice, return `fmt.Errorf` at the end if non-empty
- [x] 2.2 Update call site at `engine.go:107`: handle new error return — mark action failed with the error string
- [x] 2.3 Update `updateActionStatus` to record the placeholder error in `plan_actions.result` JSON
- [x] 2.4 Add unit test confirming error message format: `"unresolved placeholders: a1.page_id, a2.page_id"`
- [ ] 2.5 Re-run `TestReplacePlaceholders`: all 5 cases pass (green phase for D1)

## 3. Engine: Focus Fallback for Unresolved Placeholders (D4)

- [x] 3.1 Add `isPlaceholderString(s string) bool` helper in `engine.go` checking for `"{{"` substring
- [x] 3.2 Modify `hasParentID` in `engine.go:1233-1240`: parse `parent_id`, if it's a string with `isPlaceholderString == true`, return false
- [x] 3.3 Add a WARN-level log at the focusPageID injection point (`engine.go:67-74`) when the injection triggers due to placeholder
- [ ] 3.4 Re-run `TestHasParentID_*` tests (green phase for D4)

## 4. Engine: Implicit Dependency Inference (D3)

- [x] 4.1 Add `inferDependencies(paramsJSON string) []string` helper that extracts all `{{action:X.field}}` action IDs from a params JSON
- [x] 4.2 In `ExecutePlan`, after `loadActions` and before `topoSort`, iterate actions; for each, merge `inferDependencies(params)` with existing `depends_on` into an in-memory set
- [x] 4.3 Use the merged set for `topoSort` only; do NOT modify the loaded `model.PlanAction.DependsOn` (preserve audit)
- [ ] 4.4 Re-run `TestInferDependencyFromPlaceholder` (green phase for D3)

## 5. Engine: Stage Inference and Limits (D2, D5)

- [x] 5.1 Add `inferPlanStage(proposal *proposal) (stage string, err error)` in `handler/ai.go` next to `createPlanFromToolCall`
- [x] 5.2 Define stage constants: `StageMain = "main"`, `StageOutline = "outline"`, `StageContent = "content"`
- [x] 5.3 Implement stage detection logic: outline non-empty AND actions empty → outline; exactly 1 create_page AND no outline → main; only update/patch/link/move AND no create_page → content; else error
- [x] 5.4 Implement action count validation per stage: main ≤1, outline ≤30 leaf nodes, content ≤2
- [x] 5.5 Implement cross-stage validation: outline + actions both non-empty → error; content stage with create_page → error
- [x] 5.6 Call `inferPlanStage` from `createPlanFromToolCall` BEFORE `SavePlan`; on error, return error so `AIChat` returns 500 to the AI (and the user sees the rejection in the right panel)
- [x] 5.7 Add a new column `stage TEXT` to `plans` table via migration `010_add_plan_stage.sql`
- [x] 5.8 Update `model.Plan` struct and queries to include `Stage` field
- [x] 5.9 Set `Stage` on the plan in `createPlanFromToolCall` based on `inferPlanStage` result
- [x] 5.10 In `ExecutePlan`, re-validate the stage rules against the persisted plan (defense for legacy data)
- [x] 5.11 Re-run stage tests (green phase for D2/D5)

## 6. AI Prompt Rewrite (D6)

- [x] 6.1 In `provider.go:398-413`, replace the "工作节奏" / "建立知识体系的标准流程" section with three explicit stage blocks using MUST/MUST NOT language
- [x] 6.2 Add a pre-submit self-check list at the end of the workflow section: "提交前确认: (1) 阶段正确 (2) 不混合阶段 (3) parent_id 占位符对应 depends_on"
- [x] 6.3 Add explicit statement: "如果占位符未解析，plan 整体失败，不会脏写。请确保 parent_id 引用了同 plan 中已声明的 action，并把该 action 加入 depends_on"
- [x] 6.4 Verify the new prompt still fits in token budget (count chars, target ≤ 4500 chars for the prompt body)
- [x] 6.5 Manually inspect the rendered prompt by adding a debug log in `BuildSystemPrompt` to print char count (then remove the log)

## 7. Dirty Data Migration (D7)

- [x] 7.1 Create `backend/db/migrations/010_reparent_root_chapters.sql` with `BEGIN TRANSACTION;` wrapper
- [x] 7.2 Write the reparent UPDATE: `UPDATE wiki_pages SET parent_id = 32 WHERE id IN (21,22,23,24,25,26,27,28,29,30,31) AND parent_id IS NULL;`
- [x] 7.3 Recompute paths: for each reparented page, set `path = (SELECT path FROM wiki_pages WHERE id = 32) || page_id || '/'`
- [x] 7.4 Add manual ROLLBACK comment in the SQL file for emergency use
- [x] 7.5 Verify count before/after: `SELECT COUNT(*) FROM wiki_pages WHERE parent_id IS NULL;` (should drop from 11 to 0)
- [x] 7.6 Test on a backup of the DB first; if all 11 reparented cleanly, apply to the live DB
- [x] 7.7 Run the wiki page tree query to confirm rendering: `SELECT id, title, parent_id, path FROM wiki_pages WHERE path LIKE '13/14/16/%' ORDER BY id;`

## 8. Verification

- [x] 8.1 `go build ./cmd/server` — compiles clean
- [x] 8.2 `go test ./...` — all tests pass
- [x] 8.3 Start the server: `cd backend && go run ./cmd/server` — boots without panic
- [ ] 8.4 Open a new conversation and ask: "帮我建一个 Go 学习的知识体系"
- [ ] 8.5 Confirm AI submits a main-stage plan with 1 create_page (not 12 actions)
- [ ] 8.6 Approve the plan; verify the main page is created under "Go" node with parent_id = 16
- [ ] 8.7 Say "生成目录" and confirm AI submits an outline-stage plan
- [ ] 8.8 Approve; verify 11 child pages appear under the main page with correct paths
- [ ] 8.9 Say "写 1.1" and confirm AI submits a content-stage plan with 1 update_page
- [ ] 8.10 Manually craft a malformed plan (via debug log or direct SQL) where `parent_id` is a placeholder and `depends_on` is empty; confirm engine still produces correct parent_id via dependency inference
- [ ] 8.11 Manually craft a plan with 12 create_page actions; confirm engine rejects with "main stage limit exceeded" before any insert
