package handler

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
