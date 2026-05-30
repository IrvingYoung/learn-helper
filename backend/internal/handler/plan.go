package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"learn-helper/internal/engine"
	"learn-helper/internal/model"
)

// PlanHandler handles plan-related HTTP requests.
type PlanHandler struct {
	db      *sql.DB
	queries *model.Queries
	engine  *engine.ExecutionEngine
}

// NewPlanHandler creates a new PlanHandler.
func NewPlanHandler(db *sql.DB, queries *model.Queries, eng *engine.ExecutionEngine) *PlanHandler {
	return &PlanHandler{
		db:      db,
		queries: queries,
		engine:  eng,
	}
}

// GetPlan handles GET /api/plans?id=xxx
// Returns a plan with its actions.
func (h *PlanHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.URL.Query().Get("id")
	if planID == "" {
		http.Error(w, "missing plan id", http.StatusBadRequest)
		return
	}

	plan, err := h.loadPlan(r.Context(), planID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "plan not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load plan", http.StatusInternalServerError)
		return
	}

	writeJSON(w, plan)
}

// ConfirmPlan handles POST /api/plans/confirm
// Body: { "plan_id": "xxx" }
// Confirms a pending plan and executes it.
func (h *PlanHandler) ConfirmPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.PlanID == "" {
		http.Error(w, "missing plan_id", http.StatusBadRequest)
		return
	}

	// Verify plan exists and is pending
	var status string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT status FROM plans WHERE id = ?`, req.PlanID).Scan(&status)
	if err == sql.ErrNoRows {
		http.Error(w, "plan not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to query plan", http.StatusInternalServerError)
		return
	}

	if status != "pending" {
		http.Error(w, "plan is not in pending status", http.StatusBadRequest)
		return
	}

	// Update status to confirmed
	if _, err := h.db.ExecContext(r.Context(),
		`UPDATE plans SET status = 'confirmed' WHERE id = ?`, req.PlanID); err != nil {
		http.Error(w, "failed to update plan status", http.StatusInternalServerError)
		return
	}

	// Execute the plan
	report, err := h.engine.ExecutePlan(r.Context(), req.PlanID)
	if err != nil {
		http.Error(w, "plan execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, report)
}

// RejectPlan handles POST /api/plans/reject
// Body: { "plan_id": "xxx" }
// Rejects a pending plan.
func (h *PlanHandler) RejectPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.PlanID == "" {
		http.Error(w, "missing plan_id", http.StatusBadRequest)
		return
	}

	// Update status to rejected only if currently pending
	result, err := h.db.ExecContext(r.Context(),
		`UPDATE plans SET status = 'rejected' WHERE id = ? AND status = 'pending'`,
		req.PlanID)
	if err != nil {
		http.Error(w, "failed to reject plan", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "plan not found or not in pending status", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "rejected"})
}

// loadPlan loads a plan and its actions from the database.
func (h *PlanHandler) loadPlan(ctx context.Context, planID string) (*model.Plan, error) {
	var plan model.Plan
	var executedAt sql.NullString

	err := h.db.QueryRowContext(ctx,
		`SELECT id, conversation_id, reasoning, status, created_at, executed_at
		 FROM plans WHERE id = ?`, planID).Scan(
		&plan.ID, &plan.ConversationID, &plan.Reasoning, &plan.Status, &plan.CreatedAt, &executedAt)
	if err != nil {
		return nil, err
	}

	if executedAt.Valid {
		plan.ExecutedAt = &executedAt.String
	}

	// Load actions
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, plan_id, type, params, depends_on, status, result, sort_order, created_at
		 FROM plan_actions WHERE plan_id = ? ORDER BY sort_order`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var a model.PlanAction
		var result sql.NullString
		if err := rows.Scan(
			&a.ID, &a.PlanID, &a.Type, &a.Params, &a.DependsOn,
			&a.Status, &result, &a.SortOrder, &a.CreatedAt); err != nil {
			return nil, err
		}
		if result.Valid {
			a.Result = &result.String
		}
		plan.Actions = append(plan.Actions, a)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &plan, nil
}

// savePlan saves a plan and its actions to the database within a transaction.
func (h *PlanHandler) SavePlan(ctx context.Context, p *model.Plan) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert plan
	_, err = tx.ExecContext(ctx,
		`INSERT INTO plans (id, conversation_id, reasoning, status, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.ConversationID, p.Reasoning, p.Status, p.CreatedAt)
	if err != nil {
		return err
	}

	// Insert actions
	for _, a := range p.Actions {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO plan_actions (id, plan_id, type, params, depends_on, status, sort_order, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ID, a.PlanID, a.Type, a.Params, a.DependsOn, a.Status, a.SortOrder, a.CreatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// writeJSON sets the Content-Type header and encodes v to the response writer.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
