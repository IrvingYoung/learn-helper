package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"learn-helper/internal/ai"
	"learn-helper/internal/model"
)

func nullStr(s sql.NullString) interface{} {
	if s.Valid {
		return s.String
	}
	return nil
}

func nullInt(i sql.NullInt64) interface{} {
	if i.Valid {
		return i.Int64
	}
	return nil
}

func nullTime(t sql.NullTime) interface{} {
	if t.Valid {
		return t.Time
	}
	return nil
}

// PendingAction represents a wiki action awaiting user confirmation
type PendingAction struct {
	Type     string `json:"type"`          // create, update, delete
	PageID   *int64 `json:"page_id,omitempty"`
	Title    string `json:"title,omitempty"`
	Slug     string `json:"slug,omitempty"`
	Content  string `json:"content,omitempty"`
	ParentID *int64 `json:"parent_id,omitempty"`
	Preview  string `json:"preview"`
}

type AIHandler struct {
	db        *sql.DB
	providers map[string]ai.AIProvider
	queries   *model.Queries
}

func NewAIHandler(db *sql.DB) *AIHandler {
	return &AIHandler{
		db:        db,
		providers: make(map[string]ai.AIProvider),
		queries:   model.New(db),
	}
}

func (h *AIHandler) getActiveConfig() (*ai.ChatRequest, *string, error) {
	var provider, modelName, apiKey string
	err := h.db.QueryRow(`SELECT provider, model_name, api_key FROM ai_configs WHERE is_active = 1 LIMIT 1`).
		Scan(&provider, &modelName, &apiKey)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("no AI config found, please configure in settings")
	}
	if err != nil {
		return nil, nil, err
	}

	if h.providers[provider] == nil {
		p, err := ai.NewProvider(ai.ProviderType(provider), apiKey, modelName)
		if err != nil {
			return nil, nil, err
		}
		h.providers[provider] = p
	}

	req := &ai.ChatRequest{
		Model:     modelName,
		MaxTokens: 4096,
	}
	return req, &provider, nil
}

// buildWikiContext reads the wiki tree and builds a context string for the system prompt
func (h *AIHandler) buildWikiContext(ctx context.Context) string {
	pages, err := h.queries.GetWikiPageTree(ctx)
	if err != nil {
		log.Printf("failed to get wiki page tree: %v", err)
		return "无法加载 Wiki 知识库结构"
	}

	if len(pages) == 0 {
		return "Wiki 知识库当前为空"
	}

	var sb strings.Builder
	sb.WriteString("Wiki 知识库结构：\n\n")

	// Build a simple tree representation
	nodeMap := make(map[int64]*model.GetWikiPageTreeRow)
	var roots []model.GetWikiPageTreeRow

	for i := range pages {
		p := pages[i]
		nodeMap[p.ID] = &p
	}

	for i := range pages {
		p := pages[i]
		if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
			roots = append(roots, p)
		}
	}

	var renderNode func(page model.GetWikiPageTreeRow, indent string)
	renderNode = func(page model.GetWikiPageTreeRow, indent string) {
		status := ""
		if page.ContentStatus == "empty" {
			status = " [空]"
		} else if page.ContentStatus == "draft" {
			status = " [草稿]"
		}
		sb.WriteString(fmt.Sprintf("%s- %s (ID: %d, Slug: %s, Type: %s%s)\n",
			indent, page.Title, page.ID, page.Slug, page.PageType, status))

		// Find children
		for i := range pages {
			p := pages[i]
			if p.ParentID.Valid && p.ParentID.Int64 == page.ID {
				renderNode(p, indent+"  ")
			}
		}
	}

	for _, root := range roots {
		renderNode(root, "")
	}

	// Add summary
	totalPages, _ := h.queries.CountWikiPages(ctx)
	emptyPages, _ := h.queries.CountWikiPagesByStatus(ctx, "empty")
	sb.WriteString(fmt.Sprintf("\n总计: %d 个页面, %d 个空页面\n", totalPages, emptyPages))

	return sb.String()
}

