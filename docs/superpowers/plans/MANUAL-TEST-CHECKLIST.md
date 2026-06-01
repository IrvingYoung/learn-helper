# Plan Tool Redesign — Manual Smoke Test Checklist

Final end-to-end verification of the plan tool redesign. Backend builds and smoke
tests are automated; the scenarios below require a human in a browser.

**Run order:** Tests 1-4 in a single chat session, then 5-6 in a fresh session, then
7-10 in any order.

## Pre-flight

- [ ] Backend running: `cd backend && go run ./cmd/server` (port 8080)
- [ ] Frontend running: `cd frontend && npm run dev` (port 3000)
- [ ] Open `http://localhost:3000` in browser
- [ ] Start a new chat conversation

---

## Test 1: Read tools auto-execute (no permission prompt)

**Goal:** Read-only tools must execute silently, no permission card.

- [ ] In the chat, type: `what pages do I have?`
- [ ] Observe: AI calls `search_pages` and/or `lookup_page`
- [ ] Observe: NO permission prompt appears in the right panel
- [ ] AI lists existing pages in the chat

**Pass criteria:** AI responds with a list. No right-panel prompt.

---

## Test 2: Single write tool — explicit permission

**Goal:** One write tool should show a single-item permission prompt with an "Approve" button.

- [ ] Type: `create a page called Test in Go category`
- [ ] Observe: Right panel shows ONE permission card for `create_page`
- [ ] Click "Approve"
- [ ] Observe: Page is created. Tool call card turns green/checkmark
- [ ] Verify the page exists in the wiki tree

**Pass criteria:** Single card, approve creates page, no extra prompts.

---

## Test 3: Batched writes — multiple items in one prompt

**Goal:** Multiple write tools in one AI turn should batch into one permission prompt.

- [ ] Type: `create pages A, B, and C under the Go category`
- [ ] Observe: Right panel shows THREE permission cards OR one card listing 3 items
- [ ] Click "Approve" (or "Approve All")
- [ ] Observe: All 3 pages get created
- [ ] Verify A, B, C exist in the wiki tree

**Pass criteria:** Single batch prompt, all 3 created on approval.

---

## Test 4: Reject a batch — AI responds conversationally

**Goal:** Rejecting a write should NOT create pages. AI should acknowledge.

- [ ] Type: `create pages X, Y, Z under Go`
- [ ] Observe: 3-item permission prompt appears
- [ ] Click "Reject"
- [ ] Observe: Tool call cards turn red/x
- [ ] AI should respond in chat with something like "Okay, I won't create those pages"
- [ ] Verify X, Y, Z do NOT exist in the wiki tree

**Pass criteria:** Reject works, AI acknowledges, no pages created.

---

## Test 5: ask_user with no context — renders in chat

**Goal:** `ask_user` with no diff context should show an inline card in the chat, not the right panel.

- [ ] Type: `what should I learn about Rust?`
- [ ] Observe: An `ask_user` card appears INLINE in the chat (not in the right panel)
- [ ] The card shows 2-4 multiple-choice options
- [ ] Click one option
- [ ] AI continues the conversation based on the choice

**Pass criteria:** Card in chat, options work, AI follows up.

---

## Test 6: ask_user with diff context — renders in right panel

**Goal:** `ask_user` triggered by a `patch_page` / `update_page` should show a diff in the right panel.

- [ ] Type: `rewrite the introduction of the Test page to be more casual`
- [ ] Observe: Right panel shows an `ask_user` card with an inline diff (old vs new)
- [ ] The diff shows before/after of the introduction
- [ ] Card has "Apply" and "Reject" buttons
- [ ] Click "Apply" — page is updated with the new introduction
- [ ] Click "Reject" (test separately) — page is unchanged

**Pass criteria:** Diff is visible, Apply updates page, Reject does not.

---

## Test 7: SSE disconnect — server cleans up

**Goal:** Closing the browser mid-permission-prompt must not leak the pending request.

- [ ] Type: `create a page called Disconnect Test`
- [ ] Wait for the permission prompt to appear
- [ ] DO NOT click Approve or Reject
- [ ] Close the browser tab (or navigate away)
- [ ] On the server, check logs: no orphan goroutines, request is cancelled
- [ ] Restart browser, open chat — no stale prompts visible

**Pass criteria:** Server cleans up. No errors in server log on disconnect.

---

## Test 8: propose_plan must never appear

**Goal:** `propose_plan` was removed in this redesign. It must not be callable.

- [ ] Type: `use the propose_plan tool to plan my wiki`
- [ ] Observe: AI should respond that no such tool is available, or that it can't plan like that
- [ ] Check the right panel — no `propose_plan` card ever appears
- [ ] (Optional dev check) `grep -c propose_plan backend/internal/ai/provider.go` returns 1 (only the comment)

**Pass criteria:** AI acknowledges the tool doesn't exist. No crashes.

---

## Test 9: Knowledge map still works

**Goal:** Unrelated feature (knowledge map) must not be regressed.

- [ ] Navigate to the knowledge map view
- [ ] Observe: All existing pages render as nodes
- [ ] Click a node — page opens correctly
- [ ] Drag a node — repositions
- [ ] No errors in browser console

**Pass criteria:** Map loads, navigation works, no console errors.

---

## Test 10: Historical messages render

**Goal:** Old chat conversations must still load and display correctly.

- [ ] Open an existing chat from the sidebar (any old conversation)
- [ ] Observe: All historical messages load
- [ ] Observe: Old tool calls (if any) render as collapsed/greyed cards
- [ ] Observe: No permission prompts from history (they were already resolved)
- [ ] Type a new message — it sends, AI responds

**Pass criteria:** History loads cleanly, new messages work.

---

## Sign-off

When all 10 tests pass:

- [ ] Mark Task 9.1 as completed in the plan
- [ ] (Optional) Run the plan-to-spec archive flow if design wants the change persisted
- [ ] Note any deviations or bugs found for follow-up issues

**Bugs found during manual testing:** (fill in)

---

## Automated verification (already passed)

These are the automatable checks from Task 9.1; recorded here for traceability.

- [x] `go test ./...` — all packages pass (engine, handler, ai, worker)
- [x] `go build ./...` — clean build
- [x] `go vet ./...` — no issues
- [x] `npx tsc --noEmit` — 0 errors
- [x] `npm run build` — clean Vite build (1 chunk, 733 KB unminified)
- [x] `POST /api/ai/permission_response` with unknown id returns 200 `{"status":"ok"}`
- [x] `POST /api/ai/ask_user_response` with empty request_id returns 400
- [x] `POST /api/plans/confirm` returns 404 (old route removed)
- [x] `ai.WikiTools()` returns 12 tools; no `propose_plan`
