package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"
	"learn-helper/internal/ai"
	"learn-helper/internal/ai/skills"
	"learn-helper/internal/engine"
	"learn-helper/internal/model"
)

type AIHandler struct {
	db            *sql.DB
	queries       *model.Queries
	engine        *engine.ExecutionEngine
	permissions   *PermissionRegistry
	askUsers      *AskUserRegistry
	SkillRegistry *skills.Registry
}

func NewAIHandler(db *sql.DB, reg *skills.Registry) *AIHandler {
	q := model.New(db)
	return &AIHandler{
		db:            db,
		queries:       q,
		engine:        engine.NewExecutionEngine(db, q),
		permissions:   NewPermissionRegistry(),
		askUsers:      NewAskUserRegistry(),
		SkillRegistry: reg,
	}
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
	json.NewEncoder(w).Encode(map[string]any{
		"id":    id,
		"title": req.Title,
		"role":  req.Role,
	})
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

	rows, err := h.db.QueryContext(r.Context(), `SELECT id, role, content, model_provider, tool_calls, skill, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at`, id)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type msg struct {
		ID            int64   `json:"id"`
		Role          string  `json:"role"`
		Content       string  `json:"content"`
		ModelProvider string  `json:"model_provider"`
		ToolCalls     *string `json:"tool_calls"`
		Skill         string  `json:"skill"`
		CreatedAt     string  `json:"created_at"`
	}

	var result []msg
	for rows.Next() {
		var m msg
		var mp, tc sql.NullString
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &mp, &tc, &m.Skill, &m.CreatedAt); err != nil {
			continue
		}
		m.ModelProvider = mp.String
		if tc.Valid {
			m.ToolCalls = &tc.String
		}
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

	// Parse tavily_api_key from config JSON
	tavilyKey := ""
	if config.Config.Valid && config.Config.String != "" {
		var cfg struct {
			TavilyAPIKey string `json:"tavily_api_key"`
		}
		if json.Unmarshal([]byte(config.Config.String), &cfg) == nil {
			tavilyKey = cfg.TavilyAPIKey
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"configs": []map[string]any{{
			"id":             config.ID,
			"provider":       config.Provider,
			"model_name":     config.ModelName,
			"is_active":      config.IsActive.Int64 == 1,
			"api_key":        config.ApiKey,
			"tavily_api_key": tavilyKey,
		}},
	})
}

