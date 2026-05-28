# Learn Helper 项目骨架搭建

**Goal:** 搭建 Monorepo 项目骨架——Go 后端（Chi + sqlc + SQLite）+ Next.js 15 前端（App Router + Tailwind + shadcn/ui），跑通前后端联调和 API 请求闭环。

**Architecture:** 根目录 Monorepo，backend/ Go 服务负责全部业务逻辑和数据持久化，前端通过 REST API + SSE 与之通信。sqlc 根据 SQL schema 自动生成 Go 类型安全的数据访问层。前端开发时用 Vite proxy 转发 API 请求到 Go 服务。

**Tech Stack:** Go 1.25 + Chi router + sqlc + go-sqlite3, Next.js 15 + Tailwind CSS + shadcn/ui + SWR, Claude Provider (MVP)

---

## 文件结构

```
learn-helper/
├── frontend/                    # Next.js 15 (App Router)
│   ├── src/
│   │   ├── app/                # App Router 页面
│   │   ├── components/          # UI 组件
│   │   ├── lib/                # API 客户端、工具函数
│   │   └── types/              # TypeScript 类型
│   ├── package.json
│   └── ...
├── backend/                     # Go 服务
│   ├── cmd/server/             # 入口 main.go
│   ├── internal/
│   │   ├── handler/            # HTTP handler（路由注册、请求处理）
│   │   ├── service/            # 业务逻辑
│   │   ├── repository/         # 数据访问（sqlc 生成的代码）
│   │   ├── model/              # 数据模型（sqlc 生成）
│   │   └── ai/                 # AI Provider 抽象 + Claude 实现
│   ├── db/
│   │   ├── migrations/         # SQLite schema 迁移脚本
│   │   └── seed/               # 种子数据（知识点 + 练习题示例）
│   ├── sqlc.yaml              # sqlc 配置
│   └── go.mod
├── package.json                # Monorepo workspace 根配置
├── pnpm-workspace.yaml        # pnpm workspace 配置
├── CLAUDE.md
└── README.md
```

---

## Task 1: 初始化 Monorepo 结构

**Files:**
- Create: `package.json`, `pnpm-workspace.yaml`, `README.md`, `CLAUDE.md`

- [ ] **Step 1: 创建根目录 Monorepo 配置文件**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > package.json <<'EOF'
{
  "name": "learn-helper",
  "version": "0.1.0",
  "private": true,
  "workspaces": [
    "frontend",
    "backend"
  ],
  "scripts": {
    "dev": "pnpm --parallel -r dev",
    "build": "pnpm --parallel -r build",
    "db:generate": "cd backend && sqlc generate"
  },
  "engines": {
    "node": ">=20.0.0",
    "pnpm": ">=9.0.0"
  }
}
EOF
`

- [ ] **Step 2: 创建 pnpm workspace 配置**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > pnpm-workspace.yaml <<'EOF'
packages:
  - 'frontend'
  - 'backend'
EOF
`

- [ ] **Step 3: 创建 README.md**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > README.md <<'EOF'
# Learn Helper

面向软件工程师的面试学习助手。知识体系 + AI 辅导 + 练习巩固。

## Quick Start

```bash
pnpm install
pnpm db:generate    # 生成 Go 数据访问层代码
pnpm dev            # 启动前端 (3000) + 后端 (8080)
```

## 技术栈

- Frontend: Next.js 15 + Tailwind CSS + shadcn/ui
- Backend: Go 1.25 + Chi + sqlc + SQLite
- AI: Claude Provider

## 项目结构

见 [CLAUDE.md](CLAUDE.md)
EOF
`

- [ ] **Step 4: 创建 CLAUDE.md**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > CLAUDE.md <<'EOF'
# CLAUDE.md

## 项目概述

Learn Helper 是一个面向软件工程师的面试学习助手，核心功能：知识图谱学习 + AI 辅导 + 练习题库 + 学习进度追踪。

MVP 聚焦：数据结构与算法，AI 仅支持知识讲解和解题辅导两角色。

## 技术栈

- Frontend: Next.js 15 (App Router) + Tailwind CSS + shadcn/ui + SWR + TypeScript
- Backend: Go 1.25 + Chi router + sqlc + go-sqlite3
- AI: Claude Provider (MVP)，抽象层预留扩展
- DB: SQLite（单用户零运维）
- 通信: REST API + SSE

## 项目结构

```
learn-helper/
├── frontend/           # Next.js 应用
├── backend/           # Go 服务
│   ├── cmd/server/    # 入口
│   ├── internal/
│   │   ├── handler/  # HTTP handler
│   │   ├── service/  # 业务逻辑
│   │   ├── repository/ # sqlc 生成
│   │   ├── model/     # sqlc 生成
│   │   └── ai/       # AI Provider
│   └── db/
│       ├── migrations/ # schema
│       └── seed/      # 种子数据
```

## 开发命令

```bash
pnpm install
pnpm db:generate   # sqlc generate
pnpm dev           # 前后端同时启动
```

## 关键约定

- sqlc 配置: backend/sqlc.yaml，生成的代码在 backend/internal/repository/ 和 backend/internal/model/
- 前端 API 调用: 走 Vite proxy (frontend/vite.config.ts) 转发到 localhost:8080
- AI Provider 抽象: backend/internal/ai/provider.go 定义 interface，claude.go 实现
- 数据库路径: backend/learn-helper.db（.gitignore 忽略）
EOF
`

- [ ] **Step 5: Commit**

```bash
cd C:/Users/owen6/repo/learn-helper
git add package.json pnpm-workspace.yaml README.md CLAUDE.md
git commit -m "chore: init monorepo structure"
```

---

## Task 2: 初始化 Go 后端项目

**Files:**
- Create: `backend/go.mod`, `backend/cmd/server/main.go`, `backend/sqlc.yaml`
- Create: `backend/db/migrations/001_init_schema.sql`

- [ ] **Step 1: 初始化 Go Module**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && go mod init learn-helper`

- [ ] **Step 2: 安装 Go 依赖**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && go get github.com/go-chi/chi/v5@v5.1.0 github.com/mattn/go-sqlite3@v1.14.25 github.com/sqlc-dev/sqlc@v1.31.1`

- [ ] **Step 3: 创建目录结构**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && mkdir -p cmd/server internal/handler internal/service internal/repository internal/model internal/ai db/migrations db/seed`

- [ ] **Step 4: 创建 sqlc.yaml 配置**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > sqlc.yaml <<'EOF'
version: "2"
sql:
  - engine: "sqlite"
    queries: "db/migrations/queries.sql"
    schema: "db/migrations/schema.sql"
    gen:
      go:
        package: "model"
        out: "internal/model"
        sql_package: "database.sql"
EOF
`

- [ ] **Step 5: 创建 SQLite schema**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > db/migrations/schema.sql <<'EOF'
-- Learn Helper Database Schema
-- SQLite

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER REFERENCES topics(id),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    key_points TEXT,  -- JSON array
    difficulty TEXT DEFAULT 'beginner' CHECK(difficulty IN ('beginner', 'intermediate', 'advanced')),
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER NOT NULL REFERENCES topics(id),
    type TEXT DEFAULT 'algorithm' CHECK(type IN ('algorithm', 'system_design', 'knowledge')),
    title TEXT NOT NULL,
    description TEXT,
    difficulty TEXT DEFAULT 'medium' CHECK(difficulty IN ('easy', 'medium', 'hard')),
    tags TEXT,  -- JSON array
    hints TEXT,  -- JSON array, stepwise hints
    solution_outline TEXT,
    time_complexity_expected TEXT,
    space_complexity_expected TEXT,
    sample_code TEXT,  -- JSON by language
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS learning_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    status TEXT DEFAULT 'not_started' CHECK(status IN ('not_started', 'in_progress', 'completed')),
    mastery_level INTEGER CHECK(mastery_level >= 1 AND mastery_level <= 5),
    notes TEXT,
    last_reviewed_at DATETIME,
    review_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    context_type TEXT CHECK(context_type IN ('topic', 'exercise', 'dashboard')),
    title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    model_provider TEXT,
    token_count INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    model_name TEXT NOT NULL,
    api_key TEXT NOT NULL,
    is_active INTEGER DEFAULT 0,
    config TEXT,  -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics(parent_id);
