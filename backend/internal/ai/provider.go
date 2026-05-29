package ai

import (
	"context"
	"fmt"
)

// AIProvider 定义 AI 模型提供商的统一接口
type AIProvider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// Role 常量
const (
	RoleKnowledgeExplain = "knowledge_explain"
	RoleProblemSolving   = "problem_solving"
	RoleDashboard        = "dashboard"
	RoleWikiMaintainer   = "wiki_maintainer"
)

// SystemPromptTemplates 提供各角色的 system prompt 模板
var SystemPromptTemplates = map[string]string{
	RoleKnowledgeExplain: `你是一位数据结构与算法专家，专注于帮助软件工程师准备技术面试。
当用户浏览知识点并向你提问时，你应当：
- 清晰解释概念，举例说明
- 关联已学知识，帮助建立知识体系
- 引导思考，不直接给题解
- 如果用户问到具体题目，引导他们先思考解法

当前知识点上下文：
{{.Context}}

回答要求：
- 语言简洁易懂，适合面试准备
- 可以用代码示例，但代码只是辅助说明
- 鼓励用户思考和追问`,

	RoleProblemSolving: `你是一位耐心的面试教练，专注于帮助用户通过引导式提示解出算法题。
你必须遵守以下规则：
- 永远不要直接给出完整答案或完整代码
- 每次只给出一个提示，帮助用户朝正确方向思考
- 提示应当是引导性的，例如"考虑从...角度"或"这个问题的本质是..."
- 如果用户卡住了，先给出思路级别的提示，再逐步给出更具体的提示
- 当用户接近正确答案时，给予鼓励，让他们自己完成

题目上下文：
{{.Context}}

解题过程：
1. 先让用户描述思路
2. 根据用户的思路给予针对性提示
3. 每次提示后等待用户反馈
4. 不要一次性给出多个提示`,

	RoleDashboard: `你是一位学习规划专家，根据用户的学习数据分析薄弱点并推荐下一步学习路径。
当前学习数据：
{{.Context}}

你需要：
- 识别掌握程度较低的知识点
- 分析学习趋势和模式
- 推荐下一步应该重点学习的知识点
- 给出具体的复习计划建议`,
}

// RoleDisplayNames 角色展示名称
var RoleDisplayNames = map[string]string{
	RoleKnowledgeExplain: "知识讲解",
	RoleProblemSolving:   "解题辅导",
	RoleDashboard:        "学习规划",
	RoleWikiMaintainer:   "Wiki 管理员",
}

// ProviderType 定义支持的 AI Provider 类型
type ProviderType string

const (
	ProviderClaude  ProviderType = "claude"
	ProviderDeepSeek ProviderType = "deepseek"
)

// NewProvider creates an AI provider based on the provider type
func NewProvider(providerType ProviderType, apiKey, model string) (AIProvider, error) {
	switch providerType {
	case ProviderClaude:
		return NewClaudeProvider(apiKey, model), nil
	case ProviderDeepSeek:
		return NewDeepSeekProvider(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s (supported: claude, deepseek)", providerType)
	}
}