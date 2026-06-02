## Context

当前系统支持两个 AI provider：Claude (Anthropic) 和 DeepSeek。两者都通过 `AIProvider` 接口抽象，使用 factory 模式创建。Claude 使用 Anthropic 专有的 SSE 格式，DeepSeek 使用 OpenAI 兼容格式。

用户希望用 OpenCode Go（OpenAI 兼容 API）替换 Claude，以降低成本。OpenCode Go 提供多种开源模型，包括 DeepSeek V4 Pro、Qwen3.7 Max 等。

## Goals / Non-Goals

**Goals:**
- 删除 Claude provider 实现，减少维护负担
- 新增 OpenCode Go provider，使用 OpenAI 兼容 API 格式
- 保持现有 `AIProvider` 接口不变
- 迁移现有数据库配置

**Non-Goals:**
- 不保留 Claude provider（不做"同时支持"）
- 不改变 DeepSeek provider
- 不修改前端 AI 对话逻辑（只改配置选项）

## Decisions

### 1. OpenCode Go API 格式：复用 OpenAI 兼容格式

OpenCode Go 使用标准 OpenAI chat/completions API。可以参考现有 `deepseek.go` 的实现模式，因为 DeepSeek 也使用 OpenAI 兼容格式。

**实现方案**：新建 `opencode.go`，结构类似 `deepseek.go`，但：
- API endpoint: `https://api.opencode.ai/v1/chat/completions`
- Auth header: `Authorization: Bearer <key>`
- 默认模型: `deepseek-v4-pro`（OpenCode Go 中性能最强的模型之一）

**替代方案考虑**：将 DeepSeek 和 OpenCode 合并为一个通用 "OpenAI-compatible" provider。但这会增加复杂度且两个服务的默认模型、限额不同，保持独立更清晰。

### 2. SSE Parser：复用 `ParseDeepSeekSSE`

由于 OpenCode Go 使用 OpenAI 兼容的 SSE 格式，`ParseDeepSeekSSE` 可以直接复用。在 `opencode.go` 的 `StreamChat` 中调用同一个 parser。

### 3. 数据库迁移策略

在 server 启动时检测旧配置（`provider='claude'`），自动更新为 `provider='opencode'` 并设置默认模型。这样现有用户无需手动操作。

### 4. 删除 Claude SSE Parser

`ParseClaudeSSE` 和 Claude 专有的请求/响应类型将随 `claude.go` 一起删除。如果未来需要重新支持 Claude，可以从 git 历史恢复。

## Risks / Trade-offs

- **[OpenCode Go API 变更]** → OpenCode Go 是较新的服务，API 可能有变化。缓解：使用标准 OpenAI 兼容格式，变更风险低。
- **[模型质量差异]** → 开源模型在 tool calling 方面可能不如 Claude。缓解：选择 tool calling 能力较强的 DeepSeek V4 Pro 作为默认模型。
- **[用户迁移中断]** → 现有 Claude 配置用户需要更新 API key。缓解：启动时自动迁移 provider 类型，用户只需在配置页面更新 API key。
- **[删除 Claude 不可逆]** → 代码删除后需从 git 恢复。缓解：git 历史保留完整，恢复成本低。
