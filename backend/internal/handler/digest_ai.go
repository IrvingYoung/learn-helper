package handler

import (
	"context"
	"fmt"
	"time"

	"learn-helper/internal/ai"
)

// DigestConfig is the per-task subset used by the digest runner. It
// lives in the handler package so AIHandler can satisfy DigestAI
// without creating an import cycle (cron -> handler -> cron). The
// cron package exposes a type alias (see cron.DigestConfig) so callers
// in the cron package can use the unqualified name.
type DigestConfig struct {
	SinceHours          int
	MaxTweetsPerAccount int
	MaxTotalTweets      int
}

// DigestAI is the abstract surface the digest runner needs from the
// AI handler. The main package provides a concrete implementation by
// passing *AIHandler (which has the matching GenerateDigestPage method).
type DigestAI interface {
	GenerateDigestPage(ctx context.Context, runID string, cfg DigestConfig) error
}

// digestSystemPrompt is appended to the wiki_maintainer system prompt
// when the AI is being asked to generate a daily digest. It instructs
// the AI to read the tweets table, structure the output, and write a
// single wiki page (create or update, not both).
const digestSystemPrompt = `
=== 特殊任务：AI 日报生成 ===
1. 先调 list_recent_tweets 读取本次 run 抓到的推文。
2. 按以下三段结构组织日报 markdown:
   ## 今日趋势   (3-5 条要点)
   ## 主题讨论   (按主题分组,每组 2-4 条)
   ## 关键引述   (3-5 条原推 + 背景解读)
3. 调 lookup_page(title="AI 日报 · YYYY-MM-DD")
   - 若存在:用 update_page,page_id=<该页 ID>,content=<上面的 markdown>
   - 若不存在:用 create_page,title="AI 日报 · YYYY-MM-DD",content=<上面的 markdown>
4. 不要调任何其他写工具。一次提议只产出一个 create_page 或 update_page action。
`

// GenerateDigestPage satisfies DigestAI. It runs the AI in a single
// ReAct loop with a fixed digest-mode prompt, then auto-approves the
// resulting create_page / update_page call (writes are not gated in
// cron mode).
func (h *AIHandler) GenerateDigestPage(ctx context.Context, runID string, cfg DigestConfig) error {
	if h.aiProviderFactory == nil {
		return fmt.Errorf("ai provider factory not configured")
	}
	cfg2, err := h.queries.GetActiveAIConfig(ctx)
	if err != nil {
		return err
	}
	provider, err := h.aiProviderFactory(ai.ProviderType(cfg2.Provider), cfg2.ApiKey, cfg2.ModelName)
	if err != nil {
		return err
	}

	// Build a user message that includes the run boundary timestamp.
	// The AI's list_recent_tweets tool can filter on this.
	sinceHint := time.Now().Add(-time.Duration(cfg.SinceHours) * time.Hour).Format(time.RFC3339)
	userMsg := fmt.Sprintf("请为本次 digest run (id=%s, since=%s) 生成 AI 日报。\n完成后用 1 句话总结你做了什么。", runID, sinceHint)

	basePrompt := ai.BuildSystemPrompt(ai.RoleWikiMaintainer, "", h.SkillRegistry)
	systemPrompt := basePrompt + "\n" + digestSystemPrompt

	req := ai.ChatRequest{
		Messages:     []ai.Message{{Role: "user", Content: userMsg}},
		SystemPrompt: systemPrompt,
		Tools:        ai.WikiTools(),
		MaxTokens:    8192,
	}

	// Use a no-op sink — cron doesn't stream events anywhere.
	sink := &digestSink{}
	_, err = h.RunReAct(ctx, provider, req, ReActOptions{
		AutoApproveWrites: true,
		MaxSteps:          6, // 工具调用 + propose_plan
		Sink:              sink,
		RunID:             0,
	})
	return err
}

// digestSink is a no-op ReActEventSink used by the digest runner. It
// logs events at info level.
type digestSink struct{}

func (s *digestSink) WriteContent(text string)                       {}
func (s *digestSink) WriteToolCallStart(id, name, input string)      {}
func (s *digestSink) WriteToolResult(id, name, output, errStr string) {}
func (s *digestSink) WritePermissionRequired(req PermissionRequest)   {}
func (s *digestSink) WriteAskUserRequest(req AskUserRequest)          {}
func (s *digestSink) WriteDone()                                      {}
func (s *digestSink) WriteError(msg string)                           {}
