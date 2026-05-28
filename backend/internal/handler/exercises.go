package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (h *Handler) GetExercises(w http.ResponseWriter, r *http.Request) {
	topicID := r.URL.Query().Get("topic_id")
	difficulty := r.URL.Query().Get("difficulty")

	query := `SELECT e.id, e.topic_id, e.type, e.title, e.description, e.difficulty, e.tags, e.hints,
		COALESCE(lr.status, 'not_started') as status, COALESCE(lr.mastery_level, 0) as mastery_level
		FROM exercises e
		LEFT JOIN learning_records lr ON e.id = lr.exercise_id
		WHERE 1=1`
	args := []interface{}{}
	if topicID != "" {
		query += " AND e.topic_id = ?"
		args = append(args, topicID)
	}
	if difficulty != "" && difficulty != "全部" {
		query += " AND e.difficulty = ?"
		args = append(args, difficulty)
	}

	rows, err := h.db.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var exercises []map[string]interface{}
	for rows.Next() {
		var id, topicID int
		var typ, title, description, difficulty, tags, hints, statusVal string
		var masteryLevel int
		if err := rows.Scan(&id, &topicID, &typ, &title, &description, &difficulty, &tags, &hints, &statusVal, &masteryLevel); err != nil {
			continue
		}
		exercises = append(exercises, map[string]interface{}{
			"id":            id,
			"topic_id":      topicID,
			"type":          typ,
			"title":         title,
			"description":   description,
			"difficulty":    difficulty,
			"tags":          tags,
			"hints":         hints,
			"status":        statusVal,
			"mastery_level": masteryLevel,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"exercises": exercises})
}

func (h *Handler) GetExerciseByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	var exerciseID, topicID int
	var typ, title, description, difficulty, tags, hints, solutionOutline, timeComplexity, spaceComplexity, sampleCode string
	err = h.db.QueryRow(`SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected, sample_code
		FROM exercises WHERE id = ?`, id).
		Scan(&exerciseID, &topicID, &typ, &title, &description, &difficulty, &tags, &hints, &solutionOutline, &timeComplexity, &spaceComplexity, &sampleCode)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                         exerciseID,
		"topic_id":                   topicID,
		"type":                       typ,
		"title":                      title,
		"description":                description,
		"difficulty":                 difficulty,
		"tags":                       tags,
		"hints":                      hints,
		"solution_outline":            solutionOutline,
		"time_complexity_expected":   timeComplexity,
		"space_complexity_expected": spaceComplexity,
		"sample_code":                sampleCode,
	})
}