CREATE INDEX IF NOT EXISTS idx_topics_slug ON topics(slug);
CREATE INDEX IF NOT EXISTS idx_exercises_topic ON exercises(topic_id);
CREATE INDEX IF NOT EXISTS idx_learning_records_topic ON learning_records(topic_id);
CREATE INDEX IF NOT EXISTS idx_learning_records_exercise ON learning_records(exercise_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
EOF
`

- [ ] **Step 6: 创建 queries.sql（供 sqlc 生成代码）**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > db/migrations/queries.sql <<'EOF'
-- name: GetAllTopics :many
SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, created_at, updated_at FROM topics ORDER BY sort_order, id;

-- name: GetTopicByID :one
SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, created_at, updated_at FROM topics WHERE id = ?;

-- name: GetTopicBySlug :one
SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, created_at, updated_at FROM topics WHERE slug = ?;

-- name: GetTopicTree :many
SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order, created_at, updated_at FROM topics ORDER BY parent_id NULLS FIRST, sort_order, id;

-- name: GetAllExercises :many
SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected, sample_code, created_at, updated_at FROM exercises;

-- name: GetExerciseByID :one
SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected, sample_code, created_at, updated_at FROM exercises WHERE id = ?;

-- name: GetExercisesByTopicID :many
SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected, sample_code, created_at, updated_at FROM exercises WHERE topic_id = ?;

-- name: GetLearningRecords :many
SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count, created_at, updated_at FROM learning_records;

-- name: GetLearningRecordByTopic :one
SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count, created_at, updated_at FROM learning_records WHERE topic_id = ?;

-- name: GetLearningRecordByExercise :one
SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count, created_at, updated_at FROM learning_records WHERE exercise_id = ?;

-- name: UpsertLearningRecord :exec
INSERT INTO learning_records (topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    topic_id = excluded.topic_id,
    exercise_id = excluded.exercise_id,
    status = excluded.status,
    mastery_level = excluded.mastery_level,
    notes = excluded.notes,
    last_reviewed_at = excluded.last_reviewed_at,
    review_count = excluded.review_count,
    updated_at = CURRENT_TIMESTAMP;

-- name: GetAllConversations :many
SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC;

-- name: GetConversationByID :one
SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations WHERE id = ?;

-- name: CreateConversation :one
INSERT INTO conversations (topic_id, exercise_id, context_type, title) VALUES (?, ?, ?, ?) RETURNING id, topic_id, exercise_id, context_type, title, created_at, updated_at;

-- name: GetMessagesByConversation :many
SELECT id, conversation_id, role, content, model_provider, token_count, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at;

-- name: CreateMessage :one
INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, ?, ?, ?, ?) RETURNING id, conversation_id, role, content, model_provider, token_count, created_at;

-- name: GetActiveAIConfig :one
SELECT id, provider, model_name, api_key, is_active, config, created_at, updated_at FROM ai_configs WHERE is_active = 1;

-- name: UpsertAIConfig :exec
INSERT INTO ai_configs (provider, model_name, api_key, is_active, config)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    provider = excluded.provider,
    model_name = excluded.model_name,
    api_key = excluded.api_key,
    is_active = excluded.is_active,
    config = excluded.config,
    updated_at = CURRENT_TIMESTAMP;
EOF
`

- [ ] **Step 7: 运行 sqlc generate 生成代码**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && /c/Users/owen6/go/bin/sqlc.exe generate`

- [ ] **Step 8: 创建 main.go**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > cmd/server/main.go <<'EOF'
package main

import (
	"database/sql"
	"log"
	"net/http"

	"learn-helper/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./learn-helper.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	h := handler.NewHandler(db)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.HealthCheck)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/topics", h.GetTopics)
		r.Get("/topics/{slug}", h.GetTopicBySlug)
		r.Get("/topics/{slug}/exercises", h.GetExercisesByTopic)

		r.Get("/exercises", h.GetExercises)
		r.Get("/exercises/{id}", h.GetExerciseByID)

		r.Get("/learning-records", h.GetLearningRecords)
		r.Post("/learning-records", h.UpsertLearningRecord)

		r.Route("/ai", func(r chi.Router) {
			r.Post("/chat", h.AIChat)
			r.Get("/conversations", h.GetConversations)
			r.Get("/conversations/{id}", h.GetConversation)
			r.Get("/conversations/{id}/messages", h.GetMessages)
			r.Get("/configs", h.GetAIConfigs)
			r.Post("/configs", h.UpsertAIConfig)
		})
	})

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
EOF
`

- [ ] **Step 9: 创建 handler 目录基础文件**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/handler/handler.go <<'EOF'
package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
EOF
`

- [ ] **Step 10: 添加缺失的 handler 方法占位符（编译用）**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/handler/topics.go <<'EOF'
package handler

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) GetTopics(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order FROM topics ORDER BY sort_order, id`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var topics []map[string]interface{}
	for rows.Next() {
		var id, parentID, sortOrder int
		var name, slug, description, keyPoints, difficulty string
		if err := rows.Scan(&id, &parentID, &name, &slug, &description, &keyPoints, &difficulty, &sortOrder); err != nil {
			continue
		}
		topics = append(topics, map[string]interface{}{
			"id":          id,
			"parent_id":   parentID,
			"name":        name,
			"slug":        slug,
			"description": description,
			"key_points":  keyPoints,
			"difficulty":  difficulty,
			"sort_order":  sortOrder,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"topics": topics})
}

func (h *Handler) GetTopicBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id, parentID, sortOrder int
	var name, slugOut, description, keyPoints, difficulty string
	err := h.db.QueryRow(`SELECT id, parent_id, name, slug, description, key_points, difficulty, sort_order FROM topics WHERE slug = ?`, slug).
		Scan(&id, &parentID, &name, &slugOut, &description, &keyPoints, &difficulty, &sortOrder)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          id,
		"parent_id":   parentID,
		"name":        name,
		"slug":        slugOut,
		"description": description,
		"key_points":  keyPoints,
		"difficulty":  difficulty,
		"sort_order":  sortOrder,
	})
}

