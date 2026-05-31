---
name: unified-operation-center-design
description: 统一用户直接操作和 AI 发起操作的确认流程，增强知识树交互和反馈可见性
---

# 统一操作中心设计

## 概述

当前 LLM Wiki 在用户交互体验上存在三个核心痛点：

1. **知识树交互过于被动** — 纯只读导航，无法右键、拖拽、内联编辑
2. **确认系统混乱** — 两套确认机制并存（pending_actions + propose_plan）
3. **反馈和状态可见性差** — 执行失败无错误提示、页面不自动刷新

本设计通过"统一操作中心"概念一次性解决这三个问题。

## 设计原则

### 操作确认策略：按风险等级区分

不管操作由谁发起（用户直接操作 vs AI 发起），按操作本身的风险决定是否需要确认：

| 操作 | 风险等级 | 确认策略 |
|------|----------|----------|
| 创建空页面 | 低 | 直接执行 |
| 重命名页面 | 低 | 直接执行 |
| 移动页面 | 低 | 直接执行 |
| 删除页面 | 高 | 需确认 |
| 写入页面内容 | 高 | 需确认 |
| 批量操作（Plan） | 高 | 需确认 |

**Why:** 用户信任自己的低风险操作，但高风险操作（删除、内容写入）需要二次确认以防止误操作。AI 发起的操作因为不可预测性，全部需要确认。

## Part 1：知识树直接操作

### 当前状态

- 知识树是纯只读导航组件
- 用户想创建/移动/重命名页面都必须通过聊天
- PRD 明确要求"结构操作应直接在树上完成"，但未实现

### 新设计

#### 1. Hover 菜单触发器

每个节点 hover 时显示 `⋯` 菜单按钮，点击弹出操作菜单。

#### 2. 右键菜单

```
低风险（直接执行）：
├── ➕ 添加子页面
├── ✏️ 重命名
└── ↗️ 移动到...

高风险（需确认）：
├── 🗑️ 删除页面
└── 🗑️ 删除子树
```

#### 3. 快速添加按钮

每个节点底部显示虚线"+ 添加子页面"按钮，点击直接创建空子页面（低风险，直接执行）。

#### 4. 拖拽移动

- 拖拽节点到新位置 → 直接执行（移动是低风险操作）
- 拖拽时显示放置指示线
- 支持两种放置模式：
  - 拖到节点上 → 成为该节点的子页面
  - 拖到节点之间 → 成为同级页面（调整排序）

#### 5. 内联重命名

- 双击节点标题 → 进入编辑模式
- 回车确认 / Esc 取消
- 重命名后自动更新所有 `[[旧标题]]` 引用（已有逻辑）

### 前端实现要点

- `KnowledgeTree.tsx` 增加 `onContextMenu`、`onDragStart`、`onDrop`、`onDoubleClick` 事件处理
- 新增 `TreeNodeMenu` 组件（右键菜单）
- 新增 `InlineEditInput` 组件（内联编辑）
- 使用 `@dnd-kit/core` 或原生拖拽 API 实现拖拽

### 后端实现要点

- 新增 `POST /api/wiki/move` API（如果不存在）
- 移动操作需要更新 `parent_id` 和 `path`（已有 `MoveWikiPage` 方法）
- 确保移动操作的原子性（事务包裹）

## Part 2：统一确认系统

### 当前问题

- 两套确认机制并存：
  - `pending_actions`：AI 写操作转为内嵌确认卡片，通过 `confirmed_actions` 回传
  - `propose_plan`：AI 调用工具创建 Plan，在右侧面板 PlanPreview 中确认
- 代码路径复杂，`ai.go` 中有大量 legacy 代码
- 用户在不同位置看到不同样式的确认 UI

### 新设计

#### 1. 删除 Legacy 代码

删除以下代码：
- `toolCallsToPendingActions()` 函数
- `confirmedFingerprints` 去重机制
- `confirmed_actions` 请求参数处理
- `ChatPanel` 中的 `handleConfirmAction` 和 `pending_actions` 状态

#### 2. 统一为 Plan + ExecutionEngine

所有需要确认的操作都走 Plan 系统：
- AI 发起的批量操作：继续使用 `propose_plan` 工具
- 用户发起的高风险单操作（如删除）：前端调用新 API `POST /api/plans` 创建单 action Plan

#### 3. 右侧面板 Tab 切换

```
┌─────────────────────────────┐
│ [页面内容] [待确认操作(2)]  │  ← Tab 切换
├─────────────────────────────┤
│ 操作卡片列表...             │
└─────────────────────────────┘
```

- 「待确认操作」Tab 显示红色计数徽章
- 每个操作卡片标注来源（用户操作 / AI 计划）
- 用户单操作：直接显示操作卡片 + 确认/取消按钮
- AI 批量操作：复用现有 PlanPreview 的 action 列表展示

#### 4. 新增 API

```
POST /api/plans
{
  "reasoning": "用户删除页面",
  "actions": [
    {
      "type": "delete_page",
      "params": { "page_id": 123 }
    }
  ]
}

Response:
{
  "plan_id": "plan-xxx",
  "status": "pending"
}
```

#### 5. 确认/拒绝 API 保留

```
POST /api/plans/{id}/confirm
POST /api/plans/{id}/reject
```

