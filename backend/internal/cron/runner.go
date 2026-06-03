package cron

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"learn-helper/internal/ai"
	"learn-helper/internal/handler"
)

// Runner executes a single cron task by constructing a ChatRequest and
// invoking AIHandler.RunReAct. It persists a cron_runs row and a
// conversation record.
type Runner struct {
	db           DB
	hook         RunnerHooks
	DigestRunner *DigestRunner // optional; only used for twitter_digest tasks
}

// RunnerHooks is the minimum surface Runner needs from the AI handler.
// Defined as an interface so tests can mock it without importing the
// handler package.
type RunnerHooks interface {
	RunReAct(ctx context.Context, provider ai.AIProvider, chatReq ai.ChatRequest, opts handler.ReActOptions) (*handler.ReActResult, error)
}

// NewRunner creates a Runner that uses the given DB and hooks.
func NewRunner(db DB, hooks RunnerHooks) *Runner {
	return &Runner{db: db, hook: hooks}
}

// cronSink is a no-op ReActEventSink used by cron runs. It logs events at
// info level so operators can see what the AI did.
type cronSink struct {
	runID int64
}

func (s *cronSink) WriteContent(text string) {
	log.Printf("[cron-run:%d] content: %s", s.runID, truncateForLog(text, 200))
}

func (s *cronSink) WriteToolCallStart(id, name, input string) {
	log.Printf("[cron-run:%d] tool start: %s id=%s", s.runID, name, id)
}

func (s *cronSink) WriteToolResult(id, name, output, errStr string) {
	if errStr != "" {
		log.Printf("[cron-run:%d] tool %s id=%s err=%s", s.runID, name, id, errStr)
	} else {
		log.Printf("[cron-run:%d] tool %s id=%s ok", s.runID, name, id)
	}
}

func (s *cronSink) WritePermissionRequired(req handler.PermissionRequest) {
	// Should not happen in cron mode (ask_user is filtered and writes are
	// auto-approved), but log defensively.
	log.Printf("[cron-run:%d] unexpected permission_required (should be auto-approved)", s.runID)
}

func (s *cronSink) WriteAskUserRequest(req handler.AskUserRequest) {
	// Should not happen in cron mode (ask_user is filtered out of the tool
	// list), but log defensively.
	log.Printf("[cron-run:%d] unexpected ask_user_request in autonomous mode", s.runID)
}

func (s *cronSink) WriteDone() {
	log.Printf("[cron-run:%d] done", s.runID)
}

func (s *cronSink) WriteError(msg string) {
	log.Printf("[cron-run:%d] error: %s", s.runID, msg)
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(截断)"
}

