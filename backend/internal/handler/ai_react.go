package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"learn-helper/internal/ai"
)

// ReActEventSink receives events produced by the ReAct loop. The HTTP chat
// path uses an SSE-backed sink; the cron path uses a no-op or logging sink.
// Keeping the interface narrow (no http.ResponseWriter exposure) ensures cron
// callers cannot accidentally surface a permission prompt to a non-existent user.
type ReActEventSink interface {
	WriteContent(text string)
	WriteToolCallStart(id, name, input string)
	WriteToolResult(id, name, output, errStr string)
	WritePermissionRequired(PermissionRequest)
	WriteAskUserRequest(AskUserRequest)
	WriteDone()
	WriteError(msg string)
}

// ReActOptions controls one ReAct loop execution.
type ReActOptions struct {
	// AutoApproveWrites, when true, executes write tools directly without
	// waiting on the permission channel. Used by the cron runner.
	AutoApproveWrites bool

	// MaxSteps caps the number of model calls. 0 means use the default (20).
	MaxSteps int

	// Timeout is the per-loop deadline. 0 means inherit from ctx.
	Timeout time.Duration

	// Sink receives all events. Required.
	Sink ReActEventSink

	// RunID is a free-form identifier for log correlation (e.g. cron_run id).
	RunID int64

	// ConversationID, if non-zero, is used for logging only.
	ConversationID int64

	// FocusPageID is the page to scope wiki writes to (for create_page).
	// Only consulted by write tools that take a parent_id.
	FocusPageID *int64
}

// ReActResult is returned by RunReAct.
type ReActResult struct {
	FinalContent    string
	Steps           int
	WriteCount      int
	ToolCount       int
	ToolCallResults []ToolCallResult
}

// sseSink writes ReAct events as Server-Sent Events to the HTTP client.
type sseSink struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	canFlush bool
}

func (s *sseSink) WriteContent(text string) {
	sseWrite(s.w, "content", text, s.canFlush, s.flusher)
}

func (s *sseSink) WriteToolCallStart(id, name, input string) {
	sseWriteToolCallStart(s.w, id, name, input, s.canFlush, s.flusher)
}

func (s *sseSink) WriteToolResult(id, name, output, errStr string) {
	sseWriteToolResult(s.w, id, name, output, errStr, s.canFlush, s.flusher)
}

func (s *sseSink) WritePermissionRequired(req PermissionRequest) {
	sseWritePermissionRequired(s.w, req, s.canFlush, s.flusher)
}

func (s *sseSink) WriteAskUserRequest(req AskUserRequest) {
	sseWriteAskUserRequest(s.w, req, s.canFlush, s.flusher)
}

func (s *sseSink) WriteDone() {
	sseWrite(s.w, "done", `{"token_count":0}`, s.canFlush, s.flusher)
}

func (s *sseSink) WriteError(msg string) {
	sseWrite(s.w, "error", msg, s.canFlush, s.flusher)
}

