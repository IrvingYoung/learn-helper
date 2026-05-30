## Why

DeepSeek API 提供了成本更低的 AI 能力，支持中文语境下的代码生成和推理。将 DeepSeek 添加为 Provider 选项，可以让用户在配置中选择性价比更高的模型。

## What Changes

- 新增 `DeepSeekProvider` 实现 `AIProvider` 接口
- 配置支持选择 Provider 类型（Claude / DeepSeek）
- DeepSeek 支持流式和非流式 API 调用
- 与现有 ClaudeProvider 保持一致的接口设计

## Capabilities

### New Capabilities
- `ai-provider`: AI 模型提供商抽象层，支持 Claude 和 DeepSeek

### Modified Capabilities
- 无

## Impact

- 新增文件：`backend/internal/ai/deepseek.go`
- 配置修改：支持 Provider 类型和 API Key 配置
- handler 层需要根据配置选择对应的 Provider 实例