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
	"learn-helper/internal/engine"
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

func (h *AIHandler) AIChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID int64  `json:"conversation_id"`
		Message        string `json:"message"`
		PlanID         string `json:"plan_id"`
		FocusPageID    *int64 `json:"focus_page_id"`
		CurrentSlug    string `json:"current_slug"`
		SelectedText   string `json:"selected_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("[AIChat] request: convID=%d msg=%q current_slug=%q selected_text=%q focusPageID=%v planID=%q",
		req.ConversationID, req.Message, req.CurrentSlug, req.SelectedText, req.FocusPageID, req.PlanID)

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

	// Merge selected text into user message so AI sees it directly
	if req.SelectedText != "" {
		prefix := fmt.Sprintf("关于选中的内容「%s」", req.SelectedText)
		if req.Message != "" {
			req.Message = prefix + "：\n\n" + req.Message
		} else {
			req.Message = prefix
		}
	}

	// Add current page context to user message
	if req.CurrentSlug != "" {
		page, err := h.queries.GetWikiPageBySlug(ctx, req.CurrentSlug)
		if err == nil {
			req.Message += fmt.Sprintf("\n\n[当前页面：%s (ID=%d)]", page.Title, page.ID)
		} else {
			log.Printf("[AIChat] current page slug not found: slug=%s err=%v", req.CurrentSlug, err)
		}
	}

	// Save user message (only when there's actual text)
	if req.Message != "" {
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
			req.ConversationID, req.Message, config.Provider)
	}

	// Auto-title: only on first user message with actual content
	var needsTitle bool
	if req.Message != "" {
		var currentTitle sql.NullString
		h.db.QueryRowContext(ctx, `SELECT title FROM conversations WHERE id = ?`, req.ConversationID).Scan(&currentTitle)
		needsTitle = !currentTitle.Valid || currentTitle.String == ""
	}

	// Handle plan confirmation
	if req.PlanID != "" {
		// Verify plan is pending before executing
		var planStatus string
		var outline sql.NullString
		var actionCount int64
		err := h.db.QueryRowContext(ctx,
			"SELECT p.status, p.outline, (SELECT COUNT(*) FROM plan_actions WHERE plan_id = ?) FROM plans p WHERE p.id = ?",
			req.PlanID, req.PlanID).Scan(&planStatus, &outline, &actionCount)
		if err != nil || planStatus != "pending" {
			http.Error(w, "plan not found or not pending", http.StatusBadRequest)
			return
		}
		// Mark as confirmed
		_, err = h.db.ExecContext(ctx, "UPDATE plans SET status = 'confirmed' WHERE id = ?", req.PlanID)
		if err != nil {
			http.Error(w, fmt.Sprintf("update plan status: %v", err), http.StatusInternalServerError)
			return
		}
		eng := engine.NewExecutionEngine(h.db, h.queries)
		var confirmContent string
		if outline.Valid && outline.String != "" && actionCount == 0 {
			result, err := eng.ExecOutline(ctx, outline.String, nil)
			if err != nil {
				http.Error(w, fmt.Sprintf("outline execution failed: %v", err), http.StatusInternalServerError)
				return
			}
			resultJSON, _ := json.Marshal(result)
			confirmContent = fmt.Sprintf("大纲已确认，知识骨架已创建：\n%s", string(resultJSON))
		} else {
			report, err := eng.ExecutePlan(ctx, req.PlanID)
			if err != nil {
				http.Error(w, fmt.Sprintf("plan execution failed: %v", err), http.StatusInternalServerError)
				return
			}
			reportJSON, _ := json.Marshal(report)
			confirmContent = fmt.Sprintf("操作计划已执行完成：\n%s", string(reportJSON))
		}
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
			req.ConversationID, confirmContent, config.Provider)
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

	// When current page context is active, use full tree (not subtree via FocusPageID)
	var focusID *int64 = req.FocusPageID
	if req.CurrentSlug != "" {
		focusID = nil
	}
	wikiContext := h.buildWikiContext(ctx, focusID)
	if req.CurrentSlug != "" {
		page, err := h.queries.GetWikiPageBySlug(ctx, req.CurrentSlug)
		if err == nil {
			wikiContext += "\n当用户询问关于\"这个页面\"或\"当前页面\"的问题时，请使用 read_page 读取当前页面内容后再回答。不要发起空搜索。\n"
			log.Printf("[AIChat] injected current page context: %s (slug=%s)", page.Title, page.Slug)
		} else {
			log.Printf("[AIChat] current page slug not found: slug=%s err=%v", req.CurrentSlug, err)
		}
	}
	log.Printf("[AIChat] wikiContext length=%d", len(wikiContext))
	log.Printf("[AIChat] wikiContext excerpt: %s", wikiContext[:min(len(wikiContext), 500)])
	systemPrompt := ai.BuildSystemPrompt(convRole, wikiContext)

	// ====== ReAct Loop ======
	// Uses token-by-token streaming Chat(), auto-executes read-only tools,
	// injects results, then lets the AI see them and reason further,
	// until it stops calling tools, requests user confirmation, or hits max iterations.

	fullContent := &strings.Builder{}
	const maxIterations = 10

reactLoop:
	for iteration := 0; iteration < maxIterations; iteration++ {
		log.Printf("[ReAct] iteration=%d messages=%d", iteration, len(aiMessages))

		chatReq := ai.ChatRequest{
			Messages:     aiMessages,
			SystemPrompt: systemPrompt,
			MaxTokens:    8192,
		}
		if convRole == ai.RoleWikiMaintainer {
			chatReq.Tools = ai.WikiTools()
		}

		streamCh, err := provider.StreamChat(ctx, chatReq)
		if err != nil {
			log.Printf("[ReAct] StreamChat error: %v", err)
			sseWrite(w, "error", fmt.Sprintf("AI stream error: %v", err), canFlush, flusher)
			return
		}

		var textBuilder strings.Builder
		var respToolCalls []ai.ToolCall

	streamLoop:
		for chunk := range streamCh {
			if chunk.Content != "" {
				sseWrite(w, "content", chunk.Content, canFlush, flusher)
				textBuilder.WriteString(chunk.Content)
			}
			if chunk.ToolCall != nil {
				respToolCalls = append(respToolCalls, *chunk.ToolCall)
			}
			if chunk.Done {
				break streamLoop
			}
		}

		respContent := textBuilder.String()
		log.Printf("[ReAct] iteration=%d content_len=%d tool_calls=%d", iteration, len(respContent), len(respToolCalls))
		for i, tc := range respToolCalls {
			log.Printf("[ReAct]   tool_call[%d]: name=%s input_len=%d", i, tc.Name, len(tc.Input))
		}

		// Accumulate streamed content
		if respContent != "" {
			fullContent.WriteString(respContent)
			fullContent.WriteString("\n\n")
		}

		// No tool calls → AI is done reasoning
		toolCalls := respToolCalls
		if len(toolCalls) == 0 {
			log.Printf("[ReAct] iteration=%d no tool calls, done", iteration)
			break reactLoop
		}

		// Separate auto-executed tools from propose_plan
		var autoCalls []ai.ToolCall
		var planCall *ai.ToolCall
		for _, tc := range toolCalls {
			if tc.Name == "propose_plan" {
				planCall = &tc
			} else {
				autoCalls = append(autoCalls, tc)
			}
		}
		log.Printf("[ReAct] iteration=%d auto_calls=%d plan_call=%v", iteration, len(autoCalls), planCall != nil)

		// Build structured content for this assistant turn
		var blocks []ai.ContentBlock
		if respContent != "" {
			blocks = append(blocks, ai.ContentBlock{Type: ai.ContentTypeText, Text: respContent})
		}

		// If propose_plan is present, this is the terminal iteration
		if planCall != nil {
			// Add all tool_use blocks (auto + plan)
			for _, tc := range autoCalls {
				var input json.RawMessage
				if tc.Input != "" {
					input = json.RawMessage(tc.Input)
				}
				blocks = append(blocks, ai.ContentBlock{
					Type: ai.ContentTypeToolUse, ID: tc.ID, Name: tc.Name, Input: input,
				})
			}
			var planInput json.RawMessage
			if planCall.Input != "" {
				planInput = json.RawMessage(planCall.Input)
			}
			blocks = append(blocks, ai.ContentBlock{
				Type: ai.ContentTypeToolUse, ID: planCall.ID, Name: planCall.Name, Input: planInput,
			})

			if assistantContent, err := ai.ContentBlocksToJSON(blocks); err == nil {
				aiMessages = append(aiMessages, ai.Message{Role: "assistant", Content: assistantContent})
			}

			// Execute auto tools, stream results in real-time
			for _, tc := range autoCalls {
				log.Printf("[ReAct] executing auto tool: %s", tc.Name)
				result := h.executeAutoTool(ctx, tc)
				log.Printf("[ReAct] auto tool %s result_len=%d", tc.Name, len(result))
				aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: tc.ID})
			}

			// Create Plan from propose_plan call
			log.Printf("[ReAct] creating plan from propose_plan, input_len=%d", len(planCall.Input))
			plan, err := h.createPlanFromToolCall(req.ConversationID, planCall.Input)
			if err != nil {
				log.Printf("[ReAct] createPlanFromToolCall FAILED: %v", err)
				sseWrite(w, "error", fmt.Sprintf("create plan failed: %v", err), canFlush, flusher)
				break reactLoop
			}
			log.Printf("[ReAct] plan created: id=%s actions=%d", plan.ID, len(plan.Actions))

			// Save assistant message with plan info
			planSummary := ""
			if plan.Outline != nil && *plan.Outline != "" && len(plan.Actions) == 0 {
				planSummary = fmt.Sprintf("[操作计划 - 大纲] %s\n大纲已生成，请在右侧查看。", plan.Reasoning)
			} else {
				planSummary = fmt.Sprintf("[操作计划] %s\n共 %d 个操作待确认。", plan.Reasoning, len(plan.Actions))
			}
			_, _ = h.db.ExecContext(ctx,
				"INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)",
				req.ConversationID, planSummary, config.Provider)

			// Send plan to frontend via SSE meta event
			metaData := map[string]any{
				"plan": plan,
			}
			metaJSON, _ := json.Marshal(metaData)
			sseWrite(w, "meta", string(metaJSON), canFlush, flusher)
			break reactLoop // Exit loop — wait for user confirmation
		}

		// Only auto tools: add tool_use blocks, execute, stream results, and loop
		for _, tc := range autoCalls {
			var input json.RawMessage
			if tc.Input != "" {
				input = json.RawMessage(tc.Input)
			}
			blocks = append(blocks, ai.ContentBlock{
				Type: ai.ContentTypeToolUse, ID: tc.ID, Name: tc.Name, Input: input,
			})
		}

		if assistantContent, err := ai.ContentBlocksToJSON(blocks); err == nil {
			aiMessages = append(aiMessages, ai.Message{Role: "assistant", Content: assistantContent})
		}

		for _, tc := range autoCalls {
			log.Printf("[ReAct] executing auto tool: %s", tc.Name)
			result := h.executeAutoTool(ctx, tc)
			log.Printf("[ReAct] auto tool %s result_len=%d", tc.Name, len(result))
			aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: tc.ID})
		}

		log.Printf("[ReAct] iteration=%d auto tools done, looping for AI to reason further", iteration)

		// Loop continues → AI sees tool results and can reason further
		if iteration == maxIterations-1 {
			msg := "抱歉，我还没有得出结论，请重新描述您的问题。"
			sseWrite(w, "content", msg, canFlush, flusher)
			fullContent.WriteString(msg)
		}
	}

	// ====== Save assistant message ======
	assistantText := strings.TrimSpace(fullContent.String())
	if assistantText != "" {
		h.db.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
			req.ConversationID, assistantText, config.Provider)
	}

	// Auto-title after first response: use first 48 chars of user's first message
	if needsTitle {
		title := req.Message
		if len([]rune(title)) > 48 {
			title = string([]rune(title)[:48]) + "…"
		}
		h.db.ExecContext(ctx, `UPDATE conversations SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, title, req.ConversationID)
	}

	// ====== Send done event ======
	sseWrite(w, "done", `{"token_count":0}`, canFlush, flusher)

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