### 前端实现要点

- `PageViewer.tsx` 改为 Tab 布局，新增「待确认操作」Tab
- 新增 `OperationQueue` 组件，显示待确认操作列表
- 新增 `usePendingPlans` SWR hook，轮询或 SSE 获取待确认 Plan
- `KnowledgeTree` 删除操作调用 `POST /api/plans` 创建 Plan

### 后端实现要点

- `ai.go` 删除 `toolCallsToPendingActions` 相关代码
- `plan.go` 新增 `CreatePlan` handler 处理 `POST /api/plans`
- AI ReAct 循环中，所有写操作统一走 `propose_plan`

## Part 3：反馈和状态可见性

### 当前问题

- 计划执行失败时 `catch {}` 块为空，用户看不到错误
- Overview 页面异步更新无完成通知
- 确认后当前页面不自动刷新
- SSE `error` 事件被前端忽略
- Agent 进度条步数不匹配（前端 20 vs 后端 10）

### 新设计

#### 1. 执行结果卡片

确认后显示执行结果，三种状态：

| 状态 | 背景色 | 内容 |
|------|--------|------|
| 成功 | 绿色 | 操作摘要 + "查看新页面"链接 |
| 部分失败 | 黄色 | 成功/失败数量 + 失败原因 + "重试"链接 |
| 失败 | 红色 | 错误原因 + 替代方案链接 |

#### 2. 自动刷新

执行成功后：
- 刷新知识树（已有 `treeVersion` 机制）
- 刷新当前页面内容（当前缺失）
- 如果操作涉及当前查看的页面，自动切换到新页面/更新内容

#### 3. Overview 更新通知

- Overview 页面更新完成后，在聊天区显示轻量 Toast：「Overview 已更新」
- 后端 `updateOverviewPage()` 完成后通过 SSE 发送 `overview_updated` 事件

#### 4. SSE 错误处理

前端 `streamChat` 监听 `error` 事件：
```typescript
case 'error':
  setError(event.data);
  setLoading(false);
  break;
```

#### 5. Agent 进度修正

前端 `agentStatus.maxSteps` 从 20 改为 10，与后端 `maxIterations = 10` 一致。

### 前端实现要点

- `PlanPreview.tsx` 增加 `executionResult` 状态，显示执行结果卡片
- `ChatPanel.tsx` 增加 SSE `error` 事件处理
- `WikiPage.tsx` 确认后刷新当前页面
- 新增 Toast 组件用于 Overview 更新通知

### 后端实现要点

- `updateOverviewPage()` 完成后通过 SSE 发送 `overview_updated` 事件
- `ExecutionEngine` 返回详细的错误信息（已有 `ExecutionReport`）

## 实现顺序

1. **Phase 1：统一确认系统**（清理 legacy，降低复杂度）
   - 删除 `pending_actions` 相关代码
   - 新增 `POST /api/plans` API
   - 右侧面板 Tab 化

2. **Phase 2：反馈和状态可见性**
   - 执行结果卡片
   - 自动刷新
   - SSE 错误处理
   - Agent 进度修正

3. **Phase 3：知识树直接操作**
   - 右键菜单
   - 快速添加按钮
   - 拖拽移动
   - 内联重命名

**Why this order:** 先清理 legacy 降低代码复杂度，再改善反馈让用户能看到操作结果，最后增加直接操作功能。这样每个阶段都能独立交付价值。

## 文件变更预估

### 前端

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `KnowledgeTree.tsx` | 大改 | 增加右键菜单、拖拽、内联编辑 |
| `PageViewer.tsx` | 大改 | Tab 布局、操作队列 |
| `PlanPreview.tsx` | 中改 | 执行结果展示 |
| `ChatPanel.tsx` | 中改 | 删除 pending_actions、SSE 错误处理 |
| `api.ts` | 小改 | 新增 createPlan API |
| `types/index.ts` | 小改 | 类型更新 |
| 新增 `TreeNodeMenu.tsx` | 新增 | 右键菜单组件 |
| 新增 `OperationQueue.tsx` | 新增 | 操作队列组件 |
| 新增 `Toast.tsx` | 新增 | Toast 通知组件 |

### 后端

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `handler/ai.go` | 大改 | 删除 pending_actions 逻辑 |
| `handler/plan.go` | 中改 | 新增 CreatePlan handler |
| `engine/engine.go` | 小改 | 错误信息增强 |
| `model/models.go` | 无 | Plan 模型已存在 |

## 风险和缓解

1. **拖拽移动的原子性**
   - 风险：MoveWikiPage 的 path 迁移不是事务性的
   - 缓解：用事务包裹整个移动操作

2. **Plan 创建频率**
   - 风险：用户频繁删除操作会创建大量单 action Plan
   - 缓解：执行结果 Plan 可以定期清理（保留最近 50 条）

3. **学习曲线**
   - 风险：用户习惯了通过聊天操作，可能不知道可以直接在树上操作
   - 缓解：首次使用时显示引导提示

## 成功指标

- 用户直接操作占比 > 50%（vs 通过聊天操作）
- 操作确认到执行的平均时间 < 2 秒
- 操作失败时用户能看到错误信息的比例 = 100%
- 用户满意度调研中"操作体验"评分提升
