-- Migration: 添加知识点详细讲解内容字段
-- 2026-05-28

PRAGMA foreign_keys = ON;

-- topics 表：添加详细讲解相关字段
ALTER TABLE topics ADD COLUMN content TEXT;
ALTER TABLE topics ADD COLUMN code_examples TEXT;
ALTER TABLE topics ADD COLUMN common_mistakes TEXT;

-- exercises 表：添加详细解法相关字段
ALTER TABLE exercises ADD COLUMN solution_detail TEXT;
ALTER TABLE exercises ADD COLUMN common_errors TEXT;