## Why

Claude API 直连成本高且需要海外信用卡。OpenCode Go 是低成本订阅服务（首月 $5，之后 $10/月），提供对多个开源模型的可靠访问，且 API 兼容 OpenAI 格式。替换后可用 DeepSeek V4 Pro、Qwen3.7 Max 等模型，大幅降低成本。

## What Changes

- **BREAKING**: 删除 `claude.go` 中的 Claude (Anthropic) provider 实现
- 新增 `opencode.go` provider，对接 OpenCode Go 的 OpenAI 兼容 API
- 更新 provider factory，将 `ProviderClaude` 替换为 `ProviderOpenCode`
- 更新 `ParseClaudeSSE` 相关代码（移除或保留供其他用途）
- 更新数据库默认值：`provider` 默认改为 `opencode`，`model_name` 默认改为 `deepseek-v4-pro`
- 更新前端 AI 配置界面，移除 Claude 选项，新增 OpenCode Go 选项

## Capabilities

### New Capabilities

- `opencode-provider`: OpenCode Go provider 实现，通过 OpenAI 兼容 API 对接 OpenCode Go 服务，支持 Chat 和 StreamChat

### Modified Capabilities

（无现有 spec 需要修改）

## Impact

- `backend/internal/ai/claude.go` — 删除
- `backend/internal/ai/opencode.go` — 新增
- `backend/internal/ai/provider.go` — 更新 factory、ProviderType 常量、SSE parser
- `backend/cmd/server/main.go` — 更新数据库建表默认值
- `backend/internal/handler/ai.go` — 更新配置默认值
- 前端 AI 配置组件 — 更新 provider 选项
- 现有使用 Claude 配置的用户需要迁移（数据库中 `provider='claude'` 的记录需更新）
