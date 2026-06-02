package cron

import (
	"context"
	"log"
	"sync"
	"time"
)

// Scheduler ticks at a fixed interval, claims due tasks, and dispatches
// them to the runner. A single Scheduler instance is intended per process.
type Scheduler struct {
	db     DB
	runner *Runner

	// tickInterval is the wait between ticks. Default 60s.
	tickInterval time.Duration
	// staleRunningAfter is how old a 'running' task can be before the
	// scheduler will consider it crashed and re-trigger it. Default 2h.
	staleRunningAfter time.Duration

	mu        sync.Mutex
	executing map[int64]bool // task IDs currently being executed
}

// NewScheduler creates a Scheduler wired to the given DB and Runner.
func NewScheduler(db DB, runner *Runner) *Scheduler {
	return &Scheduler{
		db:                db,
		runner:            runner,
		tickInterval:      60 * time.Second,
		staleRunningAfter: 2 * time.Hour,
		executing:         make(map[int64]bool),
	}
}

// SetTickInterval overrides the default 60s tick interval. Used by tests.
func (s *Scheduler) SetTickInterval(d time.Duration) { s.tickInterval = d }

// SetStaleRunningAfter overrides the default 2h stale-running threshold.
func (s *Scheduler) SetStaleRunningAfter(d time.Duration) { s.staleRunningAfter = d }

// Run blocks until ctx is cancelled. The first tick fires immediately
// (so a server restart catches any tasks that became due while the server
// was down — startup backfill).
func (s *Scheduler) Run(ctx context.Context) {
	// Startup backfill: trigger immediately on Run start.
	if n, err := s.tick(ctx); err != nil {
		log.Printf("[cron-scheduler] initial tick: %v", err)
	} else if n > 0 {
		log.Printf("[cron-scheduler] startup backfill: dispatched %d due task(s)", n)
	}

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[cron-scheduler] shutting down")
			return
		case <-ticker.C:
			if _, err := s.tick(ctx); err != nil {
				log.Printf("[cron-scheduler] tick: %v", err)
			}
		}
	}
}

// Tick is exposed for tests to advance the scheduler manually.
func (s *Scheduler) Tick(ctx context.Context) (int, error) {
	return s.tick(ctx)
}

func (s *Scheduler) tick(ctx context.Context) (int, error) {
	now := time.Now()
	stale := now.Add(-s.staleRunningAfter)

	tasks, err := s.db.ListEnabledTasksDue(ctx, now, stale)
	if err != nil {
		return 0, err
	}
	if len(tasks) == 0 {
		return 0, nil
	}

	dispatched := 0
	for _, task := range tasks {
		// In-process guard against the same task being dispatched twice in
		// the same tick window (e.g., slow DB updates from previous tick).
		s.mu.Lock()
		if s.executing[task.ID] {
			s.mu.Unlock()
			log.Printf("[cron-scheduler] task %d already executing in-process, skipping", task.ID)
			continue
		}
		s.executing[task.ID] = true
		s.mu.Unlock()

		nextAt, err := NextRunAt(task.CronExpr, now)
		if err != nil {
			log.Printf("[cron-scheduler] task %d has invalid cron %q: %v — disabling", task.ID, task.CronExpr, err)
			_ = s.db.SetTaskRunningError(ctx, task.ID, "invalid cron expression")
			s.mu.Lock()
			delete(s.executing, task.ID)
			s.mu.Unlock()
			continue
		}
		if err := s.db.ClaimTask(ctx, task.ID, nextAt); err != nil {
			log.Printf("[cron-scheduler] claim task %d: %v", task.ID, err)
			s.mu.Lock()
			delete(s.executing, task.ID)
			s.mu.Unlock()
			continue
		}
		// Re-fetch the task with the new state for the runner.
		task, err = s.db.GetTask(ctx, task.ID)
		if err != nil {
			log.Printf("[cron-scheduler] re-fetch task %d: %v", task.ID, err)
			s.mu.Lock()
			delete(s.executing, task.ID)
			s.mu.Unlock()
			continue
		}

		dispatched++
		go s.executeAndCleanup(task)
	}

	return dispatched, nil
}

// executeAndCleanup runs the task and removes it from the in-process guard.
func (s *Scheduler) executeAndCleanup(task *Task) {
	defer func() {
		s.mu.Lock()
		delete(s.executing, task.ID)
		s.mu.Unlock()
		if r := recover(); r != nil {
			log.Printf("[cron-scheduler] task %d panicked: %v", task.ID, r)
		}
	}()

	// Use a fresh background context for the runner — the scheduler's ctx
	// is only for the dispatcher loop, not the run itself. A long-running
	// run shouldn't be cancelled just because the scheduler is shutting
	// down. The per-task timeout (set in Runner.Run) handles abort.
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.runner.Run(runCtx, task, RunOpts{TriggerSource: "scheduler"}); err != nil {
		log.Printf("[cron-scheduler] task %d run error: %v", task.ID, err)
	}
}