func (h *AIHandler) UpsertAIConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider     string `json:"provider"`
		ModelName    string `json:"model_name"`
		ApiKey       string `json:"api_key"`
		IsActive     bool   `json:"is_active"`
		TavilyAPIKey string `json:"tavily_api_key"`
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
		req.Provider = "opencode"
	}
	if req.ModelName == "" {
		req.ModelName = "deepseek-v4-pro"
	}

	ctx := r.Context()

	if req.IsActive {
		h.queries.DeactivateAllConfigs(ctx)
	}

	existing, err := h.queries.GetAIConfigByProvider(ctx, req.Provider)

	// Build config JSON for Tavily API key
	var configJSON sql.NullString
	if req.TavilyAPIKey != "" {
		cfgMap := map[string]string{"tavily_api_key": req.TavilyAPIKey}
		cfgBytes, _ := json.Marshal(cfgMap)
		configJSON = sql.NullString{String: string(cfgBytes), Valid: true}
	} else if err == nil && existing.Config.Valid {
		// Preserve existing config when tavily key not provided
		configJSON = existing.Config
	}
	if err == sql.ErrNoRows {
		_, err = h.queries.CreateAIConfig(ctx, model.CreateAIConfigParams{
			Provider:  req.Provider,
			ModelName: req.ModelName,
			ApiKey:    req.ApiKey,
			IsActive:  sql.NullInt64{Int64: 1, Valid: true},
			Config:    configJSON,
		})
	} else if err == nil {
		err = h.queries.UpdateAIConfig(ctx, model.UpdateAIConfigParams{
			Provider:  req.Provider,
			ModelName: req.ModelName,
			ApiKey:    req.ApiKey,
			IsActive:  sql.NullInt64{Int64: boolToInt64(req.IsActive), Valid: true},
			Config:    configJSON,
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
//
// This is now a thin wrapper around RunReAct (see ai_react.go). The HTTP
// path:
//   - parses the request, saves the user message, loads history
//   - builds the system prompt with the wiki context
//   - constructs a ChatRequest (messages + tools + system prompt)
//   - calls h.RunReAct with an SSE sink and AutoApproveWrites=false
//   - saves the assistant message, sends the SSE "done" event
//   - updates the overview page asynchronously

func (h *AIHandler) AIChat(w http.ResponseWriter, r *http.Request) {
	// Cancel any pending permission/ask_user gates when this SSE stream ends
	// (normal return, error, panic, or client disconnect). CancelAll is a
	// no-op when nothing is pending, so it is safe on every request.
	defer func() {
		if h.permissions != nil {
			h.permissions.CancelAll()
		}
		if h.askUsers != nil {
			h.askUsers.CancelAll()
		}
	}()

	var req struct {
		ConversationID int64  `json:"conversation_id"`
		Message        string `json:"message"`
		FocusPageID    *int64 `json:"focus_page_id"`
		CurrentSlug    string `json:"current_slug"`
		SelectedText   string `json:"selected_text"`
		Skill          string `json:"skill,omitempty"` // optional: SKILL.md name
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("[AIChat] request: convID=%d msg=%q current_slug=%q selected_text=%q focusPageID=%v",
		req.ConversationID, req.Message, req.CurrentSlug, req.SelectedText, req.FocusPageID)

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

	// Save original message before modification (for DB persistence)
	originalMessage := req.Message

	// Merge selected text into user message so AI sees it directly
	if req.SelectedText != "" {
		prefix := fmt.Sprintf("关于选中的内容「%s」", req.SelectedText)
		if req.Message != "" {
			req.Message = prefix + "：\n\n" + req.Message
		} else {
			req.Message = prefix
		}
	}

	// Save original user message (before context modifications) to DB
	if originalMessage != "" {
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count, skill) VALUES (?, 'user', ?, ?, 0, ?)`,
			req.ConversationID, originalMessage, config.Provider, req.Skill)
	} else if req.SelectedText != "" && req.Message != "" {
		// When only selected text was provided (no manual input), save the contextual message
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count, skill) VALUES (?, 'user', ?, ?, 0, ?)`,
			req.ConversationID, req.Message, config.Provider, req.Skill)
	}

	// Auto-title: only on first user message with actual content
	needsTitle := false
	if req.Message != "" {
		var currentTitle sql.NullString
		h.db.QueryRowContext(ctx, `SELECT title FROM conversations WHERE id = ?`, req.ConversationID).Scan(&currentTitle)
		needsTitle = !currentTitle.Valid || currentTitle.String == ""
	}

	// --- Common path: provider, load history, context, streaming ---

	provider, err := ai.NewProvider(ai.ProviderType(config.Provider), config.ApiKey, config.ModelName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid provider: %v", err), http.StatusBadRequest)
		return
	}

	// Load history
	rows, err := h.db.QueryContext(ctx, `SELECT role, content, tool_call_id, tool_summary FROM messages WHERE conversation_id = ? ORDER BY created_at`, req.ConversationID)
	if err != nil {
		http.Error(w, "Failed to load history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var aiMessages []ai.Message
	for rows.Next() {
		var role, content string
		var toolCallID, toolSummary sql.NullString
		if err := rows.Scan(&role, &content, &toolCallID, &toolSummary); err != nil {
			continue
		}
		// Inject the tool summary as a one-line prefix in the assistant
		// content so the model can see what tools it used in prior turns.
		// This is the long-term memory substitute for the full tool_result
		// messages (which we don't persist — see comment in
		// summarizeToolCall).
		if role == "assistant" && toolSummary.Valid && toolSummary.String != "" {
			content = content + "\n\n[本轮工具调用: " + toolSummary.String + "]"
		}
		msg := ai.Message{Role: role, Content: content}
		if toolCallID.Valid {
			msg.ToolCallID = toolCallID.String
		}
		aiMessages = append(aiMessages, msg)
	}

	// Sliding window: keep only recent messages to avoid context drift and tool-calling degradation
	const maxContextMessages = 40
	if len(aiMessages) > maxContextMessages {
		kept := aiMessages[len(aiMessages)-maxContextMessages:]
		dropped := len(aiMessages) - maxContextMessages
		aiMessages = append([]ai.Message{{Role: "user", Content: fmt.Sprintf("[系统提示：早期 %d 条消息已压缩以节省上下文空间]", dropped)}}, kept...)
	}

	// Inject selected text into the last user message for AI context,
	// without persisting the merged text to DB (frontend renders clean originals)
	if req.SelectedText != "" && len(aiMessages) > 0 {
		last := &aiMessages[len(aiMessages)-1]
		if last.Role == "user" {
			prefix := fmt.Sprintf("关于选中的内容「%s」", req.SelectedText)
			if last.Content != "" {
				last.Content = prefix + "：\n\n" + last.Content
			} else {
				last.Content = prefix
			}
		}
	}

	// Inject current page context into the last user message (like selected_text)
	if req.CurrentSlug != "" && len(aiMessages) > 0 {
		last := &aiMessages[len(aiMessages)-1]
		if last.Role == "user" {
			page, err := h.queries.GetWikiPageBySlug(ctx, req.CurrentSlug)
			if err == nil {
				context := fmt.Sprintf("\n\n我当前正在查看的页面是「%s」。", page.Title)
				last.Content += context
			}
		}
	}

	// SSE setup
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, canFlush := w.(http.Flusher)

	// When current page context is active, use full tree (not subtree via FocusPageID)
	var focusID *int64 = req.FocusPageID
	if req.CurrentSlug != "" {
		focusID = nil
	}
	wikiContext := buildKnowledgeMap(ctx, &wikiContextDBAdapter{q: h.queries}, focusID)
	if req.CurrentSlug != "" {
		page, err := h.queries.GetWikiPageBySlug(ctx, req.CurrentSlug)
		if err == nil {
			wikiContext += fmt.Sprintf(
				"\n用户当前正在查看的页面：%s (slug: %s, ID: %d)\n当用户询问关于\"这个页面\"或\"当前页面\"的问题时，请使用 read_page 读取当前页面内容后再回答。不要发起空搜索。\n",
				page.Title, page.Slug, page.ID,
			)
			log.Printf("[AIChat] injected current page context: %s (slug=%s)", page.Title, page.Slug)
		} else {
			log.Printf("[AIChat] current page slug not found: slug=%s err=%v", req.CurrentSlug, err)
		}
	}
	log.Printf("[AIChat] wikiContext length=%d", len(wikiContext))
	log.Printf("[AIChat] wikiContext excerpt: %s", wikiContext[:min(len(wikiContext), 500)])
	// Look up skill if specified (for /command user-initiated loading).
	// The body is NOT injected into the system prompt — instead we synthesize
	// a load_skill tool call + result and append it to the message history so
	// the LLM sees the skill was loaded through the same channel as an
	// LLM-initiated load_skill call. (Progressive disclosure.)
	if req.Skill != "" {
		s, ok := h.SkillRegistry.Get(req.Skill)
		if !ok {
			available := h.SkillRegistry.List()
			names := make([]string, 0, len(available))
			for _, x := range available {
				names = append(names, x.Name)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":     "unknown skill: " + req.Skill,
				"available": names,
			})
			return
		}
		log.Printf("[AIChat] skill=%q body_len=%d conv=%d (synthesizing tool call)", s.Name, len(s.Body), req.ConversationID)
		aiMessages = append(aiMessages, synthesizeLoadSkillCall(s)...)
	}
	systemPrompt := ai.BuildChatSystemPrompt(convRole, wikiContext, h.SkillRegistry)

	// Build the chat request
	chatReq := ai.ChatRequest{
		Messages:     aiMessages,
		SystemPrompt: systemPrompt,
		MaxTokens:    8192,
	}
	if convRole == ai.RoleWikiMaintainer {
		chatReq.Tools = ai.WikiTools()
	}

	// Delegate the ReAct loop to RunReAct
	sink := &sseSink{w: w, flusher: flusher, canFlush: canFlush}
	result, _ := h.RunReAct(ctx, provider, chatReq, ReActOptions{
		MaxSteps:          20,
		Sink:              sink,
		RunID:             req.ConversationID,
		ConversationID:    req.ConversationID,
		FocusPageID:       req.FocusPageID,
		AutoApproveWrites: false, // HTTP path always uses the permission gate
	})

	// ====== Save assistant message ======
	assistantText := ""
	toolCallResults := []ToolCallResult{}
	if result != nil {
		assistantText = result.FinalContent
		toolCallResults = result.ToolCallResults
	}
	if assistantText != "" {
		toolCallsJSON, _ := json.Marshal(toolCallResults)
		toolSummary := summarizeToolCalls(toolCallResults)
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count, tool_calls, tool_summary) VALUES (?, 'assistant', ?, ?, 0, ?, ?)`,
			req.ConversationID, assistantText, config.Provider, string(toolCallsJSON), toolSummary)
	}

	// Auto-title after first response: use first 48 chars of user's first message
	if needsTitle {
		title := req.Message
		if len([]rune(title)) > 48 {
			title = string([]rune(title)[:48]) + "…"
		}
		h.db.ExecContext(ctx, `UPDATE conversations SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, title, req.ConversationID)
	}

	// Send done event. RunReAct does not call sink.WriteDone() itself — the
	// caller decides when to signal completion.
	sink.WriteDone()

	if convRole == ai.RoleWikiMaintainer {
		go h.updateOverviewPage()
	}
}

// --- File Upload ---

func (h *AIHandler) UploadFile(w http.ResponseWriter, r *http.Request) {

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

	if ext == ".pdf" {
		text := "[PDF file uploaded - " + header.Filename + ", " + fmt.Sprintf("%d", len(content)) + " bytes. PDF text extraction not yet supported.]"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content":  text,
			"filename": header.Filename,
			"size":     len(content),
		})
		return
	}

	text := string(content)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"content":  text,
		"filename": header.Filename,
		"size":     len(content),
	})
}

