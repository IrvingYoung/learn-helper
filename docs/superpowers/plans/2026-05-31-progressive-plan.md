# Progressive Plan Enhancement - Implementation Plan

## Task 1: Backend - propose_plan tool schema & system prompt

**Files:** `backend/internal/ai/provider.go`

- Add `outline`, `phases`, `phase_index`, `total_phases` to propose_plan InputSchema
- Update system prompt: add single-page vs multi-page judgment rule + outline-first rule

## Task 2: Backend - database schema migration

**Files:** `backend/internal/model/models.go` + migration SQL

- Add `outline TEXT`, `phase_index INTEGER`, `total_phases INTEGER` to Plan struct
- Add migration SQL in init script or run manually

## Task 3: Backend - ExecOutline in engine

**Files:** `backend/internal/engine/engine.go`

- Add `ExecOutline(ctx, outline, conversationID)` method
- Recursively create_page for each outline node, content empty, content_status=empty
- Return page_id mapping for AI consumption

## Task 4: Backend - handler changes

**Files:** `backend/internal/handler/ai.go`, `backend/internal/handler/plan.go`

- `ai.go`: after plan execution completes, stop ReAct loop (don't auto-continue)
- `plan.go`: ConfirmPlan - detect outline-only plans, route to ExecOutline instead of ExecutePlan

## Task 5: Frontend - type definitions

**Files:** `frontend/src/types/index.ts`

- Add `OutlineNode`, `Phase` interfaces
- Add `outline`, `phases`, `phase_index`, `total_phases` to Plan interface

## Task 6: Frontend - PlanPreview OutlineTree component

**Files:** `frontend/src/components/PlanPreview.tsx`

- Add `OutlineTree` component (recursive, collapsible, page-type icons)
- Branch PlanPreview rendering: outline-only mode / outline+actions mode / actions-only mode
- Pending outline plan replace logic (new propose_plan with outline replaces old pending one)