// Run executes one task. It is safe to call from a goroutine; the caller is
// responsible for cancellation via ctx.
//
// The flow:
//  1. Apply per-task timeout (in addition to any caller-provided ctx deadline).
//  2. Create a new conversation (context_type='cron_task').
//  3. Load previous run summary (if any) and prepend to the system prompt.
//  4. Construct ChatRequest with the cron prompt + WikiToolsForCron.
//  5. Invoke AIHandler.RunReAct with AutoApproveWrites=true.
//  6. Persist user / assistant messages into the conversation.
//  7. Update cron_runs and cron_tasks.last_status.
func (r *Runner) Run(ctx context.Context, task *Task, opts RunOpts) error {
	start := time.Now()

	// 1. Per-task timeout
	timeout := time.Duration(task.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 2. Insert a cron_runs row immediately so we have an ID for the sink
	//    and so the UI can show the run as "running" while it executes.
	run := &Run{
		TaskID:    task.ID,
		Status:    RunStatusRunning,
		StartedAt: start,
	}
	runID, err := r.db.InsertRun(runCtx, run)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	run.ID = runID

	// 2b. Branch for twitter_digest: the fetch+AI flow lives in DigestRunner
	//     and uses a different conversation, tool set, and prompt. The
	//     cron_runs row above is still needed for the UI to show the run
	//     (and for runTwitterDigest to persist errors back to it), but we
	//     skip the conversation/prompt/ReAct setup below.
	if task.TaskType == "twitter_digest" {
		return r.runTwitterDigest(runCtx, task, runID)
	}

	// 3. Create a conversation for this run
	title := fmt.Sprintf("[cron] %s @ %s", task.Name, start.Format("2006-01-02 15:04:05"))
	convID, err := r.db.CreateConversation(runCtx, title)
	if err != nil {
		r.finalizeRunFailure(runCtx, run, fmt.Errorf("create conversation: %w", err))
		return err
	}
	run.ConversationID = sql.NullInt64{Int64: convID, Valid: true}
	if err := r.db.UpdateRun(runCtx, run); err != nil {
		log.Printf("[cron-run:%d] failed to set conversation_id: %v", runID, err)
	}

	// 4. Load last successful run summary (memory model A)
	lastSummary, _ := r.db.LoadLastRunSummary(runCtx, task.ID)

	// 5. Get active AI config + provider
	providerName, modelName, apiKey, err := r.db.GetActiveAIConfig(runCtx)
	if err != nil {
		r.finalizeRunFailure(runCtx, run, fmt.Errorf("get active AI config: %w", err))
		return err
	}
	provider, err := ai.NewProvider(ai.ProviderType(providerName), apiKey, modelName)
	if err != nil {
		r.finalizeRunFailure(runCtx, run, fmt.Errorf("create provider: %w", err))
		return err
	}

	// 6. Build the user message. The task's name and prompt MUST appear here
	// in full — the system prompt is huge (wiki_maintainer + tree context) and
	// the AI will treat a "本次任务" line buried in the system prompt as
	// boilerplate rather than the actual instruction. Putting the task
	// directly in the user message makes the AI treat it as the user's
	// explicit request.
	userMsg := buildUserMessage(task, time.Now())

	// 7. Build the system prompt: base wiki_maintainer + cron prefix.
	//    We still pass task.Prompt so the cron mode prefix can reference it,
	//    but the actual instruction lives in the user message.
	baseSystemPrompt := ai.BuildSystemPrompt(ai.RoleWikiMaintainer, "", nil)
	systemPrompt := ai.BuildCronSystemPrompt(baseSystemPrompt, task.Prompt, lastSummary, time.Now())

	// 8. Build ChatRequest
	chatReq := ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: userMsg},
		},
		SystemPrompt: systemPrompt,
		Tools:        ai.WikiToolsForCron(),
		MaxTokens:    8192,
	}

	// 9. Save user message
	if err := r.db.AppendMessage(runCtx, convID, "user", userMsg, providerName, ""); err != nil {
		log.Printf("[cron-run:%d] failed to save user message: %v", runID, err)
	}

	// 10. Invoke RunReAct
	sink := &cronSink{runID: runID}
	result, runErr := r.hook.RunReAct(runCtx, provider, chatReq, handler.ReActOptions{
		AutoApproveWrites: true,
		MaxSteps:          task.MaxSteps,
		Timeout:           timeout,
		Sink:              sink,
		RunID:             runID,
		ConversationID:    convID,
	})

	// 11. Determine status
	finishedAt := time.Now()
	run.FinishedAt = sql.NullTime{Time: finishedAt, Valid: true}
	run.DurationMS = sql.NullInt64{Int64: finishedAt.Sub(start).Milliseconds(), Valid: true}

	switch {
	case runErr != nil && runCtx.Err() == context.DeadlineExceeded:
		run.Status = RunStatusTimeout
		run.Error = sql.NullString{String: "timeout", Valid: true}
	case runErr != nil:
		run.Status = RunStatusFailed
		run.Error = sql.NullString{String: runErr.Error(), Valid: true}
	default:
		run.Status = RunStatusSuccess
	}

	if result != nil {
		run.StepsUsed = int64(result.Steps)
		run.WriteCount = int64(result.WriteCount)
		// Use the AI's final content as the output summary, truncated.
		summary := result.FinalContent
		if runes := []rune(summary); len(runes) > 200 {
			summary = string(runes[:200]) + "..."
		}
		summary = strings.TrimSpace(summary)
		if summary != "" {
			run.OutputSummary = sql.NullString{String: summary, Valid: true}
		}

		// Save assistant message
		if result.FinalContent != "" {
			toolCallsJSON, _ := json.Marshal(result.ToolCallResults)
			if err := r.db.AppendMessage(runCtx, convID, "assistant", result.FinalContent, providerName, string(toolCallsJSON)); err != nil {
				log.Printf("[cron-run:%d] failed to save assistant message: %v", runID, err)
			}
		}
	}

	// 12. Update run row
	if err := r.db.UpdateRun(runCtx, run); err != nil {
		log.Printf("[cron-run:%d] failed to update run row: %v", runID, err)
	}

	// 13. Update task last_status
	if err := r.db.SetTaskLastStatus(runCtx, task.ID, run.Status, run.Error.String); err != nil {
		log.Printf("[cron-run:%d] failed to update task last_status: %v", runID, err)
	}

	return runErr
}