// --- Helpers ---

// wikiContextDBAdapter wraps *model.Queries to satisfy the handler.KnowledgeMapDB
// composite interface. The only missing method is GetPageContentsForFallback,
// which the renderer uses when a page summary is pending/failed/empty.
// The adapter delegates to GetFallbackContents and returns a pageID → content
// map, batched into a single query to avoid N+1.
type wikiContextDBAdapter struct {
	q *model.Queries
}

// Methods forwarded from *model.Queries (explicitly listed so a future change
// to the query file does not silently remove coverage).
func (a *wikiContextDBAdapter) CountWikiPages(ctx context.Context) (int64, error) {
	return a.q.CountWikiPages(ctx)
}

func (a *wikiContextDBAdapter) CountWikiPagesByStatus(ctx context.Context, status string) (int64, error) {
	return a.q.CountWikiPagesByStatus(ctx, status)
}

func (a *wikiContextDBAdapter) GetRecentlyUpdatedWikiPages(ctx context.Context) ([]model.GetRecentlyUpdatedWikiPagesRow, error) {
	return a.q.GetRecentlyUpdatedWikiPages(ctx)
}

func (a *wikiContextDBAdapter) GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error) {
	return a.q.GetWikiPageTree(ctx)
}

func (a *wikiContextDBAdapter) GetRecentWikiLog(ctx context.Context, arg model.GetRecentWikiLogParams) ([]model.WikiLog, error) {
	return a.q.GetRecentWikiLog(ctx, arg)
}

