# Unified Operation Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify user direct operations and AI-initiated operations through a single Plan + ExecutionEngine confirmation flow, add knowledge tree direct interaction, and improve feedback visibility.

**Architecture:** All write operations requiring confirmation go through the Plan system. Legacy `pending_actions` code is removed. The right-side panel gains a tab layout with "Page Content" and "Pending Operations" tabs. Knowledge tree gets right-click menu, drag-and-drop, inline rename, and quick-add buttons.

**Tech Stack:** Go (Chi + SQLite), React 19, TypeScript, SWR, Tailwind CSS

---

## Phase 1: Unified Confirmation System

### Task 1: Remove legacy pending_actions from backend

**Files:**
- Modify: `backend/internal/handler/ai.go`

- [ ] **Step 1: Remove `ConfirmedActions` from request struct**

In `ai.go` line 296, remove the `ConfirmedActions` field:

```go
// BEFORE:
var req struct {
    ConversationID   int64           `json:"conversation_id"`
    Message          string          `json:"message"`
    ConfirmedActions []PendingAction `json:"confirmed_actions"`
    PlanID           string          `json:"plan_id"`
    FocusPageID      *int64          `json:"focus_page_id"`
    CurrentSlug      string          `json:"current_slug"`
    SelectedText     string          `json:"selected_text"`
}

// AFTER:
var req struct {
    ConversationID int64   `json:"conversation_id"`
    Message        string  `json:"message"`
    PlanID         string  `json:"plan_id"`
    FocusPageID    *int64  `json:"focus_page_id"`
    CurrentSlug    string  `json:"current_slug"`
    SelectedText   string  `json:"selected_text"`
}
```

- [ ] **Step 2: Remove confirmed_actions execution block**

In `ai.go` lines 324-329, remove the block that executes confirmed actions:

```go
// DELETE these lines:
if len(req.ConfirmedActions) > 0 {
    resultContent := h.executeConfirmedActions(ctx, req.ConfirmedActions)
    h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
        req.ConversationID, resultContent, config.Provider)
}
```

- [ ] **Step 3: Remove confirmedFingerprints block**

In `ai.go` lines 438-454, remove the fingerprint building code:

```go
// DELETE these lines:
confirmedFingerprints := make(map[string]bool)
selfTypeToToolName := map[string]string{
    "create": "create_page",
    "update": "update_page",
    "delete": "delete_page",
}
for _, a := range req.ConfirmedActions {
    toolName, ok := selfTypeToToolName[a.Type]
    if !ok {
        continue
    }
    dj, err := json.Marshal(a.Details)
    if err != nil {
        continue
    }
    confirmedFingerprints[toolName+":"+string(dj)] = true
}
```

- [ ] **Step 4: Remove fingerprint filtering in ReAct loop**

In `ai.go` lines 544-549, replace the fingerprint-filtered tool calls with direct assignment:

```go
// BEFORE:
var toolCalls []ai.ToolCall
for _, tc := range respToolCalls {
    if !confirmedFingerprints[tc.Name+":"+tc.Input] {
        toolCalls = append(toolCalls, tc)
    }
}

// AFTER:
toolCalls := respToolCalls
```

- [ ] **Step 5: Remove injectToolResults call**

In `ai.go` lines 433-435, remove:

```go
// DELETE these lines:
if len(req.ConfirmedActions) > 0 {
    aiMessages = injectToolResults(aiMessages, req.ConfirmedActions)
}
```

- [ ] **Step 6: Remove confirmed_actions system prompt addition**

In `ai.go` lines 481-483, remove:

```go
// DELETE these lines:
if len(req.ConfirmedActions) > 0 {
    systemPrompt += "\n\n【本次请求特别说明】以上对话中已经包含 tool（tool_result）返回结果，表明对应操作已被用户确认执行完毕。请直接回复告知用户结果即可，不要再次调用相同的工具。"
}
```

- [ ] **Step 7: Remove dead pendingActions variable and related code**

In `ai.go` line 492, remove `var pendingActions []PendingAction`.

In `ai.go` lines 671-682, replace the save block:

```go
// BEFORE:
if assistantText != "" || len(pendingActions) > 0 {
    savedContent := assistantText
    if len(pendingActions) > 0 {
        savedContent += "\n[操作建议]\n"
        for _, pa := range pendingActions {
            detailsJSON, _ := json.Marshal(pa.Details)
            savedContent += fmt.Sprintf("- %s: %s\n", pa.Type, string(detailsJSON))
        }
    }
    h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
        req.ConversationID, savedContent, config.Provider)
}

// AFTER:
if assistantText != "" {
    h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
        req.ConversationID, assistantText, config.Provider)
}
```

In `ai.go` lines 694-697, remove the dead pending_actions SSE send:

```go
// DELETE these lines:
if len(pendingActions) > 0 {
    metaBytes, _ := json.Marshal(map[string]any{"pending_actions": pendingActions})
    sseWrite(w, "meta", string(metaBytes), canFlush, flusher)
}
```

- [ ] **Step 8: Delete dead-code functions**

Delete the following functions entirely from `ai.go`:
- `toolCallsToPendingActions` (lines 1063-1116)
- `executeConfirmedActions` (lines 1118-1209)
- `injectToolResults` (lines 1228-1321)

- [ ] **Step 9: Delete PendingAction type**

In `ai.go` lines 37-41, delete:

```go
// DELETE:
type PendingAction struct {
    Type    string `json:"type"`
    Preview string `json:"preview"`
    Details any    `json:"details"`
}
```

- [ ] **Step 10: Verify backend compiles**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: compiles with no errors

- [ ] **Step 11: Commit**

```bash
git add backend/internal/handler/ai.go
git commit -m "refactor: remove legacy pending_actions code from ai handler"
```

---

### Task 2: Remove legacy pending_actions from frontend

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: Remove PendingAction type from types/index.ts**

