// Package cron implements user-configurable scheduled tasks that trigger the
// AI to autonomously execute a user-defined prompt at cron-scheduled times.
//
// The package has three main types:
//
//   - DB (db.go) wraps *model.Queries with cron-specific operations
//     (ListEnabledTasksDue, ClaimTask, InsertRun, etc.).
//   - Runner (runner.go) builds a ChatRequest and invokes AIHandler.RunReAct
//     for one task run, persisting the conversation and run record.
//   - Scheduler (scheduler.go) ticks every minute, claims due tasks, and
//     dispatches them to the Runner.
//
// Cron expression parsing is delegated to github.com/robfig/cron/v3.
package cron
