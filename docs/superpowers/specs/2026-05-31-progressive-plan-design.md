# LLM Wiki 渐进式操作计划设计

## 动机

当前 `propose_plan` 系统将所有操作一次性平铺展示，用户只能整体确认或拒绝。此法在以下场景有不足：

1. **复杂知识体系建设** — 创建 10+ 页面时，一次性展示所有 actions，信息密度过高，用户难以细审
2. **内容填充节奏** — 大纲确认、骨架创建、内容填充三个阶段的节奏应不同，而非一刀切
3. **用户主动权** — 系统不应在 plan 执行后自动推进，应将主动权交还用户

## 设计目标

- AI 能灵活选择单页操作或渐进式多阶段计划
- 大纲（outline）作为计划的一部分，独立展示和确认
- plan 执行后停止自动推进，由用户指示下一步
- 所有改动尽量轻量，不引入新表、不重写引擎

## 核心改动概述

1. `propose_plan` 工具扩展：新增 `outline`、`phases`、`phase_index`、`total_phases` 字段
2. PlanPreview 扩展：新增 OutlineTree 组件，支持大纲/操作两种渲染模式
3. 交互流程扩展：确认大纲 → 引擎根据 outline 递归创建骨架页面
4. ReAct 循环：plan 执行后停止自动循环，回到用户消息等待状态
5. AI 判断场景：单页直接 actions，多页从大纲开始

## 数据模型变化

无需新表。`plans` 表增加 3 个可选字段：

```sql
ALTER TABLE plans ADD COLUMN outline TEXT;       -- JSON, 大纲树结构
ALTER TABLE plans ADD COLUMN phase_index INTEGER;  -- 当前阶段序号（0-based）
ALTER TABLE plans ADD COLUMN total_phases INTEGER; -- 总阶段数
```

现有 `reasoning`、`actions`、`status` 等字段不变。

## propose_plan 工具定义

### 新增字段

```go
// 新增可选字段
"outline": map[string]any{
    "type": "array",
    "description": "知识大纲树（可选）。展示为可折叠的树状结构，确认后自动创建骨架页面。适用于 3+ 页面以上的体系建设",
    "items": map[string]any{
        "type": "object",
        "properties": map[string]any{
            "id":        map[string]any{"type": "string", "description": "节点标识，可选，供后续 action 引用 page_id"},
            "title":     map[string]any{"type": "string", "description": "页面标题"},
            "page_type": map[string]any{"type": "string", "enum": []string{"entity", "concept", "overview"}, "description": "页面类型"},
            "children": map[string]any{
                "type": "array",
                "items": map[string]any{"$ref": "#"},
                "description": "子节点，递归结构",
            },
        },
    },
},
"phases": map[string]any{
    "type": "array",
    "description": "整体路线图（可选）。首次 propose_plan 时让用户了解全貌，后续可不传或保持不变。纯信息字段，不做系统级追踪",
    "items": map[string]any{
        "type": "object",
        "properties": map[string]any{
            "title":       map[string]any{"type": "string", "description": "阶段标题"},
            "description": map[string]any{"type": "string", "description": "阶段简述"},
        },
    },
},
"phase_index": map[string]any{
    "type": "integer",
    "description": "当前阶段序号，从 0 开始。首次调用传 0",
},
"total_phases": map[string]any{
    "type": "integer",
    "description": "总阶段数。首次调用时必填",
},
```

### system prompt 指导

在 prompt 中新增 2 条规则：

1. **单页 vs 多页判断**：1-2 个页面直接 propose_plan(actions=[...])；3+ 页面先用 outline 从大纲开始
2. **先给大纲，再写内容**：复杂知识体系先以 outline 形式展示整体结构，用户确认后自动创建骨架页面，后续再填充内容

### 运行时可替换的 plan

发起新的 propose_plan（有 outline）时，如果前一个 plan 还是 `pending` 状态且无 outline，则前端自动替换为新的 plan。使用户和 AI 可在聊天中迭代大纲，不必每次 reject。

## 交互流程

### 1. 单页操作（现有流程不变）

```
用户: "帮我创建一个 Redis 页面"

AI 调用 propose_plan(actions=[create_page], reasoning=...)
  → 右侧显示 actions 列表
  → 用户确认 → 执行 → 完成
```

### 2. 多页体系建设（新增渐进流程）