In `types/index.ts` lines 28-32, delete:

```typescript
// DELETE:
export interface PendingAction {
  type: 'create' | 'update' | 'delete';
  preview: string;
  details?: Record<string, unknown>;
}
```

In `types/index.ts` line 57, remove `pending_actions` from `ConversationMessage`:

```typescript
// BEFORE:
export interface ConversationMessage {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  model_provider: string | null;
  token_count: number | null;
  created_at: string;
  pending_actions?: PendingAction[];
  confirmed?: boolean;
  plan?: Plan;
}

// AFTER:
export interface ConversationMessage {
  id: number;
  role: 'user' | 'assistant';
  content: string;
  model_provider: string | null;
  token_count: number | null;
  created_at: string;
  plan?: Plan;
}
```

- [ ] **Step 2: Remove PendingAction from api.ts**

In `api.ts` line 1, remove `PendingAction` from the import:

```typescript
// BEFORE:
import type { WikiPage, WikiTreeNode, Conversation, ConversationMessage, PendingAction, Plan, ExecutionReport } from "../types";

// AFTER:
import type { WikiPage, WikiTreeNode, Conversation, ConversationMessage, Plan, ExecutionReport } from "../types";
```

In `api.ts` lines 72-82, remove `confirmed_actions` from `ChatRequest`:

```typescript
// BEFORE:
export interface ChatRequest {
  conversation_id: number;
  message: string;
  role?: string;
  context_type?: string;
  confirmed_actions?: PendingAction[];
  plan_id?: string;
  focus_page_id?: number | null;
  current_slug?: string;
  selected_text?: string;
}

// AFTER:
export interface ChatRequest {
  conversation_id: number;
  message: string;
  role?: string;
  context_type?: string;
  plan_id?: string;
  focus_page_id?: number | null;
  current_slug?: string;
  selected_text?: string;
}
```

In `api.ts` line 87, remove `pending_actions` from `onMeta` type:

```typescript
// BEFORE:
onMeta: (data: { conversation_id?: number; pending_actions?: PendingAction[]; plan?: Plan }) => void,

// AFTER:
onMeta: (data: { conversation_id?: number; plan?: Plan }) => void,
```

- [ ] **Step 3: Remove pending_actions handling from ChatPanel.tsx**

In `ChatPanel.tsx` line 2, remove `PendingAction` from the import:

```typescript
// BEFORE:
import type { Conversation, ConversationMessage, PendingAction, Plan } from "../types";

// AFTER:
import type { Conversation, ConversationMessage, Plan } from "../types";
```

In `ChatPanel.tsx` lines 227-243, remove `pending_actions` handling from `onMeta`:

```typescript
// BEFORE (inside onMeta callback):
if (data.conversation_id) {
  setActiveConv(prev => prev ? { ...prev, id: data.conversation_id! } : null);
}
if (data.pending_actions) {
  // handle pending actions
}
if (data.plan) {
  onPlanCreated?.(data.plan);
}

// AFTER:
if (data.conversation_id) {
  setActiveConv(prev => prev ? { ...prev, id: data.conversation_id! } : null);
}
if (data.plan) {
  onPlanCreated?.(data.plan);
}
```

In `ChatPanel.tsx`, delete the `handleConfirmAction` function (lines 279-284).

In `ChatPanel.tsx`, delete the inline pending_actions confirmation UI (lines 310-331).

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/index.ts frontend/src/lib/api.ts frontend/src/components/ChatPanel.tsx
git commit -m "refactor: remove legacy pending_actions from frontend types, api, and chat panel"
```

---

### Task 3: Add POST /api/plans endpoint for frontend-created plans

**Files:**
- Modify: `backend/internal/handler/plan.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add CreatePlan handler to plan.go**

Add the following handler to `plan.go` after the `RejectPlan` function (after line 158):

