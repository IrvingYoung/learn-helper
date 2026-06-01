package handler

import (
	"sync"
)

// AskUserAnswer is the value returned to the LLM as tool_result content.
// Answer is one of: string (single option or free text), []any (multi-select), "no_answer".
type AskUserAnswer struct {
	Answer any `json:"answer"`
}

// AskUserRequest is the SSE event payload sent to the frontend.
type AskUserRequest struct {
	RequestID      string          `json:"request_id"`
	ConversationID int64           `json:"conversation_id"`
	Question       string          `json:"question"`
	Options        []string        `json:"options"`
	Context        *AskUserContext `json:"context,omitempty"`
	MultiSelect    bool            `json:"multi_select"`
	AllowFreeText  bool            `json:"allow_free_text"`
	Header         string          `json:"header,omitempty"`
}

// AskUserContext is the optional payload the LLM sends to show alongside the question.
type AskUserContext struct {
	Kind string `json:"kind"` // "outline" | "page" | "markdown" | "diff"
	Data any    `json:"data"`
}

// AskUserResponse is the HTTP body the frontend posts back.
type AskUserResponse struct {
	RequestID string `json:"request_id"`
	Answer    any    `json:"answer"` // string | []string | "no_answer"
}

// AskUserRegistry tracks pending ask_user requests.
type AskUserRegistry struct {
	mu       sync.Mutex
	channels map[string]chan AskUserResponse
}

func NewAskUserRegistry() *AskUserRegistry {
	return &AskUserRegistry{channels: map[string]chan AskUserResponse{}}
}

func (r *AskUserRegistry) Register(requestID string) chan AskUserResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.channels[requestID]; ok {
		return existing
	}
	ch := make(chan AskUserResponse, 1)
	r.channels[requestID] = ch
	return ch
}

// Resolve snapshots the channel under lock, deletes the entry, releases the lock,
// then sends + closes outside the lock so a blocking send cannot stall other
// registry operations.
func (r *AskUserRegistry) Resolve(requestID string, resp AskUserResponse) {
	r.mu.Lock()
	ch, ok := r.channels[requestID]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.channels, requestID)
	r.mu.Unlock()
	ch <- resp
	close(ch)
}

// CancelAll collects channels to close under the lock, then closes them outside
// the lock so a slow consumer cannot stall other registry operations.
func (r *AskUserRegistry) CancelAll() {
	r.mu.Lock()
	chans := make([]chan AskUserResponse, 0, len(r.channels))
	for _, ch := range r.channels {
		chans = append(chans, ch)
	}
	r.channels = map[string]chan AskUserResponse{}
	r.mu.Unlock()
	for _, ch := range chans {
		close(ch)
	}
}

func (r *AskUserRegistry) Pending() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.channels)
}
