# AI-Native Knowledge Collaboration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 LLM Wiki 的协作模型从"审批前置"重构为"迭代后置"，让知识库构建是人和 AI 自然对话的副产品。

**Architecture:** 核心改动在 AI system prompt（改变 AI 角色和交互行为）和右栏页面展示（内容预览 + 确认条替代 PlanPreview action 列表）。后端改动集中在 system prompt 策略和确认 API，前端改动集中在 PageViewer 和 ChatPanel。

**Tech Stack:** Go (Chi + SQLite), React 19 (Vite + Tailwind + SWR)

---

### Task 1: 重写 AI System Prompt

**Files:**
- Modify: `backend/internal/ai/provider.go:273-317`

**核心改动：** 从"知识库管理员"改为"学习伙伴 + 内容创作者"。AI 不再是知识树的"管理者"，而是用户学习的"帮手"——用户决定记什么，AI 执行写入。

- [ ] **Step 1: 替换 `buildWikiMaintainerPrompt` 函数内容**

把这段：

```go
func buildWikiMaintainerPrompt(wikiContext string) string {
	// ... old prompt "你是 LLM Wiki 的知识库维护者..."
	// 替换为新的 prompt
```

新的 prompt 应该包含：

```
你是 LLM Wiki 的学习帮手。你的职责是协助用户构建和维护个人知识库。

## 协作方式

1. **你决定要不要记** — 用户说有收获时说"记下来"，你再写入知识库。不需要 AI 自动判断什么内容应该入库。
2. **大纲 = 页面内容 + 子目录** — 生成大纲时，直接在页面内写大纲内容，同时建子页面目录。大纲和知识树是同一个东西。
3. **先问再写** — 写内容前先了解用户的需要。用 1-2 个问题校准方向（目标读者水平、想要什么深度、注重什么角度）。
4. **记下来后在页面里展示** — 内容写在页面上，不在聊天里展示大段文字。页面顶部显示确认条让用户确认。
5. **迭代优先** — 用户说"这里改一下"时，直接修改页面内容，不需要重新走提案流程。
6. **主动建议，但不擅自改动** — 发现知识体系不完整时在聊天中提建议，由用户决定。
7. **接受调整** — 用户可以在聊天中直接调整结构："把 X 放到 Y 下面"，你理解意图后执行。

## 行为规则

- 用户说"记下来" → 写内容到当前查看的页面或最相关的页面
- 用户说"帮我生成大纲" → 在当前页面写大纲内容 + 创建子页面目录
- 用户说"改这里"、"重写"、"补充……" → 直接修改页面
- 用户说"算了"、"删掉" → 撤销未确认的内容
- 用户在聊结构调整 → 理解意图后操作（不需要确认流程）
- 用户不操作 → AI 不自行创建内容
```

确保保留已有的 `treeContext` 追加和 `Request Timestamp` 逻辑。

- [ ] **Step 2: 运行后端测试**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: PASS（prompt 改的是纯文本，不影响编译）

- [ ] **Step 3: 提交**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/ai/provider.go
git commit -m "feat: rewrite AI system prompt from admin to learning partner"
```

---

### Task 2: 添加页面内容确认 API

**Files:**
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wiki.go`

**核心改动：** 新增一个轻量 API，用于标记页面内容的确认状态。当前的 Plan 确认 `/api/plans/{id}/confirm` 不变，但新增 `/api/wiki/{id}/confirm` 用于页面级内容确认。

当前 `updateOverviewPage()` 在每次写操作后自动调用。这个逻辑保留，但确认 API 需要触发它。

- [ ] **Step 1: 在 handler.go 注册路由**

```go
// 在 RegisterRoutes 中新增
r.Put("/api/wiki/{id}/confirm", h.ConfirmPageContent)
```

- [ ] **Step 2: 在 wiki.go 实现 ConfirmPageContent**

```go
func (h *WikiHandler) ConfirmPageContent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid page id", http.StatusBadRequest)
		return
	}

	// 更新 content_status 为 published
	err = h.q.UpdatePageContentStatus(r.Context(), model.UpdatePageContentStatusParams{
		ID:             id,
		ContentStatus:  "published",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 触发概览页面更新
	go h.updateOverviewPage()

	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
```