// GetPageContentsForFallback returns a map of pageID → content for all pages
// with non-ready summary status and non-empty content, batched into a single
// query. Used by the renderer to populate per-page summary fallbacks.
func (a *wikiContextDBAdapter) GetPageContentsForFallback(ctx context.Context) (map[int64]string, error) {
	rows, err := a.q.GetFallbackContents(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Content
	}
	return out, nil
}

func extractFirstJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s // no balanced closing brace; return as-is
}

func (h *AIHandler) executeAutoTool(ctx context.Context, tc ai.ToolCall) string {
	switch tc.Name {
	case "lookup_page":
		return h.executeLookupPage(ctx, tc)
	case "search_pages":
		return h.executeSearchPages(ctx, tc)
	case "read_page":
		return h.executeReadPage(ctx, tc)
	case "websearch":
		return h.executeWebSearch(ctx, tc)
	case "webfetch":
		return h.executeWebFetch(ctx, tc)
	default:
		return fmt.Sprintf("[系统] 未知工具: %s", tc.Name)
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
	subtreeContext := buildKnowledgeMap(ctx, &wikiContextDBAdapter{q: h.queries}, &page.ID)

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

	pages, err := h.queries.SearchWikiPages(ctx, sql.NullString{String: details.Query, Valid: true})
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

func (h *AIHandler) getTavilyAPIKey(ctx context.Context) string {
	// Check env var first (for quick dev setup)
	if key := os.Getenv("TAVILY_API_KEY"); key != "" {
		return key
	}
	// Fall back to DB config JSON column
	config, err := h.queries.GetActiveAIConfig(ctx)
	if err != nil || !config.Config.Valid || config.Config.String == "" {
		return ""
	}
	var cfg struct {
		TavilyAPIKey string `json:"tavily_api_key"`
	}
	if json.Unmarshal([]byte(config.Config.String), &cfg) != nil || cfg.TavilyAPIKey == "" {
		return ""
	}
	return cfg.TavilyAPIKey
}

func (h *AIHandler) executeWebSearch(ctx context.Context, tc ai.ToolCall) string {
	var details struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &details); err != nil || details.Query == "" {
		return "[系统] websearch 执行失败：参数无效"
	}

	apiKey := h.getTavilyAPIKey(ctx)
	if apiKey == "" {
		return "[系统] websearch 执行失败：未配置 Tavily API Key，请在设置页中配置"
	}

	if details.MaxResults <= 0 || details.MaxResults > 10 {
		details.MaxResults = 5
	}

	body := map[string]any{
		"api_key":      apiKey,
		"query":        details.Query,
		"search_depth": "basic",
		"max_results":  details.MaxResults,
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Sprintf("[系统] websearch 执行失败：%v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("[系统] websearch 搜索失败：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("[系统] websearch 搜索失败 (HTTP %d)：%s", resp.StatusCode, string(respBody))
	}

	var tavilyResp struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return fmt.Sprintf("[系统] websearch 解析结果失败：%v", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[系统] 网络搜索「%s」结果：\n\n", details.Query))
	for i, r := range tavilyResp.Results {
		b.WriteString(fmt.Sprintf("%d. **%s**\n   URL: %s\n   摘要：%s\n\n", i+1, r.Title, r.URL, r.Content))
	}
	if len(tavilyResp.Results) == 0 {
		b.WriteString("（无搜索结果）")
	}

	return b.String()
}

func (h *AIHandler) executeWebFetch(ctx context.Context, tc ai.ToolCall) string {
	var details struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &details); err != nil || details.URL == "" {
		return "[系统] webfetch 执行失败：参数无效"
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", details.URL, nil)
	if err != nil {
		return fmt.Sprintf("[系统] webfetch 执行失败：%v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LLMWiki/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("[系统] webfetch 获取页面失败：%v", err)
	}
	defer resp.Body.Close()

	// Limit to 500KB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 500*1024))
	if err != nil {
		return fmt.Sprintf("[系统] webfetch 读取页面失败：%v", err)
	}

	// Extract text from HTML
	text := extractHTMLText(string(body))

	// Limit output to 3000 chars
	runes := []rune(text)
	if len(runes) > 3000 {
		text = string(runes[:3000]) + "\n\n...（内容过长，已截断）"
	}

	return fmt.Sprintf("[系统] 成功获取页面「%s」内容（共 %d 字符）：\n\n%s", details.URL, len(runes), text)
}

func extractHTMLText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// Fallback: strip tags
		re := regexp.MustCompile("<[^>]*>")
		text := re.ReplaceAllString(htmlContent, "")
		return strings.TrimSpace(text)
	}

	var b strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
		}
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			// Skip script, style, noscript
			if tag == "script" || tag == "style" || tag == "noscript" {
				return
			}
			// Add newline for block-level elements
			if tag == "p" || tag == "br" || tag == "div" || tag == "h1" || tag == "h2" || tag == "h3" ||
				tag == "h4" || tag == "h5" || tag == "h6" || tag == "li" || tag == "tr" {
				b.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)

	// Clean up multiple newlines
	result := strings.TrimSpace(b.String())
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return result
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

// ToolCallStartEvent is sent via SSE when a tool starts executing.
type ToolCallStartEvent struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolCallResult is the persisted result of a tool execution.
type ToolCallResult struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Output string          `json:"output"`
	Error  string          `json:"error,omitempty"`
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "...(截断)"
	}
	return s
}

