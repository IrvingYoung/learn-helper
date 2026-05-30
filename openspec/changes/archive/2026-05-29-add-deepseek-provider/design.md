## Context

当前 AI 层已实现 `ClaudeProvider`，通过 `AIProvider` 接口抽象支持多 Provider 扩展。需新增 DeepSeek Provider 以支持成本更低的模型选择。

## Goals / Non-Goals

**Goals:**
- 新增 `DeepSeekProvider` 实现 `AIProvider` 接口
- DeepSeek API 支持流式和非流式调用
- 与 ClaudeProvider 保持一致的接口设计

**Non-Goals:**
- 不修改现有的 Claude Provider 实现
- 不涉及前端 Provider 选择 UI（配置层面）
- 不添加 Provider 自动切换逻辑

## Decisions

1. **沿用现有 AIProvider 接口**：`DeepSeekProvider` 实现与 `ClaudeProvider` 相同的接口，无需修改接口定义。

2. **DeepSeek API 适配**：
   - API Endpoint: `https://api.deepseek.com/chat/completions`
   - 请求体格式遵循 OpenAI 兼容格式
   - 模型名称: `deepseek-chat`（默认）
   - Stream 响应使用 SSE 格式

3. **工厂模式创建 Provider**：在 handler 层根据配置创建对应 Provider 实例。

## Risks / Trade-offs

- [风险] DeepSeek API 定价和限制可能变化 → 监控 API 响应，处理错误
- [风险] DeepSeek 流式响应格式与 Claude 不同 → 适配 `streamResponse` 方法

## Open Questions

- 是否需要在配置中添加 DeepSeek 模型选择（如 deepseek-chat / deepseek-coder）？
- 是否需要为 DeepSeek 单独配置 System Prompt？