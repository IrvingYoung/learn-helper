-- Add role column to conversations table for persisting AI chat role
ALTER TABLE conversations ADD COLUMN role TEXT CHECK(role IN ('knowledge_explain', 'problem_solving', 'dashboard'));

-- Update schema to include role in the CREATE TABLE definition for future reference
-- (SQLite ALTER TABLE adds column to existing table; schema.sql should be updated separately)