// executeConfirmedActions executes wiki actions that have been confirmed by the user
func (h *AIHandler) executeConfirmedActions(ctx context.Context, actions []PendingAction) error {
	for _, action := range actions {
		switch action.Type {
		case "create":
			var parentID sql.NullInt64
			if action.ParentID != nil {
				parentID = sql.NullInt64{Int64: *action.ParentID, Valid: true}
			}
			contentStatus := "published"
			if action.Content == "" {
				contentStatus = "empty"
			}
			_, err := h.queries.CreateWikiPage(ctx, model.CreateWikiPageParams{
				Title:         action.Title,
				Slug:          action.Slug,
				PageType:      "entity",
				Content:       action.Content,
				ParentID:      parentID,
				ContentStatus: contentStatus,
				SortOrder:     0,
			})
			if err != nil {
				return fmt.Errorf("failed to create page %s: %w", action.Slug, err)
			}
			log.Printf("Created wiki page: %s (slug: %s)", action.Title, action.Slug)

		case "update":
			if action.PageID == nil {
				return fmt.Errorf("page_id is required for update action")
			}
			contentStatus := "published"
			if action.Content == "" {
				contentStatus = "empty"
			}
			// Get existing page to preserve other fields
			existing, err := h.queries.GetWikiPageByID(ctx, *action.PageID)
			if err != nil {
				return fmt.Errorf("failed to get page %d: %w", *action.PageID, err)
			}
			title := action.Title
			if title == "" {
				title = existing.Title
			}
			slug := action.Slug
			if slug == "" {
				slug = existing.Slug
			}
			err = h.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
				ID:            *action.PageID,
				Title:         title,
				Slug:          slug,
				PageType:      existing.PageType,
				Content:       action.Content,
				ParentID:      existing.ParentID,
				ContentStatus: contentStatus,
				SortOrder:     existing.SortOrder,
			})
			if err != nil {
				return fmt.Errorf("failed to update page %d: %w", *action.PageID, err)
			}
			log.Printf("Updated wiki page ID %d: %s", *action.PageID, title)

		case "delete":
			if action.PageID == nil {
				return fmt.Errorf("page_id is required for delete action")
			}
			err := h.queries.DeleteWikiPage(ctx, *action.PageID)
			if err != nil {
				return fmt.Errorf("failed to delete page %d: %w", *action.PageID, err)
			}
			log.Printf("Deleted wiki page ID %d", *action.PageID)

		default:
			return fmt.Errorf("unknown action type: %s", action.Type)
		}
	}
	return nil
}