- [ ] **Step 3: 检查 queries.sql.go 中是否有 UpdatePageContentStatus**

如果没有，添加 SQL：

```sql
-- migration
UPDATE wiki_pages SET content_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;
```

和 Go 代码：

```go
type UpdatePageContentStatusParams struct {
	ID            int64
	ContentStatus string
}

func (q *Queries) UpdatePageContentStatus(ctx context.Context, arg UpdatePageContentStatusParams) error {
	_, err := q.db.ExecContext(ctx, `UPDATE wiki_pages SET content_status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, arg.ContentStatus, arg.ID)
	return err
}
```

- [ ] **Step 4: 编译验证**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./cmd/server`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/handler/handler.go backend/internal/handler/wiki.go backend/db/migrations/
git commit -m "feat: add page content confirmation API"
```

---

### Task 3: 修改 PageViewer 组件——添加内容确认条

**Files:**
- Modify: `frontend/src/components/PageViewer.tsx`
- Modify: `frontend/src/lib/api.ts`

**核心改动：** 移除 Tab 布局（内容 / 操作），让 PageViewer 直接展示页面内容。当页面包含未确认的 AI 写入内容时，在顶部显示确认条。

- [ ] **Step 1: 在 api.ts 中添加 confirmContent API**

```typescript
export async function confirmPageContent(pageId: number): Promise<void> {
  await fetch(`/api/wiki/${pageId}/confirm`, { method: 'PUT' });
}
```

- [ ] **Step 2: 修改 PageViewer 移除 Tab 布局**

移除 `activeTab` 状态和 Tab bar，直接展示内容。

- [ ] **Step 3: 在 PageViewer 中添加确认条逻辑**

当一个页面有未确认的 content（`content_status === 'draft'`），在页面顶部显示确认条：

```tsx
{page.content_status === 'draft' && unconfirmed && (
  <div className="flex items-center gap-3 px-4 py-2 bg-amber-50 border-b border-amber-200">
    <span className="text-sm text-amber-700">有未确认的内容</span>
    <button
      onClick={handleConfirm}
      className="px-3 py-1 text-xs font-medium bg-amber-600 text-white rounded hover:bg-amber-700"
    >
      确认
    </button>
    <button
      onClick={handleDiscard}
      className="px-3 py-1 text-xs font-medium text-amber-600 hover:bg-amber-100 rounded"
    >
      不要了
    </button>
  </div>
)}
```

- [ ] **Step 4: 调整 props**

PageViewer 不再需要 `plan`, `pendingPlans`, `executionResults`, `onConfirmPlan`, `onRejectPlan`, `confirmingPlan`, `onPlanConfirmed`, `onPlanRejected`。新 props：

```typescript
interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
  onViewPage?: (slug: string) => void;
  onSelectPage: (slug: string) => void;
  onAskAI?: (text: string, pageTitle: string) => void;
  onContentConfirmed: (pageId: number) => void;
}
```

- [ ] **Step 5: 编译验证**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: PASS（可能会因为积压的 props mismatch 报错，逐个修复）

- [ ] **Step 6: 提交**

```bash
cd /Users/irving/repo/learn-helper
git add frontend/src/components/PageViewer.tsx frontend/src/lib/api.ts
git commit -m "feat: redesign PageViewer with inline confirm bar, remove tab layout"
```

---

### Task 4: 简化 PlanPreview——只保留 Outline 树展示

**Files:**
- Modify: `frontend/src/components/PlanPreview.tsx`

**核心改动：** PlanPreview 不再用于内容确认，只保留大纲树展示。移除 `ActionList`、`PhaseProgress`、action 确认按钮——这些被页面内确认条替代。

- [ ] **Step 1: 确定 PlanPreview 的用途**

当 AI 返回的 Plan 包含 `outline` 字段时，PlanPreview 展示大纲树。当 Plan 只有 actions 时，PlanPreview 不展示（改用页面内确认条）。

- [ ] **Step 2: 简化 PlanPreview**

移除以下内容：
- `PhaseProgress` 组件（阶段进度现由 outline 树本身展示）
- `ActionList` 组件（用户在页面内直接看内容）
- `ActionPreview` 组件
- `resolveRef` / `depLabel` 工具函数
- confirm/reject 按钮（确认移到了页面内）
- `isOutlineOnly` 判断逻辑外的所有 action 相关展示

保留：
- `OutlineNodeRow` 组件
- `OutlineTree` 组件
- 大纲展示的 header（reasoning 等）
- 确认按钮（只在大纲模式下）

```tsx
export function PlanPreview({ plan, onConfirm, confirming }: PlanPreviewProps) {
  const hasOutline = plan.outline && plan.outline.length > 0;

  if (!hasOutline) return null; // 不展示——内容阶段的 Plan 走页面内确认

  return (
    <div className="flex flex-col h-full">
      <div className="p-6 border-b border-th-separator">
        <h2 className="text-xl font-display text-th-text">知识大纲</h2>
        <p className="text-th-muted text-sm mt-2">{plan.reasoning}</p>
        <div className="text-xs text-th-muted mt-2">确认后将创建骨架页面</div>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        <OutlineTree outline={plan.outline} />
      </div>
      <div className="p-4 border-t border-th-separator">
        <button
          onClick={() => onConfirm(plan.id)}
          disabled={confirming || plan.status !== 'pending'}
          className="w-full px-4 py-2.5 rounded-lg bg-th-accent text-white font-medium hover:opacity-90 disabled:opacity-50 transition-opacity"
        >
          {confirming ? '创建中...' : '确认大纲'}
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
cd /Users/irving/repo/learn-helper
git add frontend/src/components/PlanPreview.tsx
git commit -m "refactor: simplify PlanPreview to outline-only, remove action confirm UI"
```

---

### Task 5: 删除 OperationQueue 和相关组件

**Files:**
- Delete: `frontend/src/components/OperationQueue.tsx`
- Modify: `frontend/src/components/PageViewer.tsx`
- Modify: `frontend/src/app/wiki/page.tsx`

**核心改动：** OperationQueue 不再需要——内容确认在页面内完成，执行结果在聊天里展示。移除相关引用。

- [ ] **Step 1: 删除文件**

```bash
cd /Users/irving/repo/learn-helper
rm frontend/src/components/OperationQueue.tsx
```

- [ ] **Step 2: 清理所有引用**

在 `PageViewer.tsx` 中移除：
- `OperationQueue` import
- `pendingPlans` props
- `executionResults` props
- 相关 handler props

在 `app/wiki/page.tsx` 中移除：
- `OperationQueue` 或 `pendingPlans` 相关状态
- 传递给 PageViewer 的已移除 props

- [ ] **Step 3: 编译验证**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
cd /Users/irving/repo/learn-helper
git add frontend/src/components/OperationQueue.tsx frontend/src/components/PageViewer.tsx frontend/src/app/wiki/page.tsx
git commit -m "refactor: remove OperationQueue, inline content confirm in PageViewer"
```

---

### Task 6: 校准追问支持（propose_plan 扩展）

**Files:**
- Modify: `backend/internal/ai/provider.go`

**核心改动：** 在 `propose_plan` 工具中新增 `calibration_question` 字段，让 AI 在写内容前可以先问校准问题。这是支持"先问再写"行为的基础设施。

当前 `propose_plan` 已经扩展了 `outline` / `phases` / `phase_index` / `total_phases`。只需要新增一个可选的 `calibration_question` 字段。

- [ ] **Step 1: 在 provider.go 的 WikiTools 中新增字段**

```go
"calibration_question": map[string]any{
    "type":        "object",
    "description": "可选。在写内容前，如果方向不确定，先提一个校准问题让用户决定方向",
    "properties": map[string]any{
        "question": map[string]any{
            "type":        "string",
            "description": "你的问题，如'关于变量声明，你是想和 Python 对比学，还是注重底层原理？'",
        },
        "options": map[string]any{
            "type":        "array",
            "items":       map[string]any{"type": "string"},
            "description": "选项列表，如 ['和 Python 对比', '底层原理', '实际踩坑']",
        },
    },
    "required": []string{"question"},
},
```

同时需要在 `BuildSystemPrompt` 中加入使用说明：

```
## 校准追问

写内容前如果方向不确定，使用 propose_plan 的 calibration_question 字段提问。
用户回答后，你再调用 propose_plan 写入内容。不要在同一个调用中同时提问和写内容。
```

- [ ] **Step 2: 编译验证**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: PASS

- [ ] **Step 3: 在 Typescript 类型中添加对应字段**

```typescript
// types/index.ts 中 Plan 类型新增
export interface CalibrationQuestion {
  question: string;
  options?: string[];
}

export interface Plan {
  // ... 现有字段
  calibration_question?: CalibrationQuestion;
}
```

- [ ] **Step 4: 在 ChatPanel 中处理 calibration_question**

当 AI 返回的 Plan 包含 `calibration_question`，在聊天中展示为一个选择题卡片，用户选择后继续：

```tsx
{plan.calibration_question && (
  <div className="bg-th-bg-tertiary rounded-lg p-3 my-2">
    <p className="text-sm text-th-text-primary mb-2">{plan.calibration_question.question}</p>
    {plan.calibration_question.options?.map((opt, i) => (
      <button
        key={i}
        onClick={() => handleCalibrationAnswer(plan.id, opt)}
        className="block w-full text-left px-3 py-1.5 text-sm rounded hover:bg-th-accent-bg hover:text-th-accent transition-colors"
      >
        {opt}
      </button>
    ))}
  </div>
)}
```

- [ ] **Step 5: 编译验证**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/ai/provider.go frontend/src/types/index.ts frontend/src/components/ChatPanel.tsx
git commit -m "feat: add calibration_question field to propose_plan for pre-write direction alignment"
```

---

### Task 7: ChatPanel 清理——移除待确认操作相关逻辑

**Files:**
- Modify: `frontend/src/components/ChatPanel.tsx`

**核心改动：** 移除与 Plan 确认相关的 UI——不再在聊天中显示确认/拒绝按钮（Plan 的 outline 阶段确认在右栏 PlanPreview 完成，内容阶段确认在页面内确认条完成）。聊天内的 inline actions、pending_actions 已清理。

- [ ] **Step 1: 审查 ChatPanel 中的 Plan 相关状态**

查找并移除：
- `pendingPlans` 相关逻辑
- `executionResults` 相关逻辑
- `confirming` 状态（不再是聊天的一部分）
- `onConfirm` / `onReject` 回调（不在聊天中处理）

保留：
- `plan` → PlanPreview 还是作为组件存在，但只在聊天中展示 outline 模式（PlanPreview 改为大纲组件后，仅在 AI 提议大纲时展示）
- 执行结果 → 简化，失败在聊天报错，成功不展示（内容已写入页面）

- [ ] **Step 2: 编译验证**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
cd /Users/irving/repo/learn-helper
git add frontend/src/components/ChatPanel.tsx
git commit -m "refactor: remove Plan confirm/reject UI from ChatPanel, inline in PageViewer"
```

---

### Task 8: 端到端测试

- [ ] **Step 1: 构建后端**

```bash
cd /Users/irving/repo/learn-helper/backend && go build ./cmd/server
```

- [ ] **Step 2: 构建前端**

```bash
cd /Users/irving/repo/learn-helper/frontend && npm run build
```

- [ ] **Step 3: 启动服务**

后端 on `:8080`，前端 on `:3000`。

- [ ] **Step 4: 验证关键流程**

1. 用户在知识树右键新建「Go」页面
2. 用户说"帮我生成学习大纲"，AI 展示大纲树在右栏
3. 用户调整结构：聊天说"把并发放到后面"
4. 确认后执行 outline，骨架页面创建成功
5. 用户学基础语法，"记下来"
6. AI 问校准问题，用户选择方向
7. AI 写入内容到「Go 基础语法」页面，显示确认条
8. 用户确认，内容固定
9. 用户拖拽节点重新排序
10. 用户说"这里改一下"，AI 更新内容

- [ ] **Step 5: 修复发现的问题**

## 自审

- [ ] 覆盖率检查：每个 spec 需求都有对应 Task
- [ ] 占位符扫描：没有 TBD/TODO
- [ ] 类型一致性：Props、API、数据库字段名跨 Task 一致