```go
// CreatePlan handles POST /api/plans
// Body: { "reasoning": "...", "actions": [...] }
// Creates a plan from user-initiated operations (e.g., delete from tree).
func (h *PlanHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reasoning string `json:"reasoning"`
		Actions   []struct {
			Type   string          `json:"type"`
			Params json.RawMessage `json:"params"`
		} `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Reasoning == "" {
		req.Reasoning = "用户操作"
	}
	if len(req.Actions) == 0 {
		http.Error(w, "at least one action required", http.StatusBadRequest)
		return
	}

	planID := fmt.Sprintf("user-%d-%s", time.Now().UnixMilli(), uuid.New().String()[:8])

	plan := &model.Plan{
		ID:             planID,
		ConversationID: 0,
		Reasoning:      req.Reasoning,
		Status:         "pending",
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	for i, a := range req.Actions {
		actionID := fmt.Sprintf("%s-a%d", planID, i+1)
		plan.Actions = append(plan.Actions, model.PlanAction{
			ID:        actionID,
			PlanID:    planID,
			Type:      a.Type,
			Params:    a.Params,
			DependsOn: json.RawMessage("[]"),
			Status:    "pending",
			SortOrder: int64(i),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	if err := h.SavePlan(r.Context(), plan); err != nil {
		http.Error(w, "failed to save plan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, plan)
}
```

Add the required imports at the top of `plan.go`:

```go
import (
	"fmt"
	"time"

	"github.com/google/uuid"
)
```

- [ ] **Step 2: Add route in main.go**

In `main.go` after line 220 (`r.Get("/plans", planHandler.GetPlan)`), add:

```go
r.Post("/plans", planHandler.CreatePlan)
```

- [ ] **Step 3: Verify uuid dependency**

Run: `cd /Users/irving/repo/learn-helper/backend && grep google/uuid go.mod`
If not present, run: `go get github.com/google/uuid`

- [ ] **Step 4: Verify backend compiles**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: compiles with no errors

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/plan.go backend/cmd/server/main.go backend/go.mod backend/go.sum
git commit -m "feat: add POST /api/plans endpoint for frontend-created plans"
```

---

### Task 4: Add createPlan API function to frontend

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add createPlan function**

In `api.ts` after the `getPlan` function (after line 203), add:

```typescript
export async function createPlan(params: {
  reasoning: string;
  actions: { type: string; params: Record<string, unknown> }[];
}): Promise<Plan> {
  const res = await fetch(`${BASE}/plans`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  if (!res.ok) throw new Error("Failed to create plan");
  return res.json();
}
```

- [ ] **Step 2: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat: add createPlan API function for frontend-created plans"
```

---

### Task 5: Add right-side panel tab layout with OperationQueue

**Files:**
- Create: `frontend/src/components/OperationQueue.tsx`
- Modify: `frontend/src/components/PageViewer.tsx`
- Modify: `frontend/src/components/WikiPage.tsx`

- [ ] **Step 1: Create OperationQueue component**

Create `frontend/src/components/OperationQueue.tsx`:

```typescript
import type { Plan, ExecutionReport } from "../types";
import { confirmPlan, rejectPlan } from "../lib/api";

interface OperationQueueProps {
  plans: Plan[];
  onPlanConfirmed: (planId: string, report: ExecutionReport) => void;
  onPlanRejected: (planId: string) => void;
}

const ACTION_TYPE_LABELS: Record<string, { label: string; color: string }> = {
  create_page: { label: "创建", color: "bg-green-100 text-green-700" },
  update_page: { label: "更新", color: "bg-amber-100 text-amber-700" },
  delete_page: { label: "删除", color: "bg-red-100 text-red-700" },
  link_pages: { label: "链接", color: "bg-blue-100 text-blue-700" },
  move_page: { label: "移动", color: "bg-purple-100 text-purple-700" },
};

export function OperationQueue({ plans, onPlanConfirmed, onPlanRejected }: OperationQueueProps) {
  if (plans.length === 0) {
    return (
      <div className="h-full flex items-center justify-center text-th-text-muted">
        <p className="text-sm">没有待确认的操作</p>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto p-4 space-y-3">
      {plans.map((plan) => (
        <OperationCard
          key={plan.id}
          plan={plan}
          onConfirm={async (planId) => {
            const report = await confirmPlan(planId);
            onPlanConfirmed(planId, report);
          }}
          onReject={async (planId) => {
            await rejectPlan(planId);
            onPlanRejected(planId);
          }}
        />
      ))}
    </div>
  );
}

function OperationCard({
  plan,
  onConfirm,
  onReject,
}: {
  plan: Plan;
  onConfirm: (planId: string) => void;
  onReject: (planId: string) => void;
}) {
  const isUserPlan = plan.conversation_id === 0;
  const sourceLabel = isUserPlan ? "用户操作" : "AI 计划";
  const sourceColor = isUserPlan ? "bg-gray-100 text-gray-600" : "bg-amber-100 text-amber-700";

  return (
    <div className="border border-th-border rounded-lg p-3 space-y-2">
      <div className="flex items-center gap-2">
        <span className={`text-xs font-medium px-1.5 py-0.5 rounded ${sourceColor}`}>
          {sourceLabel}
        </span>
        {plan.actions.length > 1 && (
          <span className="text-xs text-th-text-muted">{plan.actions.length} 个操作</span>
        )}
      </div>
      <div className="text-sm text-th-text-primary">{plan.reasoning}</div>
      {plan.actions.length > 0 && (
        <div className="text-xs text-th-text-muted pl-2 border-l-2 border-th-border space-y-0.5">
          {plan.actions.map((action) => {
            const typeInfo = ACTION_TYPE_LABELS[action.type] || { label: action.type, color: "bg-gray-100 text-gray-600" };
            const title = (action.params.title as string) || (action.params.page_id ? `页面 #${action.params.page_id}` : action.type);
            return (
              <div key={action.id} className="flex items-center gap-1.5">
                <span className={`text-xs font-medium px-1 py-0 rounded ${typeInfo.color}`}>{typeInfo.label}</span>
                <span>{title}</span>
              </div>
            );
          })}
        </div>
      )}
      <div className="flex gap-2 pt-1">
        <button
          onClick={() => onConfirm(plan.id)}
          disabled={plan.status !== "pending"}
          className="px-3 py-1 text-xs font-medium bg-th-accent text-white rounded hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          确认执行
        </button>
        <button
          onClick={() => onReject(plan.id)}
          disabled={plan.status !== "pending"}
          className="px-3 py-1 text-xs font-medium bg-th-bg-tertiary text-th-text-secondary rounded hover:bg-th-bg-primary disabled:opacity-50 disabled:cursor-not-allowed"
        >
          拒绝
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Modify PageViewer to support tab layout**

In `PageViewer.tsx`, add tab state and OperationQueue integration. Replace the component with tab-aware rendering:

Add import at top:
```typescript
import { OperationQueue } from "./OperationQueue";
import type { ExecutionReport, Plan } from "../types";
```

Add to `PageViewerProps`:
```typescript
interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
  plan: Plan | null;
  pendingPlans: Plan[];
  onConfirmPlan: (planId: string) => void;
  onRejectPlan: (planId: string) => void;
  confirmingPlan: boolean;
  onSelectPage: (slug: string) => void;
  onAskAI: (text: string, pageTitle: string) => void;
  onPlanConfirmed: (planId: string, report: ExecutionReport) => void;
  onPlanRejected: (planId: string) => void;
}
```

Add tab state inside the component:
```typescript
const [activeTab, setActiveTab] = useState<"content" | "operations">("content");
```

Add tab auto-switch effect:
```typescript
useEffect(() => {
  if (pendingPlans.length > 0 && activeTab === "content") {
    setActiveTab("operations");
  }
}, [pendingPlans.length]);
```

Replace the main render with tab layout. The header area gets two tab buttons:

```tsx
return (
  <div className="h-full flex flex-col bg-th-bg-primary">
    {/* Tab bar */}
    <div className="flex border-b border-th-border bg-th-bg-secondary/50 shrink-0">
      <button
        onClick={() => setActiveTab("content")}
        className={`px-4 py-2 text-xs font-medium transition-colors ${
          activeTab === "content"
            ? "text-th-accent border-b-2 border-th-accent"
            : "text-th-text-muted hover:text-th-text-secondary"
        }`}
      >
        页面内容
      </button>
      <button
        onClick={() => setActiveTab("operations")}
        className={`px-4 py-2 text-xs font-medium transition-colors relative ${
          activeTab === "operations"
            ? "text-th-accent border-b-2 border-th-accent"
            : "text-th-text-muted hover:text-th-text-secondary"
        }`}
      >
        待确认操作
        {pendingPlans.length > 0 && (
          <span className="absolute -top-0.5 -right-1 bg-red-500 text-white rounded-full w-4 h-4 text-[10px] flex items-center justify-center">
            {pendingPlans.length}
          </span>
        )}
      </button>
    </div>

    {/* Tab content */}
    <div className="flex-1 overflow-hidden">
      {activeTab === "content" ? (
        plan ? (
          <PlanPreview plan={plan} onConfirm={onConfirmPlan} onReject={onRejectPlan} confirming={confirmingPlan} />
        ) : (
          page ? (
            <div className="h-full overflow-y-auto">
              {/* Render page content here - same as current PageViewer rendering */}
              <div className="p-6">
                <div className="flex items-center gap-2 mb-4">
                  <h2 className="text-xl font-semibold text-th-text-primary font-display">{page.title}</h2>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${
                    page.content_status === 'published' ? 'bg-green-100 text-green-700'
                    : page.content_status === 'draft' ? 'bg-amber-100 text-amber-700'
                    : 'bg-gray-100 text-gray-500'
                  }`}>{page.content_status}</span>
                </div>
                <MarkdownContent content={page.content} />
              </div>
            </div>
          ) : (
            <div className="h-full flex items-center justify-center text-th-text-muted">
              <p>选择页面查看内容</p>
            </div>
          )
        )
      ) : (
        <OperationQueue
          plans={pendingPlans}
          onPlanConfirmed={onPlanConfirmed}
          onPlanRejected={onPlanRejected}
        />
      )}
    </div>
  </div>
);
```

- [ ] **Step 3: Update WikiPage to manage pending plans**

In `WikiPage.tsx`, add state for pending plans and wire up the new props:

Add import:
```typescript
import { createPlan } from "../lib/api";
```

Add state:
```typescript
const [pendingPlans, setPendingPlans] = useState<Plan[]>([]);
```

Update `handlePlanCreated` to add to pending plans instead of replacing activePlan:
```typescript
const handlePlanCreated = (plan: Plan) => {
  setPendingPlans(prev => [...prev, plan]);
};
```

Update `handleConfirmPlan` to remove from pending plans and refresh:
```typescript
const handleConfirmPlan = async (planId: string) => {
  setConfirmingPlan(true);
  try {
    const report = await confirmPlan(planId);
    setPendingPlans(prev => prev.filter(p => p.id !== planId));
    handlePageChanged();
    mutateCurrentPage();
  } catch (err) {
    console.error("Plan confirmation failed:", err);
  } finally {
    setConfirmingPlan(false);
  }
};
```

Update `handleRejectPlan`:
```typescript
const handleRejectPlan = async (planId: string) => {
  try {
    await rejectPlan(planId);
    setPendingPlans(prev => prev.filter(p => p.id !== planId));
  } catch (err) {
    console.error("Plan rejection failed:", err);
  }
};
```

Add `mutateCurrentPage` helper for refreshing current page content:
```typescript
const { data: page, mutate: mutatePage } = useSWR(
  selectedSlug ? `wiki-page-${selectedSlug}` : null,
  () => selectedSlug ? fetchWikiPage(selectedSlug) : null
);

const mutateCurrentPage = useCallback(() => {
  mutatePage();
}, [mutatePage]);
```

Remove `activePlan` state (replaced by `pendingPlans`).

Update PageViewer props:
```tsx
<PageViewer
  page={displayPage}
  collapsed={rightCollapsed}
  pendingPlans={pendingPlans}
  onConfirmPlan={handleConfirmPlan}
  onRejectPlan={handleRejectPlan}
  confirmingPlan={confirmingPlan}
  onSelectPage={(slug) => setSelectedSlug(slug)}
  onAskAI={handleAskAI}
  onPlanConfirmed={handlePlanConfirmed}
  onPlanRejected={handleRejectPlan}
/>
```

Add `handlePlanConfirmed`:
```typescript
const handlePlanConfirmed = (planId: string, report: ExecutionReport) => {
  setPendingPlans(prev => prev.filter(p => p.id !== planId));
  handlePageChanged();
  mutateCurrentPage();
};
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/OperationQueue.tsx frontend/src/components/PageViewer.tsx frontend/src/components/WikiPage.tsx
git commit -m "feat: add tab layout with operation queue for unified plan confirmation"
```

---

## Phase 2: Feedback and Status Visibility

### Task 6: Add execution result display to OperationQueue

**Files:**
- Modify: `frontend/src/components/OperationQueue.tsx`

- [ ] **Step 1: Add ExecutionResultCard component**

Add to `OperationQueue.tsx`:

```typescript
interface ExecutionResultCardProps {
  report: ExecutionReport;
  onViewPage?: (slug: string) => void;
  onRetry?: (planId: string) => void;
}

function ExecutionResultCard({ report, onViewPage, onRetry }: ExecutionResultCardProps) {
  const isSuccess = report.status === "completed";
  const isPartial = report.status === "completed_with_failures";
  const failedActions = report.actions.filter(a => a.status === "failed");

  const bgClass = isSuccess
    ? "bg-green-50 border-green-200"
    : isPartial
    ? "bg-amber-50 border-amber-200"
    : "bg-red-50 border-red-200";

  const iconClass = isSuccess ? "text-green-600" : isPartial ? "text-amber-600" : "text-red-600";
  const textClass = isSuccess ? "text-green-800" : isPartial ? "text-amber-800" : "text-red-800";

  return (
    <div className={`border rounded-lg p-3 space-y-2 ${bgClass}`}>
      <div className="flex items-center gap-2">
        <span className={iconClass}>{isSuccess ? "✓" : isPartial ? "⚠" : "✗"}</span>
        <span className={`text-sm font-medium ${textClass}`}>
          {isSuccess ? "执行成功" : isPartial ? "部分失败" : "执行失败"}
        </span>
      </div>
      {isPartial && (
        <div className="text-xs text-amber-700">
          {report.actions.filter(a => a.status === "completed").length}/{report.actions.length} 操作成功
        </div>
      )}
      {failedActions.length > 0 && (
        <div className="space-y-1">
          {failedActions.map(action => (
            <div key={action.id} className="text-xs bg-white rounded p-1.5 text-red-700">
              ✗ {action.type}: {action.error || "未知错误"}
            </div>
          ))}
        </div>
      )}
      {isSuccess && onViewPage && report.actions.find(a => a.result?.slug) && (
        <button
          onClick={() => onViewPage(report.actions.find(a => a.result?.slug)!.result!.slug as string)}
          className="text-xs text-th-accent underline"
        >
          查看新页面
        </button>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Add execution results state to OperationQueue**

Update `OperationQueueProps`:

```typescript
interface OperationQueueProps {
  plans: Plan[];
  executionResults: Map<string, ExecutionReport>;
  onPlanConfirmed: (planId: string, report: ExecutionReport) => void;
  onPlanRejected: (planId: string) => void;
  onViewPage?: (slug: string) => void;
}
```

Update the component to show execution results alongside pending plans:

```typescript
export function OperationQueue({ plans, executionResults, onPlanConfirmed, onPlanRejected, onViewPage }: OperationQueueProps) {
  return (
    <div className="h-full overflow-y-auto p-4 space-y-3">
      {plans.map((plan) => (
        <OperationCard
          key={plan.id}
          plan={plan}
          onConfirm={async (planId) => {
            const report = await confirmPlan(planId);
            onPlanConfirmed(planId, report);
          }}
          onReject={async (planId) => {
            await rejectPlan(planId);
            onPlanRejected(planId);
          }}
        />
      ))}
      {Array.from(executionResults.entries()).map(([planId, report]) => (
        <ExecutionResultCard
          key={`result-${planId}`}
          report={report}
          onViewPage={onViewPage}
        />
      ))}
      {plans.length === 0 && executionResults.size === 0 && (
        <div className="h-full flex items-center justify-center text-th-text-muted">
          <p className="text-sm">没有待确认的操作</p>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Wire up execution results in WikiPage**

In `WikiPage.tsx`, add state:

```typescript
const [executionResults, setExecutionResults] = useState<Map<string, ExecutionReport>>(new Map());
```

Update `handlePlanConfirmed` and `handleConfirmPlan` to store results:

```typescript
const handlePlanConfirmed = (planId: string, report: ExecutionReport) => {
  setPendingPlans(prev => prev.filter(p => p.id !== planId));
  setExecutionResults(prev => new Map(prev).set(planId, report));
  handlePageChanged();
  mutateCurrentPage();
};
```

Pass to PageViewer:
```tsx
executionResults={executionResults}
onViewPage={(slug) => setSelectedSlug(slug)}
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/OperationQueue.tsx frontend/src/components/WikiPage.tsx frontend/src/components/PageViewer.tsx
git commit -m "feat: add execution result display with success/partial/failure states"
```

---

### Task 7: Fix SSE error handling and agent progress

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: Add onError callback to streamChat**

In `api.ts`, update `streamChat` signature:

```typescript
export async function streamChat(
  req: ChatRequest,
  onChunk: (content: string) => void,
  onMeta: (data: { conversation_id?: number; plan?: Plan }) => void,
  onStatus?: (data: { step: number; max_steps: number; status: string }) => void,
  onError?: (error: string) => void,
): Promise<void> {
```

In the `dispatchEvent` function, add error handling:

```typescript
// BEFORE (line 136):
// Unknown events (done, error, etc.) — ignore
currentEvent = "";
currentData = "";

// AFTER:
if (currentEvent === "error" && onError) {
  try { onError(currentData); } catch { /* ignore */ }
  currentEvent = "";
  currentData = "";
  return;
}
// Unknown events (done, etc.) — ignore
currentEvent = "";
currentData = "";
```

- [ ] **Step 2: Add error state and handling in ChatPanel**

In `ChatPanel.tsx`, add error state:

```typescript
const [streamError, setStreamError] = useState<string | null>(null);
```

Update `handleSend` to pass `onError`:

```typescript
await streamChat(
  chatReq,
  onChunk,
  onMeta,
  onStatus,
  (error) => {
    setStreamError(error);
    setLoading(false);
  },
);
```

Clear error on new send:
```typescript
// At the start of handleSend:
setStreamError(null);
```

Add error display in the chat area, after the messages list:

```tsx
{streamError && (
  <div className="mx-4 mb-2 p-2 bg-red-50 border border-red-200 rounded text-xs text-red-700">
    {streamError}
    <button onClick={() => setStreamError(null)} className="ml-2 underline">关闭</button>
  </div>
)}
```

- [ ] **Step 3: Fix agent progress maxSteps**

In `ChatPanel.tsx` line 190, change:

```typescript
// BEFORE:
setAgentStatus({ step: 0, maxSteps: 20, running: true });

// AFTER:
setAgentStatus({ step: 0, maxSteps: 10, running: true });
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/api.ts frontend/src/components/ChatPanel.tsx
git commit -m "fix: handle SSE error events and correct agent progress maxSteps to 10"
```

---

### Task 8: Add auto-refresh of current page after plan confirmation

**Files:**
- Modify: `frontend/src/components/WikiPage.tsx`

- [ ] **Step 1: Ensure mutateCurrentPage is called after confirmation**

This is already done in Task 5 Step 3 where `mutateCurrentPage()` is called in `handlePlanConfirmed` and `handleConfirmPlan`. Verify the SWR mutate is properly wired:

```typescript
const { data: page, mutate: mutatePage } = useSWR(
  selectedSlug ? `wiki-page-${selectedSlug}` : null,
  () => selectedSlug ? fetchWikiPage(selectedSlug) : null
);

const mutateCurrentPage = useCallback(() => {
  mutatePage();
}, [mutatePage]);
```

- [ ] **Step 2: Add overview page refresh after confirmation**

Add mutate for overview page:

```typescript
const { data: overviewPage, mutate: mutateOverview } = useSWR(
  !selectedSlug ? 'wiki-overview' : null,
  fetchOverviewPage
);
```

Update `handlePageChanged` to also refresh overview:

```typescript
const handlePageChanged = useCallback(() => {
  setTreeVersion(v => v + 1);
  mutateOverview();
}, [mutateOverview]);
```

- [ ] **Step 3: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/WikiPage.tsx
git commit -m "fix: auto-refresh current page and overview after plan confirmation"
```

---

## Phase 3: Knowledge Tree Direct Operations

### Task 9: Add right-click context menu to knowledge tree

**Files:**
- Create: `frontend/src/components/TreeNodeMenu.tsx`
- Modify: `frontend/src/components/KnowledgeTree.tsx`

- [ ] **Step 1: Create TreeNodeMenu component**

Create `frontend/src/components/TreeNodeMenu.tsx`:

```typescript
import { useEffect, useRef } from "react";

interface TreeNodeMenuProps {
  x: number;
  y: number;
  nodeId: number;
  nodeTitle: string;
  hasChildren: boolean;
  onAddChild: (parentId: number) => void;
  onRename: (nodeId: number, currentTitle: string) => void;
  onMove: (nodeId: number, newParentId: number | null) => void;
  onDelete: (nodeId: number, hasChildren: boolean) => void;
  onClose: () => void;
}

export function TreeNodeMenu({
  x, y, nodeId, nodeTitle, hasChildren,
  onAddChild, onRename, onMove, onDelete, onClose,
}: TreeNodeMenuProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [onClose]);

  // Adjust position to stay within viewport
  const adjustedX = Math.min(x, window.innerWidth - 180);
  const adjustedY = Math.min(y, window.innerHeight - 200);

  return (
    <div
      ref={ref}
      className="fixed z-50 bg-th-bg-secondary border border-th-border rounded-lg shadow-lg py-1 min-w-[160px]"
      style={{ left: adjustedX, top: adjustedY }}
    >
      <button
        onClick={() => { onAddChild(nodeId); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
      >
        <span className="text-th-text-muted">+</span> 添加子页面
      </button>
      <button
        onClick={() => { onRename(nodeId, nodeTitle); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
      >
        <span className="text-th-text-muted">✎</span> 重命名
      </button>
      <button
        onClick={() => { onMove(nodeId, null); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
      >
        <span className="text-th-text-muted">↗</span> 移动到...
      </button>
      <div className="border-t border-th-border my-1" />
      <button
        onClick={() => { onDelete(nodeId, hasChildren); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 flex items-center gap-2"
      >
        <span>🗑</span> {hasChildren ? "删除子树" : "删除页面"}
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Update KnowledgeTree to support context menu and callbacks**

In `KnowledgeTree.tsx`, update props:

```typescript
interface KnowledgeTreeProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  collapsed: boolean;
  onAddChild?: (parentId: number) => void;
  onRename?: (nodeId: number, currentTitle: string) => void;
  onMove?: (nodeId: number) => void;
  onDelete?: (nodeId: number, hasChildren: boolean) => void;
}
```

Add context menu state:

```typescript
const [menuState, setMenuState] = useState<{
  x: number; y: number; nodeId: number; nodeTitle: string; hasChildren: boolean;
} | null>(null);
```

Add context menu handler:

```typescript
const handleContextMenu = (e: React.MouseEvent, node: WikiTreeNode) => {
  e.preventDefault();
  e.stopPropagation();
  setMenuState({
    x: e.clientX,
    y: e.clientY,
    nodeId: node.id,
    nodeTitle: node.title,
    hasChildren: !!(node.children && node.children.length > 0),
  });
};
```

Pass `onContextMenu` to each `TreeNode`:

```tsx
<TreeNode
  key={node.id}
  node={node}
  selectedSlug={selectedSlug}
  onSelect={onSelect}
  depth={0}
  onContextMenu={handleContextMenu}
/>
```

Update `TreeNodeProps`:

```typescript
interface TreeNodeProps {
  node: WikiTreeNode;
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  depth: number;
  onContextMenu?: (e: React.MouseEvent, node: WikiTreeNode) => void;
}
```

Add `onContextMenu` to the node div click handler.

Render the menu at the end of the KnowledgeTree component:

```tsx
{menuState && (
  <TreeNodeMenu
    x={menuState.x}
    y={menuState.y}
    nodeId={menuState.nodeId}
    nodeTitle={menuState.nodeTitle}
    hasChildren={menuState.hasChildren}
    onAddChild={onAddChild || (() => {})}
    onRename={onRename || (() => {})}
    onMove={onMove || (() => {})}
    onDelete={onDelete || (() => {})}
    onClose={() => setMenuState(null)}
  />
)}
```

- [ ] **Step 3: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/TreeNodeMenu.tsx frontend/src/components/KnowledgeTree.tsx
git commit -m "feat: add right-click context menu to knowledge tree nodes"
```

---

### Task 10: Add quick-add child button and hover menu trigger

**Files:**
- Modify: `frontend/src/components/KnowledgeTree.tsx`

- [ ] **Step 1: Add hover state and ⋯ button to TreeNode**

In `KnowledgeTree.tsx`, add hover state to `TreeNode`:

```typescript
const [hovered, setHovered] = useState(false);
```

Add hover handlers to the node div:

```tsx
<div
  className={`flex items-center gap-1.5 px-2 py-1.5 rounded-md cursor-pointer transition-all duration-150 ${
    isSelected
      ? 'bg-th-accent-bg text-th-accent font-medium shadow-sm'
      : 'text-th-text-primary hover:bg-th-bg-primary hover:text-th-text-primary'
  }`}
  style={{ paddingLeft: `${depth * 16 + 8}px` }}
  onClick={handleClick}
  onMouseEnter={() => setHovered(true)}
  onMouseLeave={() => setHovered(false)}
  onContextMenu={(e) => onContextMenu?.(e, node)}
>
  {/* ... existing expand/collapse and status dot ... */}
  <span className={`flex-1 text-sm truncate ${isSelected ? 'font-medium' : ''}`}>
    {node.title}
  </span>
  {node.page_type === 'overview' && (
    <span className="text-xs text-th-text-muted shrink-0 font-mono tracking-tight">概览</span>
  )}
  {/* Hover menu trigger */}
  {hovered && onContextMenu && (
    <button
      onClick={(e) => { e.stopPropagation(); onContextMenu(e, node); }}
      className="w-4 h-4 flex items-center justify-center text-th-text-muted hover:text-th-text-secondary shrink-0"
    >
      ⋯
    </button>
  )}
</div>
```

- [ ] **Step 2: Add quick-add child button after expanded children**

After the children rendering block, add:

```tsx
{expanded && onAddChild && (
  <button
    onClick={() => onAddChild(node.id)}
    className="flex items-center gap-1.5 px-2 py-1 rounded-md text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-primary transition-colors border border-dashed border-transparent hover:border-th-border"
    style={{ paddingLeft: `${(depth + 1) * 16 + 8}px` }}
  >
    <span className="w-4 shrink-0" />
    <span className="text-xs">+ 添加子页面</span>
  </button>
)}
```

- [ ] **Step 3: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/KnowledgeTree.tsx
git commit -m "feat: add hover menu trigger and quick-add child button to tree nodes"
```

---

### Task 11: Add inline rename to knowledge tree

**Files:**
- Modify: `frontend/src/components/KnowledgeTree.tsx`

- [ ] **Step 1: Add inline editing state to TreeNode**

In `TreeNode` component, add:

```typescript
const [editing, setEditing] = useState(false);
const [editTitle, setEditTitle] = useState(node.title);
const inputRef = useRef<HTMLInputElement>(null);
```

Add effect to focus input when editing starts:

```typescript
useEffect(() => {
  if (editing && inputRef.current) {
    inputRef.current.focus();
    inputRef.current.select();
  }
}, [editing]);
```

- [ ] **Step 2: Add double-click handler for rename**

```typescript
const handleDoubleClick = (e: React.MouseEvent) => {
  e.stopPropagation();
  if (node.page_type === 'overview') return;
  setEditing(true);
  setEditTitle(node.title);
};
```

Add `onDoubleClick={handleDoubleClick}` to the node div.

- [ ] **Step 3: Add inline edit input rendering**

Replace the title span with conditional rendering:

```tsx
{editing ? (
  <input
    ref={inputRef}
    type="text"
    value={editTitle}
    onChange={(e) => setEditTitle(e.target.value)}
    onKeyDown={(e) => {
      if (e.key === 'Enter' && editTitle.trim() && editTitle.trim() !== node.title) {
        onRename?.(node.id, editTitle.trim());
        setEditing(false);
      }
      if (e.key === 'Escape') {
        setEditing(false);
        setEditTitle(node.title);
      }
    }}
    onBlur={() => {
      setEditing(false);
      setEditTitle(node.title);
    }}
    className="flex-1 text-sm bg-th-bg-primary border border-th-accent rounded px-1 py-0 outline-none"
    onClick={(e) => e.stopPropagation()}
  />
) : (
  <span className={`flex-1 text-sm truncate ${isSelected ? 'font-medium' : ''}`} onDoubleClick={handleDoubleClick}>
    {node.title}
  </span>
)}
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/KnowledgeTree.tsx
git commit -m "feat: add inline rename with double-click on tree nodes"
```

---

### Task 12: Wire up tree operations in WikiPage

**Files:**
- Modify: `frontend/src/components/WikiPage.tsx`
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add API functions for tree operations**

In `api.ts`, add:

```typescript
export async function createEmptyWikiPage(title: string, parentId: number | null): Promise<WikiPage> {
  const res = await fetch(`${BASE}/wiki/quick-create`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title, parent_id: parentId }),
  });
  if (!res.ok) throw new Error("Failed to create page");
  return res.json();
}

export async function renameWikiPage(pageId: number, newTitle: string): Promise<void> {
  const res = await fetch(`${BASE}/wiki/${pageId}/rename`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title: newTitle }),
  });
  if (!res.ok) throw new Error("Failed to rename page");
}

