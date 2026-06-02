## ADDED Requirements

### Requirement: Cron Task Management
The system SHALL allow users to create, list, update, delete, and toggle cron tasks via the frontend and HTTP API.

A cron task is defined by: a name, a description, a cron expression, a prompt (the natural-language task description), an enabled flag, a max_steps budget, and a timeout in seconds. The system SHALL persist tasks in the `cron_tasks` table and validate the cron expression at creation time, returning HTTP 400 with a descriptive error if the expression is unparseable.

#### Scenario: Create a valid cron task
- **WHEN** user submits a create-task request with name="GitHub 每日趋势", cron_expr="0 9 * * *", prompt="fetch github.com/trending, summarize top 10 in Chinese, create a new wiki page"
- **THEN** system inserts a row into `cron_tasks` with `enabled=1`, `next_run_at` set to the next 09:00 in server local time, and returns the created task with HTTP 201

#### Scenario: Reject an invalid cron expression
- **WHEN** user submits a create-task request with cron_expr="not a cron"
- **THEN** system returns HTTP 400 with body `{"error": "invalid cron expression: ..."}` and does not insert a row

#### Scenario: List all tasks
- **WHEN** user requests `GET /api/cron/tasks`
- **THEN** system returns HTTP 200 with an array of all tasks ordered by `created_at DESC`, including their current `next_run_at` and `last_status`

#### Scenario: Toggle a task enabled flag
- **WHEN** user requests `PATCH /api/cron/tasks/{id}` with `{"enabled": false}`
- **THEN** system updates the row and the scheduler SHALL NOT trigger that task on subsequent ticks

#### Scenario: Delete a task
- **WHEN** user requests `DELETE /api/cron/tasks/{id}`
- **THEN** system deletes the task and its associated `cron_runs` rows (via foreign key cascade) and returns HTTP 204

### Requirement: Cron Scheduler
The system SHALL run a background scheduler goroutine that triggers enabled tasks whose `next_run_at` is in the past.

The scheduler SHALL tick at most every 60 seconds. On each tick, the scheduler SHALL atomically claim all enabled tasks where `next_run_at <= now` AND the task is not currently `running`, compute the next `next_run_at` from the cron expression, and dispatch each claimed task to a runner goroutine. A task whose previous run is still in `running` state SHALL NOT be re-triggered on the same tick.

#### Scenario: Scheduler picks up a due task
- **WHEN** the current time reaches or passes a task's `next_run_at`
- **THEN** on the next scheduler tick, the system starts executing that task and updates its `last_status` to `running`

#### Scenario: Scheduler does not re-trigger a running task
- **WHEN** a task is currently in `last_status='running'` and its next computed `next_run_at` is reached
- **THEN** the scheduler SHALL skip the task on that tick and log a warning

#### Scenario: Stale running task is recoverable
- **WHEN** a task has been in `last_status='running'` for more than 2 hours
- **THEN** the scheduler SHALL allow the task to be triggered again on the next tick

#### Scenario: Scheduler recomputes next_run_at after each run
- **WHEN** a task finishes executing (success or failure)
- **THEN** the system updates `next_run_at` to the next scheduled time per the cron expression, regardless of the run outcome

#### Scenario: Startup backfill
- **WHEN** the server starts and there exist enabled tasks with `next_run_at` in the past
- **THEN** the scheduler SHALL trigger those tasks on the first tick after startup

### Requirement: Cron Task Execution
The system SHALL execute a triggered task by invoking the AI to autonomously complete the user-defined prompt, using the same AI provider and tool set as the interactive chat flow, with `ask_user` excluded and write tools auto-approved.

