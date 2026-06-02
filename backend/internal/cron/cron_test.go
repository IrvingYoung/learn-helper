package cron

import (
	"testing"
	"time"
)

func TestValidateCronExpr(t *testing.T) {
	cases := []struct {
		expr   string
		valid  bool
	}{
		{"0 9 * * *", true},
		{"*/5 * * * *", true},
		{"0 0 * * 0", true},
		{"30 14 1 * *", true},
		{"not a cron", false},
		{"", false},
		{"60 9 * * *", false}, // minute out of range
	}
	for _, c := range cases {
		err := ValidateCronExpr(c.expr)
		if c.valid && err != nil {
			t.Errorf("ValidateCronExpr(%q) = %v, want nil", c.expr, err)
		}
		if !c.valid && err == nil {
			t.Errorf("ValidateCronExpr(%q) = nil, want error", c.expr)
		}
	}
}

func TestNextRunAt(t *testing.T) {
	// 2026-06-02 08:00:00 UTC, every 5 minutes
	from := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)
	got, err := NextRunAt("*/5 * * * *", from)
	if err != nil {
		t.Fatalf("NextRunAt: %v", err)
	}
	want := time.Date(2026, 6, 2, 8, 5, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("NextRunAt = %v, want %v", got, want)
	}
}