```
用户: "帮我建一个 Docker 知识体系"

AI 探索 →
  AI 调用 propose_plan(outline=[...标题和层级...], phases=[...3 阶段...])
    → 右侧显示大纲树（可折叠展开）
    → 底部按钮：仅[确认大纲]
  → 用户确认
  → 执行引擎递归遍历 outline，自动创建所有骨架页面（content_status=empty）
  → 执行结果返回 AI
  → AI 回复摘要："Docker 知识体系骨架已创建（3 个页面），告诉我填充哪个"
  → ReAct 循环结束，用户主导下一步
  → 用户: "填充 Docker 安装的内容"
  → AI 调用 propose_plan(actions=[update_page])
    → 右侧显示 actions 列表
    → 用户确认 → 执行 → 完成
```

## PlanPreview 渲染模式

PlanPreview 根据 propose_plan 的内容切换三种模式：

| propose_plan 中有什么 | 右侧展示 | 按钮 |
|---|---|---|
| 仅有 outline | 大纲树（递归、可折叠） | [确认大纲] |
| outline + actions | actions 列表（同现有） | [确认执行] [拒绝] |
| 仅有 actions | actions 列表（同现有） | [确认执行] [拒绝] |

### OutlineTree 组件

新增纯 UI 组件，递归渲染 outline 树：

- 每行显示 page_type 图标 + 标题
- 子树可折叠/展开（类似左侧知识树交互）
- 缩进表示层级
- 纯展示模式，无编辑能力

### Action 展示微调

- 多阶段时显示阶段进度（"阶段 1/3"）
- actions 列表仍为当前 phase 的所有 action，不分批

## 执行引擎

### outline 执行（新方法 `execOutline`）

```
确认大纲 Plan
  → handler 检测到无 actions、有 outline
  → 调用引擎 ExecOutline(ctx, outline, parentID?)
  → 递归遍历大纲树:
    → 每个节点调用 create_page(title, page_type, parent_id)
    → content 留空，content_status = "empty"
    → 递归处理 children（传入新建的 page_id 作为新的 parent_id）
  → 返回所有新 page_id 的映射
  → Plan 标记为 confirmed
  → 执行结果通过 tool_result 返回 AI
```

`execOutline` 不会修改现有数据库结构，完全复用已有的 `execCreatePage` 逻辑。引擎不需要感知 phase，只处理 actions 和 outline 两种模式。

## 改进依据

### 为何 phases 只是信息字段而非状态追踪

- **用户主导**：AI 不自动推进下一步，系统不需要追踪"当前到了第几阶段"
- **真实进度靠 content_status**：哪些页面还是 empty、哪些已有内容，比 phases 计数器更准确反映实际状态
- **phases 唯一用途**：第一次 propose_plan 时让用户看到整体路线图、形成预期

### 为何不添加持久化进度 UI

- **用户主导**原则意味着系统不应主动"提醒"用户还有未完成的工作
- 左侧知识树本身就是最好的待办清单（empty 页面一目了然）
- 添加 dashboard widget 会增加界面复杂度，且可能干扰用户正常的页面浏览

### 为何大纲替换旧 Plan 的策略对用户友好

- 用户和 AI 聊天迭代结构时，频繁 reject → 重来 的流程打断思考
- 自动替换 pending 的 outline plan 使用户可以在聊天中说"这里加一个子栏目"，AI 直接更新，不必手动确认取消旧 plan
- 一旦用户点击确认大纲，plan 锁定，不再接受替换

## 涉及的组件和改动文件

| 文件 | 改动 |
|---|---|
| `backend/internal/ai/provider.go` | propose_plan schema 扩展（outline, phases, phase_index, total_phases）|
| `backend/internal/ai/provider.go` | system prompt 新增单页/多页判断 + 大纲先行规则 |
| `backend/internal/handler/ai.go` | ReAct 循环：执行结果返回后结束循环，不自动继续 |
| `backend/internal/handler/plan.go` | ConfirmPlan 分流：根据有无 outline 决定走大纲执行还是正常执行 |
| `backend/internal/engine/engine.go` | 新增 ExecOutline 方法，递归创建骨架页面 |
| `frontend/src/components/PlanPreview.tsx` | 新增 OutlineTree 组件 + 三种模式切换逻辑 |
| `frontend/src/types/index.ts` | 类型扩展：outline, phases 等 |

## 不涉及改动的部分

- [x] 执行引擎的 topology sort 和 placeholder 替换（完全不变）
- [x] 现有 Plan 的操作流程和确认机制（单页场景不受影响）
- [x] 数据库表结构（只加列，不新建表）
- [x] SSE 流式传输和 provider 抽象
- [x] 前端项目管理状态（WikiPage 等现有组件）
