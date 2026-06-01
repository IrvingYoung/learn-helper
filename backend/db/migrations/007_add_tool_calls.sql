-- Add tool_calls column to messages table for tool call output persistence
ALTER TABLE messages ADD COLUMN tool_calls TEXT;