func (h *Handler) GetExercisesByTopic(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var topicID int
	if err := h.db.QueryRow(`SELECT id FROM topics WHERE slug = ?`, slug).Scan(&topicID); err != nil {
		http.NotFound(w, r)
		return
	}

	rows, err := h.db.Query(`SELECT id, topic_id, type, title, description, difficulty, tags, hints FROM exercises WHERE topic_id = ?`, topicID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var exercises []map[string]interface{}
	for rows.Next() {
		var id, topicID int
		var typ, title, description, difficulty, tags, hints string
		if err := rows.Scan(&id, &topicID, &typ, &title, &description, &difficulty, &tags, &hints); err != nil {
			continue
		}
		exercises = append(exercises, map[string]interface{}{
			"id":          id,
			"topic_id":    topicID,
			"type":        typ,
			"title":       title,
			"description": description,
			"difficulty":  difficulty,
			"tags":        tags,
			"hints":       hints,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"exercises": exercises})
}
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/handler/exercises.go <<'EOF'
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (h *Handler) GetExercises(w http.ResponseWriter, r *http.Request) {
	topicID := r.URL.Query().Get("topic_id")
	difficulty := r.URL.Query().Get("difficulty")
	status := r.URL.Query().Get("status")

	query := `SELECT e.id, e.topic_id, e.type, e.title, e.description, e.difficulty, e.tags, e.hints,
		COALESCE(lr.status, 'not_started') as status, COALESCE(lr.mastery_level, 0) as mastery_level
		FROM exercises e
		LEFT JOIN learning_records lr ON e.id = lr.exercise_id
		WHERE 1=1`
	args := []interface{}{}
	if topicID != "" {
		query += " AND e.topic_id = ?"
		args = append(args, topicID)
	}
	if difficulty != "" && difficulty != "全部" {
		query += " AND e.difficulty = ?"
		args = append(args, difficulty)
	}

	rows, err := h.db.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var exercises []map[string]interface{}
	for rows.Next() {
		var id, topicID int
		var typ, title, description, difficulty, tags, hints, statusVal string
		var masteryLevel int
		if err := rows.Scan(&id, &topicID, &typ, &title, &description, &difficulty, &tags, &hints, &statusVal, &masteryLevel); err != nil {
			continue
		}
		exercises = append(exercises, map[string]interface{}{
			"id":            id,
			"topic_id":      topicID,
			"type":          typ,
			"title":         title,
			"description":   description,
			"difficulty":    difficulty,
			"tags":          tags,
			"hints":         hints,
			"status":        statusVal,
			"mastery_level": masteryLevel,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"exercises": exercises})
}

func (h *Handler) GetExerciseByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	var exerciseID, topicID int
	var typ, title, description, difficulty, tags, hints, solutionOutline, timeComplexity, spaceComplexity, sampleCode string
	err = h.db.QueryRow(`SELECT id, topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected, sample_code
		FROM exercises WHERE id = ?`, id).
		Scan(&exerciseID, &topicID, &typ, &title, &description, &difficulty, &tags, &hints, &solutionOutline, &timeComplexity, &spaceComplexity, &sampleCode)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                       exerciseID,
		"topic_id":                 topicID,
		"type":                     typ,
		"title":                    title,
		"description":              description,
		"difficulty":               difficulty,
		"tags":                     tags,
		"hints":                    hints,
		"solution_outline":         solutionOutline,
		"time_complexity_expected": timeComplexity,
		"space_complexity_expected": spaceComplexity,
		"sample_code":              sampleCode,
	})
}
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/handler/learning.go <<'EOF'
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type LearningRecordInput struct {
	TopicID     *int    `json:"topic_id"`
	ExerciseID  *int    `json:"exercise_id"`
	Status      string  `json:"status"`
	MasteryLevel *int   `json:"mastery_level"`
	Notes       *string `json:"notes"`
}

func (h *Handler) GetLearningRecords(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count FROM learning_records`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id, topicID, exerciseID, masteryLevel, reviewCount int
		var status, notes string
		var lastReviewedAt *time.Time
		if err := rows.Scan(&id, &topicID, &exerciseID, &status, &masteryLevel, &notes, &lastReviewedAt, &reviewCount); err != nil {
			continue
		}
		records = append(records, map[string]interface{}{
			"id":              id,
			"topic_id":        topicID,
			"exercise_id":     exerciseID,
			"status":          status,
			"mastery_level":   masteryLevel,
			"notes":           notes,
			"last_reviewed_at": lastReviewedAt,
			"review_count":    reviewCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"records": records})
}

func (h *Handler) UpsertLearningRecord(w http.ResponseWriter, r *http.Request) {
	var input LearningRecordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", 400)
		return
	}

	topicID := 0
	exerciseID := 0
	if input.TopicID != nil {
		topicID = *input.TopicID
	}
	if input.ExerciseID != nil {
		exerciseID = *input.ExerciseID
	}

	status := "not_started"
	if input.Status != "" {
		status = input.Status
	}

	notes := ""
	if input.Notes != nil {
		notes = *input.Notes
	}

	_, err := h.db.Exec(`INSERT INTO learning_records (topic_id, exercise_id, status, mastery_level, notes, last_reviewed_at, review_count)
		VALUES (?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(topic_id, exercise_id) DO UPDATE SET
		status = excluded.status,
		mastery_level = COALESCE(excluded.mastery_level, mastery_level),
		notes = excluded.notes,
		last_reviewed_at = CURRENT_TIMESTAMP,
		review_count = review_count + 1`,
		topicID, exerciseID, status, input.MasteryLevel, notes)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return i
}
EOF
`

- [ ] **Step 11: Commit**

```bash
cd C:/Users/owen6/repo/learn-helper
git add backend/go.mod backend/cmd/server/main.go backend/internal/handler/ backend/db/migrations/ backend/sqlc.yaml
git commit -m "feat: init Go backend with schema and basic handlers"
```

---

## Task 3: AI 模块骨架（Provider 抽象 + Claude 实现）

**Files:**
- Create: `backend/internal/ai/provider.go`, `backend/internal/ai/claude.go`, `backend/internal/ai/models.go`
- Modify: `backend/internal/handler/ai.go`

- [ ] **Step 1: 创建 AI Provider 接口和模型**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/ai/models.go <<'EOF'
package ai

import "time"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages     []Message
	SystemPrompt string
	Model        string
	MaxTokens    int
}

type ChatChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

type ChatResponse struct {
	Content    string `json:"content"`
	TokenCount int    `json:"token_count"`
}
EOF
`

- [ ] **Step 2: 创建 AI Provider 接口**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/ai/provider.go <<'EOF'
package ai

import (
	"context"
	"io"
)

// AIProvider 定义 AI 模型提供商的统一接口
type AIProvider interface {
	// Chat 发送消息并获取完整响应
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// StreamChat 发送消息并返回流式响应（SSE chunk）
	StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// Role 常量
const (
	RoleKnowledgeExplain = "knowledge_explain" // 知识讲解角色
	RoleProblemSolving    = "problem_solving"  // 解题辅导角色
)

// SystemPromptTemplates 提供各角色的 system prompt 模板
var SystemPromptTemplates = map[string]string{
	RoleKnowledgeExplain: `你是一位数据结构与算法专家，专注于帮助软件工程师准备技术面试。
当用户浏览知识点并向你提问时，你应当：
- 清晰解释概念，举例说明
- 关联已学知识，帮助建立知识体系
- 引导思考，不直接给题解
- 如果用户问到具体题目，引导他们先思考解法

当前知识点上下文：
{{.Context}}

回答要求：
- 语言简洁易懂，适合面试准备
- 可以用代码示例，但代码只是辅助说明
- 鼓励用户思考和追问`,

	RoleProblemSolving: `你是一位耐心的面试教练，专注于帮助用户通过引导式提示解出算法题。
你必须遵守以下规则：
- 永远不要直接给出完整答案或完整代码
- 每次只给出一个提示，帮助用户朝正确方向思考
- 提示应当是引导性的，例如"考虑从...角度"或"这个问题的本质是..."
- 如果用户卡住了，先给出思路级别的提示，再逐步给出更具体的提示
- 当用户接近正确答案时，给予鼓励，让他们自己完成

题目上下文：
{{.Context}}

解题过程：
1. 先让用户描述思路
2. 根据用户的思路给予针对性提示
3. 每次提示后等待用户反馈
4. 不要一次性给出多个提示`,

	RoleDashboard: `你是一位学习规划专家，根据用户的学习数据分析薄弱点并推荐下一步学习路径。
当前学习数据：
{{.Context}}

你需要：
- 识别掌握程度较低的知识点
- 分析学习趋势和模式
- 推荐下一步应该重点学习的知识点
- 给出具体的复习计划建议`,
}

// RoleDisplayNames 角色展示名称
var RoleDisplayNames = map[string]string{
	RoleKnowledgeExplain: "知识讲解",
	RoleProblemSolving:    "解题辅导",
	RoleDashboard:         "学习规划",
}
EOF
`

- [ ] **Step 3: 创建 Claude Provider 实现**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/ai/claude.go <<'EOF'
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ClaudeConfig Claude Provider 配置
type ClaudeConfig struct {
	APIKey string
	Model  string
}

// claudeMessage 对应 Claude API 的消息格式
type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model         string           `json:"model"`
	MaxTokens     int              `json:"max_tokens"`
	SystemPrompt string           `json:"system,omitempty"`
	Messages     []claudeMessage `json:"messages"`
	Stream       bool             `json:"stream"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type claudeStreamResponse struct {
	Type string `json:"type"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ClaudeProvider Claude API 实现
type ClaudeProvider struct {
	apiKey string
	model  string
}

func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	if model == "" {
		model = "claude-sonnet-4-7-20250514"
	}
	return &ClaudeProvider{apiKey: apiKey, model: model}
}

