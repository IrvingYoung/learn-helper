# Learn Helper - 软件工程师学习助手设计文档

## 概述

Learn Helper 是一个面向软件工程师的系统性学习 Web 应用，以面试准备为导向，围绕知识体系组织学习内容，AI 辅助理解和解题。第一期聚焦数据结构与算法。

## 定位

不是刷题器，而是学习助手。知识点体系化学习为核心，练习题是巩固手段。学习流程：知识点学习 → AI 互动理解 → 练习巩固 → 学习进度追踪。

## 架构

方案 A：轻量单体架构

```
┌──────────────────────────────────────────────────┐
│                Next.js Frontend                   │
│  (知识图谱 | AI对话 | 练习题 | 学习进度 | 设置)    │
└──────────────────┬───────────────────────────────┘
                   │ REST API + SSE
┌──────────────────▼───────────────────────────────┐
│              Go Backend (单体)                    │
│                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │
│  │ 知识模块 │ │ AI模块   │ │ 练习模块         │ │
│  │ (体系/树) │ │ (多模型) │ │ (题目/提交/统计) │ │
│  └────┬─────┘ └────┬─────┘ └────────┬─────────┘ │
│       │            │               │            │
│  ┌────▼────────────▼───────────────▼──────────┐ │
│  │            SQLite Database                 │ │
│  └────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘
```

单用户自用，SQLite 零运维，Go 单体后端处理所有逻辑。

## 核心模块

### 1. 知识模块

以树状结构组织知识点。每个知识点包含概念讲解、关键要点、常见考点。

示例层级：数据结构 → 树 → 二叉树 → BST → AVL树

### 2. AI 模块

Provider 抽象层，支持多模型切换。MVP 包含两种角色：

| 角色 | 触发场景 | 核心行为 |
|------|---------|---------|
| 知识讲解 | 用户浏览知识点时提问 | 解释概念、举例说明、回答追问，不直接给题解 |
| 解题辅导 | 用户做练习题时求助 | 给提示而非答案、分析思路、优化已有解法 |

> 学习规划角色（分析薄弱点、推荐学习路径）为 P1，需积累足够学习数据后才有分析价值，第二期补充。

对话上下文管理：
- 每次对话关联到具体知识点或练习题
- 自动注入当前知识点/题目的上下文到 system prompt
- 保留完整对话历史，支持续聊
- 单次对话 token 上限控制，超限时摘要压缩早期消息

### 3. 练习模块

题目关联到具体知识点，追踪每题状态和掌握程度，支持按知识点/难度/状态筛选。

## 数据模型

### 知识点 (topics)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| parent_id | INTEGER FK | 父知识点，NULL 为根节点 |
| name | TEXT | 知识点名称 |
| slug | TEXT UNIQUE | URL 友好标识 |
| description | TEXT | 概念描述 |
| key_points | JSON | 关键要点列表 |
| difficulty | TEXT | beginner/intermediate/advanced |
| sort_order | INTEGER | 同级排序 |

### 练习题 (exercises)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| topic_id | INTEGER FK | 关联知识点 |
| type | TEXT | algorithm/system_design/knowledge |
| title | TEXT | 题目标题 |
| description | TEXT | 题目描述 |
| difficulty | TEXT | easy/medium/hard |
| tags | JSON | 标签列表 |
| hints | JSON | 提示列表（逐步展开） |
| solution_outline | TEXT | 解题思路概要 |
| time_complexity_expected | TEXT | 期望时间复杂度（算法题） |
| space_complexity_expected | TEXT | 期望空间复杂度（算法题） |
| sample_code | JSON | 示例代码（按语言） |

### 学习记录 (learning_records)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| topic_id | INTEGER FK | 关联知识点 |
| exercise_id | INTEGER FK | 关联练习题（可空） |
| status | TEXT | not_started/in_progress/completed |
| mastery_level | INTEGER | 掌握程度 1-5 |
| notes | TEXT | 个人笔记 |
| last_reviewed_at | DATETIME | 最后复习时间 |
| review_count | INTEGER | 复习次数 |

### AI 对话 (conversations)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| topic_id | INTEGER FK | 关联知识点（可空） |
| exercise_id | INTEGER FK | 关联练习题（可空） |
| role | TEXT | tutor/interviewer/planner |
| title | TEXT | 对话标题 |
| created_at | DATETIME | 创建时间 |

### 对话消息 (messages)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| conversation_id | INTEGER FK | 关联对话 |
| role | TEXT | user/assistant |
| content | TEXT | 消息内容 |
| model_provider | TEXT | 使用的 AI 模型 |
| token_count | INTEGER | token 消耗 |
| created_at | DATETIME | 创建时间 |

### AI 配置 (ai_configs)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| provider | TEXT | claude/openai/... |
| model_name | TEXT | 模型名称 |
| api_key | TEXT | API 密钥（加密存储） |
| is_active | BOOLEAN | 是否为当前活跃配置 |
| config | JSON | 额外配置参数 |

