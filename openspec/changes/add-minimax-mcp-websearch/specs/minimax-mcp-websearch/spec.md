## ADDED Requirements

### Requirement: Configurable websearch backend

The system SHALL allow the user to select which backend the `websearch` AI tool dispatches to. The active backend is determined by a `search_provider` field in the AI configuration. Supported values are `tavily` (default) and `minimax_mcp`. The `websearch` tool name, input schema, and auto-execute semantics MUST remain unchanged regardless of the active backend.

#### Scenario: Default value when search_provider is unset
- **WHEN** the AI configuration JSON does not contain a `search_provider` field
- **THEN** the system SHALL treat the active backend as `tavily`
- **AND** the existing Tavily call path SHALL execute

#### Scenario: User selects minimax_mcp
- **WHEN** the AI configuration JSON contains `"search_provider": "minimax_mcp"`
- **THEN** the system SHALL dispatch `websearch` calls to the MiniMax MCP backend
- **AND** the Tavily call path SHALL NOT execute

#### Scenario: Unknown search_provider value
- **WHEN** the AI configuration JSON contains an unrecognized `search_provider` value
- **THEN** the system SHALL fall back to `tavily` for that call
- **AND** the system SHALL log a warning identifying the unrecognized value

### Requirement: MiniMax MCP backend uses stdio MCP transport

When the active backend is `minimax_mcp`, the system SHALL call the MiniMax `web_search` tool by spawning `uvx minimax-coding-plan-mcp` as a child process and speaking the MCP protocol over its stdio. The system MUST use an MCP client library that implements MCP spec 2025-03-26 (initialize / tools/list / tools/call) over the stdio transport.

#### Scenario: First call spawns MCP server
- **WHEN** the first `websearch` call with `search_provider=minimax_mcp` is made after backend startup
- **THEN** the system SHALL spawn `uvx minimax-coding-plan-mcp -y` as a child process
- **AND** pass `MINIMAX_API_KEY` and `MINIMAX_API_HOST` in the child process environment
- **AND** send the MCP `initialize` request and `notifications/initialized` before the first `tools/call`

#### Scenario: Subsequent calls within TTL reuse the process
- **WHEN** a second `websearch` call arrives within 5 minutes of the previous successful call
- **THEN** the system SHALL reuse the existing MCP client without spawning a new child process
- **AND** SHALL NOT re-run `initialize`

#### Scenario: Idle timeout recycles the process
- **WHEN** no `websearch` call has been made for 5 minutes since the last completed call
- **THEN** the system SHALL terminate the MCP child process on the next call
- **AND** spawn a fresh child process for that call

#### Scenario: web_search tool must be available after initialize
- **WHEN** the MCP `initialize` handshake completes
- **THEN** the system SHALL verify that the `tools/list` response includes a tool named `web_search`
- **AND** if `web_search` is missing, the system SHALL close the client and return a Chinese error message identifying the missing tool

### Requirement: Required configuration for MiniMax backend

To use `search_provider=minimax_mcp`, the AI configuration MUST contain a `minimax_api_key`. The configuration MAY contain a `minimax_api_host`; when present it is used as the API host, and when absent the system SHALL use `https://api.minimaxi.com`. The system MUST also accept these values from the `MINIMAX_API_KEY` and `MINIMAX_API_HOST` environment variables, which take precedence over the database values.

#### Scenario: Missing minimax_api_key
- **WHEN** a `websearch` call is made with `search_provider=minimax_mcp` and neither the environment variable nor the AI configuration provides `minimax_api_key`
- **THEN** the system SHALL return the Chinese error `[系统] websearch 执行失败:未配置 MiniMax API Key,请在设置页中配置`
- **AND** SHALL NOT spawn the MCP server

#### Scenario: minimax_api_host defaults when not set
- **WHEN** a `websearch` call is made with `search_provider=minimax_mcp` and no `minimax_api_host` is configured anywhere
- **THEN** the system SHALL pass `https://api.minimaxi.com` to the MCP server as `MINIMAX_API_HOST`

#### Scenario: Environment variable overrides database
- **WHEN** both `MINIMAX_API_HOST` env var and the `minimax_api_host` config field are set
- **THEN** the system SHALL use the env var value for spawning the MCP child process

### Requirement: websearch response is normalized to a consistent shape