The runner SHALL:
- Apply a per-task timeout (default 300 seconds) and a max-steps cap (default 10)
- Create a new `conversations` row with `context_type='cron_task'` and `role='wiki_maintainer'` for the run, persisting all user/assistant/tool messages
- Build a system prompt by prepending a cron-mode prefix to the standard wiki_maintainer prompt. The prefix SHALL declare autonomous mode, current time, the task prompt, and (if present) the previous run's output summary
- Pass `WikiToolsForCron()` (read tools + write tools, no `ask_user`) to the AI
- Short-circuit the write tool permission gate and call `executeWriteTool` directly for every write tool the AI invokes
- Record a `cron_runs` row with `status`, `started_at`, `output_summary`, `error`, `write_count`, `steps_used`, and the `conversation_id`

#### Scenario: AI successfully completes a task with writes
- **WHEN** a task runs and the AI fetches github.com/trending, summarizes the top 10 repos, and calls `create_page` once
- **THEN** the system creates a wiki page directly (no permission prompt), the `cron_runs` row has `status='success'`, `write_count=1`, `steps_used>=1`, `output_summary` containing the AI's 1-2 sentence summary, and the conversation is persisted with all tool calls visible

#### Scenario: AI hits max_steps
- **WHEN** the ReAct loop reaches `max_steps` iterations without the AI stopping naturally
- **THEN** the system stops the loop, sets the `cron_runs` row's `status='failed'`, `error='max_steps_reached'`, and updates `next_run_at` for the next scheduled time

#### Scenario: AI hits per-task timeout
- **WHEN** the per-task timeout elapses before the run completes
- **THEN** the system cancels the AI's context, sets the `cron_runs` row's `status='failed'`, `error='timeout'`, and `duration_ms` reflects the elapsed time

#### Scenario: AI tool call fails
- **WHEN** the AI calls `webfetch` and the call returns an error
- **THEN** the system injects the error into the conversation as a tool result, the AI continues reasoning, and the run's final status reflects the AI's overall determination (success or failed)

#### Scenario: Previous run summary is injected
- **WHEN** a task runs and a previous `cron_runs` row exists for that task with non-empty `output_summary`
- **THEN** the system prepends a "上次运行摘要" section to the system prompt containing that summary

#### Scenario: No previous run summary
- **WHEN** a task runs for the first time
- **THEN** the system omits the "上次运行摘要" section from the system prompt

### Requirement: Manual Run Trigger
The system SHALL allow the user to manually trigger a task immediately via an HTTP endpoint, regardless of its `next_run_at` schedule.

The manual trigger SHALL:
- Reuse the same runner path as the scheduler-triggered execution
- Not update `next_run_at` (the schedule is preserved)
- Create a separate `cron_runs` row marked as manually triggered (the `output_summary` or a dedicated field can be used to distinguish; v1 stores this implicitly via run timing)

#### Scenario: Manually trigger a task
- **WHEN** user requests `POST /api/cron/tasks/{id}/run-now`
- **THEN** the system starts a new run for that task within a few seconds and returns HTTP 202 with the new `cron_run` id

#### Scenario: Manually trigger a disabled task
- **WHEN** user requests `POST /api/cron/tasks/{id}/run-now` on a task with `enabled=0`
- **THEN** the system still starts the run (manual trigger bypasses the enabled flag)

### Requirement: Run History Visibility
The system SHALL persist every run attempt in the `cron_runs` table and expose them through the API and frontend.

The system SHALL provide:
- `GET /api/cron/tasks/{id}/runs` — list runs for a task, ordered by `started_at DESC`, paginated (default page size 20)
- `GET /api/cron/runs/{id}` — fetch a single run, including its linked `conversation_id` so the frontend can deep-link to the conversation view
- The frontend SHALL display a list of runs per task with status, started_at, duration_ms, write_count, and a link to the run detail

#### Scenario: List runs for a task
- **WHEN** user requests `GET /api/cron/tasks/{id}/runs`
- **THEN** system returns HTTP 200 with an array of run summaries (id, status, started_at, finished_at, duration_ms, output_summary excerpt, write_count) ordered newest first

#### Scenario: View run detail with conversation link
- **WHEN** user clicks on a specific run
- **THEN** the system returns the full run record including `output_summary`, `error` (if any), `steps_used`, `write_count`, and `conversation_id`. The frontend provides a link to the conversation view at `/conversations/{conversation_id}`.