// finalizeRunFailure marks the run and task as failed (used when we can't
// even start the run — e.g. missing AI config).
func (r *Runner) finalizeRunFailure(ctx context.Context, run *Run, cause error) {
	run.Status = RunStatusFailed
	run.Error = sql.NullString{String: cause.Error(), Valid: true}
	run.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if err := r.db.UpdateRun(ctx, run); err != nil {
		log.Printf("[cron-run:%d] failed to mark run failed: %v", run.ID, err)
	}
	if err := r.db.SetTaskRunningError(ctx, run.TaskID, cause.Error()); err != nil {
		log.Printf("[cron-run:%d] failed to mark task failed: %v", run.ID, err)
	}
}

// runTwitterDigest executes the twitter_digest task variant. It
// bridges the existing Runner (which manages cron_runs + cron_tasks
// lifecycle) with the DigestRunner (which manages the digest-specific
// fetch/AI flow).
func (r *Runner) runTwitterDigest(ctx context.Context, task *Task, cronRunID int64) error {
	if r.DigestRunner == nil {
		return fmt.Errorf("DigestRunner not wired in")
	}
	cfg := DigestConfig{
		SinceHours:          task.SinceHours,
		MaxTweetsPerAccount: task.MaxTweetsPerAccount,
		MaxTotalTweets:      task.MaxTotalTweets,
	}
	if cfg.SinceHours <= 0 {
		cfg.SinceHours = 24
	}
	if cfg.MaxTweetsPerAccount <= 0 {
		cfg.MaxTweetsPerAccount = 50
	}
	if cfg.MaxTotalTweets <= 0 {
		cfg.MaxTotalTweets = 200
	}
	_, _, err := r.DigestRunner.Run(ctx, &cronRunID, cfg)
	if err != nil {
		// persist the error to cron_runs so the UI shows it
		_ = r.db.UpdateRun(ctx, &Run{
			ID:         cronRunID,
			Status:     RunStatusFailed,
			FinishedAt: sql.NullTime{Time: time.Now(), Valid: true},
			Error:      sql.NullString{String: err.Error(), Valid: true},
		})
	} else {
		_ = r.db.UpdateRun(ctx, &Run{
			ID:         cronRunID,
			Status:     RunStatusSuccess,
			FinishedAt: sql.NullTime{Time: time.Now(), Valid: true},
		})
	}
	return err
}

// buildUserMessage constructs the user-role message sent to the AI for a
// cron run. The task's name and prompt are placed here directly so the AI
// treats them as an explicit instruction rather than background context.
//
// Format:
//
//	请执行以下定时任务。
//
//	任务名称: <name>
//	当前时间: <now>
//
//	任务内容:
//	<prompt>
//
//	完成后用 1-2 句中文总结你做了什么。
func buildUserMessage(task *Task, now time.Time) string {
	return fmt.Sprintf(`请执行以下定时任务。

任务名称: %s
当前时间: %s

任务内容:
%s

完成后请用 1-2 句中文总结你做了什么,这个总结会保存到运行历史里。
`, task.Name, now.Format("2006-01-02 15:04:05"), task.Prompt)
}