The MiniMax MCP backend MUST return its result formatted as a numbered list of search entries. Each entry SHALL include a title, a URL (if present in the underlying MCP response), and a snippet. The output text MUST be wrapped in the prefix `[系统] 网络搜索「<query>」结果:` to match the existing Tavily output format, so the AI consumes identical input regardless of backend.

#### Scenario: MiniMax returns structured results
- **WHEN** the MCP `tools/call` response for `web_search` contains a structured result with title/url/snippet fields
- **THEN** the system SHALL format each entry as `<n>. **<title>**\n   URL: <url>\n   摘要: <snippet>`
- **AND** wrap the list in the standard prefix

#### Scenario: MiniMax returns plain text fallback
- **WHEN** the MCP `tools/call` response text cannot be parsed into structured entries
- **THEN** the system SHALL return the raw text content wrapped in the standard prefix
- **AND** include a note indicating the content is raw MCP output

#### Scenario: Empty results
- **WHEN** the MCP `tools/call` response contains no result entries
- **THEN** the system SHALL return the standard prefix followed by `（无搜索结果）`

### Requirement: Error handling and Chinese user-facing messages

The MiniMax MCP backend MUST surface all failure modes as Chinese error strings returned from the `websearch` tool. The system MUST distinguish at least: (a) missing configuration, (b) `uvx` not installed, (c) MCP server spawn/initialize failure, (d) tool call timeout (>30s), (e) authentication failure from MiniMax, (f) other MCP server errors. The technical error detail SHALL be logged to the server log with the original error chain.

#### Scenario: uvx binary not found
- **WHEN** the system attempts to spawn `uvx minimax-coding-plan-mcp` and the `uvx` executable is not on `PATH`
- **THEN** the system SHALL return `[系统] websearch 执行失败:未检测到 uvx,请先安装 uv (https://docs.astral.sh/uv/)`
- **AND** SHALL log the underlying `exec: "uvx": executable file not found in $PATH` error

#### Scenario: MCP server crashes during call
- **WHEN** the MCP child process exits unexpectedly during a `tools/call`
- **THEN** the system SHALL close the broken client, evict it from the pool
- **AND** return `[系统] websearch 执行失败:MCP server 异常退出,请重试`
- **AND** the next call SHALL spawn a fresh process

#### Scenario: Tool call timeout
- **WHEN** a `tools/call` does not complete within 30 seconds
- **THEN** the system SHALL cancel the call context, close the client, evict from pool
- **AND** return `[系统] websearch 执行失败:调用超时(>30s),已重置 MCP 连接`

#### Scenario: MiniMax returns authentication error
- **WHEN** the MCP server reports an authentication / 401-like error
- **THEN** the system SHALL return `[系统] websearch 执行失败:MiniMax 鉴权失败,请检查 API Key`
- **AND** log the raw MCP error

### Requirement: Search provider selection exposed in AI settings UI

The AI settings page MUST render a segmented control with two options, `Tavily` and `MiniMax MCP`, reflecting the active `search_provider`. The API key input fields shown below the control SHALL switch to match the selected provider: selecting `Tavily` shows the `tavily_api_key` input, selecting `MiniMax MCP` shows the `minimax_api_key` and `minimax_api_host` inputs (with `minimax_api_host` pre-filled to the default). Submitting the form MUST persist the selected `search_provider` and the relevant key fields; switching providers MUST NOT clear the other provider's stored key.

#### Scenario: Initial render reflects stored provider
- **WHEN** the settings page loads with stored `search_provider=minimax_mcp`
- **THEN** the segmented control SHALL highlight the `MiniMax MCP` option
- **AND** the `minimax_api_key` and `minimax_api_host` inputs SHALL be visible
- **AND** the `tavily_api_key` input SHALL be hidden

#### Scenario: Switching from Tavily to MiniMax MCP
- **WHEN** the user clicks the `MiniMax MCP` segment after previously having `Tavily` selected
- **THEN** the `tavily_api_key` input SHALL hide
- **AND** the `minimax_api_key` and `minimax_api_host` inputs SHALL show
- **AND** the previously stored `tavily_api_key` value SHALL remain in the database (not cleared)

#### Scenario: Submitting MiniMax MCP configuration
- **WHEN** the user fills `minimax_api_key`, optionally edits `minimax_api_host`, and submits while `MiniMax MCP` is selected
- **THEN** the API request body SHALL include `search_provider: "minimax_mcp"`, `minimax_api_key`, and `minimax_api_host`
- **AND** SHALL NOT include `tavily_api_key` in the request body
