-- Migration 005: Add links and backlinks columns to wiki_pages
-- Stores JSON arrays of int64 IDs representing outgoing and incoming page references.

ALTER TABLE wiki_pages ADD COLUMN links TEXT NOT NULL DEFAULT '[]';
ALTER TABLE wiki_pages ADD COLUMN backlinks TEXT NOT NULL DEFAULT '[]';
