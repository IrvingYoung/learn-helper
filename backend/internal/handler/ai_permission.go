package handler

// PermissionDecision is a single user's decision for one tool call in a permission batch.
type PermissionDecision struct {
	ID          string         `json:"id"`
	Action      string         `json:"action"` // "approve" | "reject" | "edit"
	EditedInput map[string]any `json:"edited_input,omitempty"`
}

// PermissionRequest is the SSE event payload sent to the frontend.
type PermissionRequest struct {
	RequestID      string                  `json:"request_id"`
	ConversationID int64                   `json:"conversation_id"`
	Items          []PermissionRequestItem `json:"items"`
}

// PermissionRequestItem is one write op in a permission batch.
type PermissionRequestItem struct {
	ID      string         `json:"id"`
	Tool    string         `json:"tool"`
	Input   map[string]any `json:"input"`
	Preview string         `json:"preview"`
}

// PermissionResponse is the HTTP body the frontend posts back.
type PermissionResponse struct {
	RequestID string               `json:"request_id"`
	Decisions []PermissionDecision `json:"decisions"`
}
