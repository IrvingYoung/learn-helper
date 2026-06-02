## 1. Dependencies

- [ ] 1.1 Add `github.com/mark3labs/mcp-go` to `backend/go.mod` and run `go mod tidy`
- [ ] 1.2 Verify `uvx` is documented in `README.md` as a runtime requirement (with install link to https://docs.astral.sh/uv/)

## 2. MCP Client Package

- [ ] 2.1 Create `backend/internal/mcp/client.go` with a `Pool` struct that holds a single `*mcp.Client` + `lastUsed time.Time`, guarded by `sync.Mutex`
- [ ] 2.2 Implement `Pool.Get(ctx, env) (*mcp.Client, error)`: idle check (5min TTL) → reuse or close+respawn; spawn `uvx minimax-coding-plan-mcp -y` via `mcp-go`'s `NewStdioMCPClient`; call `Start(ctx)` then `Initialize(ctx)`
- [ ] 2.3 Implement `Pool.Close()`: gracefully close the active client and nil the pointer
- [ ] 2.4 Implement a process-level `sync.Once` registration so the pool is closed on `SIGINT` / `SIGTERM`

## 3. MiniMax MCP Backend

- [ ] 3.1 Create `backend/internal/mcp/minimax.go` exporting `Search(ctx, pool, query, apiKey, apiHost) (string, error)`
- [ ] 3.2 Inside `Search`: build env vars (`MINIMAX_API_KEY`, `MINIMAX_API_HOST` with default `https://api.minimaxi.com`), call `pool.Get(ctx, env)`
- [ ] 3.3 After `Get`, verify `tools/list` response includes `web_search`; if missing, close the pool entry and return Chinese error
- [ ] 3.4 Call `client.CallTool(ctx, "web_search", {query: query})` with a 30s `context.WithTimeout`
- [ ] 3.5 Parse the `CallToolResult.Content` text: try JSON list-of-results first; on failure, return the raw text wrapped in the standard prefix
- [ ] 3.6 Format successful results as `1. **Title**\n   URL: ...\n   摘要: ...` matching the Tavily output, prefixed with `[系统] 网络搜索「<query>」结果:`
- [ ] 3.7 Map error types to Chinese messages per the spec (uvx not found / spawn failure / timeout / auth / generic)

## 4. AI Handler Wiring

- [ ] 4.1 In `backend/internal/handler/ai.go`, add `getSearchProvider(ctx) string` reading `LEARN_HELPER_SEARCH_PROVIDER` env then `ai_configs.config.search_provider`, default `tavily`
- [ ] 4.2 Add `getMiniMaxConfig(ctx) (apiKey, host string)` reading `MINIMAX_API_KEY` / `MINIMAX_API_HOST` envs then DB JSON, with default host `https://api.minimaxi.com`
- [ ] 4.3 Refactor `executeWebSearch`: at the top, call `getSearchProvider`; if `minimax_mcp`, validate key then call `mcp.Search` and return; otherwise fall through to the existing Tavily code path
- [ ] 4.4 Update the `websearch` tool description in `internal/ai/provider.go` to note "首次调用需要初始化 MCP server,可能较慢" so the AI tolerates latency

## 5. AI Config Persistence

- [ ] 5.1 In `GetAIConfigs` (`handler/ai.go`): parse and expose `search_provider`, `minimax_api_key`, `minimax_api_host` in the response (alongside the existing `tavily_api_key`)
- [ ] 5.2 In `UpsertAIConfig` request struct: add `SearchProvider string`, `MiniMaxAPIKey string`, `MiniMaxAPIHost string`
- [ ] 5.3 In the upsert handler: read existing JSON, merge only the new fields that are non-empty, serialize back into `ai_configs.config`
- [ ] 5.4 Empty values MUST NOT overwrite stored values (so switching providers preserves the other key)

## 6. Frontend Settings UI

- [ ] 6.1 In `frontend/src/app/settings/page.tsx`, add `searchProvider` state, default from `cfg.search_provider || 'tavily'`
- [ ] 6.2 Add `minimaxApiKey`, `minimaxApiHost` states with defaults from cfg
- [ ] 6.3 Render a segmented control with two buttons (`Tavily` / `MiniMax MCP`) below the provider/model section; active button gets the highlight style
- [ ] 6.4 Conditionally render the key inputs: Tavily selected → existing `tavilyApiKey` input; MiniMax selected → `minimaxApiKey` + `minimaxApiHost` inputs (host pre-filled with `https://api.minimaxi.com`)
- [ ] 6.5 Update the submit body to include `search_provider` and the active provider's key(s); never include the inactive provider's key

## 7. Verification

- [ ] 7.1 `cd backend && go build ./...` compiles cleanly
- [ ] 7.2 `cd backend && go test ./...` passes (no existing tests should regress; add a unit test for `mcp.minimax.Search` happy path if feasible)
- [ ] 7.3 Manual smoke: start server, set `search_provider=tavily`, send a chat message asking AI to websearch → verify Tavily path still works
- [ ] 7.4 Manual smoke: with `uvx` installed and a real MiniMax key, set `search_provider=minimax_mcp`, trigger a websearch → verify spawn, tool call, and Chinese-formatted result
- [ ] 7.5 Manual smoke: kill `uvx` (`pkill -f minimax-coding-plan-mcp`) mid-call, retry → verify pool evicts the dead client and the next call spawns a fresh process
- [ ] 7.6 Manual smoke: with `search_provider=minimax_mcp` but no `minimax_api_key`, trigger a websearch → verify the Chinese "未配置 MiniMax API Key" error is returned
- [ ] 7.7 Frontend: load settings, switch between Tavily and MiniMax MCP, submit each, reload → verify the selection and only the relevant keys are persisted
