-- Migration 010: Add stage column to plans
-- Records which stage a plan represents: 'main' (single create_page),
-- 'outline' (outline-based skeleton creation), or 'content' (update/patch).
-- Inferred at plan creation time; legacy plans default to 'main'.

ALTER TABLE plans ADD COLUMN stage TEXT NOT NULL DEFAULT 'main' CHECK(stage IN ('main', 'outline', 'content'));

CREATE INDEX IF NOT EXISTS idx_plans_stage ON plans(stage);
