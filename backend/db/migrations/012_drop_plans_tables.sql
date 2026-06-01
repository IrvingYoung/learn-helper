-- Migration 012: Drop plans / plan_actions tables.
--
-- The plan tool has been redesigned and no longer uses these tables
-- (tool migration happened in code; see tasks 4.2-4.4). The user is
-- single-tenant, so any pending plans in the DB are discarded without
-- ceremony.
--
-- Drop order: child (plan_actions) before parent (plans) to respect
-- the FK constraint from plan_actions.plan_id -> plans.id.

DROP TABLE IF EXISTS plan_actions;
DROP TABLE IF EXISTS plans;
