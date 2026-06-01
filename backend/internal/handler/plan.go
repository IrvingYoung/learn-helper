package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

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
// Body: { "plan_id": "xxx", "focus_page_id": 123 }
// Confirms a pending plan and executes it.
func (h *PlanHandler) ConfirmPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID      string `json:"plan_id"`
		FocusPageID *int64 `json:"focus_page_id"`
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

	// Check if this is an outline-only plan (no actions)
	plan, err := h.loadPlan(r.Context(), req.PlanID)
	if err != nil {
		http.Error(w, "failed to load plan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Resolve effective focusPageID: request body takes priority over plan's saved value
	effectiveFocusPageID := plan.FocusPageID
	if req.FocusPageID != nil {
		effectiveFocusPageID = req.FocusPageID
	}

	// Helper to save execution result to conversation messages
	saveResultMessage := func(content string) {
		if plan.ConversationID == nil {
			return
		}
		h.db.ExecContext(r.Context(),
			`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, '', 0)`,
			*plan.ConversationID, content)
	}

	if len(plan.Outline) > 0 && len(plan.Actions) == 0 {
		// Outline-only: create skeleton pages
		result, err := h.engine.ExecOutline(r.Context(), string(plan.Outline), effectiveFocusPageID)
		if err != nil {
			http.Error(w, "outline execution failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		saveResultMessage(fmt.Sprintf("我之前提议的大纲已创建完成（共 %d 个页面骨架）。", len(result)))
		writeJSON(w, map[string]any{
			"plan_id": req.PlanID,
			"status":  "completed",
			"outline": result,
		})
		return
	}

	// Execute the plan (has actions)
	report, err := h.engine.ExecutePlan(r.Context(), req.PlanID, effectiveFocusPageID)
	if err != nil {
		http.Error(w, "plan execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	successCount := 0
	for _, a := range report.Actions {
		if a.Status == "completed" {
			successCount++
		}
	}
	saveResultMessage(fmt.Sprintf("我之前的执行计划已执行完成（共 %d 个操作，成功 %d 个）。", len(report.Actions), successCount))

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

// CreatePlan handles POST /api/plans
// Body: { "reasoning": "...", "actions": [...] }
// Creates a plan from user-initiated operations (e.g., delete from tree).
func (h *PlanHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reasoning string `json:"reasoning"`
		Actions   []struct {
			Type   string          `json:"type"`
			Params json.RawMessage `json:"params"`
		} `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Reasoning == "" {
		req.Reasoning = "用户操作"
	}
	if len(req.Actions) == 0 {
		http.Error(w, "at least one action required", http.StatusBadRequest)
		return
	}

	planID := fmt.Sprintf("user-%d-%s", time.Now().UnixMilli(), uuid.New().String()[:8])

	plan := &model.Plan{
		ID:             planID,
		ConversationID: nil,
		Reasoning:      req.Reasoning,
		Status:         "pending",
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	for i, a := range req.Actions {
		actionID := fmt.Sprintf("%s-a%d", planID, i+1)
		plan.Actions = append(plan.Actions, model.PlanAction{
			ID:        actionID,
			PlanID:    planID,
			Type:      a.Type,
			Params:    a.Params,
			DependsOn: json.RawMessage("[]"),
			Status:    "pending",
			SortOrder: int64(i),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	if err := h.SavePlan(r.Context(), plan); err != nil {
		http.Error(w, "failed to save plan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, plan)
}

// loadPlan loads a plan and its actions from the database.
func (h *PlanHandler) loadPlan(ctx context.Context, planID string) (*model.Plan, error) {
	var plan model.Plan
	var executedAt sql.NullString
	var outline sql.NullString
	var conversationID sql.NullInt64
	var focusPageID sql.NullInt64
	var calQuestion sql.NullString

	err := h.db.QueryRowContext(ctx,
		`SELECT id, conversation_id, reasoning, status, outline, phase_index, total_phases, focus_page_id, calibration_question, created_at, executed_at
		 FROM plans WHERE id = ?`, planID).Scan(
		&plan.ID, &conversationID, &plan.Reasoning, &plan.Status, &outline, &plan.PhaseIndex, &plan.TotalPhases, &focusPageID, &calQuestion, &plan.CreatedAt, &executedAt)
	if err != nil {
		return nil, err
	}

	if conversationID.Valid {
		plan.ConversationID = &conversationID.Int64
	}
	if focusPageID.Valid {
		plan.FocusPageID = &focusPageID.Int64
	}
	if calQuestion.Valid {
		plan.CalibrationQuestion = json.RawMessage(calQuestion.String)
	}
	if executedAt.Valid {
		plan.ExecutedAt = &executedAt.String
	}
	if outline.Valid {
		s := outline.String
		plan.Outline = json.RawMessage(s)
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
	stage := p.Stage
	if stage == "" {
		stage = "main"
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO plans (id, conversation_id, reasoning, status, outline, phase_index, total_phases, focus_page_id, calibration_question, created_at, stage)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.ConversationID, p.Reasoning, p.Status, p.Outline, p.PhaseIndex, p.TotalPhases, p.FocusPageID, p.CalibrationQuestion, p.CreatedAt, stage)
	if err != nil {
		return err
	}

	// Insert actions (if any)
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
