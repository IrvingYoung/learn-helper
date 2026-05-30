-- name: GetAllTopics :many
SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, content, code_examples, common_mistakes, created_at, updated_at FROM topics ORDER BY sort_order, id;

-- name: GetTopicByID :one
SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, content, code_examples, common_mistakes, created_at, updated_at FROM topics WHERE id = ?;

-- name: GetTopicBySlug :one
SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, content, code_examples, common_mistakes, created_at, updated_at FROM topics WHERE slug = ?;

-- name: GetAllExercises :many
SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, solution_detail, common_errors, time_complexity_expected, space_complexity_expected, sample_code, created_at, updated_at FROM exercises;

-- name: GetExerciseByID :one
SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, solution_detail, common_errors, time_complexity_expected, space_complexity_expected, sample_code, created_at, updated_at FROM exercises WHERE id = ?;

-- name: GetExercisesByTopicID :many
SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, solution_detail, common_errors, time_complexity_expected, space_complexity_expected, sample_code, created_at, updated_at FROM exercises WHERE topic_id = ?;

-- name: GetLearningRecords :many
SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count, created_at, updated_at FROM learning_records;

-- name: GetLearningRecordByTopic :one
SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count, created_at, updated_at FROM learning_records WHERE topic_id = ?;

-- name: GetLearningRecordByExercise :one
SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count, created_at, updated_at FROM learning_records WHERE exercise_id = ?;

-- name: UpsertLearningRecord :exec
INSERT INTO learning_records (topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    topic_id = excluded.topic_id,
    exercise_id = excluded.exercise_id,
    status = excluded.status,
    mastery_level = excluded.mastery_level,
    notes = excluded.notes,
    last_reviewed_at = excluded.last_reviewed_at,
    review_count = excluded.review_count,
    updated_at = CURRENT_TIMESTAMP;

-- name: GetAllConversations :many
SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC;

-- name: GetConversationByID :one
SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations WHERE id = ?;

-- name: CreateConversation :one
INSERT INTO conversations (topic_id, exercise_id, context_type, title) VALUES (?, ?, ?, ?) RETURNING id, topic_id, exercise_id, context_type, title, created_at, updated_at;

-- name: GetMessagesByConversation :many
SELECT id, conversation_id, role, content, model_provider, token_count, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at;

-- name: CreateMessage :one
INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, ?, ?, ?, ?) RETURNING id, conversation_id, role, content, model_provider, token_count, created_at;

-- name: UpdateTopicContent :exec
UPDATE topics SET content = ?, code_examples = ?, common_mistakes = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?;

-- name: UpdateExerciseSolution :exec
UPDATE exercises SET solution_detail = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateExerciseErrors :exec
UPDATE exercises SET common_errors = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: BatchUpdateTopicContent :exec
UPDATE topics SET content = ?, code_examples = ?, common_mistakes = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?;

-- name: GetPrevTopic :one
SELECT t1.id, t1.slug, t1.name, t1.difficulty FROM topics t1
WHERE t1.parent_id = (SELECT t2.parent_id FROM topics t2 WHERE t2.slug = ?)
  AND t1.sort_order < (SELECT t3.sort_order FROM topics t3 WHERE t3.slug = ?)
ORDER BY t1.sort_order DESC
LIMIT 1;

-- name: GetNextTopic :one
SELECT t1.id, t1.slug, t1.name, t1.difficulty FROM topics t1
WHERE t1.parent_id = (SELECT t2.parent_id FROM topics t2 WHERE t2.slug = ?)
  AND t1.sort_order > (SELECT t3.sort_order FROM topics t3 WHERE t3.slug = ?)
ORDER BY t1.sort_order ASC
LIMIT 1;

-- name: GetExerciseCountByTopic :many
SELECT topic_id, COUNT(*) as exercise_count FROM exercises
GROUP BY topic_id;

-- name: GetTopicAncestors :many
WITH RECURSIVE ancestors AS (
    SELECT t.id, t.slug, t.name, t.parent_id, 0 as depth FROM topics t WHERE t.slug = ?
    UNION ALL
    SELECT p.id, p.slug, p.name, p.parent_id, a.depth + 1
    FROM topics p JOIN ancestors a ON p.id = a.parent_id
)
SELECT id, slug, name FROM ancestors WHERE depth > 0 ORDER BY depth DESC;

-- Wiki queries

-- name: GetAllWikiPages :many
SELECT id, title, slug, page_type, content, tags, parent_id, content_status, sort_order, created_at, updated_at
FROM wiki_pages
ORDER BY sort_order, id;

-- name: GetWikiPageTree :many
SELECT id, title, slug, page_type, content_status, parent_id, sort_order
FROM wiki_pages
ORDER BY sort_order, id;

-- name: GetWikiPageBySlug :one
SELECT id, title, slug, page_type, content, tags, parent_id, content_status, sort_order, created_at, updated_at
FROM wiki_pages
WHERE slug = ?;

-- name: GetWikiPageByID :one
SELECT * FROM wiki_pages WHERE id = ?;

-- name: GetOverviewPage :one
SELECT id, title, slug, page_type, content, tags, parent_id, content_status, sort_order, created_at, updated_at
FROM wiki_pages
WHERE page_type = 'overview' LIMIT 1;

-- name: CreateWikiPage :execresult
INSERT INTO wiki_pages (title, slug, page_type, content, tags, parent_id, content_status, sort_order)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateWikiPage :exec
UPDATE wiki_pages
SET title = ?, slug = ?, page_type = ?, content = ?, tags = ?, parent_id = ?, content_status = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateWikiPageContent :exec
UPDATE wiki_pages
SET content = ?, content_status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteWikiPage :exec
DELETE FROM wiki_pages WHERE id = ?;

-- name: GetRecentlyUpdatedWikiPages :many
SELECT id, title, slug, page_type, content_status, updated_at
FROM wiki_pages
WHERE page_type != 'overview'
ORDER BY updated_at DESC
LIMIT 10;

-- name: GetActiveAIConfig :one
SELECT * FROM ai_configs WHERE is_active = 1 LIMIT 1;

-- name: GetAIConfigByProvider :one
SELECT * FROM ai_configs WHERE provider = ? LIMIT 1;

-- name: DeactivateAllConfigs :exec
UPDATE ai_configs SET is_active = 0 WHERE is_active = 1;

-- name: CreateAIConfig :execresult
INSERT INTO ai_configs (provider, model_name, api_key, is_active)
VALUES (?, ?, ?, ?);

-- name: UpdateAIConfig :exec
UPDATE ai_configs
SET provider = ?, model_name = ?, api_key = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: GetWikiPageChildren :many
SELECT id, title, slug, page_type, content_status, sort_order
FROM wiki_pages
WHERE parent_id = ?
ORDER BY sort_order, id;

-- name: CountWikiPages :one
SELECT COUNT(*) FROM wiki_pages;

-- name: CountWikiPagesByStatus :one
SELECT COUNT(*) FROM wiki_pages WHERE content_status = ?;

-- name: GetEmptyWikiPages :many
SELECT id, title, slug, parent_id
FROM wiki_pages
WHERE content_status = 'empty'
ORDER BY sort_order, id;

-- Agent Loop migration: add tool_call_id and tool_name columns
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_call_id TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_name TEXT;
-- name: SearchWikiPages :many
SELECT id, title, slug, page_type, content, tags, parent_id, content_status, sort_order, created_at, updated_at
FROM wiki_pages
WHERE title LIKE '%' || ?1 || '%' OR content LIKE '%' || ?1 || '%'
ORDER BY sort_order, id;
