package handler

import (
	"encoding/json"
	"net/http"

	"learn-helper/internal/model"
)

type SiblingTopic struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Difficulty string `json:"difficulty"`
}

type BreadcrumbItem struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (h *Handler) GetTopics(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, COALESCE(parent_id, 0), name, slug, description, key_points, content, code_examples, common_mistakes, difficulty, sort_order FROM topics ORDER BY sort_order, id`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	// Fetch exercise counts
	counts, _ := h.queries.GetExerciseCountByTopic(r.Context())
	countMap := make(map[int64]int64)
	for _, c := range counts {
		countMap[c.TopicID] = c.ExerciseCount
	}

	var topics []map[string]interface{}
	for rows.Next() {
		var id, parentID, sortOrder int
		var name, slug, description, keyPoints, difficulty string
		var content, codeExamples, commonMistakes *string
		if err := rows.Scan(&id, &parentID, &name, &slug, &description, &keyPoints, &content, &codeExamples, &commonMistakes, &difficulty, &sortOrder); err != nil {
			continue
		}
		topic := map[string]interface{}{
			"id":            id,
			"parent_id":     parentID,
			"name":          name,
			"slug":          slug,
			"description":   description,
			"key_points":    keyPoints,
			"difficulty":    difficulty,
			"sort_order":    sortOrder,
			"exercise_count": countMap[int64(id)],
		}
		if content != nil {
			topic["content"] = *content
		}
		if codeExamples != nil {
			topic["code_examples"] = *codeExamples
		}
		if commonMistakes != nil {
			topic["common_mistakes"] = *commonMistakes
		}
		topics = append(topics, topic)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"topics": topics})
}

func (h *Handler) GetTopicBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id, parentID, sortOrder int
	var name, slugOut, description, keyPoints, difficulty string
	var content, codeExamples, commonMistakes *string
	err := h.db.QueryRow(`SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, content, code_examples, common_mistakes FROM topics WHERE slug = ?`, slug).
		Scan(&id, &parentID, &name, &slugOut, &description, &keyPoints, &difficulty, &sortOrder, &content, &codeExamples, &commonMistakes)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Fetch sibling topics using sqlc
	var prevTopic, nextTopic *SiblingTopic
	prev, err := h.queries.GetPrevTopic(r.Context(), model.GetPrevTopicParams{Slug: slug, Slug_2: slug})
	if err == nil {
		diff := ""
		if prev.Difficulty.Valid {
			diff = prev.Difficulty.String
		}
		prevTopic = &SiblingTopic{Slug: prev.Slug, Name: prev.Name, Difficulty: diff}
	}
	next, err := h.queries.GetNextTopic(r.Context(), model.GetNextTopicParams{Slug: slug, Slug_2: slug})
	if err == nil {
		diff := ""
		if next.Difficulty.Valid {
			diff = next.Difficulty.String
		}
		nextTopic = &SiblingTopic{Slug: next.Slug, Name: next.Name, Difficulty: diff}
	}

	// Fetch ancestors for breadcrumb using sqlc
	ancestors, _ := h.queries.GetTopicAncestors(r.Context(), slug)
	breadcrumb := make([]BreadcrumbItem, 0, len(ancestors))
	for _, a := range ancestors {
		breadcrumb = append(breadcrumb, BreadcrumbItem{Slug: a.Slug, Name: a.Name})
	}

	// Fetch exercise count using sqlc
	var exerciseCount int64
	counts, _ := h.queries.GetExerciseCountByTopic(r.Context())
	for _, c := range counts {
		if c.TopicID == int64(id) {
			exerciseCount = c.ExerciseCount
			break
		}
	}

	result := map[string]interface{}{
		"id":             id,
		"parent_id":      parentID,
		"name":           name,
		"slug":           slugOut,
		"description":    description,
		"key_points":     keyPoints,
		"difficulty":     difficulty,
		"sort_order":     sortOrder,
		"exercise_count": exerciseCount,
		"breadcrumb":     breadcrumb,
		"prev_topic":     prevTopic,
		"next_topic":     nextTopic,
	}
	if content != nil {
		result["content"] = *content
	}
	if codeExamples != nil {
		result["code_examples"] = *codeExamples
	}
	if commonMistakes != nil {
		result["common_mistakes"] = *commonMistakes
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) UpdateTopicContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", 405)
		return
	}

	slug := r.PathValue("slug")

	var req struct {
		Content        *string `json:"content"`
		CodeExamples   *string `json:"code_examples"`
		CommonMistakes *string `json:"common_mistakes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", 400)
		return
	}

	result, err := h.db.Exec(`UPDATE topics SET content = ?, code_examples = ?, common_mistakes = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?`,
		req.Content, req.CodeExamples, req.CommonMistakes, slug)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "updated"})
}

func (h *Handler) BatchUpdateTopicContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		Items []struct {
			Slug           string  `json:"slug"`
			Content        *string `json:"content"`
			CodeExamples   *string `json:"code_examples"`
			CommonMistakes *string `json:"common_mistakes"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", 400)
		return
	}

	if len(req.Items) > 50 {
		http.Error(w, "max 50 items per batch", 400)
		return
	}

	var updated int
	var failed []string
	for _, item := range req.Items {
		result, err := h.db.Exec(`UPDATE topics SET content = ?, code_examples = ?, common_mistakes = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?`,
			item.Content, item.CodeExamples, item.CommonMistakes, item.Slug)
		if err != nil || result == nil {
			failed = append(failed, item.Slug)
			continue
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			updated++
		} else {
			failed = append(failed, item.Slug)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"updated": updated,
		"failed":  failed,
	})
}

func (h *Handler) GetExercisesByTopic(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var topicID int
	if err := h.db.QueryRow(`SELECT id FROM topics WHERE slug = ?`, slug).Scan(&topicID); err != nil {
		http.NotFound(w, r)
		return
	}

	rows, err := h.db.Query(`SELECT id, topic_id, type, title, description, difficulty, tags, hints FROM exercises WHERE topic_id = ?`, topicID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var exercises []map[string]interface{}
	for rows.Next() {
		var id, topicID int
		var typ, title, description, difficulty, tags, hints string
		if err := rows.Scan(&id, &topicID, &typ, &title, &description, &difficulty, &tags, &hints); err != nil {
			continue
		}
		exercises = append(exercises, map[string]interface{}{
			"id":          id,
			"topic_id":    topicID,
			"type":        typ,
			"title":       title,
			"description": description,
			"difficulty":  difficulty,
			"tags":        tags,
			"hints":       hints,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"exercises": exercises})
}