export async function moveWikiPage(pageId: number, newParentId: number | null): Promise<void> {
  const res = await fetch(`${BASE}/wiki/${pageId}/move`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ parent_id: newParentId }),
  });
  if (!res.ok) throw new Error("Failed to move page");
}
```

- [ ] **Step 2: Add tree operation handlers in WikiPage**

In `WikiPage.tsx`, add imports:

```typescript
import { createEmptyWikiPage, renameWikiPage, moveWikiPage, createPlan } from "../lib/api";
```

Add handlers:

```typescript
const handleAddChild = async (parentId: number) => {
  const title = "新页面";
  try {
    await createEmptyWikiPage(title, parentId);
    handlePageChanged();
  } catch (err) {
    console.error("Failed to add child page:", err);
  }
};

const handleRename = async (nodeId: number, newTitle: string) => {
  try {
    await renameWikiPage(nodeId, newTitle);
    handlePageChanged();
    mutateCurrentPage();
  } catch (err) {
    console.error("Failed to rename page:", err);
  }
};

const handleMove = async (nodeId: number, newParentId: number | null) => {
  if (newParentId === null) {
    // "Move to..." context menu item - prompt user via chat for target
    chatPanelRef.current?.setSelectedText(`请将页面 ID ${nodeId} 移动到合适的位置`, "");
    return;
  }
  try {
    await moveWikiPage(nodeId, newParentId);
    handlePageChanged();
  } catch (err) {
    console.error("Failed to move page:", err);
  }
};

