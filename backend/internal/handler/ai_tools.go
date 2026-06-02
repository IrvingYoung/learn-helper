package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"learn-helper/internal/ai"
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

	params := json.RawMessage(mustMarshal(parsed))

	switch tool {
	case "create_page":
		return h.engine.CreatePageFromAction(ctx, params, focusPageID)
	case "update_page":
		return h.engine.UpdatePageFromAction(ctx, params)
	case "patch_page":
		return h.engine.PatchPageFromAction(ctx, params)
	case "delete_page":
		return h.engine.DeletePageFromAction(ctx, params)
	case "link_pages":
		return h.engine.LinkPagesFromAction(ctx, params)
	case "move_page":
		return h.engine.MovePageFromAction(ctx, params)
	default:
		return "", fmt.Errorf("unsupported write tool: %s", tool)
	}
}

func mustMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// executeLoadSkillTool handles the LLM-initiated load_skill tool call.
// Looks up the named skill in the registry and returns its body. If the
// name is unknown, returns an error message (the LLM should pick a
// different skill or proceed without).
func (h *AIHandler) executeLoadSkillTool(ctx context.Context, c aiToolCall) string {
	var input struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(c.Input), &input); err != nil {
		return fmt.Sprintf("error: load_skill input must be {\"name\": \"...\"}: %v", err)
	}
	if input.Name == "" {
		return "error: load_skill requires a non-empty `name`"
	}
	if h.SkillRegistry == nil {
		return "error: skill registry not configured on this server"
	}
	skill, ok := h.SkillRegistry.Get(input.Name)
	if !ok {
		available := h.SkillRegistry.List()
		names := make([]string, 0, len(available))
		for _, s := range available {
			names = append(names, s.Name)
		}
		return fmt.Sprintf("error: unknown skill %q. available: %s", input.Name, strings.Join(names, ", "))
	}
	log.Printf("[load_skill] name=%q body_len=%d", skill.Name, len(skill.Body))
	return skill.Body
}
