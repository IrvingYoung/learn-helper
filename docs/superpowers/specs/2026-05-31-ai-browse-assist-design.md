# LLM Wiki AI 浏览辅助设计

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-05-31 | v1.0 | 初始设计 |

## 概述

用户在右栏浏览知识库页面时，AI 通过两种方式辅助学习：

1. **当前页面感知** — AI 知道用户在浏览哪个页面，提问时自动关联上下文
2. **划词询问 AI** — 用户选中页面文字，一键发送到聊天窗口让 AI 解释

核心设计原则：**用户控制触发时机，AI 按需获取内容**。不主动消耗 token，不在用户没有提问时引入 AI。

## 功能一：当前页面感知

### 设计

用户在左栏或知识树中选中一个页面后，该页面的标识（slug + ID + title）自动透传到聊天面板。当用户在聊天中提问时，AI 的 System Prompt 中注入：

```
用户当前正在查看的页面：Docker 概述 (slug: docker-overview, ID: 42)
```

AI 根据这个信息，在需要时自己调用 `read_page` 读取页面内容，不需要后端替他传页面全文。

### 数据流

```
用户点击知识树节点
    ↓
WikiPageLayout 更新 selectedSlug
    ↓
ChatPanel 收到新的 slug / id / title
    ↓
聊天输入框上方显示 "当前页面：Docker 概述"
    ↓
用户输入消息并发送
    ↓
前端在 POST /api/ai/chat 请求中附带 current_slug
    ↓
后端将页面信息注入 AI System Prompt
    ↓
AI 按需调用 read_page 获取内容，回答问题
```

### 输入框上方指示器

聊天输入框上方显示一行浅色文字，格式为：

```
📄 当前页面：Docker 概述
```

条件：
- 用户选中了某个页面时显示
- 用户未选中任何页面（知识库刚打开、概览页）时不显示
- 切换页面时即时更新

## 功能二：划词询问 AI

### 设计

用户在 PageViewer 中阅读页面内容时，选中一段文字后浮现一个浮动按钮"💬 询问 AI"。点击后，选中文本 + 页面引用自动填入聊天输入框，用户可编辑后发送。

### 交互流程

```
用户在右栏选中一段文字
    ↓
选中区域上方/附近出现浮动按钮 "💬 询问 AI"
    ↓
用户点击按钮
    ↓
选中文本以 blockquote 格式填入聊天输入框：
  > 选中的文字内容...
  >
  [来自页面：Docker 概述]
    ↓
用户可编辑输入框内容（补充自己的问题等）
    ↓
用户按回车发送
    ↓
AI 收到：当前页面上下文 + 选中文本 + 用户的问题
    ↓
AI 通过 read_page 获取完整内容，结合用户提出的具体问题回答
```

### 浮动按钮

- 仅在有选中文本且选中的文本非空时显示
- 位置：选中区域的右上方或附近，不遮挡选中文字
- 失去焦点（用户点击其他地方）时隐藏
- 样式：小号圆角按钮，带图标 + 文字"询问 AI"

### 输入框填充格式

```
> 镜像分层是 Docker 的核心机制之一，每个层...

[来自页面：Docker 概述]
```

用户可以在后面继续输入自己的问题，比如：

```
> 镜像分层是 Docker 的核心机制之一，每个层...
>
[来自页面：Docker 概述]
这里说的写时复制具体是怎么工作的？
```

## 数据流图

```
PageViewer                    ChatPanel                     Backend
    │                             │                            │
    │ 用户选中文字                  │                            │
    │──→ 显示"询问 AI"按钮          │                            │
    │                             │                            │
    │ 用户点击按钮                   │                            │
    │──→ 传递选中文本+页面信息 ─────→│ 填充到输入框                  │
    │                             │                            │
    │                             │ 用户编辑后发送                │
    │                             │──→ POST /api/ai/chat        │
    │                             │    { message,              │
    │                             │      conversation_id,      │
    │                             │      current_slug }        │
    │                             │                            │──→ 注入页面上下文到 System Prompt
    │                             │                            │──→ AI 调用 read_page（按需）
    │                             │                            │
    │                             │←── SSE 流式返回 AI 回复      │
```

## API 变更

### POST /api/ai/chat

**新增请求字段：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current_slug` | string | 否 | 用户当前正在浏览的页面 slug |

**后端处理逻辑：**

```
if req.CurrentSlug != "" {
    查询页面 title 和 ID（通过 slug）
    注入到 AI System Prompt：
      "用户当前正在查看的页面：{title} (slug: {slug}, ID: {id})"
}
```

## 前端变更

### WikiPageLayout（三栏容器）

- 将 `selectedPageId`（从 tree 节点匹配）和 `selectedPageTitle` 传给 ChatPanel
- 给 ChatPanel 绑定 ref，通过回调方式让 PageViewer 能触发输入框填充
- 给 PageViewer 传入 `onAskAI` 回调

### ChatPanel

- 新增 props: `currentSlug`, `currentPageId`, `currentPageTitle`
- 输入框上方新增当前页面指示器（仅在 `currentPageTitle` 不为空时显示）
- POST /api/ai/chat 请求体中加入 `current_slug`
- 新增 `appendToInput(text: string)` 方法（通过 forwardRef 或回调暴露给父组件）
  - 选中文本和页面信息通过此方法追加到输入框

### PageViewer

- 新增 `onAskAI` prop: `(text: string, pageTitle: string) => void`
- 使用 `document.getSelection()` 监听选中事件
- 选中文本非空时，在选中区域附近渲染"💬 询问 AI"浮动按钮
  - 使用 `getBoundingClientRect()` 定位按钮位置
  - 按钮在点击后或失去焦点时隐藏
  - 点击触发 `onAskAI(selectedText, pageTitle)`

## 后端变更

### handler/ai.go — AIChat 方法

- 解析 `CurrentSlug` 请求字段
- 如果 `CurrentSlug` 不为空，通过 slug 查询 page ID 和 title
- 将页面上下文注入到 buildWikiContext 返回的字符串中（而非额外 System Prompt）
- 示例注入格式：
  ```
  （页面上下文已包含在知识库结构中）
  用户当前正在查看的页面：Docker 概述 (slug: docker-overview, ID: 42)
  ```

## 不变的部分

- AI 工具定义不变（不需要新增工具）
- 确认流程不变（写入操作依然需要确认）
- SSE 流式传输不变
- 知识树、三栏布局不变
- 数据库 schema 不变
- System Prompt 结构不变，仅动态追加一行上下文

## 测试要点

- 前端：选中文字后按钮出现/消失逻辑
- 前端：点击按钮后输入框正确填充格式
- 前端：指示器随页面切换更新
- 后端：current_slug 注入正确页面信息
- 后端：当 slug 不存在时优雅降级（不注入，继续正常流程）
- 集成：端到端的划词→提问→AI 回复流程
