package ai

import (
	"fmt"
	"time"
)

// BuildCronSystemPrompt wraps the base wiki_maintainer system prompt with a
// cron-mode prefix that declares autonomous operation, the current time, the
// task description, and (optionally) the previous run's output summary.
//
// The prefix is prepended verbatim; the base prompt is unchanged so all the
// wiki structural / naming / tool guidance still applies. The prefix only
// overrides the human-collaboration clauses (e.g. "ask before writing").
func BuildCronSystemPrompt(basePrompt, taskPrompt, lastRunSummary string, now time.Time) string {
	prefix := fmt.Sprintf(`## 当前模式: 定时任务 (autonomous)
- 用户不在场,无法回答问题
- ask_user 工具不可用,禁止调用
- 写操作已开启 auto_approve,直接执行不需要确认
- 完成后用 1-2 句中文总结你做了什么

## 当前时间
%s

## 本次任务
%s
`, now.Format("2006-01-02 15:04:05 (MST)"), taskPrompt)

	if lastRunSummary != "" {
		prefix += fmt.Sprintf("\n## 上次运行摘要\n%s\n", lastRunSummary)
	}

	return prefix + "\n" + basePrompt
}
