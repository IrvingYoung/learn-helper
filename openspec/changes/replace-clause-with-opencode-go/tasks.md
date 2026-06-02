## 1. 删除 Claude Provider

- [x] 1.1 删除 `backend/internal/ai/claude.go` 文件
- [x] 1.2 从 `provider.go` 中移除 `ProviderClaude` 常量和 `ParseClaudeSSE` 函数
- [x] 1.3 从 `NewProvider` factory 中移除 Claude case

## 2. 新增 OpenCode Go Provider

- [x] 2.1 创建 `backend/internal/ai/opencode.go`，参考 `deepseek.go` 实现 `OpenCodeProvider` 结构体
- [x] 2.2 实现 `Chat` 方法：POST 到 `https://api.opencode.ai/v1/chat/completions`
- [x] 2.3 实现 `StreamChat` 方法：复用 `ParseDeepSeekSSE` 解析 OpenAI 兼容 SSE
- [x] 2.4 实现 `messageToOpenCodeMessage` 消息转换（复用 `messageToDeepSeekMessage`，格式相同）
- [x] 2.5 设置默认模型为 `deepseek-v4-pro`

## 3. 更新 Provider Factory

- [x] 3.1 在 `provider.go` 中添加 `ProviderOpenCode` 常量
- [x] 3.2 在 `NewProvider` switch 中添加 `ProviderOpenCode` case
- [x] 3.3 更新错误消息中的支持 provider 列表

## 4. 数据库迁移

- [x] 4.1 在 `cmd/server/main.go` 启动逻辑中添加迁移：将 `provider='claude'` 更新为 `provider='opencode'`，`model_name` 更新为 `deepseek-v4-pro`
- [x] 4.2 更新建表 SQL 中的默认值：`provider` 默认改为 `'opencode'`，`model_name` 默认改为 `'deepseek-v4-pro'`

## 5. 更新配置 Handler

- [x] 5.1 更新 `handler/ai.go` 中 `UpsertAIConfig` 的默认 provider 和 model 值
- [x] 5.2 更新错误消息和支持的 provider 列表（无需额外更新，错误消息来自 provider.go factory）

## 6. 前端更新

- [x] 6.1 更新 AI 配置页面中的 provider 下拉选项：移除 "Claude"，添加 "OpenCode Go"
- [x] 6.2 更新选择 OpenCode Go 时的默认模型为 `deepseek-v4-pro`
- [x] 6.3 更新相关文案和提示信息（API key placeholder 统一为 `sk-...`）

## 7. 验证

- [x] 7.1 编译通过：`cd backend && go build ./cmd/server`
- [ ] 7.2 测试 OpenCode Go provider 的 Chat 和 StreamChat 功能（需要 API key）
- [ ] 7.3 验证数据库迁移逻辑正确执行（需要运行 server）