const handleDelete = async (nodeId: number, hasChildren: boolean) => {
  try {
    const plan = await createPlan({
      reasoning: hasChildren ? `删除页面及其子树` : `删除页面`,
      actions: [{
        type: "delete_page",
        params: { page_id: nodeId },
      }],
    });
    setPendingPlans(prev => [...prev, plan]);
  } catch (err) {
    console.error("Failed to create delete plan:", err);
  }
};
```

- [ ] **Step 3: Pass handlers to KnowledgeTree**

Update KnowledgeTree props in WikiPage:

```tsx
<KnowledgeTree
  tree={tree || []}
  selectedSlug={selectedSlug}
  onSelect={setSelectedSlug}
  collapsed={leftCollapsed}
  onAddChild={handleAddChild}
  onRename={handleRename}
  onMove={handleMove}
  onDelete={handleDelete}
/>
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/WikiPage.tsx frontend/src/lib/api.ts
git commit -m "feat: wire up tree operations - add child, rename, delete via plan"
```

---

### Task 13: Add drag-and-drop move to knowledge tree

**Files:**
- Modify: `frontend/src/components/KnowledgeTree.tsx`
- Modify: `frontend/src/components/WikiPage.tsx`

- [ ] **Step 1: Add drag-and-drop state and handlers to KnowledgeTree**

In `KnowledgeTree.tsx`, add props:

```typescript
interface KnowledgeTreeProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  collapsed: boolean;
  onAddChild?: (parentId: number) => void;
  onRename?: (nodeId: number, currentTitle: string) => void;
  onMove?: (nodeId: number, newParentId: number | null) => void;
  onDelete?: (nodeId: number, hasChildren: boolean) => void;
}
```

Add drag state to `KnowledgeTree`:

```typescript
const [dragState, setDragState] = useState<{
  draggedId: number | null;
  dropTargetId: number | null;
  dropPosition: "on" | "before" | "after" | null;
}>({ draggedId: null, dropTargetId: null, dropPosition: null });
```

Pass drag state and handlers down to TreeNode components.

- [ ] **Step 2: Add drag handlers to TreeNode**

In `TreeNode`, add:

```typescript
const handleDragStart = (e: React.DragEvent) => {
  e.dataTransfer.setData("text/plain", String(node.id));
  e.dataTransfer.effectAllowed = "move";
};