// RunReAct executes the ReAct loop. Used by both the SSE chat handler
// (via AIChat) and the cron scheduler (via internal/cron.Runner).
//
// Caller is responsible for:
//   - Creating the AIProvider
//   - Persisting conversation messages
//   - Calling Sink.WriteDone() at the end of the surrounding flow (or the
//     sink can choose to do it itself; sseSink does, noop/log sinks can)
//
// RunReAct itself does NOT call sink.WriteDone() — that is the caller's
// responsibility, since cron paths may want to record the outcome first
// and only then signal completion.
func (h *AIHandler) RunReAct(ctx context.Context, provider ai.AIProvider, chatReq ai.ChatRequest, opts ReActOptions) (*ReActResult, error) {
	if opts.Sink == nil {
		return nil, fmt.Errorf("RunReAct: Sink is required")
	}
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 20
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	sink := opts.Sink
	logPrefix := fmt.Sprintf("[ReAct run=%d", opts.RunID)
	if opts.ConversationID > 0 {
		logPrefix += fmt.Sprintf(" conv=%d", opts.ConversationID)
	}
	logPrefix += "]"

	// Work on a copy of the message slice so the caller's chatReq is untouched.
	aiMessages := make([]ai.Message, len(chatReq.Messages))
	copy(aiMessages, chatReq.Messages)

	fullContent := &strings.Builder{}
	var toolCallResults []ToolCallResult
	steps := 0
	toolCount := 0
	writeCount := 0

reactLoop:
	for iteration := 0; iteration < opts.MaxSteps; iteration++ {
		steps++
		log.Printf("%s iteration=%d messages=%d", logPrefix, iteration, len(aiMessages))

		chatReq.Messages = aiMessages
		streamCh, err := provider.StreamChat(ctx, chatReq)
		if err != nil {
			log.Printf("%s StreamChat error: %v", logPrefix, err)
			sink.WriteError(fmt.Sprintf("AI stream error: %v", err))
			return nil, err
		}

		var textBuilder strings.Builder
		var respToolCalls []ai.ToolCall
		var respReasoning string // DeepSeek thinking mode — must be passed back next turn

	streamLoop:
		for chunk := range streamCh {
			if chunk.Content != "" {
				sink.WriteContent(chunk.Content)
				textBuilder.WriteString(chunk.Content)
			}
			if chunk.ReasoningContent != "" {
				respReasoning += chunk.ReasoningContent
			}
			if chunk.ToolCall != nil {
				respToolCalls = append(respToolCalls, *chunk.ToolCall)
			}
			if chunk.Done {
				break streamLoop
			}
		}

		respContent := textBuilder.String()
		log.Printf("%s iteration=%d content_len=%d tool_calls=%d", logPrefix, iteration, len(respContent), len(respToolCalls))
		for i, tc := range respToolCalls {
			log.Printf("%s   tool_call[%d]: name=%s input_len=%d", logPrefix, i, tc.Name, len(tc.Input))
		}

		// No tool calls → AI is done reasoning
		if len(respToolCalls) == 0 {
			if respContent != "" {
				fullContent.WriteString(respContent)
				fullContent.WriteString("\n\n")
			}
			log.Printf("%s iteration=%d no tool calls, done", logPrefix, iteration)
			break reactLoop
		}

		toolCount += len(respToolCalls)

		// Classify tool calls
		var calls []aiToolCall
		for _, tc := range respToolCalls {
			calls = append(calls, aiToolCall{Name: tc.Name, ID: tc.ID, Input: tc.Input})
		}
		readBatch, writeBatch, askBatch, loadSkillBatch := classifyToolCalls(calls)

		log.Printf("%s iteration=%d reads=%d writes=%d asks=%d load_skills=%d",
			logPrefix, iteration, len(readBatch), len(writeBatch), len(askBatch), len(loadSkillBatch))

		// Build assistant turn blocks (text + tool_use for every tool call,
		// plus reasoning_content for DeepSeek thinking mode — required on
		// the next request or the API rejects with invalid_request_error).
		var blocks []ai.ContentBlock
		if respContent != "" {
			blocks = append(blocks, ai.ContentBlock{Type: ai.ContentTypeText, Text: respContent})
		}
		for _, tc := range respToolCalls {
			var input json.RawMessage
			if tc.Input != "" {
				input = json.RawMessage(tc.Input)
			}
			blocks = append(blocks, ai.ContentBlock{Type: ai.ContentTypeToolUse, ID: tc.ID, Name: tc.Name, Input: input})
		}
		if assistantContent, err := ai.ContentBlocksToJSON(blocks); err == nil {
			msg := ai.Message{Role: "assistant", Content: assistantContent}
			if respReasoning != "" {
				msg.ReasoningContent = respReasoning
			}
			aiMessages = append(aiMessages, msg)
		}

		// Execute read tools (auto)
		for _, c := range readBatch {
			log.Printf("%s read tool: %s", logPrefix, c.Name)
			sink.WriteToolCallStart(c.ID, c.Name, c.Input)
			result := h.executeReadTool(ctx, c)
			sink.WriteToolResult(c.ID, c.Name, result, "")
			aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: c.ID})
			toolCallResults = append(toolCallResults, ToolCallResult{
				ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input), Output: result,
			})
		}

		// Execute load_skill (auto; LLM-initiated progressive disclosure)
		for _, c := range loadSkillBatch {
			log.Printf("%s load_skill tool: %s", logPrefix, c.Name)
			sink.WriteToolCallStart(c.ID, c.Name, c.Input)
			result := h.executeLoadSkillTool(ctx, c)
			sink.WriteToolResult(c.ID, c.Name, result, "")
			aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: c.ID})
			toolCallResults = append(toolCallResults, ToolCallResult{
				ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input), Output: result,
			})
		}

		// Execute write tools (permission gate OR auto-approve)
		if len(writeBatch) > 0 {
			if opts.AutoApproveWrites {
				// Cron path: execute each write tool directly, no channel.
				for _, c := range writeBatch {
					log.Printf("%s auto-approve write tool: %s", logPrefix, c.Name)
					sink.WriteToolCallStart(c.ID, c.Name, c.Input)
					result, execErr := h.executeWriteTool(ctx, c.Name, c.Input, opts.FocusPageID)
					if execErr != nil {
						sink.WriteToolResult(c.ID, c.Name, "", execErr.Error())
						aiMessages = append(aiMessages, ai.Message{
							Role: "tool", Content: fmt.Sprintf("error: %s", execErr.Error()), ToolCallID: c.ID,
						})
						toolCallResults = append(toolCallResults, ToolCallResult{
							ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input),
							Error: execErr.Error(),
						})
					} else {
						sink.WriteToolResult(c.ID, c.Name, result, "")
						aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: c.ID})
						toolCallResults = append(toolCallResults, ToolCallResult{
							ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input), Output: result,
						})
						writeCount++
					}
				}
			} else {
				// HTTP path: original permission gate behavior.
				requestID := fmt.Sprintf("perm-%d", time.Now().UnixNano())
				ch := h.permissions.Register(requestID, 1)

				items := make([]PermissionRequestItem, 0, len(writeBatch))
				for _, c := range writeBatch {
					sink.WriteToolCallStart(c.ID, c.Name, c.Input)
					parsed, _ := parseWriteInput(c.Name, json.RawMessage(c.Input))
					items = append(items, PermissionRequestItem{
						ID:      c.ID,
						Tool:    c.Name,
						Input:   parsed,
						Preview: previewWrite(c.Name, parsed),
					})
				}
				sink.WritePermissionRequired(PermissionRequest{
					RequestID:      requestID,
					ConversationID: opts.ConversationID,
					Items:          items,
				})

				log.Printf("%s waiting for permission decisions: %s (n=%d)", logPrefix, requestID, len(writeBatch))
				var decisions []PermissionDecision
				select {
				case decisions = <-ch:
				case <-ctx.Done():
					log.Printf("%s context cancelled while waiting for permission: %v", logPrefix, ctx.Err())
					return nil, ctx.Err()
				}

				// Index decisions by ID
				decByID := map[string]PermissionDecision{}
				for _, d := range decisions {
					decByID[d.ID] = d
				}

				for _, c := range writeBatch {
					dec, ok := decByID[c.ID]
					if !ok {
						dec = PermissionDecision{ID: c.ID, Action: "reject"}
					}

					switch dec.Action {
					case "approve", "edit":
						input := c.Input
						if dec.Action == "edit" && dec.EditedInput != nil {
							b, _ := json.Marshal(dec.EditedInput)
							input = string(b)
						}
						result, execErr := h.executeWriteTool(ctx, c.Name, input, opts.FocusPageID)
						if execErr != nil {
							sink.WriteToolResult(c.ID, c.Name, "", execErr.Error())
							aiMessages = append(aiMessages, ai.Message{
								Role: "tool", Content: fmt.Sprintf("error: %s", execErr.Error()), ToolCallID: c.ID,
							})
							toolCallResults = append(toolCallResults, ToolCallResult{
								ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input),
								Error: execErr.Error(),
							})
						} else {
							sink.WriteToolResult(c.ID, c.Name, result, "")
							aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: c.ID})
							toolCallResults = append(toolCallResults, ToolCallResult{
								ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input), Output: result,
							})
							writeCount++
						}
					default: // reject or unknown
						sink.WriteToolResult(c.ID, c.Name, "", "rejected by user")
						aiMessages = append(aiMessages, ai.Message{
							Role: "tool", Content: `{"error":"rejected by user"}`, ToolCallID: c.ID,
						})
						toolCallResults = append(toolCallResults, ToolCallResult{
							ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input),
							Error: "rejected by user",
						})
					}
				}
			}
		}

		// Execute ask_user (one at a time, in order) — always requires user response,
		// so this is a no-op for cron (ask_user is filtered out of the cron tool list).
		if len(askBatch) > 0 && !opts.AutoApproveWrites {
			for _, c := range askBatch {
				sink.WriteToolCallStart(c.ID, c.Name, c.Input)
				requestID := fmt.Sprintf("ask-%d", time.Now().UnixNano())
				ch := h.askUsers.Register(requestID)

				var parsed struct {
					Question      string          `json:"question"`
					Options       []string        `json:"options"`
					Context       *AskUserContext `json:"context,omitempty"`
					MultiSelect   bool            `json:"multi_select"`
					AllowFreeText bool            `json:"allow_free_text"`
					Header        string          `json:"header,omitempty"`
				}
				_ = json.Unmarshal([]byte(c.Input), &parsed)

				sink.WriteAskUserRequest(AskUserRequest{
					RequestID:      requestID,
					ConversationID: opts.ConversationID,
					Question:       parsed.Question,
					Options:        parsed.Options,
					Context:        parsed.Context,
					MultiSelect:    parsed.MultiSelect,
					AllowFreeText:  parsed.AllowFreeText,
					Header:         parsed.Header,
				})

				log.Printf("%s waiting for ask_user answer: %s", logPrefix, requestID)
				var resp AskUserResponse
				select {
				case resp = <-ch:
				case <-ctx.Done():
					log.Printf("%s context cancelled while waiting for ask_user: %v", logPrefix, ctx.Err())
					return nil, ctx.Err()
				}
				answerJSON, _ := json.Marshal(map[string]any{"answer": resp.Answer})
				sink.WriteToolResult(c.ID, c.Name, string(answerJSON), "")
				aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: string(answerJSON), ToolCallID: c.ID})
				toolCallResults = append(toolCallResults, ToolCallResult{
					ID: c.ID, Name: c.Name, Input: json.RawMessage(c.Input),
					Output: string(answerJSON),
				})
			}
		} else if len(askBatch) > 0 {
			// Cron path with ask_user somehow in the tool list (shouldn't happen
			// since we filter WikiToolsForCron, but be defensive): emit a
			// synthetic "no_user" error so the AI can adapt.
			for _, c := range askBatch {
				log.Printf("%s ask_user blocked (autonomous mode): %s", logPrefix, c.Name)
				sink.WriteToolCallStart(c.ID, c.Name, c.Input)
				errMsg := `{"error":"ask_user is not available in autonomous mode; proceed with reasonable defaults or skip the question"}`
				sink.WriteToolResult(c.ID, c.Name, errMsg, "")
				aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: errMsg, ToolCallID: c.ID})
			}
		}

		// Accumulate text content (post-tool reasoning, if any)
		if respContent != "" && len(writeBatch) == 0 && len(askBatch) == 0 {
			fullContent.WriteString(respContent)
			fullContent.WriteString("\n\n")
		}

		// Last iteration with no conclusion: emit a sorry message
		if iteration == opts.MaxSteps-1 {
			msg := "抱歉，我还没有得出结论，请重新描述您的问题。"
			sink.WriteContent(msg)
			fullContent.WriteString(msg)
		}
	}

	return &ReActResult{
		FinalContent:    strings.TrimSpace(fullContent.String()),
		Steps:           steps,
		WriteCount:      writeCount,
		ToolCount:       toolCount,
		ToolCallResults: toolCallResults,
	}, nil
}
