package handler

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) GetTopics(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order FROM topics ORDER BY sort_order, id`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var topics []map[string]interface{}
	for rows.Next() {
		var id, parentID, sortOrder int
		var name, slug, description, keyPoints, difficulty string
		if err := rows.Scan(&id, &parentID, &name, &slug, &description, &keyPoints, &difficulty, &sortOrder); err != nil {
			continue
		}
		topics = append(topics, map[string]interface{}{
			"id":          id,
			"parent_id":   parentID,
			"name":        name,
			"slug":        slug,
			"description": description,
			"key_points":  keyPoints,
			"difficulty":  difficulty,
			"sort_order":  sortOrder,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"topics": topics})
}

func (h *Handler) GetTopicBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id, parentID, sortOrder int
	var name, slugOut, description, keyPoints, difficulty string
	err := h.db.QueryRow(`SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order FROM topics WHERE slug = ?`, slug).
		Scan(&id, &parentID, &name, &slugOut, &description, &keyPoints, &difficulty, &sortOrder)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          id,
		"parent_id":   parentID,
		"name":        name,
		"slug":        slugOut,
		"description": description,
		"key_points":  keyPoints,
		"difficulty":  difficulty,
		"sort_order":  sortOrder,
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