func (p *ClaudeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]claudeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, claudeMessage{Role: m.Role, Content: m.Content})
	}

	claudeReq := claudeRequest{
		Model:         model,
		MaxTokens:     req.MaxTokens,
		SystemPrompt:  req.SystemPrompt,
		Messages:      messages,
		Stream:        false,
	}

	jsonData, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Claude API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Claude API error: %s", string(body))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	content := ""
	for _, c := range claudeResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &ChatResponse{
		Content:    content,
		TokenCount: claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
	}, nil
}

func (p *ClaudeProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]claudeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, claudeMessage{Role: m.Role, Content: m.Content})
	}

	claudeReq := claudeRequest{
		Model:         model,
		MaxTokens:     req.MaxTokens,
		SystemPrompt:  req.SystemPrompt,
		Messages:      messages,
		Stream:        true,
	}

	jsonData, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Claude API: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("Claude API error: %s", string(body))
	}

	ch := make(chan ChatChunk, 100)
	go p.streamResponse(resp.Body, ch)
	return ch, nil
}

func (p *ClaudeProvider) streamResponse(body io.Reader, ch chan<- ChatChunk) {
	defer close(ch)
	decoder := json.NewDecoder(body)

	for decoder.More() {
		var event claudeStreamResponse
		if err := decoder.Decode(&event); err != nil {
			return
		}

		if event.Type == "content_block_delta" {
			ch <- ChatChunk{Content: event.Delta.Text, Done: false}
		} else if event.Type == "message_stop" {
			ch <- ChatChunk{Done: true}
			return
		}
	}
}

func (p *ClaudeProvider) GetModel() string {
	return p.model
}
EOF
`

- [ ] **Step 4: 创建 AI Handler**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > internal/handler/ai.go <<'EOF'
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"learn-helper/internal/ai"

	"github.com/go-chi/chi/v5"
)

type AIHandler struct {
	db        *sql.DB
	providers map[string]ai.AIProvider
}

func NewAIHandler(db *sql.DB) *AIHandler {
	return &AIHandler{
		db:        db,
		providers: make(map[string]ai.AIProvider),
	}
}

// GetAIConfig 获取活跃的 AI 配置
func (h *AIHandler) getActiveConfig() (*ai.ChatRequest, error) {
	var provider, modelName, apiKey string
	err := h.db.QueryRow(`SELECT provider, model_name, api_key FROM ai_configs WHERE is_active = 1 LIMIT 1`).
		Scan(&provider, &modelName, &apiKey)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no AI config found, please configure in settings")
	}
	if err != nil {
		return nil, err
	}

	if h.providers[provider] == nil {
		if provider == "claude" {
			h.providers[provider] = ai.NewClaudeProvider(apiKey, modelName)
		}
	}

	req := &ai.ChatRequest{
		Model:     modelName,
		MaxTokens: 4096,
	}
	return req, nil
}

// GetConversations 获取对话列表
func (h *AIHandler) GetConversations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var conversations []map[string]interface{}
	for rows.Next() {
		var id int
		var topicID, exerciseID int
		var contextType, title string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &topicID, &exerciseID, &contextType, &title, &createdAt, &updatedAt); err != nil {
			continue
		}
		conversations = append(conversations, map[string]interface{}{
			"id":           id,
			"topic_id":     topicID,
			"exercise_id":  exerciseID,
			"context_type": contextType,
			"title":        title,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"conversations": conversations})
}

// GetConversation 获取单个对话
func (h *AIHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	var conversationID, topicID, exerciseID int
	var contextType, title string
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(`SELECT id, topic_id, exercise_id, context_type, title, created_at, updated_at FROM conversations WHERE id = ?`, id).
		Scan(&conversationID, &topicID, &exerciseID, &contextType, &title, &createdAt, &updatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           conversationID,
		"topic_id":     topicID,
		"exercise_id":  exerciseID,
		"context_type": contextType,
		"title":        title,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	})
}

// GetMessages 获取对话消息
func (h *AIHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	rows, err := h.db.Query(`SELECT id, conversation_id, role, content, model_provider, token_count, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at`, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var messages []map[string]interface{}
	for rows.Next() {
		var id, conversationID int
		var role, content, modelProvider string
		var tokenCount *int
		var createdAt time.Time
		if err := rows.Scan(&id, &conversationID, &role, &content, &modelProvider, &tokenCount, &createdAt); err != nil {
			continue
		}
		messages = append(messages, map[string]interface{}{
			"id":              id,
			"conversation_id": conversationID,
			"role":            role,
			"content":         content,
			"model_provider":  modelProvider,
			"token_count":     tokenCount,
			"created_at":      createdAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"messages": messages})
}

// AIChat 处理 AI 对话请求（SSE 流式）
func (h *AIHandler) AIChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID *int    `json:"conversation_id"`
		TopicID        *int    `json:"topic_id"`
		ExerciseID     *int    `json:"exercise_id"`
		ContextType    string  `json:"context_type"`
		Role           string  `json:"role"`
		Message        string  `json:"message"`
		Context        string  `json:"context"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}

	// 获取 AI 配置
	aiReq, err := h.getActiveConfig()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// 获取或创建 conversation
	conversationID := req.ConversationID
	if conversationID == nil {
		title := "新对话"
		if req.ContextType == "topic" {
			title = "知识讲解对话"
		} else if req.ContextType == "exercise" {
			title = "解题辅导对话"
		}

		topicID := 0
		exerciseID := 0
		if req.TopicID != nil {
			topicID = *req.TopicID
		}
		if req.ExerciseID != nil {
			exerciseID = *req.ExerciseID
		}

		var newID int64
		h.db.QueryRow(`INSERT INTO conversations (topic_id, exercise_id, context_type, title) VALUES (?, ?, ?, ?)`, topicID, exerciseID, req.ContextType, title).Scan(&newID)
		id := int(newID)
		conversationID = &id
	}

	// 获取 system prompt
	systemPrompt := ai.SystemPromptTemplates[req.Role]
	if req.Context != "" {
		systemPrompt = strings.Replace(systemPrompt, "{{.Context}}", req.Context, 1)
	}

	// 构建消息列表
	messages := []ai.Message{
		{Role: "user", Content: req.Message},
	}

	// 获取对话历史（最近的 10 条）
	rows, _ := h.db.Query(`SELECT role, content FROM messages WHERE conversation_id = ? ORDER BY created_at DESC LIMIT 10`, *conversationID)
	var history []ai.Message
	for rows.Next() {
		var role, content string
		rows.Scan(&role, &content)
		history = append(history, ai.Message{Role: role, Content: content})
	}
	rows.Close()
	// 反转以获得正确顺序
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}
	messages = append(history, messages...)

	aiReq.Messages = messages
	aiReq.SystemPrompt = systemPrompt

	// 流式响应（SSE）
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	provider := h.providers["claude"]
	if provider == nil {
		http.Error(w, "AI provider not configured", 500)
		return
	}

	ch, err := provider.StreamChat(context.Background(), *aiReq)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	fullContent := ""
	for chunk := range ch {
		if !chunk.Done {
			fullContent += chunk.Content
			fmt.Fprintf(w, "data: %s\n\n", chunk.Content)
			flusher.Flush()
		}
	}

	// 保存消息到数据库
	providerName := "claude"
	h.db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', ?, ?, 0)`,
		*conversationID, req.Message, providerName)
	h.db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', ?, ?, 0)`,
		*conversationID, fullContent, providerName)
	h.db.Exec(`UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, *conversationID)

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// GetAIConfigs 获取 AI 配置列表
func (h *AIHandler) GetAIConfigs(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, provider, model_name, is_active, config FROM ai_configs`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var configs []map[string]interface{}
	for rows.Next() {
		var id int
		var provider, modelName string
		var isActive bool
		var config *string
		if err := rows.Scan(&id, &provider, &modelName, &isActive, &config); err != nil {
			continue
		}
		configs = append(configs, map[string]interface{}{
			"id":          id,
			"provider":    provider,
			"model_name":  modelName,
			"is_active":  isActive,
			"config":      config,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"configs": configs})
}

// UpsertAIConfig 创建/更新 AI 配置
func (h *AIHandler) UpsertAIConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider  string `json:"provider"`
		ModelName string `json:"model_name"`
		APIKey    string `json:"api_key"`
		IsActive  bool   `json:"is_active"`
		Config    string `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid input", 400)
		return
	}

	if input.IsActive {
		h.db.Exec(`UPDATE ai_configs SET is_active = 0`)
	}

	_, err := h.db.Exec(`INSERT INTO ai_configs (provider, model_name, api_key, is_active, config)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		provider = excluded.provider,
		model_name = excluded.model_name,
		api_key = excluded.api_key,
		is_active = excluded.is_active,
		config = excluded.config`,
		input.Provider, input.ModelName, input.APIKey, input.IsActive, input.Config)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
EOF
`

- [ ] **Step 5: 更新 main.go 引入 AI Handler**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && cat > cmd/server/main.go <<'EOF'
package main

import (
	"database/sql"
	"log"
	"net/http"

	"learn-helper/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./learn-helper.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	h := handler.NewHandler(db)
	aiHandler := handler.NewAIHandler(db)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.HealthCheck)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/topics", h.GetTopics)
		r.Get("/topics/{slug}", h.GetTopicBySlug)
		r.Get("/topics/{slug}/exercises", h.GetExercisesByTopic)

		r.Get("/exercises", h.GetExercises)
		r.Get("/exercises/{id}", h.GetExerciseByID)

		r.Get("/learning-records", h.GetLearningRecords)
		r.Post("/learning-records", h.UpsertLearningRecord)

		r.Route("/ai", func(r chi.Router) {
			r.Post("/chat", aiHandler.AIChat)
			r.Get("/conversations", aiHandler.GetConversations)
			r.Get("/conversations/{id}", aiHandler.GetConversation)
			r.Get("/conversations/{id}/messages", aiHandler.GetMessages)
			r.Get("/configs", aiHandler.GetAIConfigs)
			r.Post("/configs", aiHandler.UpsertAIConfig)
		})
	})

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
EOF
`

- [ ] **Step 6: 更新 go.mod 依赖**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && go get github.com/go-chi/chi/v5 github.com/mattn/go-sqlite3 github.com/google/uuid`

