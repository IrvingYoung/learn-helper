# AI 浏览辅助实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现当前页面感知和划词询问 AI 两个功能

**Architecture:** 前端三栏组件间通过 props + ref 通信，后端在 AIChat handler 中解析 current_slug 并注入 System Prompt。AI 按需通过已有 read_page 工具获取页面内容。

**Tech Stack:** React 19 + Vite + Tailwind (frontend), Go + Chi + SQLite (backend)

---

## 文件变更总览

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `backend/internal/handler/ai.go` | 修改 | 解析 CurrentSlug，注入页面上下文 |
| `frontend/src/lib/api.ts` | 修改 | ChatRequest 新增 current_slug 字段 |
| `frontend/src/components/ChatPanel.tsx` | 修改 | 新增页面指示器、appendToInput、forwardRef 封装 |
| `frontend/src/components/PageViewer.tsx` | 修改 | 新增选中浮动按钮、onAskAI 回调 |
| `frontend/src/components/WikiPage.tsx` | 修改 | 串联 PageViewer → ChatPanel 的划词流程 |

---

### Task 1: 后端 — 解析 current_slug 并注入页面上下文到 System Prompt

**Files:**
- Modify: `backend/internal/handler/ai.go`

- [ ] **Step 1: 在请求结构体中添加 CurrentSlug 字段**

在 `AIChat` 方法的请求解析部分，找到 `var req struct` 定义，添加 `CurrentSlug` 字段：

```go
var req struct {
    ConversationID   int64           `json:"conversation_id"`
    Message          string          `json:"message"`
    ConfirmedActions []PendingAction `json:"confirmed_actions"`
    CurrentSlug      string          `json:"current_slug"`  // 新增
}
```

- [ ] **Step 2: 在 buildWikiContext 调用后注入页面上下文**

在 AIChat 方法中找到以下代码段（约第 350 行附近）：

```go
wikiContext := h.buildWikiContext(ctx, nil)
systemPrompt := ai.BuildSystemPrompt(convRole, wikiContext)
```

修改为：

```go
wikiContext := h.buildWikiContext(ctx, nil)
if req.CurrentSlug != "" {
    // 查找当前查看页面的信息并注入到 wikiContext
    pages, err := h.queries.GetWikiPageTree(ctx)
    if err == nil {
        for _, p := range pages {
            if p.Slug == req.CurrentSlug && p.PageType != "overview" {
                wikiContext += fmt.Sprintf(
                    "\n用户当前正在查看的页面：%s (slug: %s, ID: %d)\n",
                    p.Title, p.Slug, p.ID,
                )
                break
            }
        }
    }
}
systemPrompt := ai.BuildSystemPrompt(convRole, wikiContext)
```

注意：不用 `GetWikiPageBySlug` 查询，因为没有现成的查询方法。直接复用已有的 `GetWikiPageTree` 遍历即可（在 buildWikiContext 中也是这么用的）。如果 slug 不匹配（页面不存在或被删除），静默跳过，不报错。

- [ ] **Step 3: 编译验证**

```bash
cd backend && go build ./...
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handler/ai.go
git commit -m "feat: inject current page context into AI system prompt via current_slug"
```

---

### Task 2: 前端 — ChatRequest 类型和 API 层

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: ChatRequest 接口添加 current_slug**

找到 `ChatRequest` 接口定义：

```typescript
export interface ChatRequest {
  conversation_id: number;
  message: string;
  role?: string;
  context_type?: string;
  confirmed_actions?: PendingAction[];
}
```

添加 `current_slug` 字段：

```typescript
export interface ChatRequest {
  conversation_id: number;
  message: string;
  role?: string;
  context_type?: string;
  confirmed_actions?: PendingAction[];
  current_slug?: string;  // 新增
}
```

