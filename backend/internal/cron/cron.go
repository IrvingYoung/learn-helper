package cron

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// parser is the standard 5-field cron parser with seconds disabled.
// (robfig/cron supports optional 6/7-field formats; we use 5-field to match
// user expectations from tools like crontab and GitHub Actions.)
var parser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// NextRunAt returns the next scheduled time at or after `from` for the given
// cron expression. Returns an error if the expression is invalid.
func NextRunAt(expr string, from time.Time) (time.Time, error) {
	sched, err := parser.Parse(expr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return sched.Next(from), nil
}

// ValidateCronExpr returns nil if the expression parses successfully.
func ValidateCronExpr(expr string) error {
	_, err := parser.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return nil
}