- [ ] **Step 7: 验证编译**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && go build ./...`

- [ ] **Step 8: Commit**

```bash
cd C:/Users/owen6/repo/learn-helper
git add backend/internal/ai/ backend/internal/handler/ai.go backend/cmd/server/main.go backend/go.mod
git commit -m "feat: add AI provider abstraction and Claude implementation"
```

---

## Task 4: 初始化 Next.js 前端项目

**Files:**
- Create: `frontend/package.json`, `frontend/src/app/`, `frontend/src/components/`, `frontend/vite.config.ts`, etc.

- [ ] **Step 1: 创建 frontend package.json**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/package.json <<'EOF'
{
  "name": "learn-helper-frontend",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "lint": "eslint ."
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-router-dom": "^7.0.0",
    "swr": "^2.2.5",
    "clsx": "^2.1.1",
    "tailwind-merge": "^2.5.4"
  },
  "devDependencies": {
    "@eslint/js": "^9.0.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^4.3.4",
    "autoprefixer": "^10.4.20",
    "eslint": "^9.0.0",
    "globals": "^15.0.0",
    "postcss": "^8.4.49",
    "tailwindcss": "^3.4.17",
    "typescript": "~5.7.0",
    "typescript-eslint": "^8.0.0",
    "vite": "^6.0.0"
  }
}
EOF
`

- [ ] **Step 2: 创建 Vite 配置（带 API proxy）**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/vite.config.ts <<'EOF'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
EOF
`

- [ ] **Step 3: 创建 TypeScript / Tailwind / PostCSS 基础配置**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/tsconfig.json <<'EOF'
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src"]
}
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/tailwind.config.js <<'EOF'
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {},
  },
  plugins: [],
}
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/postcss.config.js <<'EOF'
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
EOF
`

- [ ] **Step 4: 创建入口 HTML 和基础文件**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/index.html <<'EOF'
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Learn Helper</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper && mkdir -p frontend/src && cat > frontend/src/index.css <<'EOF'
@tailwind base;
@tailwind components;
@tailwind utilities;
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/src/main.tsx <<'EOF'
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
EOF
`

- [ ] **Step 5: 创建 App.tsx 和路由骨架**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/src/App.tsx <<'EOF'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import LearnPage from './app/learn/page'
import PracticePage from './app/practice/page'
import DashboardPage from './app/dashboard/page'
import SettingsPage from './app/settings/page'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/learn" replace />} />
          <Route path="learn" element={<LearnPage />} />
          <Route path="learn/:slug" element={<LearnPage />} />
          <Route path="practice" element={<PracticePage />} />
          <Route path="practice/:id" element={<PracticePage />} />
          <Route path="dashboard" element={<DashboardPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
EOF
`

- [ ] **Step 6: 创建 Layout 组件**

Run: `cd C:/Users/owen6/repo/learn-helper && mkdir -p frontend/src/components frontend/src/app/learn frontend/src/app/practice frontend/src/app/dashboard frontend/src/app/settings frontend/src/lib frontend/src/types && cat > frontend/src/components/Layout.tsx <<'EOF'
import { Outlet, NavLink, useLocation } from 'react-router-dom'
import { useState } from 'react'
import AIChatPanel from './AIChatPanel'

const navItems = [
  { path: '/learn', label: '知识图谱', icon: '📚' },
  { path: '/practice', label: '练习题库', icon: '💻' },
  { path: '/dashboard', label: '学习仪表盘', icon: '📊' },
  { path: '/settings', label: '设置', icon: '⚙️' },
]

export default function Layout() {
  const [aiPanelOpen, setAIPanelOpen] = useState(false)
  const location = useLocation()

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Top Navigation */}
      <header className="bg-white border-b border-gray-200 sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-14">
            <div className="flex items-center gap-6">
              <h1 className="text-lg font-semibold text-gray-900">Learn Helper</h1>
              <nav className="flex gap-1">
                {navItems.map((item) => (
                  <NavLink
                    key={item.path}
                    to={item.path}
                    className={({ isActive }) =>
                      `px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                        isActive
                          ? 'bg-blue-50 text-blue-700'
                          : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                      }`
                    }
                  >
                    <span className="mr-1">{item.icon}</span>
                    {item.label}
                  </NavLink>
                ))}
              </nav>
            </div>
            <button
              onClick={() => setAIPanelOpen(!aiPanelOpen)}
              className="px-3 py-1.5 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 transition-colors"
            >
              {aiPanelOpen ? '关闭 AI' : '打开 AI'}
            </button>
          </div>
        </div>
      </header>

      {/* Main Content + AI Panel */}
      <div className="flex">
        <main className={`flex-1 transition-all ${aiPanelOpen ? 'mr-96' : ''}`}>
          <Outlet />
        </main>
        {aiPanelOpen && (
          <AIChatPanel onClose={() => setAIPanelOpen(false)} />
        )}
      </div>
    </div>
  )
}
EOF
`