func (h *AIHandler) buildWikiContext(ctx context.Context, focusPageID *int64) string {
	var b strings.Builder

	// --- Knowledge base overview ---
	totalCount, _ := h.queries.CountWikiPages(ctx)
	publishedCount, _ := h.queries.CountWikiPagesByStatus(ctx, "published")
	draftCount, _ := h.queries.CountWikiPagesByStatus(ctx, "draft")
	filledCount := publishedCount + draftCount
	emptyCount := totalCount - filledCount

	b.WriteString("【知识库概览】\n")
	b.WriteString(fmt.Sprintf("总页面数: %d\n", totalCount))
	if totalCount > 0 {
		pct := float64(filledCount) / float64(totalCount) * 100
		b.WriteString(fmt.Sprintf("已填充页面: %d (%.0f%%)\n", filledCount, pct))
		b.WriteString(fmt.Sprintf("空页面: %d (%.0f%%)\n", emptyCount, 100-pct))
	} else {
		b.WriteString("已填充页面: 0\n")
		b.WriteString("空页面: 0\n")
	}

	// Most recently updated page
	recentPages, _ := h.queries.GetRecentlyUpdatedWikiPages(ctx)
	if len(recentPages) > 0 {
		b.WriteString(fmt.Sprintf("最近更新: %s\n", recentPages[0].Title))
	}
	b.WriteString("\n")

	// --- Tree rendering (existing behavior) ---
	if focusPageID != nil {
		// Subtree mode: only load the target page and its descendants
		parentPage, err := h.queries.GetWikiPageByID(ctx, *focusPageID)
		if err != nil {
			return fmt.Sprintf("（未找到目标页面 ID=%d）", *focusPageID)
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
		b.WriteString(h.renderTreeContext(treeRows, parentPage.Title))
	} else {
		// Existing full-tree behavior
		pages, err := h.queries.GetWikiPageTree(ctx)
		if err != nil || len(pages) == 0 {
			return "（知识库为空）"
		}
		b.WriteString(h.renderTreeContext(pages, ""))
	}

	// --- Focus page context ---
	if focusPageID != nil {
		page, err := h.queries.GetWikiPageByID(ctx, *focusPageID)
		if err == nil {
			b.WriteString("\n【焦点页面详情】\n")
			b.WriteString(fmt.Sprintf("标题: %s (ID=%d)\n", page.Title, page.ID))

			// Resolve links (outgoing)
			var linkIDs []int64
			if page.Links != "" && page.Links != "[]" {
				if err := json.Unmarshal([]byte(page.Links), &linkIDs); err == nil && len(linkIDs) > 0 {
					b.WriteString("链接到:\n")
					for _, lid := range linkIDs {
						linkedPage, err := h.queries.GetWikiPageByID(ctx, lid)
						if err == nil {
							b.WriteString(fmt.Sprintf("  - [ID=%d] %s\n", linkedPage.ID, linkedPage.Title))
						} else {
							b.WriteString(fmt.Sprintf("  - [ID=%d] (未找到)\n", lid))
						}
					}
				}
			}

			// Resolve backlinks (incoming)
			var backlinkIDs []int64
			if page.Backlinks != "" && page.Backlinks != "[]" {
				if err := json.Unmarshal([]byte(page.Backlinks), &backlinkIDs); err == nil && len(backlinkIDs) > 0 {
					b.WriteString("被链接自:\n")
					for _, bid := range backlinkIDs {
						backPage, err := h.queries.GetWikiPageByID(ctx, bid)
						if err == nil {
							b.WriteString(fmt.Sprintf("  - [ID=%d] %s\n", backPage.ID, backPage.Title))
						} else {
							b.WriteString(fmt.Sprintf("  - [ID=%d] (未找到)\n", bid))
						}
					}
				}
			}

			// Child pages with content status
			children, err := h.queries.GetWikiPageChildren(ctx, sql.NullInt64{Int64: *focusPageID, Valid: true})
			if err == nil && len(children) > 0 {
				b.WriteString("子页面:\n")
				for _, c := range children {
					status := "空"
					if c.ContentStatus == "published" || c.ContentStatus == "draft" {
						status = "有内容"
					}
					b.WriteString(fmt.Sprintf("  - [ID=%d] %s (%s)\n", c.ID, c.Title, status))
				}
			}
		}
	}

	// --- Knowledge gaps ---
	emptyPages, _ := h.queries.GetEmptyWikiPages(ctx)
	if len(emptyPages) > 0 {
		b.WriteString("\n【知识缺口】\n")
		limit := 5
		if len(emptyPages) < limit {
			limit = len(emptyPages)
		}
		b.WriteString(fmt.Sprintf("空页面 (前 %d 个):\n", limit))
		for i := 0; i < limit; i++ {
			b.WriteString(fmt.Sprintf("  - [ID=%d] %s\n", emptyPages[i].ID, emptyPages[i].Title))
		}
		if len(emptyPages) > 5 {
			b.WriteString(fmt.Sprintf("  ... 还有 %d 个空页面\n", len(emptyPages)-5))
		}
	}

	// Count pages with no links
	var noLinksCount int64
	rows, err := h.db.QueryContext(ctx, `SELECT COUNT(*) FROM wiki_pages WHERE COALESCE(links, '[]') = '[]'`)
	if err == nil {
		if rows.Next() {
			rows.Scan(&noLinksCount)
		}
		rows.Close()
	}
	if noLinksCount > 0 {
		b.WriteString(fmt.Sprintf("无链接页面数: %d\n", noLinksCount))
	}

	return b.String()
}

// createPlanFromToolCall creates a Plan from a propose_plan tool call input.
// Supports both old format (single "params" field) and new format (type-specific "*_params" fields).
func (h *AIHandler) createPlanFromToolCall(conversationID int64, input string) (*model.Plan, error) {
	var proposal struct {
		Reasoning   string          `json:"reasoning"`
		Outline     json.RawMessage `json:"outline"`
		Phases      json.RawMessage `json:"phases"`
		PhaseIndex  *int64          `json:"phase_index"`
		TotalPhases *int64          `json:"total_phases"`
		Actions     []struct {
			ID               string         `json:"id"`
			Type             string         `json:"type"`
			Params           map[string]any `json:"params"`
			CreatePageParams map[string]any `json:"create_page_params"`
			UpdatePageParams map[string]any `json:"update_page_params"`
			DeletePageParams map[string]any `json:"delete_page_params"`
			LinkPagesParams  map[string]any `json:"link_pages_params"`
			MovePageParams   map[string]any `json:"move_page_params"`
			DependsOn        []string       `json:"depends_on"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(input), &proposal); err != nil {
		return nil, fmt.Errorf("parse propose_plan input: %w", err)
	}

	planID := fmt.Sprintf("plan-%d", time.Now().UnixNano())
	now := time.Now().Format("2006-01-02 15:04:05")

	// Serialize outline to string if present
	var outlineStr *string
	if len(proposal.Outline) > 0 && string(proposal.Outline) != "null" {
		s := string(proposal.Outline)
		outlineStr = &s
	}

	plan := &model.Plan{
		ID:             planID,
		ConversationID: &conversationID,
		Reasoning:      proposal.Reasoning,
		Status:         "pending",
		Outline:        outlineStr,
		PhaseIndex:     proposal.PhaseIndex,
		TotalPhases:    proposal.TotalPhases,
		CreatedAt:      now,
	}

	for i, a := range proposal.Actions {
		// Resolve params: prefer type-specific params, fall back to generic "params"
		params := a.Params
		switch a.Type {
		case "create_page":
			if len(a.CreatePageParams) > 0 {
				params = a.CreatePageParams
			}
		case "update_page":
			if len(a.UpdatePageParams) > 0 {
				params = a.UpdatePageParams
			}
		case "delete_page":
			if len(a.DeletePageParams) > 0 {
				params = a.DeletePageParams
			}
		case "link_pages":
			if len(a.LinkPagesParams) > 0 {
				params = a.LinkPagesParams
			}
		case "move_page":
			if len(a.MovePageParams) > 0 {
				params = a.MovePageParams
			}
		}
		if params == nil {
			params = map[string]any{}
		}
		paramsJSON, _ := json.Marshal(params)

		// Replace {{action:a1.page_id}} → {{action:planID-a1.page_id}} in params
		// so the engine can resolve placeholders against the prefixed action IDs
		paramsStr := string(paramsJSON)
		for _, a2 := range proposal.Actions {
			if a2.ID != "" {
				paramsStr = strings.ReplaceAll(paramsStr,
					fmt.Sprintf("{{action:%s.", a2.ID),
					fmt.Sprintf("{{action:%s-%s.", planID, a2.ID))
			}
		}

		// Make depends_on IDs globally unique by prefixing with planID
		var dependsOn []string
		for _, dep := range a.DependsOn {
			dependsOn = append(dependsOn, planID+"-"+dep)
		}
		// json.Marshal(nil) returns "null", column expects valid JSON array
		dependsOnJSON := []byte("[]")
		if len(dependsOn) > 0 {
			dependsOnJSON, _ = json.Marshal(dependsOn)
		}

		// Use planID prefix to make action ID globally unique (avoids UNIQUE constraint collision)
		actionID := planID + "-" + a.ID
		plan.Actions = append(plan.Actions, model.PlanAction{
			ID:        actionID,
			PlanID:    planID,
			Type:      a.Type,
			Params:    json.RawMessage(paramsStr),
			DependsOn: json.RawMessage(dependsOnJSON),
			Status:    "pending",
			SortOrder: int64(i),
			CreatedAt: now,
		})
	}

	// Save to database using PlanHandler
	planHandler := NewPlanHandler(h.db, h.queries, nil)
	if err := planHandler.SavePlan(context.Background(), plan); err != nil {
		return nil, fmt.Errorf("save plan: %w", err)
	}

	return plan, nil
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

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