const handleDragOver = (e: React.DragEvent) => {
  e.preventDefault();
  e.dataTransfer.dropEffect = "move";
};

const handleDrop = (e: React.DragEvent) => {
  e.preventDefault();
  const draggedId = parseInt(e.dataTransfer.getData("text/plain"), 10);
  if (draggedId !== node.id) {
    onMove?.(draggedId, node.id);
  }
};
```

Add `draggable`, `onDragStart`, `onDragOver`, `onDrop` to the node div.

- [ ] **Step 3: Update handleMove in WikiPage**

```typescript
const handleMove = async (nodeId: number, newParentId: number | null) => {
  try {
    await moveWikiPage(nodeId, newParentId);
    handlePageChanged();
  } catch (err) {
    console.error("Failed to move page:", err);
  }
};
```

- [ ] **Step 4: Verify frontend compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/KnowledgeTree.tsx frontend/src/components/WikiPage.tsx
git commit -m "feat: add drag-and-drop move to knowledge tree"
```

---

### Task 14: End-to-end verification

**Files:**
- No new files

- [ ] **Step 1: Start backend**

Run: `cd /Users/irving/repo/learn-helper/backend && go run ./cmd/server`

- [ ] **Step 2: Start frontend**

Run: `cd /Users/irving/repo/learn-helper/frontend && npm run dev`

- [ ] **Step 3: Test legacy cleanup**

1. Open http://localhost:3000/wiki
2. Send a chat message that triggers AI to create a plan
3. Verify: plan appears in right panel "Pending Operations" tab
4. Verify: no inline pending_actions confirmation UI appears
5. Verify: confirming the plan shows execution result card

- [ ] **Step 4: Test tree operations**

1. Right-click a tree node → verify context menu appears
2. Click "Add child page" → verify new empty page appears in tree
3. Double-click a node title → verify inline edit mode
4. Type new name, press Enter → verify page renamed
5. Right-click → "Delete page" → verify plan created in operation queue
6. Confirm delete → verify page removed from tree
7. Drag a node onto another → verify page moved

- [ ] **Step 5: Test feedback**

1. Trigger a plan that will succeed → verify green result card
2. Verify current page content refreshes after confirmation
3. Verify tree refreshes after operations
4. Verify SSE error shows error banner in chat

- [ ] **Step 6: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address issues found during end-to-end verification"
```
