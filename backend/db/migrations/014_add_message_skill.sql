-- Migration 014: Add `skill` column to messages table
-- Stores the SKILL.md name (if any) that was active when this user
-- message was sent. NULL for messages sent without a /command.
-- Used by the frontend to display a skill badge on reloaded messages.

ALTER TABLE messages ADD COLUMN skill TEXT NOT NULL DEFAULT '';
