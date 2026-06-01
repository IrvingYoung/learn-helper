package handler

import (
	"sync"
)

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

// PermissionRegistry tracks pending permission requests per conversation.
// requestID -> chan of decisions.
type PermissionRegistry struct {
	mu       sync.Mutex
	channels map[string]chan []PermissionDecision
}

func NewPermissionRegistry() *PermissionRegistry {
	return &PermissionRegistry{channels: map[string]chan []PermissionDecision{}}
}

// Register creates a pending channel for requestID. If a request with the same id
// is already registered (re-entrant), returns the existing channel.
func (r *PermissionRegistry) Register(requestID string, capacity int) chan []PermissionDecision {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.channels[requestID]; ok {
		return existing
	}
	ch := make(chan []PermissionDecision, capacity)
	r.channels[requestID] = ch
	return ch
}

// Resolve sends the decisions to the registered channel. No-op if unknown.
func (r *PermissionRegistry) Resolve(requestID string, decisions []PermissionDecision) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channels[requestID]
	if !ok {
		return
	}
	ch <- decisions
	close(ch)
	delete(r.channels, requestID)
}

// CancelAll drops every pending channel. Used on SSE disconnect.
// Decisions are NOT sent — callers default to reject on cancel.
func (r *PermissionRegistry) CancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, ch := range r.channels {
		close(ch)
		delete(r.channels, id)
	}
}

func (r *PermissionRegistry) Pending() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.channels)
}
