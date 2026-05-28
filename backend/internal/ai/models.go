package ai

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages     []Message
	SystemPrompt string
	Model        string
	MaxTokens    int
}

type ChatChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

type ChatResponse struct {
	Content    string `json:"content"`
	TokenCount int    `json:"token_count"`
}