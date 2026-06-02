## Why

`websearch` tool 目前硬编码调用 Tavily HTTP API,绑死在单一海外搜索供应商上。MiniMax Token Plan 提供的 `web_search` MCP 工具(`minimax-coding-plan-mcp`)按调用额度计费,无月费,API host 在国内(`api.minimaxi.com`)且对中文搜索结果质量更好。增加 MiniMax 作为可选项,用户在 AI 配置页就能切换,不需要修改 system prompt 或 tool 定义。

## What Changes

- 新增 `mcp` Go 包,封装 MCP stdio 客户端(server spawn + tool call),负责 `uvx minimax-coding-plan-mcp` 子进程生命周期
- `websearch` tool 在执行时分发:`search_provider=tavily` 走现有 Tavily HTTP 路径;`search_provider=minimax_mcp` 走 MCP 客户端
- `ai_configs.config` JSON 字段扩展:`minimax_api_key`、`minimax_api_host`(可选,默认 `https://api.minimaxi.com`)、`search_provider`(默认 `tavily`)
- `getTavilyAPIKey` 旁加 `getSearchProvider` + `getMiniMaxConfig`,AI handler 用统一接口读取
- 前端 AI 设置页:把 Tavily/MiniMax 二选一做成 segmented control,切换时显示对应 key 输入框;key 字段独立保存
- 错误信息保留中文并区分 provider(Tavily 缺 key / MiniMax 缺 key / MCP spawn 失败 等)
- 新增 `minimax-coding-plan-mcp` 作为隐式外部依赖(运行时由 `uvx` 拉取,不进 go.mod)

## Capabilities

### New Capabilities

- `minimax-mcp-websearch`: 通过 MCP 协议接入 MiniMax `web_search` 工具,作为 `websearch` tool 的可选后端,使用方按 `search_provider` 配置切换

### Modified Capabilities

(无现有 spec 涉及 websearch 行为契约;tool 名字、参数、auto-execute 语义保持不变)

## Impact

- `backend/internal/mcp/` - 新增包(`client.go`、`minimax.go`)
- `backend/internal/handler/ai.go` - `executeWebSearch` 分发逻辑;`getTavilyAPIKey` 旁加 MiniMax 配置读取;`UpsertAIConfig` 接受新字段
- `backend/cmd/server/main.go` - schema 不变,无需迁移
- `frontend/src/app/settings/page.tsx` - search provider 选择 UI,key 输入框切换
- `frontend/src/components/ToolCallCard.tsx` - 无改动(tool name 不变)
- 运行时新增外部依赖:`uvx` 二进制 + `minimax-coding-plan-mcp` Python 包(用户机器需有 Python 和 uv)
