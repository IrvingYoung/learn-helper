package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"

	"learn-helper/internal/ai"
	"learn-helper/internal/engine"
	"learn-helper/internal/handler"
	"learn-helper/internal/model"
	"learn-helper/internal/worker"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER REFERENCES topics(id),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    content TEXT DEFAULT '',
    difficulty TEXT DEFAULT 'beginner',
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER NOT NULL REFERENCES topics(id),
    title TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    difficulty TEXT DEFAULT 'easy',
    exercise_type TEXT DEFAULT 'coding',
    solution TEXT DEFAULT '',
    hints TEXT DEFAULT '[]',
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS learning_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER DEFAULT 1,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    status TEXT DEFAULT 'not_started',
    attempts INTEGER DEFAULT 0,
    last_attempt_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    context_type TEXT DEFAULT 'wiki',
    role TEXT DEFAULT 'wiki_maintainer',
    title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'tool')),
    content TEXT NOT NULL,
    model_provider TEXT,
    token_count INTEGER,
    tool_call_id TEXT,
    tool_name TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL DEFAULT 'claude',
    model_name TEXT NOT NULL DEFAULT 'claude-sonnet-4-7-20250514',
    api_key TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    config TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wiki_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    page_type TEXT NOT NULL DEFAULT 'entity',
    content TEXT NOT NULL DEFAULT '',
    tags TEXT DEFAULT '[]',
    parent_id INTEGER REFERENCES wiki_pages(id),
    content_status TEXT NOT NULL DEFAULT 'empty',
    sort_order INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL DEFAULT '',
    links TEXT NOT NULL DEFAULT '[]',
    backlinks TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_slug ON wiki_pages(slug);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);

CREATE TABLE IF NOT EXISTS plans (
    id TEXT PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    reasoning TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'confirmed', 'executing', 'completed', 'rejected', 'completed_with_failures')),
    outline TEXT,
    phase_index INTEGER,
    total_phases INTEGER,
    focus_page_id INTEGER REFERENCES wiki_pages(id),
    calibration_question TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    executed_at TEXT
);