// updateOverviewPage updates the overview page with current wiki statistics
func (h *AIHandler) updateOverviewPage(ctx context.Context) error {
	totalPages, err := h.queries.CountWikiPages(ctx)
	if err != nil {
		return fmt.Errorf("failed to count pages: %w", err)
	}

	emptyPages, err := h.queries.CountWikiPagesByStatus(ctx, "empty")
	if err != nil {
		return fmt.Errorf("failed to count empty pages: %w", err)
	}

	emptyList, err := h.queries.GetEmptyWikiPages(ctx)
	if err != nil {
		return fmt.Errorf("failed to get empty pages: %w", err)
	}

	var content strings.Builder
	content.WriteString("# Wiki 知识库概览\n\n")
	content.WriteString(fmt.Sprintf("## 统计信息\n\n"))
	content.WriteString(fmt.Sprintf("- 总页面数: %d\n", totalPages))
	content.WriteString(fmt.Sprintf("- 已完成页面: %d\n", totalPages-emptyPages))
	content.WriteString(fmt.Sprintf("- 空页面: %d\n\n", emptyPages))

	if len(emptyList) > 0 {
		content.WriteString("## 待补充内容的页面\n\n")
		for _, p := range emptyList {
			content.WriteString(fmt.Sprintf("- [%s](/wiki/%s) (ID: %d)\n", p.Title, p.Slug, p.ID))
		}
	}

	// Get or create overview page
	overview, err := h.queries.GetOverviewPage(ctx)
	if err == sql.ErrNoRows {
		// Create overview page
		_, err = h.queries.CreateWikiPage(ctx, model.CreateWikiPageParams{
			Title:         "Wiki 概览",
			Slug:          "overview",
			PageType:      "overview",
			Content:       content.String(),
			ContentStatus: "published",
			SortOrder:     0,
		})
		if err != nil {
			return fmt.Errorf("failed to create overview page: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get overview page: %w", err)
	} else {
		// Update existing overview page
		err = h.queries.UpdateWikiPageContent(ctx, model.UpdateWikiPageContentParams{
			ID:            overview.ID,
			Content:       content.String(),
			ContentStatus: "published",
		})
		if err != nil {
			return fmt.Errorf("failed to update overview page: %w", err)
		}
	}

	return nil
}

// ListConversations returns all conversations sorted by updated_at descending,
// with message_count and last_message_preview.
func (h *AIHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT c.id, c.topic_id, c.exercise_id, c.context_type, c.role, c.title, c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id) as message_count,
			COALESCE(SUBSTR((SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1), 1, 50), '') as last_message_preview
		FROM conversations c
		ORDER BY c.updated_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	conversations := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int
		var topicID, exerciseID sql.NullInt64
		var contextType, role, title sql.NullString
		var messageCount int
		var lastMessagePreview string
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&id, &topicID, &exerciseID, &contextType, &role, &title, &createdAt, &updatedAt, &messageCount, &lastMessagePreview); err != nil {
			continue
		}
		conversations = append(conversations, map[string]interface{}{
			"id":                   id,
			"topic_id":             nullInt(topicID),
			"exercise_id":          nullInt(exerciseID),
			"context_type":         nullStr(contextType),
			"role":                 nullStr(role),
			"title":                nullStr(title),
			"message_count":        messageCount,
			"last_message_preview": lastMessagePreview,
			"created_at":           nullTime(createdAt),
			"updated_at":           nullTime(updatedAt),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(conversations)
}

// CreateConversation creates a new conversation with role, context_type, and optional title.
func (h *AIHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Role        string `json:"role"`
		ContextType string `json:"context_type"`
		TopicID     *int   `json:"topic_id"`
		ExerciseID  *int   `json:"exercise_id"`
		Title       string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.Role == "" {
		http.Error(w, "role is required", http.StatusBadRequest)
		return
	}
	if input.ContextType == "" {
		input.ContextType = "dashboard"
	}

	title := input.Title
	if title == "" {
		title = ai.RoleDisplayNames[input.Role] + "对话"
	}

	var topicID, exerciseID sql.NullInt64
	if input.TopicID != nil {
		topicID = sql.NullInt64{Int64: int64(*input.TopicID), Valid: true}
	}
	if input.ExerciseID != nil {
		exerciseID = sql.NullInt64{Int64: int64(*input.ExerciseID), Valid: true}
	}

	var newID int64
	err := h.db.QueryRow(
		`INSERT INTO conversations (topic_id, exercise_id, context_type, role, title) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		topicID, exerciseID, input.ContextType, input.Role, title,
	).Scan(&newID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create conversation: %v", err), http.StatusInternalServerError)
		return
	}

	var contextType, role, titleStr sql.NullString
	var createdAt, updatedAt sql.NullTime
	err = h.db.QueryRow(
		`SELECT id, topic_id, exercise_id, context_type, role, title, created_at, updated_at FROM conversations WHERE id = ?`,
		newID,
	).Scan(&newID, &topicID, &exerciseID, &contextType, &role, &titleStr, &createdAt, &updatedAt)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read conversation: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           newID,
		"topic_id":     nullInt(topicID),
		"exercise_id":  nullInt(exerciseID),
		"context_type": nullStr(contextType),
		"role":         nullStr(role),
		"title":        nullStr(titleStr),
		"created_at":   nullTime(createdAt),
		"updated_at":   nullTime(updatedAt),
	})
}

// UpdateConversationTitle updates a conversation's title.
func (h *AIHandler) UpdateConversationTitle(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid conversation id", http.StatusBadRequest)
		return
	}

	var input struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if input.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	result, err := h.db.Exec(`UPDATE conversations SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, input.Title, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to update conversation: %v", err), http.StatusInternalServerError)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "conversation not found", http.StatusNotFound)
		return
	}

	var topicID, exerciseID sql.NullInt64
	var contextType, role, title sql.NullString
	var createdAt, updatedAt sql.NullTime
	err = h.db.QueryRow(
		`SELECT id, topic_id, exercise_id, context_type, role, title, created_at, updated_at FROM conversations WHERE id = ?`, id,
	).Scan(&id, &topicID, &exerciseID, &contextType, &role, &title, &createdAt, &updatedAt)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read conversation: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           id,
		"topic_id":     nullInt(topicID),
		"exercise_id":  nullInt(exerciseID),
		"context_type": nullStr(contextType),
		"role":         nullStr(role),
		"title":        nullStr(title),
		"created_at":   nullTime(createdAt),
		"updated_at":   nullTime(updatedAt),
	})
}

