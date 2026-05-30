# Web Tools: websearch & webfetch

## 概述

为 LLM Wiki 的 Agent 系统新增两个网络工具：`websearch`（联网搜索）和 `webfetch`（网页内容获取），让 AI 在维护知识库时可以自主搜索信息、读取用户提供的链接。

## 需求

1. AI 可以在对话中调用 `websearch` 搜索网络，获取结构化结果
2. AI 可以调用 `webfetch` 获取指定 URL 的内容（用户提供链接时使用）
3. 两个工具都是只读操作，走自动执行路径（无需用户确认）
4. Tavily API Key 通过设置页面配置，存储在 `ai_configs.config` 列

## 技术方案

### 工具定义（provider.go）

在 `WikiTools()` 中新增：

- **websearch** — 参数：`query`（string，必填）、`max_results`（integer，可选，默认 5）
- **webfetch** — 参数：`url`（string，必填）

两个工具描述中注明可自动执行。

### 执行层（handler/ai.go）

- 在 `executeAutoTool` 中新增 `case "websearch"` 和 `case "webfetch"`
- 在 `autoTools` map 中注册 `websearch` 和 `webfetch`
- `search_pages` 是搜索本地知识库，`websearch` 是搜索互联网，两者名字不同不会冲突

#### websearch 实现

1. 从 `ai_configs` 当前活跃配置的 `config` JSON 列读取 `tavily_api_key`
2. POST 到 `https://api.tavily.com/search`，15 秒超时
3. 返回格式化结果（标题、URL、摘要片段）

#### webfetch 实现

1. `net/http` GET 目标 URL，15 秒超时，设置合理的 User-Agent
2. 限制读取 500KB，用 `golang.org/x/net/html` 解析提取正文文本
3. 限制输出 3000 字符避免撑爆上下文

### 配置界面（前端）

在设置页面的 AI 模型配置区域，API Key 输入框下方新增 Tavily API Key 密码输入框。

### 配置存储

利用 `ai_configs` 表已有的 `config` TEXT 列（未使用），存储 JSON `{"tavily_api_key": "sk-..."}`。后端 `UpsertAIConfig` 接口新增 `tavily_api_key` 字段，`GetAIConfigs` 返回该字段。

### 不涉及改动

- 不需要改数据库表结构（`config` 列已存在）
- 不需要改 Agent 循环逻辑（复用现有自动执行框架）
- 不需要改 SSE/前端流式处理（与现有工具执行模式一致）

## 文件改动清单

| 文件 | 改动 |
|------|------|
| `backend/internal/ai/provider.go` | `WikiTools()` 新增 websearch、webfetch 工具定义 |
| `backend/internal/handler/ai.go` | `UpsertAIConfig` 支持 tavily_api_key；`GetAIConfigs` 返回；新增 `executeWebSearch`、`executeWebFetch`、`getTavilyAPIKey`；`executeAutoTool` 和 `autoTools` 注册新工具 |
| `backend/internal/handler/ai_test.go` | 酌情测试 |
| `frontend/src/app/settings/page.tsx` | 新增 Tavily API Key 输入框 |
| `backend/go.mod` / `go.sum` | 新增 `golang.org/x/net` 依赖 |