- [ ] **Step 2: 提交**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat: add current_slug to ChatRequest type"
```

---

### Task 3: 前端 — ChatPanel 增加页面感知和 appendToInput

**Files:**
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: 添加新 props（slug, id, title）和 forwardRef 封装**

ChatPanel 当前定义为：

```typescript
export function ChatPanel({ onPageChanged }: ChatPanelProps) {
```

需要改为 forwardRef，并接收新 props。找到最上面的 `interface ChatPanelProps`，添加新字段：

```typescript
interface ChatPanelProps {
  onPageChanged?: () => void;
  currentSlug?: string;
  currentPageId?: number;
  currentPageTitle?: string;
}
```

将函数签名改为：

```typescript
export const ChatPanel = forwardRef<{ appendToInput: (text: string) => void }, ChatPanelProps>(
  function ChatPanel({ onPageChanged, currentSlug, currentPageId, currentPageTitle }, ref) {
```

在文件顶部添加 `forwardRef` 导入（已有 `useState, useEffect, useRef, useCallback, useMemo`，添加 `forwardRef, useImperativeHandle`）：

```typescript
import { useState, useEffect, useRef, useCallback, useMemo, forwardRef, useImperativeHandle } from "react";
```

- [ ] **Step 2: 添加 useImperativeHandle 暴露 appendToInput**

在组件函数体开头部分（useState 之后），添加：

```typescript
const inputRef = useRef<HTMLTextAreaElement>(null);

useImperativeHandle(ref, () => ({
  appendToInput(text: string) {
    setInput((prev) => {
      const newInput = prev ? prev + "\n" + text : text;
      return newInput;
    });
    // Focus the input
    setTimeout(() => {
      inputRef.current?.focus();
    }, 0);
  },
}));
```

- [ ] **Step 3: 在输入框上方添加页面指示器**

找到 ChatPanel 中 render 输入框的部分（需要找到实际的 JSX 渲染区域）。在输入框所在的容器中，输入框上方添加：

```tsx
{currentPageTitle && (
  <div className="px-4 py-1.5 border-t border-th-border">
    <span className="text-xs text-th-text-muted flex items-center gap-1">
      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
      当前页面：{currentPageTitle}
    </span>
  </div>
)}
```

- [ ] **Step 4: 在发送消息时带上 current_slug**

找到 ChatPanel 中发起 streamChat 请求的地方。查找 `streamChat` 调用，在参数对象中加入 `current_slug`：

```typescript
// 找到类似这样的代码：
await streamChat(
  {
    conversation_id: activeConv.id,
    message: input,
    // 添加这一行
    current_slug: currentSlug,
  },
  ...
);
```

注意：current_slug 在 `confirmed_actions` 和 `context_type` 附近。需要在这个对象中。如果 currentSlug 为 undefined，API 层自然不发送该字段（因为 TypeScript 可选字段）。

- [ ] **Step 5: 给输入框 textarea 绑定 ref**

找到输入框 `<textarea>` 或 `<input>` 元素，添加 `ref={inputRef}`。如果当前没有 ref，需要确保 ref 的类型兼容。一般 textarea 用 `HTMLTextAreaElement`。

- [ ] **Step 6: 提交**

```bash
git add frontend/src/components/ChatPanel.tsx
git commit -m "feat: add current page indicator and appendToInput to ChatPanel"
```

---

### Task 4: 前端 — PageViewer 添加选中浮动按钮

**Files:**
- Modify: `frontend/src/components/PageViewer.tsx`

- [ ] **Step 1: 修改 PageViewerProps 添加 onAskAI**

```typescript
interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
  onAskAI?: (text: string, pageTitle: string) => void;  // 新增
}
```

- [ ] **Step 2: 添加选中事件处理和浮动按钮**

在 `PageViewer` 组件函数体内部，添加 state 和事件处理：

```typescript
import { useState, useEffect, useCallback } from "react"; // 确保 useState 已导入

export function PageViewer({ page, collapsed, onAskAI }: PageViewerProps) {
  // ... existing state and logic

  const [selectionTooltip, setSelectionTooltip] = useState<{
    text: string;
    x: number;
    y: number;
  } | null>(null);

  const handleMouseUp = useCallback(
    (e: React.MouseEvent) => {
      // 延迟执行以让 selection 稳定
      setTimeout(() => {
        const sel = window.getSelection();
        if (!sel || sel.isCollapsed || !sel.toString().trim()) {
          setSelectionTooltip(null);
          return;
        }
        const text = sel.toString().trim();
        const range = sel.getRangeAt(0);
        const rect = range.getBoundingClientRect();
        // 相对于 content 容器定位
        setSelectionTooltip({
          text,
          x: rect.left + rect.width / 2,
          y: rect.top - 10,
        });
      }, 10);
    },
    []
  );

  const handleAskAIClick = useCallback(() => {
    if (!selectionTooltip || !page) return;
    const quotedText = selectionTooltip.text
      .split("\n")
      .map((l) => `> ${l}`)
      .join("\n");
    const formatted = `${quotedText}\n>\n[来自页面：${page.title}]`;
    onAskAI?.(formatted, page.title);
    setSelectionTooltip(null);
    window.getSelection()?.removeAllRanges();
  }, [selectionTooltip, page, onAskAI]);

  // 点击其他区域时关闭浮动按钮
  useEffect(() => {
    const handleClickOutside = () => {
      setSelectionTooltip(null);
    };
    // 延迟注册以避免点击按钮本身触发关闭
    const timer = setTimeout(() => {
      document.addEventListener("click", handleClickOutside);
    }, 0);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("click", handleClickOutside);
    };
  }, [selectionTooltip]);
```

- [ ] **Step 3: 在内容容器上绑定 mouseUp**

找到渲染 Markdown 内容的容器（当前是 `<div className="prose-custom">` 或类似），在它外层或本身上添加 `onMouseUp={handleMouseUp}`：

```tsx
<div onMouseUp={handleMouseUp}>
  <MarkdownContent content={page.content} />
</div>
```

- [ ] **Step 4: 渲染浮动按钮**

在组件 JSX 中合适位置添加浮动按钮（例如在 return 语句末尾，或者使用 Portal）：

```tsx
{selectionTooltip && (
  <button
    className="fixed z-50 px-3 py-1.5 bg-th-accent text-white text-xs rounded-lg shadow-md hover:opacity-90 transition-opacity whitespace-nowrap"
    style={{
      left: selectionTooltip.x,
      top: selectionTooltip.y,
      transform: "translate(-50%, -100%)",
    }}
    onClick={handleAskAIClick}
    onMouseDown={(e) => e.preventDefault()} // 防止按钮点击触发 document click 关闭
  >
    💬 询问 AI
  </button>
)}
```

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/PageViewer.tsx
git commit -m "feat: add text selection floating button to ask AI"
```

---

### Task 5: 前端 — WikiPageLayout 串联整个流程

**Files:**
- Modify: `frontend/src/components/WikiPage.tsx`

- [ ] **Step 1: 从 tree 中匹配当前选中页面的 ID 和 title**

在 `WikiPageLayout` 组件中，已有 `selectedSlug` 和 `tree`。在 tree 数据加载后，计算 `selectedPageId` 和 `selectedPageTitle`：

```typescript
// 计算当前页面的 ID 和 title（从 tree 中查找）
const selectedPageInfo = useMemo(() => {
  if (!selectedSlug || !tree) return { id: undefined, title: undefined };
  const findNode = (nodes: WikiTreeNode[]): { id: number; title: string } | undefined => {
    for (const n of nodes) {
      if (n.slug === selectedSlug) return { id: n.id, title: n.title };
      if (n.children) {
        const found = findNode(n.children);
        if (found) return found;
      }
    }
    return undefined;
  };
  const info = findNode(tree);
  return { id: info?.id, title: info?.title };
}, [selectedSlug, tree]);
```

- [ ] **Step 2: 创建 chatPanelRef**

```typescript
const chatPanelRef = useRef<{ appendToInput: (text: string) => void }>(null);
```

- [ ] **Step 3: 创建 handleAskAI 回调**

```typescript
const handleAskAI = useCallback((text: string, pageTitle: string) => {
  chatPanelRef.current?.appendToInput(text);
}, []);
```

- [ ] **Step 4: 将新 props 传给 ChatPanel 和 PageViewer**

修改 ChatPanel 和 PageViewer 的渲染代码：

```tsx
<ChatPanel
  ref={chatPanelRef}
  onPageChanged={handlePageChanged}
  currentSlug={selectedSlug ?? undefined}
  currentPageId={selectedPageInfo.id}
  currentPageTitle={selectedPageInfo.title}
/>
```

```tsx
<PageViewer
  page={displayPage}
  collapsed={rightCollapsed}
  onAskAI={handleAskAI}
/>
```

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/WikiPage.tsx
git commit -m "feat: wire page context and ask-AI flow between panels"
```

---

## 自审

**Spec 覆盖检查：**
- ✅ 功能一（当前页面感知）— Task 1（后端注入） + Task 2（API 类型） + Task 3（ChatPanel 指示器和 current_slug 发送） + Task 5（WikiPageLayout 传递 props）
- ✅ 功能二（划词询问 AI）— Task 3（ChatPanel appendToInput） + Task 4（PageViewer 选中按钮） + Task 5（WikiPageLayout 串联）
- ✅ 用户控制触发时机 — 划词按钮只在用户选中文字时出现；AI 不主动触发
- ✅ AI 按需获取内容 — 后端只传页面标识，AI 自己调 read_page
- ✅ 输入框上方指示器 — Task 3 Step 3

**占位符检查：** 无 TBD、TODO 或占位符。每个步骤包含完整代码。

**类型一致性检查：**
- `current_slug` → `CurrentSlug`（Go） → `currentSlug`（TypeScript） → `current_slug`（JSON） — 一致
- `appendToInput` 在 ChatPanel 的 `useImperativeHandle` 中定义，在 WikiPageLayout 的 `handleAskAI` 中调用 — 一致
- `onAskAI` 签名 `(text: string, pageTitle: string) => void` 在 PageViewer 和 WikiPageLayout 中一致