// GetConversationMessages returns all messages for a conversation, sorted by created_at ascending.
func (h *AIHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid conversation id", http.StatusBadRequest)
		return
	}

	var exists int
	err = h.db.QueryRow(`SELECT 1 FROM conversations WHERE id = ?`, id).Scan(&exists)
	if err == sql.ErrNoRows {
		http.Error(w, "conversation not found", http.StatusNotFound)
		return
	}

	rows, err := h.db.Query(
		`SELECT id, role, content, model_provider, token_count, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	messages := make([]map[string]interface{}, 0)
	for rows.Next() {
		var msgID int
		var role, content string
		var modelProvider sql.NullString
		var tokenCount sql.NullInt64
		var createdAt sql.NullTime
		if err := rows.Scan(&msgID, &role, &content, &modelProvider, &tokenCount, &createdAt); err != nil {
			continue
		}
		messages = append(messages, map[string]interface{}{
			"id":             msgID,
			"role":           role,
			"content":        content,
			"model_provider": nullStr(modelProvider),
			"token_count":    nullInt(tokenCount),
			"created_at":     nullTime(createdAt),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// DeleteConversation deletes a conversation and all its messages (cascade).
func (h *AIHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid conversation id", http.StatusBadRequest)
		return
	}

	result, err := h.db.Exec(`DELETE FROM conversations WHERE id = ?`, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to delete conversation: %v", err), http.StatusInternalServerError)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "conversation not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AIChat handles streaming AI chat requests with conversation history.
func (h *AIHandler) AIChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID   *int            `json:"conversation_id"`
		TopicID          *int            `json:"topic_id"`
		ExerciseID       *int            `json:"exercise_id"`
		ContextType      string          `json:"context_type"`
		Role             string          `json:"role"`
		Message          string          `json:"message"`
		Context          string          `json:"context"`
		ConfirmedActions []PendingAction `json:"confirmed_actions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.ConversationID == nil {
		http.Error(w, "conversation_id is required", http.StatusBadRequest)
		return
	}

	conversationID := *req.ConversationID

	var convRole sql.NullString
	err := h.db.QueryRow(`SELECT role FROM conversations WHERE id = ?`, conversationID).Scan(&convRole)
	if err == sql.ErrNoRows {
		http.Error(w, "conversation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query conversation: %v", err), http.StatusInternalServerError)
		return
	}

	role := convRole.String
	if role == "" {
		role = req.Role
	}

	aiReq, providerName, err := h.getActiveConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle wiki-maintainer specific behavior
	if role == ai.RoleWikiMaintainer {
		ctx := r.Context()

		// Execute confirmed actions if provided
		if len(req.ConfirmedActions) > 0 {
			if err := h.executeConfirmedActions(ctx, req.ConfirmedActions); err != nil {
				log.Printf("failed to execute confirmed actions: %v", err)
				// Continue with the conversation even if actions fail
			} else {
				// Update overview page after successful actions
				if err := h.updateOverviewPage(ctx); err != nil {
					log.Printf("failed to update overview page: %v", err)
				}
			}
		}
	}

	// Load historical messages with sliding window (max 20)
	rows, err := h.db.Query(
		`SELECT role, content FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`, conversationID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load history: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var allHistory []ai.Message
	for rows.Next() {
		var msgRole, content string
		if err := rows.Scan(&msgRole, &content); err != nil {
			continue
		}
		allHistory = append(allHistory, ai.Message{Role: msgRole, Content: content})
	}

	maxHistory := 20
	history := allHistory
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	messages := append(history, ai.Message{Role: "user", Content: req.Message})

	systemPrompt := ai.SystemPromptTemplates[role]

	// Build context for wiki-maintainer
	if role == ai.RoleWikiMaintainer {
		wikiContext := h.buildWikiContext(r.Context())
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{.Context}}", wikiContext)
	} else if req.Context != "" {
		systemPrompt = strings.Replace(systemPrompt, "{{.Context}}", req.Context, 1)
	}

	aiReq.Messages = messages
	aiReq.SystemPrompt = systemPrompt

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "event: meta\ndata: {\"conversation_id\":%d}\n\n", conversationID)
	flusher.Flush()

	provider := h.providers[*providerName]
	if provider == nil {
		http.Error(w, "AI provider not configured", http.StatusInternalServerError)
		return
	}

	ch, err := provider.StreamChat(context.Background(), *aiReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	if _, err := h.db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
		conversationID, req.Message, providerName); err != nil {
		log.Printf("failed to save user message: %v", err)
	}
	if _, err := h.db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
		conversationID, fullContent, providerName); err != nil {
		log.Printf("failed to save assistant message: %v", err)
	}
	if _, err := h.db.Exec(`UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, conversationID); err != nil {
		log.Printf("failed to update conversation timestamp: %v", err)
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (h *AIHandler) GetAIConfigs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, provider, model_name, is_active, config FROM ai_configs`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	configs := make([]map[string]interface{}, 0)
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
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	if input.IsActive {
		h.db.Exec(`UPDATE ai_configs SET is_active = 0`)
	}

	h.db.Exec(`DELETE FROM ai_configs WHERE provider = ? AND is_active = 0`, input.Provider)

	_, err := h.db.Exec(`INSERT INTO ai_configs (provider, model_name, api_key, is_active, config)
		VALUES (?, ?, ?, ?, ?)`,
		input.Provider, input.ModelName, input.APIKey, input.IsActive, input.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	delete(h.providers, input.Provider)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
