package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"learn-helper/internal/ai"
	"learn-helper/internal/model"
)

// executeReadTool handles the auto-executed read tools.
// Wraps the existing executeAutoTool (used by the old ReAct loop) to handle
// lookup_page / read_page / search_pages / websearch / webfetch.
func (h *AIHandler) executeReadTool(ctx context.Context, c aiToolCall) string {
	tc := ai.ToolCall{ID: c.ID, Name: c.Name, Input: c.Input}
	return h.executeAutoTool(ctx, tc)
}

// executeWriteTool handles one approved write tool call.
// focusPageID is the request's optional focus, used as parent_id fallback for create_page.
func (h *AIHandler) executeWriteTool(ctx context.Context, tool, input string, focusPageID *int64) (string, error) {
	parsed, err := parseWriteInput(tool, json.RawMessage(input))
	if err != nil {
		return "", err
	}

	// FocusPageID fallback for create_page with no parent_id
	if tool == "create_page" && focusPageID != nil {
		if _, hasParent := parsed["parent_id"]; !hasParent {
			parsed["parent_id"] = float64(*focusPageID)
		}
	}

	action := model.PlanAction{
		ID:     c_actionID(tool),
		Type:   tool,
		Params: json.RawMessage(mustMarshal(parsed)),
		Status: "pending",
	}

	switch tool {
	case "create_page":
		return h.engine.CreatePageFromAction(ctx, action, focusPageID)
	case "update_page":
		return h.engine.UpdatePageFromAction(ctx, action)
	case "patch_page":
		return h.engine.PatchPageFromAction(ctx, action)
	case "delete_page":
		return h.engine.DeletePageFromAction(ctx, action)
	case "link_pages":
		return h.engine.LinkPagesFromAction(ctx, action)
	case "move_page":
		return h.engine.MovePageFromAction(ctx, action)
	default:
		return "", fmt.Errorf("unsupported write tool: %s", tool)
	}
}

func c_actionID(tool string) string {
	return fmt.Sprintf("ad-hoc-%s-%d", tool, time.Now().UnixNano())
}

func mustMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