- [ ] **Step 7: 创建 AI 对话侧边面板**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/src/components/AIChatPanel.tsx <<'EOF'
import { useState, useRef, useEffect } from 'react'

type Role = 'knowledge_explain' | 'problem_solving' | 'dashboard'

interface Message {
  role: 'user' | 'assistant'
  content: string
}

interface AIChatPanelProps {
  onClose: () => void
  initialContext?: {
    type: Role
    topicSlug?: string
    exerciseId?: number
    context?: string
  }
}

const roleLabels: Record<Role, string> = {
  knowledge_explain: '知识讲解',
  problem_solving: '解题辅导',
  dashboard: '学习规划',
}

export default function AIChatPanel({ onClose, initialContext }: AIChatPanelProps) {
  const [role, setRole] = useState<Role>(initialContext?.type || 'knowledge_explain')
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const sendMessage = async () => {
    if (!input.trim() || loading) return

    const userMessage = input.trim()
    setInput('')
    setMessages((prev) => [...prev, { role: 'user', content: userMessage }])
    setLoading(true)

    try {
      const resp = await fetch('/api/ai/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          role,
          message: userMessage,
          context_type: role === 'knowledge_explain' ? 'topic' : role === 'problem_solving' ? 'exercise' : 'dashboard',
          context: initialContext?.context || '',
          topic_id: null,
          exercise_id: null,
        }),
      })

      if (!resp.ok) throw new Error('API error')

      const reader = resp.body?.getReader()
      const decoder = new TextDecoder()
      let fullContent = ''

      setMessages((prev) => [...prev, { role: 'assistant', content: '' }])

      while (reader) {
        const { done, value } = await reader.read()
        if (done) break

        const chunk = decoder.decode(value)
        const lines = chunk.split('\n')
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data === '[DONE]') continue
            fullContent += data
            setMessages((prev) => {
              const updated = [...prev]
              updated[updated.length - 1] = { role: 'assistant', content: fullContent }
              return updated
            })
          }
        }
      }
    } catch (err) {
      setMessages((prev) => [
        ...prev,
        { role: 'assistant', content: '抱歉，AI 暂时不可用，请稍后重试。' },
      ])
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed right-0 top-14 bottom-0 w-96 bg-white border-l border-gray-200 flex flex-col z-50">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
        <div className="flex items-center gap-2">
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as Role)}
            className="text-sm border border-gray-300 rounded px-2 py-1"
          >
            <option value="knowledge_explain">知识讲解</option>
            <option value="problem_solving">解题辅导</option>
          </select>
        </div>
        <button onClick={onClose} className="text-gray-500 hover:text-gray-700">
          ✕
        </button>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 0 && (
          <div className="text-center text-gray-500 mt-8 text-sm">
            <p>选择角色后输入问题开始对话</p>
            <p className="mt-2 text-xs">
              {role === 'knowledge_explain'
                ? 'AI 会帮你解释概念、举例说明'
                : 'AI 会引导你思考，不直接给答案'}
            </p>
          </div>
        )}
        {messages.map((msg, i) => (
          <div
            key={i}
            className={`rounded-lg p-3 ${
              msg.role === 'user'
                ? 'bg-blue-100 ml-8'
                : 'bg-gray-100 mr-8'
            }`}
          >
            <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
          </div>
        ))}
        {loading && (
          <div className="bg-gray-100 mr-8 rounded-lg p-3">
            <p className="text-sm text-gray-500">思考中...</p>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="p-4 border-t border-gray-200">
        <div className="flex gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
            placeholder="输入你的问题..."
            className="flex-1 border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            onClick={sendMessage}
            disabled={!input.trim() || loading}
            className="px-4 py-2 bg-blue-600 text-white rounded-md text-sm hover:bg-blue-700 disabled:opacity-50"
          >
            发送
          </button>
        </div>
      </div>
    </div>
  )
}
EOF
`

- [ ] **Step 8: 创建四个页面骨架**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/src/app/learn/page.tsx <<'EOF'
import { useParams } from 'react-router-dom'
import useSWR from 'swr'
import { useState } from 'react'

interface Topic {
  id: number
  parent_id: number
  name: string
  slug: string
  description: string
  key_points: string
  difficulty: string
  sort_order: number
}

const fetcher = (url: string) => fetch(url).then((r) => r.json())

// 知识点树组件
function TopicTree({ topics, onSelect }: { topics: Topic[]; onSelect: (t: Topic) => void }) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  // 构建树
  const roots = topics.filter((t) => !t.parent_id)
  const getChildren = (pid: number) => topics.filter((t) => t.parent_id === pid)

  const toggleExpand = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const renderNode = (topic: Topic, depth: number = 0) => {
    const children = getChildren(topic.id)
    const hasChildren = children.length > 0
    const isExpanded = expanded.has(topic.id)

    return (
      <div key={topic.id}>
        <div
          className="flex items-center gap-2 px-3 py-2 hover:bg-gray-100 rounded-md cursor-pointer"
          style={{ paddingLeft: `${depth * 20 + 12}px` }}
          onClick={() => onSelect(topic)}
        >
          {hasChildren && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                toggleExpand(topic.id)
              }}
              className="text-gray-400 hover:text-gray-600"
            >
              {isExpanded ? '▼' : '▶'}
            </button>
          )}
          <span className="text-sm text-gray-700">{topic.name}</span>
          <span className={`text-xs px-1.5 py-0.5 rounded ${
            topic.difficulty === 'advanced' ? 'bg-red-100 text-red-700' :
            topic.difficulty === 'intermediate' ? 'bg-yellow-100 text-yellow-700' :
            'bg-green-100 text-green-700'
          }`}>
            {topic.difficulty}
          </span>
        </div>
        {isExpanded && children.map((c) => renderNode(c, depth + 1))}
      </div>
    )
  }

  return <div>{roots.map((r) => renderNode(r))}</div>
}

export default function LearnPage() {
  const { slug } = useParams()
  const { data } = useSWR<{ topics: Topic[] }>('/api/topics', fetcher)
  const { data: topicData } = useSWR(slug ? `/api/topics/${slug}` : null, fetcher)

  const [selectedTopic, setSelectedTopic] = useState<Topic | null>(null)

  const topics = data?.topics || []

  return (
    <div className="flex h-[calc(100vh-3.5rem)]">
      {/* Left: Topic Tree */}
      <div className="w-72 border-r border-gray-200 bg-white overflow-y-auto p-4">
        <h2 className="text-sm font-semibold text-gray-700 mb-4">知识图谱</h2>
        {topics.length === 0 ? (
          <p className="text-sm text-gray-400">暂无知识点</p>
        ) : (
          <TopicTree topics={topics} onSelect={setSelectedTopic} />
        )}
      </div>

      {/* Right: Topic Detail */}
      <div className="flex-1 overflow-y-auto p-8">
        {!selectedTopic && !topicData ? (
          <div className="text-center text-gray-400 mt-20">
            <p className="text-lg">选择左侧知识点开始学习</p>
          </div>
        ) : (
          <div className="max-w-3xl mx-auto">
            {selectedTopic && (
              <>
                <div className="flex items-center gap-3 mb-6">
                  <h1 className="text-2xl font-bold text-gray-900">{selectedTopic.name}</h1>
                  <span className={`text-sm px-2 py-1 rounded ${
                    selectedTopic.difficulty === 'advanced' ? 'bg-red-100 text-red-700' :
                    selectedTopic.difficulty === 'intermediate' ? 'bg-yellow-100 text-yellow-700' :
                    'bg-green-100 text-green-700'
                  }`}>
                    {selectedTopic.difficulty}
                  </span>
                </div>
                <div className="prose max-w-none">
                  <p className="text-gray-600 mb-6">{selectedTopic.description}</p>
                  {selectedTopic.key_points && (
                    <div className="bg-gray-50 rounded-lg p-4">
                      <h3 className="font-semibold text-gray-800 mb-2">关键要点</h3>
                      <ul className="list-disc list-inside text-gray-600 space-y-1">
                        {(() => {
                          try {
                            const points = JSON.parse(selectedTopic.key_points)
                            return Array.isArray(points) ? points.map((p: string, i: number) => (
                              <li key={i}>{p}</li>
                            )) : null
                          } catch {
                            return <li>{selectedTopic.key_points}</li>
                          }
                        })()}
                      </ul>
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/src/app/practice/page.tsx <<'EOF'
import { useSearchParams } from 'react-router-dom'
import useSWR from 'swr'

const fetcher = (url: string) => fetch(url).then((r) => r.json())

interface Exercise {
  id: number
  topic_id: number
  type: string
  title: string
  description: string
  difficulty: string
  tags: string
  hints: string
  status: string
  mastery_level: number
}

export default function PracticePage() {
  const [params] = useSearchParams()
  const id = params.get('id')
  const { data } = useSWR<{ exercises: Exercise[] }>(
    id ? `/api/exercises/${id}` : '/api/exercises',
    fetcher
  )

  const exercise = data

  if (id) {
    return (
      <div className="p-8">
        <div className="max-w-3xl mx-auto">
          {exercise ? (
            <>
              <h1 className="text-2xl font-bold mb-4">{exercise.title}</h1>
              <div className="mb-4 flex gap-2">
                <span className={`text-sm px-2 py-1 rounded ${
                  exercise.difficulty === 'easy' ? 'bg-green-100 text-green-700' :
                  exercise.difficulty === 'medium' ? 'bg-yellow-100 text-yellow-700' :
                  'bg-red-100 text-red-700'
                }`}>
                  {exercise.difficulty}
                </span>
                <span className="text-sm px-2 py-1 rounded bg-blue-100 text-blue-700">
                  {exercise.type}
                </span>
              </div>
              <div className="prose">
                <p>{exercise.description}</p>
              </div>
            </>
          ) : (
            <p className="text-gray-400">加载中...</p>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-bold mb-6">练习题库</h1>
        {exercise?.exercises ? (
          <div className="grid gap-4">
            {exercise.exercises.map((e) => (
              <div key={e.id} className="bg-white rounded-lg border border-gray-200 p-4">
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="font-medium text-gray-900">{e.title}</h3>
                    <div className="flex gap-2 mt-2">
                      <span className={`text-xs px-2 py-1 rounded ${
                        e.difficulty === 'easy' ? 'bg-green-100 text-green-700' :
                        e.difficulty === 'medium' ? 'bg-yellow-100 text-yellow-700' :
                        'bg-red-100 text-red-700'
                      }`}>
                        {e.difficulty}
                      </span>
                      <span className="text-xs text-gray-500">{e.type}</span>
                    </div>
                  </div>
                  <span className={`text-xs px-2 py-1 rounded ${
                    e.status === 'completed' ? 'bg-green-100 text-green-700' :
                    e.status === 'in_progress' ? 'bg-yellow-100 text-yellow-700' :
                    'bg-gray-100 text-gray-500'
                  }`}>
                    {e.status === 'not_started' ? '未开始' : e.status === 'in_progress' ? '进行中' : '已完成'}
                  </span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-gray-400">加载中...</p>
        )}
      </div>
    </div>
  )
}
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/src/app/dashboard/page.tsx <<'EOF'
import useSWR from 'swr'

const fetcher = (url: string) => fetch(url).then((r) => r.json())

export default function DashboardPage() {
  const { data: stats } = useSWR('/api/learning-records', fetcher)

  return (
    <div className="p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-bold mb-6">学习仪表盘</h1>
        <div className="grid grid-cols-3 gap-4 mb-8">
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <p className="text-gray-500 text-sm">学习知识点</p>
            <p className="text-3xl font-bold text-blue-600">{stats?.records?.length || 0}</p>
          </div>
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <p className="text-gray-500 text-sm">掌握程度</p>
            <p className="text-3xl font-bold text-green-600">--</p>
          </div>
          <div className="bg-white rounded-lg border border-gray-200 p-6">
            <p className="text-gray-500 text-sm">薄弱点</p>
            <p className="text-3xl font-bold text-red-600">--</p>
          </div>
        </div>
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <h2 className="text-lg font-semibold mb-4">学习记录</h2>
          {stats?.records?.length ? (
            <ul className="space-y-2">
              {stats.records.map((r: any) => (
                <li key={r.id} className="flex justify-between text-sm">
                  <span>Topic #{r.topic_id}</span>
                  <span className={`px-2 py-0.5 rounded ${
                    r.status === 'completed' ? 'bg-green-100 text-green-700' :
                    r.status === 'in_progress' ? 'bg-yellow-100 text-yellow-700' :
                    'bg-gray-100 text-gray-500'
                  }`}>
                    {r.status}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-gray-400">暂无学习记录</p>
          )}
        </div>
      </div>
    </div>
  )
}
EOF
`

