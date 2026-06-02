## Context

当前 `internal/handler/ai.go:executeWebSearch` 直接调 `https://api.tavily.com/search`,API key 来自 `ai_configs.config.tavily_api_key`(或 `TAVILY_API_KEY` 环境变量)。要接入 MiniMax 的 `web_search` MCP 工具,核心问题是:它只通过 stdio MCP 协议暴露,没有公开的 HTTP endpoint 文档。所以必须按 MCP 客户端的方式 spawn 子进程(`uvx minimax-coding-plan-mcp`)再走 JSON-RPC。

约束:
- 单进程单用户本地应用,搜索量低(每天几次到几十次)
- 不想破坏现有 `websearch` tool 名 + 参数 + auto-execute 语义(AI 侧 system prompt 不需要改)
- 不能把 `uvx` 和 Python 包打进 Go 二进制,作为外部运行时依赖
- 现有 `ai_configs.config` 已经是 JSON 字段,直接扩展最自然

## Goals / Non-Goals

**Goals:**
- `websearch` tool 保持原名/原参数/原 auto-execute 行为,执行时分发到 Tavily 或 MiniMax
- 用户在 AI 设置页切换 `search_provider`,key 独立保存
- MCP server 子进程懒启动 + 5 分钟空闲回收,首次调用延迟 < 2s
- 错误信息全部中文,且能区分缺 key / spawn 失败 / 调用超时 / 鉴权失败
- 给后续接入其他 MCP server 留干净的扩展点(`mcp` 包不绑死 MiniMax)

**Non-Goals:**
- 不修改 AI system prompt(tool 名不变,AI 无感知)
- 不做 MCP server 多实例 / 集群
- 不实现 MCP Streamable HTTP transport(只支持 stdio,与官方文档一致)
- 不自动下载 `uvx` 或 Python(用户需自行安装;错误信息提示到位即可)
- 不缓存搜索结果

## Decisions

### 1. MCP 客户端:用 `github.com/mark3labs/mcp-go`

事实标准,Go 生态最成熟的 MCP 库,支持 stdio transport / tools/call / resources/read,API 与 MCP spec 2025-03 对齐。`NewStdioMCPClient` + `client.Start(ctx)` 即可拿到 session。

**替代方案**:手写 JSON-RPC over stdio。
- 不选:实现 MCP initialize/ping/tools/list/tools/call 协议细节成本高,且未来加新 MCP server 还得重写

### 2. 进程生命周期:Lazy spawn + 5 分钟 idle TTL

搜索是低频操作,启动时跑 1 个空闲 Python 进程没意义。改用懒启动:
- 首次 `websearch` 调用 → spawn `uvx minimax-coding-plan-mcp`,做 `initialize` + `tools/list` 握手(~1-2s)
- 调用完成后保留 client,5 分钟内复用
- 5 分钟无调用 → `client.Close()` 杀子进程;下次调用重新 spawn

实现:`mcp.Pool` 内部一个 `sync.Mutex` 保护的 `*mcp.Client` + `lastUsed time.Time`,`Get(ctx)` 检查 TTL,过期就 close + 新建。

**替代方案 A**:Eager spawn,启动时跑子进程。
- 不选:本地小工具,启动耗时 + 空转资源都是负收益

**替代方案 B**:每次调用重新 spawn。
- 不选:每次 1-2s 冷启动 + uvx 包解析延迟,体验差

### 3. Tool 调用:静态 `web_search` 形状,运行时调 `client.CallTool`

MiniMax MCP 只暴露 `web_search` 一个 tool(参数 `query: string`),且该信息是公开的、不会频繁变。`mcp/minimax.go` 里硬编码 tool 名 + JSON schema,不走 `tools/list` 动态发现。这样:
- 失败更早(进程启动时就验证 tool 存在,而不是第一次 search 时)
- 不依赖 server 一直在线(`tools/list` 需要连通的 server)
- 调用路径短:handler → `mcpClient.CallTool(ctx, "web_search", args)` → 返回 MCP `CallToolResult`

### 4. 配置 schema:`search_provider` 字段,默认 `tavily`

```json
{
  "tavily_api_key": "tvly-...",
  "minimax_api_key": "...",
  "minimax_api_host": "https://api.minimaxi.com",
  "search_provider": "tavily" | "minimax_mcp"
}
```

**Default = "tavily"** 是为了不破坏现有用户配置;未设置 `search_provider` 时回退到 `tavily`。`minimax_api_host` 可选,不传走 `https://api.minimaxi.com`。

### 5. 配置读取:`getSearchProvider` + `getMiniMaxConfig`,`getTavilyAPIKey` 保留

`handler/ai.go` 加 2 个 helper,跟 `getTavilyAPIKey` 同样模式(先看 env 变量,再读 DB JSON 字段):
- `getSearchProvider(ctx) string` -> `"tavily"` / `"minimax_mcp"`,env 优先 `LEARN_HELPER_SEARCH_PROVIDER`
- `getMiniMaxConfig(ctx) (apiKey, host string)`