CREATE TABLE IF NOT EXISTS plan_actions (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK(type IN ('create_page', 'update_page', 'delete_page', 'link_pages', 'move_page')),
    params TEXT NOT NULL,
    depends_on TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
    result TEXT,
    sort_order INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_plans_conversation ON plans(conversation_id);
CREATE INDEX IF NOT EXISTS idx_plan_actions_plan ON plan_actions(plan_id);
`

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "learn-helper.db"
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		// Checkpoint WAL before closing so all data is in the main db file
		db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		db.Close()
	}()
	// SQLite doesn't support concurrent writes; limit to one connection.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schemaSQL); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	// Migrate existing databases: add links/backlinks columns if missing
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN links TEXT NOT NULL DEFAULT '[]'`)
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN backlinks TEXT NOT NULL DEFAULT '[]'`)

	// Migrate existing databases: add outline/phase columns to plans if missing
	db.Exec(`ALTER TABLE plans ADD COLUMN outline TEXT`)
	db.Exec(`ALTER TABLE plans ADD COLUMN phase_index INTEGER`)
	db.Exec(`ALTER TABLE plans ADD COLUMN total_phases INTEGER`)

	// Migrate: add focus_page_id column to plans if missing
	db.Exec(`ALTER TABLE plans ADD COLUMN focus_page_id INTEGER REFERENCES wiki_pages(id)`)

	// Migrate: add calibration_question column to plans if missing
	db.Exec(`ALTER TABLE plans ADD COLUMN calibration_question TEXT`)

	db.Exec(`INSERT OR IGNORE INTO wiki_pages (title, slug, page_type, content, content_status, sort_order) VALUES ('概览', 'overview', 'overview', '# 知识库概览\n\n欢迎使用 LLM Wiki！\n\n通过与 AI 对话来构建你的知识库。试试说：\n\n- "我要学 Go 后端"\n- "总结一下 Redis 的核心数据结构"\n- "帮我梳理数据库索引的知识"', 'published', 0)`)

	wikiHandler := handler.NewWikiHandler(db)
	aiHandler := handler.NewAIHandler(db)
	queries := model.New(db)
	eng := engine.NewExecutionEngine(db, queries)
	planHandler := handler.NewPlanHandler(db, queries, eng)

	// --- Summary worker: generates AI summaries for wiki pages asynchronously ---
	// We try to load the active AI config and start the worker. If no config is
	// set up yet (e.g. fresh install before user configures the API key), the
	// worker is skipped — the server still starts and serves other routes.
	{
		startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		aiConfig, cfgErr := queries.GetActiveAIConfig(startupCtx)
		startupCancel()
		if cfgErr != nil {
			log.Printf("[summary-worker] no active AI config; worker disabled until API key is configured: %v", cfgErr)
		} else {
			aiProvider, provErr := ai.NewProvider(ai.ProviderType(aiConfig.Provider), aiConfig.ApiKey, aiConfig.ModelName)
			if provErr != nil {
				log.Printf("[summary-worker] failed to create provider: %v", provErr)
			} else {
				// Build a function the worker can call to invoke the AI provider's Chat.
				summaryChat := func(ctx context.Context, prompt, system string) (string, error) {
					resp, err := aiProvider.Chat(ctx, ai.ChatRequest{
						Messages:     []ai.Message{{Role: "user", Content: prompt}},
						SystemPrompt: system,
						MaxTokens:    256,
					})
					if err != nil {
						return "", err
					}
					return resp.Content, nil
				}

				summaryWorker := worker.NewSummaryWorker(
					worker.NewProviderAdapter(summaryChat),
					worker.NewSQLDBAdapter(queries),
				)

				// Wire engine + handlers to enqueue page summaries on write.
				eng.SetOnPageWritten(func(pageID int64) {
					summaryWorker.Enqueue(pageID)
				})
				wikiHandler.SetOnPageWritten(func(pageID int64) {
					summaryWorker.Enqueue(pageID)
				})

				workerCtx, workerCancel := context.WithCancel(context.Background())
				defer workerCancel()

				// On startup, transition all 'empty' pages with content to 'pending'
				// so the backfill loop picks them up. This is a one-shot per server start.
				{
					res, err := db.Exec(`UPDATE wiki_pages SET summary_status='pending', summary_content_hash=NULL WHERE summary_status='empty' AND content != ''`)
					if err != nil {
						log.Printf("[summary-worker] failed to transition empty→pending: %v", err)
					} else if n, _ := res.RowsAffected(); n > 0 {
						log.Printf("[summary-worker] marked %d pages as pending for backfill", n)
					}
				}

				// Backfill pending/failed summaries on startup.
				if os.Getenv("SKIP_SUMMARY_BACKFILL") != "1" {
					go func() {
						for {
							if err := summaryWorker.BackfillOnce(workerCtx, 10); err != nil {
								log.Printf("[summary-worker] backfill: %v", err)
							}
							// Check if more remain; if not, exit the backfill loop.
							rows, err := queries.ListPendingSummaries(workerCtx, 1000)
							if err != nil || len(rows) == 0 {
								return
							}
							time.Sleep(1 * time.Second)
						}
					}()
				}

				// Start the worker loop.
				go summaryWorker.Run(workerCtx)
				log.Printf("[summary-worker] started (provider=%s model=%s)", aiConfig.Provider, aiConfig.ModelName)
			}
		}
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		// Wiki routes
		r.Get("/wiki", wikiHandler.GetWikiTree)
		r.Get("/wiki/overview", wikiHandler.GetOverviewPage)
		r.Get("/wiki/by-id", wikiHandler.GetWikiPageByID)
		r.Get("/wiki/{slug}", wikiHandler.GetWikiPageBySlug)
		r.Post("/wiki", wikiHandler.CreateWikiPage)
		r.Put("/wiki/{id}", wikiHandler.UpdateWikiPage)
		r.Delete("/wiki/{id}", wikiHandler.DeleteWikiPage)
		r.Put("/wiki/{id}/confirm", wikiHandler.ConfirmPageContent)

		// Wiki structure operations (no confirmation needed)
			r.Patch("/wiki/{id}/rename", wikiHandler.RenameWikiPage)
			r.Patch("/wiki/{id}/move", wikiHandler.MoveWikiPage)
			r.Post("/wiki/quick-create", wikiHandler.CreateEmptyWikiPage)

			// AI routes
		r.Route("/ai", func(r chi.Router) {
			r.Post("/chat", aiHandler.AIChat)
			r.Post("/upload", aiHandler.UploadFile)
			r.Get("/conversations", aiHandler.ListConversations)
			r.Post("/conversations", aiHandler.CreateConversation)
			r.Patch("/conversations/{id}", aiHandler.UpdateConversationTitle)
			r.Delete("/conversations/{id}", aiHandler.DeleteConversation)
			r.Get("/conversations/{id}/messages", aiHandler.GetConversationMessages)
			r.Get("/configs", aiHandler.GetAIConfigs)
			r.Post("/configs", aiHandler.UpsertAIConfig)
		})

		// Plan routes
		r.Get("/plans", planHandler.GetPlan)
		r.Post("/plans", planHandler.CreatePlan)
		r.Post("/plans/confirm", planHandler.ConfirmPlan)
		r.Post("/plans/reject", planHandler.RejectPlan)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on :%s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