Run: `cd C:/Users/owen6/repo/learn-helper && cat > frontend/src/app/settings/page.tsx <<'EOF'
import { useState } from 'react'

export default function SettingsPage() {
  const [provider, setProvider] = useState('claude')
  const [model, setModel] = useState('claude-sonnet-4-7-20250514')
  const [apiKey, setApiKey] = useState('')
  const [saved, setSaved] = useState(false)

  const handleSave = async () => {
    const resp = await fetch('/api/ai/configs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        provider,
        model_name: model,
        api_key: apiKey,
        is_active: true,
      }),
    })
    if (resp.ok) {
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    }
  }

  return (
    <div className="p-8">
      <div className="max-w-xl mx-auto">
        <h1 className="text-2xl font-bold mb-6">设置</h1>
        <div className="bg-white rounded-lg border border-gray-200 p-6">
          <h2 className="text-lg font-semibold mb-4">AI 模型配置</h2>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Provider</label>
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                className="w-full border border-gray-300 rounded-md px-3 py-2"
              >
                <option value="claude">Claude</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Model</label>
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="w-full border border-gray-300 rounded-md px-3 py-2"
              >
                <option value="claude-sonnet-4-7-20250514">Claude Sonnet 4.7</option>
                <option value="claude-opus-4-7-20250514">Claude Opus 4.7</option>
                <option value="claude-haiku-4-5-20250501">Claude Haiku 4.5</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="sk-ant-..."
                className="w-full border border-gray-300 rounded-md px-3 py-2"
              />
            </div>
            <button
              onClick={handleSave}
              className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
            >
              保存配置
            </button>
            {saved && <span className="ml-3 text-green-600 text-sm">配置已保存</span>}
          </div>
        </div>
      </div>
    </div>
  )
}
EOF
`

- [ ] **Step 9: 安装依赖并验证前端构建**

Run: `cd C:/Users/owen6/repo/learn-helper/frontend && npx pnpm install`

- [ ] **Step 10: Commit**

```bash
cd C:/Users/owen6/repo/learn-helper
git add frontend/
git commit -m "feat: init Next.js frontend skeleton with routing and AI chat panel"
```

---

## Task 5: 种子数据（可选，先跑通骨架）

**Files:**
- Create: `backend/db/seed/seed.sql`