#### Scenario: Failed run shows error
- **WHEN** a run has `status='failed'` and `error='timeout'`
- **THEN** the frontend SHALL display the error in the run detail view in a visually distinct way (e.g., red text or error icon)

### Requirement: ReAct Loop Refactor for Reuse
The system SHALL refactor the existing `AIHandler.AIChat` to extract a reusable `runReAct` method that can be invoked from both the HTTP SSE path and the cron runner path, with no behavioral change to the existing HTTP flow.

The refactor SHALL introduce a `ReActEventSink` interface with methods for `WriteContent`, `WriteToolCallStart`, `WriteToolResult`, `WritePermissionRequired`, `WriteAskUserRequest`, `WriteDone`, and `WriteError`. The HTTP path SHALL provide an SSE-backed sink; the cron path SHALL provide a no-op (or logging) sink. The existing `sseWrite*` helper functions SHALL be preserved and the SSE sink SHALL delegate to them. After the refactor, the full existing `ai_test.go` suite SHALL continue to pass.

#### Scenario: HTTP chat behavior unchanged
- **WHEN** a user sends a chat message via `POST /api/ai/chat` after the refactor
- **THEN** the SSE event stream (content, tool_call_start, tool_result, permission_required, ask_user_request, done, error) is identical to pre-refactor behavior byte-for-byte

#### Scenario: Cron runner uses the same loop
- **WHEN** a cron task triggers a run
- **THEN** the cron runner invokes the same `runReAct` method, with a different sink and `AutoApproveWrites=true` option. The ReAct loop, tool classification, tool execution, and message persistence logic are shared with the HTTP path.

### Requirement: Tool Set Filtering for Cron Mode
The system SHALL provide a `WikiToolsForCron()` function in `internal/ai/provider.go` that returns the wiki tool definitions excluding `ask_user`.

The `ask_user` tool SHALL NOT be included in the tool list passed to the AI for cron runs, so the AI has no opportunity to invoke it. The full `WikiTools()` function SHALL continue to return the unfiltered list and SHALL continue to be used by the HTTP chat path.

#### Scenario: Cron AI cannot call ask_user
- **WHEN** a cron task runs and the AI considers asking the user a clarifying question
- **THEN** the AI's tool choice is restricted to the cron-mode tool list, so it cannot call `ask_user` and will instead make a reasonable decision autonomously (or skip the task if it cannot proceed)

#### Scenario: HTTP AI retains ask_user
- **WHEN** a user sends a chat message
- **THEN** the AI's tool list includes `ask_user` as before, and the system can still request user clarification through the ask_user channel

### Requirement: Auto-Approve Writes in Cron Mode
The system SHALL bypass the permission gate for write tools when executing a cron run with `AutoApproveWrites=true`, executing each write tool's action immediately and recording the result as a tool message in the conversation.

The runner SHALL NOT call `permissions.Register` when `AutoApproveWrites=true`. Instead, for each write tool the AI invokes, the runner SHALL directly call `executeWriteTool` and append the result to the AI message history. The frontend SHALL show a clear indication in the run detail that writes were auto-approved.

#### Scenario: AI creates a page in cron mode
- **WHEN** a cron task runs with `AutoApproveWrites=true` and the AI calls `create_page`
- **THEN** the system executes the page creation immediately (no SSE permission_required event, no user response needed) and the new page appears in the wiki

#### Scenario: Auto-approved writes are counted
- **WHEN** a cron run completes with N successful write tool executions
- **THEN** the corresponding `cron_runs.write_count` is N, and the frontend run detail view displays "写入了 N 个页面" (or equivalent)

#### Scenario: Auto-approved write failure surfaces in run
- **WHEN** a cron run's auto-approved write tool call returns an error from `executeWriteTool`
- **THEN** the system injects the error into the conversation as a tool result, the AI may continue or abort, and the `cron_runs.error` field reflects the final outcome