调用路径:`executeWebSearch` 读 provider → switch:
- `"minimax_mcp"`:校验 key → `mcp.Search(ctx, query)` → 格式化结果
- `"tavily"` 或未设:走现有 Tavily 路径,代码原样

### 6. MCP 结果格式化:沿用 Tavily 风格

Tavily 当前输出 `**Title** \n URL: ... \n 摘要: ...` 的中文格式。MiniMax MCP `CallToolResult.Content` 是 MCP `TextContent` 数组(每项是搜索结果 JSON 或纯文本,具体取决于 server 实现),需要解析后转成同等格式,让 AI 侧 `tool_result` 注入后体验一致。

`mcp/minimax.go` 内做一次内容解析:遍历 `Content[].Text`,尝试 JSON 解析,失败时按纯文本处理,统一输出 `Title / URL / Snippet` 三元组列表。如果 server 返回的格式跟预期对不上,回退到 `原文回灌`(把整个 TextContent 拼起来给 AI,让 AI 自己抽)。

### 7. 超时与取消

- 每次 MCP `CallTool` 给 30s `context.WithTimeout`(MiniMax 网络搜索典型 < 5s,30s 包含 cold start)
- spawn 阶段 60s(uvx 首次拉包可能要 10-20s)
- handler ctx 取消时(`ctx.Done()`),透传到 MCP client,让它发 `notifications/cancelled` 然后杀进程

### 8. 前端 UI:segmented control 二选一

`settings/page.tsx` 改造:
- 搜索 provider 段:两个按钮 `Tavily` / `MiniMax MCP`,active 状态高亮
- 下面 key 输入框按选择切换:
  - Tavily:现有 `tavily_api_key` 输入框
  - MiniMax:`minimax_api_key` + 可选 `minimax_api_host` 输入框(host 默认值预填)
- 提交时:把当前选中的 provider 写到 `search_provider` 字段,key 字段按 provider 单独发;切换 provider 不清空另一个的 key(用户可能两边都配)

`/api/ai/configs` GET 返回时合并 `search_provider` 字段;POST 接受新 schema,handler 端按字段合并写入 JSON。

## Risks / Trade-offs

- **[uvx 未安装]** → 首次调用返回 `[系统] websearch 执行失败:未检测到 uvx,需先安装 uv (https://docs.astral.sh/uv/)`;设置页加一次性提示(读不到 version 不强制,后端返回错误时显示)
- **[MCP server 冷启动慢]** → 首次 1-2s + uvx 拉包 10-20s(仅首次);在 websearch 工具描述里加一句"首次调用需要初始化 MCP server,可能较慢"避免 AI 误判超时
- **[MiniMax MCP server 接口变更]** → 进程启动时验证 `tools/list` 包含 `web_search`,否则报错(防御性,文档没承诺稳定 schema)
- **[MCP 库依赖维护风险]** → `mark3labs/mcp-go` 是事实标准,pin 一个 minor 版本;若弃用,迁移到 stdio JSON-RPC 手写也只需 ~300 行
- **[配置向后兼容]** → 旧配置没有 `search_provider` → 默认 `tavily`,行为不变;有 `minimax_api_key` 但没 `search_provider` → 忽略,需用户主动选
- **[并发安全]** → `mcp.Pool` 用 mutex 串行化 initialize/close,CallTool 期间持有锁;本地单用户场景下并发量极低,可接受
- **[子进程残留]** → server 正常关闭 / panic 时 defer `client.Close()`;TTL 过期时也 Close;异常退出时 Go runtime 关闭父进程会杀掉子进程(Unix signal propagation)

## Migration Plan

无 schema migration:不新增表 / 不改列。`ai_configs.config` 是 JSON 字段,旧值原样保留,只是 handler 端多读两个 key。

部署:
1. 拉代码,`go mod tidy`(新增 `mark3labs/mcp-go`)
2. 用户机器装 `uvx`(`brew install uv` 或 `curl -LsSf https://astral.sh/uv/install.sh | sh`)
3. 首次 websearch 调用自动触发 `uvx minimax-coding-plan-mcp` 拉包
4. 设置页填 MiniMax API key + 选 `MiniMax MCP`,提交

回滚:`search_provider` 改回 `tavily` 即恢复旧行为;不需要删代码。

## Open Questions

- MiniMax MCP server 返回 `CallToolResult.Content` 的具体 JSON 结构文档没给,需要在实施时实测一次确认;若结构跟 Tavily 差异大,format 函数可能要迭代
- `uvx minimax-coding-plan-mcp -y` 的 `-y` 行为(是否自动确认)在某些 uvx 版本会变,实施时验证 spawn 命令的稳定性
- 是否要给 MCP 加一个调试日志开关(写到 `learn-helper-mcp.log`)?实施时决定