- [ ] **Step 1: 创建种子数据 SQL**

Run: `cd C:/Users/owen6/repo/learn-helper && cat > backend/db/seed/seed.sql <<'EOF'
-- Seed data for Learn Helper MVP
-- Topics: Data Structures and Algorithms

-- 根节点
INSERT INTO topics (id, parent_id, name, slug, description, key_points, difficulty, sort_order) VALUES
(1, NULL, '数据结构与算法', 'dsa', '软件工程师面试的核心考核内容', '["数组", "链表", "栈和队列", "树", "图", "排序算法", "搜索算法", "动态规划"]', 'beginner', 0);

-- 数据结构子节点
INSERT INTO topics (id, parent_id, name, slug, description, key_points, difficulty, sort_order) VALUES
(2, 1, '基础数据结构', 'data-structures', '最常用的线性数据结构', '["数组", "链表", "栈", "队列"]', 'beginner', 1),
(3, 2, '数组', 'array', '最基本的数据结构，内存连续分布，支持 O(1) 随机访问', '["数组索引", "二维数组", "前缀和", "双指针"]', 'beginner', 10),
(4, 2, '链表', 'linked-list', '节点通过指针连接，适合插入删除', '["单链表", "双链表", "反转链表", "环形链表检测"]', 'beginner', 11),
(5, 2, '栈', 'stack', 'LIFO，后进先出', '["单调栈", "括号匹配", "表达式求值"]', 'beginner', 12),
(6, 2, '队列', 'queue', 'FIFO，先进先出', '["普通队列", "双端队列", "BFS 广度优先"]', 'beginner', 13),
(7, 3, '二分查找', 'binary-search', '有序数组的高效搜索', '["左闭右闭", "左闭右开", "搜索边界"]', 'intermediate', 20),
(8, 1, '树结构', 'trees', '层级数据组织结构', '["二叉树", "BST", "平衡树", "线段树"]', 'intermediate', 2),
(9, 8, '二叉树', 'binary-tree', '每个节点最多两个子节点的树结构', '["前中后序遍历", "层序遍历", "深度计算", "递归与迭代"]', 'intermediate', 30),
(10, 8, 'BST', 'bst', '二叉搜索树，左小右大', '["插入", "删除", "搜索", "验证 BST"]', 'intermediate', 31),
(11, 1, '算法思想', 'algorithms', '解决问题的核心范式', '["排序算法", "搜索算法", "动态规划", "贪心算法"]', 'intermediate', 3),
(12, 11, '排序算法', 'sorting', '将数据按特定顺序排列', '["快速排序", "归并排序", "堆排序", "计数排序"]', 'intermediate', 40),
(13, 11, '动态规划', 'dynamic-programming', '最优子结构 + 状态转移', '["一维 DP", "二维 DP", "背包问题", "LIS"]', 'advanced', 41);

-- 练习题示例（关联到数组知识点）
INSERT INTO exercises (topic_id, type, title, description, difficulty, tags, hints, solution_outline, time_complexity_expected, space_complexity_expected) VALUES
(3, 'algorithm', '两数之和', '给定一个整数数组 nums 和一个目标值 target，找出数组中和为目标值的两个数的下标。\n\n示例：\n输入: nums = [2, 7, 11, 15], target = 9\n输出: [0, 1]\n解释: nums[0] + nums[1] = 2 + 7 = 9', 'easy', '["数组", "哈希表"]', '["尝试暴力解法 O(n²)", "考虑用哈希表优化到 O(n)", "在遍历时检查 target - nums[i] 是否已在哈希表中"]', '使用哈希表存储已遍历的元素，遍历时检查差值是否存在', 'O(n)', 'O(n)'),
(3, 'algorithm', '三数之和', '给定一个数组 nums，判断是否能从中选出三个数使它们的和为零。\n\n示例：\n输入: nums = [-1, 0, 1, 2, -1, -4]\n输出: [[-1, -1, 2], [-1, 0, 1]]', 'medium', '["数组", "双指针"]', '["排序后处理", "固定一个数，双指针找另外两个", "去重技巧"]', '先排序，固定一个数后用双指针找两数之和为 target-nums[i]', 'O(n²)', 'O(1)'),
(4, 'algorithm', '反转链表', '给定一个单链表，反转链表并返回反转后的链表头节点。', 'easy', '["链表", "递归"]', '["递归版本", "迭代版本（双指针）"]', '遍历时反转 next 指针方向', 'O(n)', 'O(1)'),
(9, 'algorithm', '二叉树的中序遍历', '给定一个二叉树根节点，返回其中序遍历结果。', 'easy', '["二叉树", "遍历", "栈"]', '["递归版本（简洁）", "迭代版本（用栈模拟递归）"]', '左子树 -> 根节点 -> 右子树的顺序遍历', 'O(n)', 'O(h)'),
(10, 'algorithm', '验证 BST', '给定一个二叉树的根节点，验证它是否为有效的二叉搜索树。', 'medium', '["BST", "递归"]', '["BST 的定义是左子树所有节点小于根，右子树所有节点大于根", "需要传递上下界约束"]', '递归时传递当前节点允许的最小值和最大值', 'O(n)', 'O(h)'),
(13, 'algorithm', '爬楼梯', '假设你正在爬楼梯。需要 n 阶你才能到达楼顶。每次你可以爬 1 或 2 个台阶。有多少种不同的方法可以爬到楼顶？', 'easy', '["动态规划", "斐波那契"]', '["找规律：n=1:1, n=2:2, n=3:3, n=4:5...", "第 i 阶的方法数 = f(i-1) + f(i-2)", "这就是斐波那契数列"]', 'dp[i] = dp[i-1] + dp[i-2]，可用滚动数组优化', 'O(n)', 'O(1)');
EOF
`

- [ ] **Step 2: 执行种子数据（手动）**

Run: `cd C:/Users/owen6/repo/learn-helper/backend && sqlite3 learn-helper.db < db/migrations/schema.sql && sqlite3 learn-helper.db < db/seed/seed.sql`

- [ ] **Step 3: Commit**

```bash
cd C:/Users/owen6/repo/learn-helper
git add backend/db/seed/
git commit -m "feat: add seed data for MVP (topics + exercises)"
```

---

## 自查清单

**1. Spec 覆盖检查：**
- [x] 知识模块（topics CRUD）— Task 2, 3
- [x] 练习模块（exercises CRUD）— Task 2, 3
- [x] 学习记录（learning_records）— Task 2, 3
- [x] AI 对话（conversations + messages）— Task 2, 3
- [x] AI 配置（ai_configs）— Task 2, 3
- [x] AI Provider 抽象（Claude）— Task 3
- [x] SSE 流式输出 — Task 3
- [x] 前端路由骨架 — Task 4
- [x] AI 侧边对话面板 — Task 4

**2. 占位符检查：**
- SQL schema 使用真实字段，无 TBD/TODO
- 前端页面骨架有真实 UI 结构
- AI system prompt 有具体模板
- seed 数据有实际内容

**3. 类型一致性：**
- conversations 表使用 `context_type`（topic/exercise/dashboard），与 PRD 一致
- 学习记录状态使用 `not_started/in_progress/completed`，与 PRD 一致

---

## 执行方式选择

**Plan complete.** 任务已拆解为 5 个独立任务：

1. **Task 1**: Monorepo 配置文件
2. **Task 2**: Go 后端 + SQLite schema
3. **Task 3**: AI 模块（Provider 抽象 + Claude 实现）
4. **Task 4**: Next.js 前端骨架
5. **Task 5**: 种子数据（可选）

**1. Subagent-Driven（推荐）** — 我为每个任务启动独立子 agent，完成后 review，再推进下一个

**2. Inline Execution** — 在当前 session 直接执行所有任务

你选哪个？