func sseWrite(w http.ResponseWriter, eventType, data string, canFlush bool, flusher http.Flusher) {
	fmt.Fprintf(w, "event: %s\n", eventType)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
	if canFlush {
		flusher.Flush()
	}
}

func sseWriteToolCallStart(w http.ResponseWriter, id, name, input string, canFlush bool, flusher http.Flusher) {
	in := json.RawMessage(input)
	if in == nil {
		in = json.RawMessage("{}")
	}
	data, _ := json.Marshal(ToolCallStartEvent{ID: id, Name: name, Input: in})
	sseWrite(w, "tool_call_start", string(data), canFlush, flusher)
}

func sseWriteToolResult(w http.ResponseWriter, id, name, output, errStr string, canFlush bool, flusher http.Flusher) {
	data, _ := json.Marshal(ToolCallResult{
		ID: id, Name: name, Output: output, Error: errStr,
	})
	sseWrite(w, "tool_result", string(data), canFlush, flusher)
}

func sseWritePermissionRequired(w http.ResponseWriter, req PermissionRequest, canFlush bool, flusher http.Flusher) {
	data, _ := json.Marshal(req)
	sseWrite(w, "permission_required", string(data), canFlush, flusher)
}

func sseWriteAskUserRequest(w http.ResponseWriter, req AskUserRequest, canFlush bool, flusher http.Flusher) {
	data, _ := json.Marshal(req)
	sseWrite(w, "ask_user_request", string(data), canFlush, flusher)
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// synthesizeLoadSkillCall returns a pair of messages that mimic an
// LLM-initiated load_skill tool call + result, for the case where the USER
// invoked the skill via /<name>. The assistant turn has a single tool_use
// block (load_skill), followed by a tool result containing the body. From
// the LLM's perspective, it looks identical to a self-initiated load.
//
// We append these at the end of the conversation so the most-recent context
// is the loaded skill body, which the LLM will use for the response to the
// user's current message.
func synthesizeLoadSkillCall(s *skills.Skill) []ai.Message {
	callID := fmt.Sprintf("skill_user_%d", time.Now().UnixNano())
	blocks := []ai.ContentBlock{{
		Type:  ai.ContentTypeToolUse,
		ID:    callID,
		Name:  "load_skill",
		Input: json.RawMessage(fmt.Sprintf(`{"name":%q}`, s.Name)),
	}}
	assistantJSON, _ := ai.ContentBlocksToJSON(blocks)
	return []ai.Message{
		{Role: "assistant", Content: assistantJSON},
		{Role: "tool", ToolCallID: callID, ToolName: "load_skill", Content: s.Body},
	}
}
