package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"learn-helper/internal/ai"
	"learn-helper/internal/model"
)

type AIHandler struct {
	db      *sql.DB
	queries *model.Queries
}

func NewAIHandler(db *sql.DB) *AIHandler {
	return &AIHandler{
		db:      db,
		queries: model.New(db),
	}
}

type PendingAction struct {
	Type    string `json:"type"`
	Preview string `json:"preview"`
	Details any    `json:"details"`
}

// --- Conversation handlers (direct SQL, schema has role column) ---

func (h *AIHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, context_type, role, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		http.Error(w, "Failed to list conversations", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type conv struct {
		ID        int64  `json:"id"`
		Role      string `json:"role"`
		Title     string `json:"title"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	var result []conv
	for rows.Next() {
		var c conv
		var ctxType, title sql.NullString
		if err := rows.Scan(&c.ID, &ctxType, &c.Role, &title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.Title = title.String
		result = append(result, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"conversations": result})
}

func (h *AIHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = ai.RoleWikiMaintainer
	}

	var id int64
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO conversations (context_type, role, title) VALUES ('wiki', ?, ?) RETURNING id`,
		req.Role, req.Title,
	).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to create conversation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func (h *AIHandler) UpdateConversationTitle(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	_, err = h.db.ExecContext(r.Context(), `UPDATE conversations SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, req.Title, id)
	if err != nil {
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *AIHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	_, err = h.db.ExecContext(r.Context(), `DELETE FROM conversations WHERE id = ?`, id)
	if err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AIHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `SELECT id, role, content, model_provider, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at`, id)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type msg struct {
		ID            int64  `json:"id"`
		Role          string `json:"role"`
		Content       string `json:"content"`
		ModelProvider string `json:"model_provider"`
		CreatedAt     string `json:"created_at"`
	}

	var result []msg
	for rows.Next() {
		var m msg
		var mp sql.NullString
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &mp, &m.CreatedAt); err != nil {
			continue
		}
		m.ModelProvider = mp.String
		result = append(result, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"messages": result})
}

// --- AI Config handlers ---

func (h *AIHandler) GetAIConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	config, err := h.queries.GetActiveAIConfig(ctx)
	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"configs": []any{}})
		return
	}
	if err != nil {
		http.Error(w, "Failed to fetch config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"configs": []map[string]any{{
			"id":         config.ID,
			"provider":   config.Provider,
			"model_name": config.ModelName,
			"is_active":  config.IsActive.Int64 == 1,
		}},
	})
}

func (h *AIHandler) UpsertAIConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider  string `json:"provider"`
		ModelName string `json:"model_name"`
		ApiKey    string `json:"api_key"`
		IsActive  bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ApiKey == "" {
		http.Error(w, "API key is required", http.StatusBadRequest)
		return
	}
	if req.Provider == "" {
		req.Provider = "claude"
	}
	if req.ModelName == "" {
		req.ModelName = "claude-sonnet-4-7-20250514"
	}

	ctx := r.Context()

	if req.IsActive {
		h.queries.DeactivateAllConfigs(ctx)
	}

	existing, err := h.queries.GetAIConfigByProvider(ctx, req.Provider)
	if err == sql.ErrNoRows {
		_, err = h.queries.CreateAIConfig(ctx, model.CreateAIConfigParams{
			Provider:  req.Provider,
			ModelName: req.ModelName,
			ApiKey:    req.ApiKey,
			IsActive:  sql.NullInt64{Int64: 1, Valid: true},
		})
	} else if err == nil {
		err = h.queries.UpdateAIConfig(ctx, model.UpdateAIConfigParams{
			Provider:  req.Provider,
			ModelName: req.ModelName,
			ApiKey:    req.ApiKey,
			IsActive:  sql.NullInt64{Int64: boolToInt64(req.IsActive), Valid: true},
			ID:        existing.ID,
		})
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- AI Chat (SSE streaming) ---

func (h *AIHandler) AIChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID   int64           `json:"conversation_id"`
		Message          string          `json:"message"`
		ConfirmedActions []PendingAction `json:"confirmed_actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get AI config
	config, err := h.queries.GetActiveAIConfig(ctx)
	if err != nil {
		http.Error(w, "No AI configuration found. Please configure in settings.", http.StatusBadRequest)
		return
	}

	// Get conversation role (needed early for confirmation response path)
	var convRole string
	h.db.QueryRowContext(ctx, `SELECT role FROM conversations WHERE id = ?`, req.ConversationID).Scan(&convRole)

	// Execute confirmed actions and inject results, OR save user message
	if len(req.ConfirmedActions) > 0 {
		h.executeConfirmedActions(ctx, req.ConfirmedActions)

		// Build a readable execution result
		var resultBuilder strings.Builder
		resultBuilder.WriteString("[系统] 以下操作已执行完成：\n")
		for _, action := range req.ConfirmedActions {
			details, ok := action.Details.(map[string]any)
			if !ok {
				continue
			}
			switch action.Type {
			case "create":
				if title, _ := details["title"].(string); title != "" {
					resultBuilder.WriteString(fmt.Sprintf("- 创建页面「%s」\n", title))
				}
			case "update":
				if title, _ := details["title"].(string); title != "" {
					resultBuilder.WriteString(fmt.Sprintf("- 更新页面「%s」\n", title))
				} else if pid, _ := details["page_id"].(float64); pid > 0 {
					resultBuilder.WriteString(fmt.Sprintf("- 更新页面 #%d\n", int(pid)))
				}
			case "delete":
				resultBuilder.WriteString("- 删除页面\n")
			}
		}
		resultContent := resultBuilder.String()

		// Save execution result as user message so the AI can continue reasoning
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
			req.ConversationID, resultContent, config.Provider)
	}

	// Save user message (only when there's actual text)
	if req.Message != "" {
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
			req.ConversationID, req.Message, config.Provider)
	}

	// --- Common path: provider, load history, context, streaming ---

	provider, err := ai.NewProvider(ai.ProviderType(config.Provider), config.ApiKey, config.ModelName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid provider: %v", err), http.StatusBadRequest)
		return
	}

	// Load history
	rows, err := h.db.QueryContext(ctx, `SELECT role, content FROM messages WHERE conversation_id = ? ORDER BY created_at`, req.ConversationID)
	if err != nil {
		http.Error(w, "Failed to load history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var aiMessages []ai.Message
	for rows.Next() {
		var role, content string
		if rows.Scan(&role, &content) == nil {
			aiMessages = append(aiMessages, ai.Message{Role: role, Content: content})
		}
	}

	// SSE setup
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, canFlush := w.(http.Flusher)

	// Build context and prompt
	wikiContext := h.buildWikiContext(ctx, nil)
	systemPrompt := ai.BuildSystemPrompt(convRole, wikiContext)

	chatReq := ai.ChatRequest{
		Messages:     aiMessages,
		SystemPrompt: systemPrompt,
		MaxTokens:    4096,
	}

	if convRole == ai.RoleWikiMaintainer {
		chatReq.Tools = ai.WikiTools()
	}

	// First: non-streaming call to detect lookup_page (read-only) tool calls
	resp, err := provider.Chat(ctx, chatReq)
	if err != nil {
		sseWrite(w, "error", fmt.Sprintf("AI error: %v", err), canFlush, flusher)
		return
	}

	// Check if the AI called read-only tools (lookup_page, search_pages, read_page)
	// that don't need user confirmation
	autoTools := map[string]bool{"lookup_page": true, "search_pages": true, "read_page": true}
	hasAutoTool := false
	var otherToolCalls []ai.ToolCall
	for _, tc := range resp.ToolCalls {
		if autoTools[tc.Name] {
			hasAutoTool = true
		} else {
			otherToolCalls = append(otherToolCalls, tc)
		}
	}

	if hasAutoTool && convRole == ai.RoleWikiMaintainer {
		// Add the first assistant response to conversation history
		var firstContentBuf strings.Builder
		firstContentBuf.WriteString(resp.Content)
		if len(otherToolCalls) > 0 {
			firstContentBuf.WriteString("\n[操作建议]\n")
			for _, tc := range otherToolCalls {
				firstContentBuf.WriteString(fmt.Sprintf("- %s: %s\n", tc.Name, tc.Input))
			}
		}
		if firstContentBuf.Len() > 0 {
			aiMessages = append(aiMessages, ai.Message{Role: "assistant", Content: firstContentBuf.String()})
		}

		// Execute each read-only tool call and inject result as a user message
		for _, tc := range resp.ToolCalls {
			if !autoTools[tc.Name] {
				continue
			}
			var result string
			switch tc.Name {
			case "lookup_page":
				result = h.executeLookupPage(ctx, tc)
			case "search_pages":
				result = h.executeSearchPages(ctx, tc)
			case "read_page":
				result = h.executeReadPage(ctx, tc)
			}
			if result != "" {
				aiMessages = append(aiMessages, ai.Message{Role: "user", Content: result})
			}
		}

		// Streaming follow-up call with lookup results injected
		chatReq.Messages = aiMessages
		ch, err := provider.StreamChat(ctx, chatReq)
		if err != nil {
			sseWrite(w, "error", fmt.Sprintf("AI error: %v", err), canFlush, flusher)
			return
		}

		var fullContent strings.Builder
		var toolCalls []*ai.ToolCall

		for chunk := range ch {
			if chunk.Content != "" {
				fullContent.WriteString(chunk.Content)
				sseWrite(w, "content", chunk.Content, canFlush, flusher)
			}
			if chunk.ToolCall != nil {
				toolCalls = append(toolCalls, chunk.ToolCall)
			}
			if chunk.Done {
				if len(toolCalls) > 0 {
					pendingActions := h.toolCallsToPendingActions(toolCalls)
					metaBytes, _ := json.Marshal(map[string]any{"pending_actions": pendingActions})
					sseWrite(w, "meta", string(metaBytes), canFlush, flusher)
				}
				sseWrite(w, "done", `{"token_count":0}`, canFlush, flusher)
			}
		}

		// Save assistant message
		content := fullContent.String()
		if content != "" || len(toolCalls) > 0 {
			savedContent := content
			if len(toolCalls) > 0 {
				savedContent += "\n[操作建议]\n"
				for _, tc := range toolCalls {
					savedContent += fmt.Sprintf("- %s: %s\n", tc.Name, tc.Input)
				}
			}
			h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
				req.ConversationID, savedContent, config.Provider)
		}
	} else {
		// No lookup_page — stream the response normally
		ch, err := provider.StreamChat(ctx, chatReq)
		if err != nil {
			sseWrite(w, "error", fmt.Sprintf("AI error: %v", err), canFlush, flusher)
			return
		}

		var fullContent strings.Builder
		var toolCalls []*ai.ToolCall

		for chunk := range ch {
			if chunk.Content != "" {
				fullContent.WriteString(chunk.Content)
				sseWrite(w, "content", chunk.Content, canFlush, flusher)
			}
			if chunk.ToolCall != nil {
				toolCalls = append(toolCalls, chunk.ToolCall)
			}
			if chunk.Done {
				if len(toolCalls) > 0 {
					pendingActions := h.toolCallsToPendingActions(toolCalls)
					metaBytes, _ := json.Marshal(map[string]any{"pending_actions": pendingActions})
					sseWrite(w, "meta", string(metaBytes), canFlush, flusher)
				}
				sseWrite(w, "done", `{"token_count":0}`, canFlush, flusher)
			}
		}

		// Save assistant message
		content := fullContent.String()
		if content != "" || len(toolCalls) > 0 {
			savedContent := content
			if len(toolCalls) > 0 {
				savedContent += "\n[操作建议]\n"
				for _, tc := range toolCalls {
					savedContent += fmt.Sprintf("- %s: %s\n", tc.Name, tc.Input)
				}
			}
			h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
				req.ConversationID, savedContent, config.Provider)
		}
	}

	h.db.ExecContext(ctx, `UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, req.ConversationID)

	if convRole == ai.RoleWikiMaintainer {
		go h.updateOverviewPage()
	}
}

// --- File Upload ---

func (h *AIHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		http.Error(w, "File too large (max 5MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(header.Filename[strings.LastIndex(header.Filename, "."):])
	if ext != ".md" && ext != ".txt" && ext != ".pdf" {
		http.Error(w, "Only .md, .txt, and .pdf files are supported", http.StatusBadRequest)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	var text string
	if ext == ".pdf" {
		text = "[PDF file uploaded - " + header.Filename + ", " + fmt.Sprintf("%d", len(content)) + " bytes. PDF text extraction not yet supported.]"
	} else {
		text = string(content)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"content":  text,
		"filename": header.Filename,
		"size":     len(content),
	})
}

// --- Helpers ---

func (h *AIHandler) renderTreeContext(pages []model.GetWikiPageTreeRow, header string) string {
	if len(pages) == 0 {
		return ""
	}

	type node struct {
		page     model.GetWikiPageTreeRow
		children []int64
	}
	nodeMap := make(map[int64]*node)
	var roots []int64

	for i := range pages {
		p := pages[i]
		nodeMap[p.ID] = &node{page: p}
	}
	for i := range pages {
		p := pages[i]
		if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
			roots = append(roots, p.ID)
		} else if parent, ok := nodeMap[p.ParentID.Int64]; ok {
			parent.children = append(parent.children, p.ID)
		}
	}

	var b strings.Builder
	if header != "" {
		b.WriteString(fmt.Sprintf("【%s】的子页面:\n", header))
	}
	var render func(ids []int64, indent string)
	render = func(ids []int64, indent string) {
		for _, id := range ids {
			n := nodeMap[id]
			status := "空"
			if n.page.ContentStatus == "published" {
				status = "有内容"
			}
			b.WriteString(fmt.Sprintf("%s- [ID=%d] %s (%s)\n", indent, n.page.ID, n.page.Title, status))
			if len(n.children) > 0 {
				render(n.children, indent+"  ")
			}
		}
	}
	render(roots, "")
	return b.String()
}

func (h *AIHandler) buildWikiContext(ctx context.Context, targetPageID *int64) string {
	if targetPageID != nil {
		// Subtree mode: only load the target page and its descendants
		parentPage, err := h.queries.GetWikiPageByID(ctx, *targetPageID)
		if err != nil {
			return fmt.Sprintf("（未找到目标页面 ID=%d）", *targetPageID)
		}
		pages, err := h.queries.GetSubtreePages(ctx, sql.NullString{String: parentPage.Path, Valid: true})
		if err != nil || len(pages) == 0 {
			return fmt.Sprintf("（页面「%s」下没有子节点）", parentPage.Title)
		}
		// Convert to GetWikiPageTreeRow for the render helper (identical fields)
		treeRows := make([]model.GetWikiPageTreeRow, len(pages))
		for i, p := range pages {
			treeRows[i] = model.GetWikiPageTreeRow{
				ID:            p.ID,
				Title:         p.Title,
				Slug:          p.Slug,
				PageType:      p.PageType,
				ContentStatus: p.ContentStatus,
				ParentID:      p.ParentID,
				SortOrder:     p.SortOrder,
				Path:          p.Path,
			}
		}
		return h.renderTreeContext(treeRows, parentPage.Title)
	}

	// Existing full-tree behavior
	pages, err := h.queries.GetWikiPageTree(ctx)
	if err != nil || len(pages) == 0 {
		return "（知识库为空）"
	}
	return h.renderTreeContext(pages, "")
}

func (h *AIHandler) toolCallsToPendingActions(toolCalls []*ai.ToolCall) []PendingAction {
	autoTools := map[string]bool{"lookup_page": true, "search_pages": true, "read_page": true}
	var actions []PendingAction

	for _, tc := range toolCalls {
		// Skip read-only tools — they should be auto-executed, not pending confirmation
		if autoTools[tc.Name] {
			continue
		}

		var details map[string]any
		if err := json.Unmarshal([]byte(tc.Input), &details); err != nil {
			details = map[string]any{"raw": tc.Input}
		}

		var action PendingAction
		switch tc.Name {
		case "create_page":
			title, _ := details["title"].(string)
			action = PendingAction{
				Type:    "create",
				Preview: fmt.Sprintf("创建页面「%s」", title),
				Details: details,
			}
		case "update_page":
			pageID, _ := details["page_id"].(float64)
			title, _ := details["title"].(string)
			preview := fmt.Sprintf("更新页面 #%d", int(pageID))
			if title != "" {
				preview = fmt.Sprintf("更新页面「%s」", title)
			}
			action = PendingAction{
				Type:    "update",
				Preview: preview,
				Details: details,
			}
		case "delete_page":
			pageID, _ := details["page_id"].(float64)
			action = PendingAction{
				Type:    "delete",
				Preview: fmt.Sprintf("删除页面 #%d", int(pageID)),
				Details: details,
			}
		default:
			action = PendingAction{
				Type:    tc.Name,
				Preview: fmt.Sprintf("操作: %s", tc.Name),
				Details: details,
			}
		}
		actions = append(actions, action)
	}
	return actions
}

func (h *AIHandler) executeConfirmedActions(ctx context.Context, actions []PendingAction) {
	for _, action := range actions {
		details, ok := action.Details.(map[string]any)
		if !ok {
			continue
		}
		switch action.Type {
		case "create":
			var parentID sql.NullInt64
			if pid, ok := details["parent_id"].(float64); ok && pid > 0 {
				parentID = sql.NullInt64{Int64: int64(pid), Valid: true}
			}
			slug, _ := details["slug"].(string)
			title, _ := details["title"].(string)
			content := ""
			if c, ok := details["content"].(string); ok {
				content = c
			}
			status := "published"
			if content == "" {
				status = "empty"
			}
			h.queries.CreateWikiPage(ctx, model.CreateWikiPageParams{
				Title: title, Slug: slug, PageType: "entity",
				Content: content, ParentID: parentID,
				ContentStatus: status, SortOrder: 0,
			})
		case "update":
			pageID, ok := details["page_id"].(float64)
			if !ok {
				continue
			}
			page, err := h.queries.GetWikiPageByID(ctx, int64(pageID))
			if err != nil {
				continue
			}
			title := page.Title
			if t, ok := details["title"].(string); ok && t != "" {
				title = t
			}
			content := page.Content
			if c, ok := details["content"].(string); ok && c != "" {
				content = c
			}
			h.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
				Title: title, Slug: page.Slug, PageType: page.PageType,
				Content: content, Tags: page.Tags, ParentID: page.ParentID,
				ContentStatus: "published", SortOrder: page.SortOrder, ID: int64(pageID),
			})
		case "delete":
			pageID, ok := details["page_id"].(float64)
			if !ok {
				continue
			}
			h.queries.DeleteWikiPage(ctx, int64(pageID))
		}
	}
}

func (h *AIHandler) executeLookupPage(ctx context.Context, tc ai.ToolCall) string {
	var details struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &details); err != nil || details.Title == "" {
		return "[系统] lookup_page 执行失败：参数无效"
	}

	page, err := h.queries.GetWikiPageByTitle(ctx, details.Title)
	if err != nil {
		return fmt.Sprintf("[系统] lookup_page 未找到页面「%s」", details.Title)
	}

	result, _ := json.Marshal(map[string]any{
		"id":             page.ID,
		"title":          page.Title,
		"slug":           page.Slug,
		"content_status": page.ContentStatus,
	})

	// Build subtree context for this page
	subtreeContext := h.buildWikiContext(ctx, &page.ID)

	return fmt.Sprintf(
		"[系统] 工具 lookup_page 已执行完毕，查询「%s」结果：%s\n\n%s",
		details.Title, string(result), subtreeContext,
	)
}

func (h *AIHandler) executeSearchPages(ctx context.Context, tc ai.ToolCall) string {
	var details struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &details); err != nil || details.Query == "" {
		return "[系统] search_pages 执行失败：参数无效"
	}

	pages, err := h.queries.SearchWikiPages(ctx, details.Query)
	if err != nil {
		return fmt.Sprintf("[系统] search_pages 执行失败：%v", err)
	}

	if len(pages) == 0 {
		return fmt.Sprintf("[系统] search_pages 未找到匹配「%s」的页面", details.Query)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[系统] 搜索「%s」找到 %d 个匹配页面：\n\n", details.Query, len(pages)))
	for _, p := range pages {
		status := "空"
		if p.ContentStatus == "published" {
			status = "有内容"
		}
		// Show a preview of the content (first 100 chars)
		preview := ""
		if p.Content != "" {
			runes := []rune(p.Content)
			if len(runes) > 100 {
				preview = string(runes[:100]) + "..."
			} else {
				preview = p.Content
			}
		}
		b.WriteString(fmt.Sprintf("- [ID=%d] %s (%s)\n", p.ID, p.Title, status))
		if preview != "" {
			b.WriteString(fmt.Sprintf("  %s\n\n", preview))
		}
	}

	return b.String()
}

func (h *AIHandler) executeReadPage(ctx context.Context, tc ai.ToolCall) string {
	var details struct {
		PageID float64 `json:"page_id"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &details); err != nil || details.PageID == 0 {
		return "[系统] read_page 执行失败：参数无效"
	}

	page, err := h.queries.GetWikiPageByID(ctx, int64(details.PageID))
	if err != nil {
		return fmt.Sprintf("[系统] read_page 未找到页面 #%d", int(details.PageID))
	}

	return fmt.Sprintf(
		"[系统] 工具 read_page 已执行完毕，读取页面「%s」(ID=%d) 内容：\n\n%s",
		page.Title, page.ID, page.Content,
	)
}

func (h *AIHandler) updateOverviewPage() {
	ctx := context.Background()

	pages, err := h.queries.GetWikiPageTree(ctx)
	if err != nil {
		return
	}

	total, published, empty := 0, 0, 0
	var emptyPages []string
	for _, p := range pages {
		if p.PageType == "overview" {
			continue
		}
		total++
		if p.ContentStatus == "published" {
			published++
		} else {
			empty++
			emptyPages = append(emptyPages, p.Title)
		}
	}

	recentPages, _ := h.queries.GetRecentlyUpdatedWikiPages(ctx)

	var b strings.Builder
	b.WriteString("# 知识库概览\n\n")
	b.WriteString(fmt.Sprintf("📊 **总页面数**: %d | ✅ **已完成**: %d | 📝 **待补充**: %d\n\n", total, published, empty))
	if total > 0 {
		b.WriteString(fmt.Sprintf("**覆盖率**: %.0f%%\n\n", float64(published)/float64(total)*100))
	}
	if len(recentPages) > 0 {
		b.WriteString("## 最近更新\n\n")
		for _, p := range recentPages {
			b.WriteString(fmt.Sprintf("- **%s** — %s\n", p.Title, p.UpdatedAt.Format("2006-01-02")))
		}
		b.WriteString("\n")
	}
	if len(emptyPages) > 0 {
		b.WriteString("## 待补充内容\n\n")
		for _, t := range emptyPages {
			b.WriteString(fmt.Sprintf("- %s\n", t))
		}
		b.WriteString("\n")
	}
	b.WriteString("---\n*此页面由系统自动维护*\n")

	overview, err := h.queries.GetOverviewPage(ctx)
	if err != nil {
		return
	}
	h.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
		Title: overview.Title, Slug: overview.Slug, PageType: overview.PageType,
		Content: b.String(), Tags: overview.Tags, ParentID: overview.ParentID,
		ContentStatus: "published", SortOrder: overview.SortOrder, ID: overview.ID,
	})
}

func sseWrite(w http.ResponseWriter, eventType, data string, canFlush bool, flusher http.Flusher) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
	if canFlush {
		flusher.Flush()
	}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
