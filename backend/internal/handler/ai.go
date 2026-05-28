package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"learn-helper/internal/ai"
)

type AIHandler struct {
	db        *sql.DB
	providers map[string]ai.AIProvider
}

func NewAIHandler(db *sql.DB) *AIHandler {
	return &AIHandler{
		db:        db,
		providers: make(map[string]ai.AIProvider),
	}
}

func (h *AIHandler) getActiveConfig() (*ai.ChatRequest, error) {
	var provider, modelName, apiKey string
	err := h.db.QueryRow(`SELECT provider, model_name, api_key FROM ai_configs WHERE is_active = 1 LIMIT 1`).
		Scan(&provider, &modelName, &apiKey)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no AI config found, please configure in settings")
	}
	if err != nil {
		return nil, err
	}

	if h.providers[provider] == nil {
		if provider == "claude" {
			h.providers[provider] = ai.NewClaudeProvider(apiKey, modelName)
		}
	}

	req := &ai.ChatRequest{
		Model:     modelName,
		MaxTokens: 4096,
	}
	return req, nil
}

func (h *AIHandler) GetConversations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var conversations []map[string]interface{}
	for rows.Next() {
		var id int
		var topicID, exerciseID int
		var contextType, title string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &topicID, &exerciseID, &contextType, &title, &createdAt, &updatedAt); err != nil {
			continue
		}
		conversations = append(conversations, map[string]interface{}{
			"id":           id,
			"topic_id":     topicID,
			"exercise_id":  exerciseID,
			"context_type": contextType,
			"title":        title,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"conversations": conversations})
}

func (h *AIHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	var conversationID, topicID, exerciseID int
	var contextType, title string
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(`SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations WHERE id = ?`, id).
		Scan(&conversationID, &topicID, &exerciseID, &contextType, &title, &createdAt, &updatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           conversationID,
		"topic_id":     topicID,
		"exercise_id":  exerciseID,
		"context_type": contextType,
		"title":        title,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	})
}

func (h *AIHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	rows, err := h.db.Query(`SELECT id, conversation_id, role, content, model_provider, token_count, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at`, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var messages []map[string]interface{}
	for rows.Next() {
		var id, conversationID int
		var role, content, modelProvider string
		var tokenCount *int
		var createdAt time.Time
		if err := rows.Scan(&id, &conversationID, &role, &content, &modelProvider, &tokenCount, &createdAt); err != nil {
			continue
		}
		messages = append(messages, map[string]interface{}{
			"id":              id,
			"conversation_id": conversationID,
			"role":            role,
			"content":         content,
			"model_provider":  modelProvider,
			"token_count":     tokenCount,
			"created_at":      createdAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"messages": messages})
}

func (h *AIHandler) AIChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID *int    `json:"conversation_id"`
		TopicID        *int    `json:"topic_id"`
		ExerciseID     *int    `json:"exercise_id"`
		ContextType    string  `json:"context_type"`
		Role           string  `json:"role"`
		Message        string  `json:"message"`
		Context        string  `json:"context"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}

	aiReq, err := h.getActiveConfig()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	conversationID := req.ConversationID
	if conversationID == nil {
		title := "新对话"
		if req.ContextType == "topic" {
			title = "知识讲解对话"
		} else if req.ContextType == "exercise" {
			title = "解题辅导对话"
		}

		topicID := 0
		exerciseID := 0
		if req.TopicID != nil {
			topicID = *req.TopicID
		}
		if req.ExerciseID != nil {
			exerciseID = *req.ExerciseID
		}

		var newID int64
		h.db.QueryRow(`INSERT INTO conversations (topic_id, exercise_id, context_type, title) VALUES (?, ?, ?, ?)`, topicID, exerciseID, req.ContextType, title).Scan(&newID)
		id := int(newID)
		conversationID = &id
	}

	systemPrompt := ai.SystemPromptTemplates[req.Role]
	if req.Context != "" {
		systemPrompt = strings.Replace(systemPrompt, "{{.Context}}", req.Context, 1)
	}

	messages := []ai.Message{
		{Role: "user", Content: req.Message},
	}

	aiReq.Messages = messages
	aiReq.SystemPrompt = systemPrompt

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	provider := h.providers["claude"]
	if provider == nil {
		http.Error(w, "AI provider not configured", 500)
		return
	}

	ch, err := provider.StreamChat(context.Background(), *aiReq)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	fullContent := ""
	for chunk := range ch {
		if !chunk.Done {
			fullContent += chunk.Content
			fmt.Fprintf(w, "data: %s\n\n", chunk.Content)
			flusher.Flush()
		}
	}

	providerName := "claude"
	h.db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
		*conversationID, req.Message, providerName)
	h.db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
		*conversationID, fullContent, providerName)
	h.db.Exec(`UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, *conversationID)

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (h *AIHandler) GetAIConfigs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, provider, model_name, is_active, config FROM ai_configs`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var configs []map[string]interface{}
	for rows.Next() {
		var id int
		var provider, modelName string
		var isActive bool
		var config *string
		if err := rows.Scan(&id, &provider, &modelName, &isActive, &config); err != nil {
			continue
		}
		configs = append(configs, map[string]interface{}{
			"id":         id,
			"provider":   provider,
			"model_name": modelName,
			"is_active":  isActive,
			"config":     config,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"configs": configs})
}

func (h *AIHandler) UpsertAIConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider  string `json:"provider"`
		ModelName string `json:"model_name"`
		APIKey    string `json:"api_key"`
		IsActive  bool   `json:"is_active"`
		Config    string `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", 400)
		return
	}

	if input.IsActive {
		h.db.Exec(`UPDATE ai_configs SET is_active = 0`)
	}

	_, err := h.db.Exec(`INSERT INTO ai_configs (provider, model_name, api_key, is_active, config)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		provider = excluded.provider,
		model_name = excluded.model_name,
		api_key = excluded.api_key,
		is_active = excluded.is_active,
		config = excluded.config`,
		input.Provider, input.ModelName, input.APIKey, input.IsActive, input.Config)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}