## AI Provider 抽象

```go
type AIProvider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

type ChatRequest struct {
    Messages     []Message
    SystemPrompt string
    Model        string
    MaxTokens    int
}
```

实现 ClaudeProvider 和 OpenAIProvider，通过配置切换活跃 provider。统一流式响应接口，前端用 SSE 接收。

## API 端点

### 知识模块
- `GET /api/topics` — 获取知识点树
- `GET /api/topics/:slug` — 获取知识点详情
- `POST /api/topics` — 创建知识点
- `PUT /api/topics/:id` — 更新知识点

### 练习模块
- `GET /api/exercises` — 列出练习题（支持筛选）
- `GET /api/exercises/:id` — 获取练习题详情
- `POST /api/exercises` — 创建练习题
- `PUT /api/exercises/:id` — 更新练习题

### 学习记录
- `GET /api/learning-records` — 获取学习记录
- `POST /api/learning-records` — 创建/更新学习记录
- `GET /api/learning-records/stats` — 获取学习统计

### AI 模块
- `POST /api/ai/chat` — 发送消息，返回流式响应 (SSE)
- `GET /api/ai/conversations` — 列出对话
- `GET /api/ai/conversations/:id` — 获取对话详情
- `PUT /api/ai/configs/:id` — 更新 AI 配置
- `GET /api/ai/configs` — 获取 AI 配置列表

## 前端页面

### 知识图谱页 (`/learn`)
- 左侧：知识点树状导航（可展开/折叠）
- 右侧：选中知识点的详情（概念、要点、关联练习）
- 顶部：学习进度概览

### 知识点详情页 (`/learn/:topicSlug`)
- 知识点内容展示
- "向 AI 提问"按钮 → 打开侧边对话面板
- 关联练习题列表
- 学习状态标记

### 练习页 (`/practice`)
- 按知识点/难度/状态筛选
- 题目卡片展示

### 练习详情页 (`/practice/:id`)
- 题目描述、提示（逐步展开）
- AI 辅导对话面板
- 标记完成 + 自评掌握程度

### 仪表盘 (`/dashboard`)
- 学习统计和趋势
- 基本掌握程度展示

> 薄弱点分析 + AI 推荐和间隔重复复习提醒为 P1，第二期补充。

### 设置页 (`/settings`)
- AI 模型配置
- 主题切换

### 通用交互
- AI 对话以侧边面板形式出现，不离开当前页面
- 流式输出，打字机效果
- 对话历史持久化

## 技术栈

| 层 | 选型 | 理由 |
|---|------|------|
| 前端框架 | Next.js 15 (App Router) | SSR/SSG、React 生态、TypeScript |
| UI 组件 | Tailwind CSS + shadcn/ui | 定制灵活、无样式冲突 |
| 状态管理 | React Context + SWR | 单用户应用，不需要复杂状态管理 |
| 后端 | Go 1.22+ + Chi router | 轻量、高性能 |
| 数据库访问 | sqlc | 类型安全、Go 代码生成 |
| 数据库 | SQLite (go-sqlite3) | 单用户零运维 |
| AI 接口 | Go provider 抽象 + 各 SDK | Claude 用 anthropic-sdk-go，OpenAI 用 go-openai |
| 前后端通信 | REST + SSE | 简单直接 |

## 项目结构

```
learn-helper/
├── frontend/                  # Next.js 应用
│   ├── src/
│   │   ├── app/              # App Router 页面
│   │   │   ├── learn/        # 知识图谱
│   │   │   ├── practice/     # 练习
│   │   │   ├── dashboard/    # 仪表盘
│   │   │   └── settings/     # 设置
│   │   ├── components/       # UI 组件
│   │   ├── lib/              # API 客户端、工具函数
│   │   └── types/            # TypeScript 类型
│   └── package.json
├── backend/                   # Go 服务
│   ├── cmd/server/           # 入口
│   ├── internal/
│   │   ├── handler/          # HTTP handler
│   │   ├── service/          # 业务逻辑
│   │   ├── repository/       # 数据访问
│   │   ├── model/            # 数据模型
│   │   └── ai/              # AI provider 抽象与实现
│   ├── db/                   # 迁移脚本 + 种子数据
│   └── go.mod
└── docs/                     # 文档
```

## 部署

- 开发：Next.js dev server (3000) + Go server (8080)，前端 proxy 转发 API
- 部署：Next.js 构建为静态站或部署 Vercel，Go 服务单独部署，SQLite 持久化

## 第一期范围（MVP）

- 数据结构与算法知识体系
- 内置题库（算法题为主，少量系统设计和八股文）
- AI 两角色（知识讲解、解题辅导）
- 基本学习进度追踪和统计
- 单用户，无登录系统

### 暂不包含（第二期补充）

- AI 学习规划角色（需积累学习数据后才有分析价值）
- 间隔重复复习提醒
- 仪表盘薄弱点分析 + AI 推荐
- 系统设计 / 八股文题库深度建设
