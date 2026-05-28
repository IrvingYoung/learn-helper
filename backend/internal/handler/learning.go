package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type LearningRecordInput struct {
	TopicID     *int    `json:"topic_id"`
	ExerciseID  *int    `json:"exercise_id"`
	Status      string  `json:"status"`
	MasteryLevel *int   `json:"mastery_level"`
	Notes       *string `json:"notes"`
}

func (h *Handler) GetLearningRecords(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count FROM learning_records`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id, topicID, exerciseID, masteryLevel, reviewCount int
		var status, notes string
		var lastReviewedAt *time.Time
		if err := rows.Scan(&id, &topicID, &exerciseID, &status, &masteryLevel, &notes, &lastReviewedAt, &reviewCount); err != nil {
			continue
		}
		records = append(records, map[string]interface{}{
			"id":               id,
			"topic_id":         topicID,
			"exercise_id":      exerciseID,
			"status":           status,
			"mastery_level":    masteryLevel,
			"notes":            notes,
			"last_reviewed_at": lastReviewedAt,
			"review_count":     reviewCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"records": records})
}

func (h *Handler) UpsertLearningRecord(w http.ResponseWriter, r *http.Request) {
	var input LearningRecordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", 400)
		return
	}

	topicID := 0
	exerciseID := 0
	if input.TopicID != nil {
		topicID = *input.TopicID
	}
	if input.ExerciseID != nil {
		exerciseID = *input.ExerciseID
	}

	status := "not_started"
	if input.Status != "" {
		status = input.Status
	}

	notes := ""
	if input.Notes != nil {
		notes = *input.Notes
	}

	// Check if record exists
	var existingID int
	err := h.db.QueryRow(`SELECT id FROM learning_records WHERE topic_id = ? AND exercise_id = ?`, topicID, exerciseID).Scan(&existingID)

	if err == sql.ErrNoRows {
		_, err = h.db.Exec(`INSERT INTO learning_records (topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, 1)`,
			topicID, exerciseID, status, input.MasteryLevel, notes)
	} else if err == nil {
		_, err = h.db.Exec(`UPDATE learning_records SET status = ?, mastery_level = ?, notes = ?,
			last_reviewed_at = CURRENT_TIMESTAMP, review_count = review_count + 1
			WHERE id = ?`,
			status, input.MasteryLevel, notes, existingID)
	}

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}