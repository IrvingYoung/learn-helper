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

-- name: GetActiveAIConfig :one
SELECT id, provider, model_name, api_key, is_active, config, created_at, updated_at FROM ai_configs WHERE is_active = 1;

-- name: UpsertAIConfig :exec
INSERT INTO ai_configs (provider, model_name, api_key, is_active, config)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    provider = excluded.provider,
    model_name = excluded.model_name,
    api_key = excluded.api_key,
    is_active = excluded.is_active,
    config = excluded.config,
    updated_at = CURRENT_TIMESTAMP;

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