package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"

	"learn-helper/internal/ai"
	"learn-helper/internal/ai/skills"
	"learn-helper/internal/cron"
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
    code_examples TEXT,
    common_mistakes TEXT,
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
    solution_detail TEXT,
    common_errors TEXT,
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
    tool_calls TEXT,
    tool_summary TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL DEFAULT 'opencode',
    model_name TEXT NOT NULL DEFAULT 'deepseek-v4-pro',
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
    summary TEXT NOT NULL DEFAULT '',
    summary_status TEXT NOT NULL DEFAULT 'empty',
    summary_generated_at DATETIME,
    summary_content_hash TEXT,
    link_count INTEGER NOT NULL DEFAULT 0,
    backlink_count INTEGER NOT NULL DEFAULT 0,
    tags_normalized TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_slug ON wiki_pages(slug);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);

CREATE TABLE IF NOT EXISTS wiki_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,
    page_id INTEGER,
    page_title TEXT NOT NULL,
    page_path TEXT,
    source TEXT NOT NULL DEFAULT 'plan',
    summary TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wiki_log_created_at ON wiki_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_wiki_log_page_id ON wiki_log(page_id);

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
    type TEXT NOT NULL CHECK(type IN ('create_page', 'update_page', 'patch_page', 'delete_page', 'link_pages', 'move_page')),
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

//go:embed all:dist
var embeddedDistFS embed.FS

func main() {
	// Wire the embedded SPA dist filesystem so production serves the
	// frontend assets from inside this binary. Must run before any request
	// can reach a handler that calls getDistIndexHTML().
	//
	// The embed directive puts files under a "dist" prefix, so we use
	// fs.Sub to expose the dist contents at the FS root — getDistIndexHTML
	// then reads "index.html" and "assets/..." directly.
	spaRoot, err := fs.Sub(embeddedDistFS, "dist")
	if err != nil {
		log.Fatalf("embed dist sub: %v", err)
	}
	handler.SetSPAFS(spaRoot)

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

	// Migrate 001: add topic detail columns
	db.Exec(`ALTER TABLE topics ADD COLUMN code_examples TEXT`)
	db.Exec(`ALTER TABLE topics ADD COLUMN common_mistakes TEXT`)
	db.Exec(`ALTER TABLE exercises ADD COLUMN solution_detail TEXT`)
	db.Exec(`ALTER TABLE exercises ADD COLUMN common_errors TEXT`)

	// Migrate 005: add links/backlinks columns if missing
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

	// Migrate 007: add tool_calls column to messages
	db.Exec(`ALTER TABLE messages ADD COLUMN tool_calls TEXT`)

	// Migrate: add tool_call_id and tool_name to messages
	db.Exec(`ALTER TABLE messages ADD COLUMN tool_call_id TEXT`)
	db.Exec(`ALTER TABLE messages ADD COLUMN tool_name TEXT`)

	// Migrate: add tool_summary column for cross-request tool context.
	// Heuristic-generated one-liner per assistant turn; replaces the need
	// to persist full tool_result messages (which violated protocol on reload).
	db.Exec(`ALTER TABLE messages ADD COLUMN tool_summary TEXT NOT NULL DEFAULT ''`)

	// Migrate 014: add skill column to messages (persists which /skill was active)
	db.Exec(`ALTER TABLE messages ADD COLUMN skill TEXT NOT NULL DEFAULT ''`)

	// Migrate existing databases: add summary columns from migration 008
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN summary TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN summary_status TEXT NOT NULL DEFAULT 'empty'`)
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN summary_generated_at DATETIME`)
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN summary_content_hash TEXT`)
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN link_count INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN backlink_count INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN tags_normalized TEXT NOT NULL DEFAULT ''`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_wiki_pages_summary_status ON wiki_pages(summary_status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_wiki_pages_tags_normalized ON wiki_pages(tags_normalized)`)

	// Migration: add share_token column for public link sharing (share-page-as-link).
	// Idempotent: SQLite errors on duplicate-column ADD, but we ignore the error
	// (matches the project's migration convention — see ADD COLUMN calls above).
	db.Exec(`ALTER TABLE wiki_pages ADD COLUMN share_token TEXT NOT NULL DEFAULT ''`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_wiki_pages_share_token ON wiki_pages(share_token)`)

	// Migrate existing databases: add wiki_log table from migration 009
	db.Exec(`CREATE TABLE IF NOT EXISTS wiki_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT NOT NULL,
		page_id INTEGER,
		page_title TEXT NOT NULL,
		page_path TEXT,
		source TEXT NOT NULL DEFAULT 'plan',
		summary TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_wiki_log_created_at ON wiki_log(created_at DESC)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_wiki_log_page_id ON wiki_log(page_id)`)

	// Migrate 010: allow 'patch_page' action type in plan_actions.
	// AI was upgraded to use patch_page for targeted edits, but the CHECK
	// constraint on plan_actions.type was never updated. SQLite has no
	// ALTER CONSTRAINT, so we recreate the table. Idempotent: skips if
	// the constraint already includes patch_page.
	var planActionsSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='plan_actions'`).Scan(&planActionsSQL); err == nil {
		if !strings.Contains(planActionsSQL, "patch_page") {
			log.Println("Migrating plan_actions to allow patch_page action type...")
			if err := recreatePlanActionsForPatchPage(db); err != nil {
				log.Printf("WARN: plan_actions migration failed: %v", err)
			} else {
				log.Println("plan_actions migrated to allow patch_page")
			}
		}
	}

	// Migrate 012: drop plans / plan_actions tables.
	// The plan tool has been redesigned and no longer uses these tables
	// (tool migration happened in code; see tasks 4.2-4.4). User is
	// single-tenant, so any pending plans in the DB are discarded.
	// Child (plan_actions) is dropped before parent (plans) to respect
	// the FK constraint from plan_actions.plan_id -> plans.id.
	db.Exec(`DROP TABLE IF EXISTS plan_actions`)
	db.Exec(`DROP TABLE IF EXISTS plans`)

	// Migrate: update legacy claude provider to opencode
	db.Exec(`UPDATE ai_configs SET provider = 'opencode', model_name = 'deepseek-v4-pro' WHERE provider = 'claude'`)

	db.Exec(`INSERT OR IGNORE INTO wiki_pages (title, slug, page_type, content, content_status, sort_order) VALUES ('概览', 'overview', 'overview', '# 知识库概览\n\n欢迎使用 LLM Wiki！\n\n通过与 AI 对话来构建你的知识库。试试说：\n\n- "我要学 Go 后端"\n- "总结一下 Redis 的核心数据结构"\n- "帮我梳理数据库索引的知识"', 'published', 0)`)

	// --- Migration 013: cron_tasks + cron_runs (idempotent) ---
	db.Exec(`CREATE TABLE IF NOT EXISTS cron_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		cron_expr TEXT NOT NULL,
		prompt TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		auto_approve INTEGER NOT NULL DEFAULT 1,
		max_steps INTEGER NOT NULL DEFAULT 10,
		timeout_sec INTEGER NOT NULL DEFAULT 300,
		next_run_at DATETIME,
		last_run_at DATETIME,
		last_status TEXT,
		last_error TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cron_tasks_enabled_next_run ON cron_tasks(enabled, next_run_at)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS cron_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL REFERENCES cron_tasks(id) ON DELETE CASCADE,
		status TEXT NOT NULL,
		started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME,
		duration_ms INTEGER,
		output_summary TEXT,
		error TEXT,
		write_count INTEGER NOT NULL DEFAULT 0,
		steps_used INTEGER NOT NULL DEFAULT 0,
		conversation_id INTEGER REFERENCES conversations(id) ON DELETE SET NULL
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cron_runs_task_id ON cron_runs(task_id, started_at DESC)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cron_runs_status ON cron_runs(status)`)

	// --- Skill registry ---
	var skillReg *skills.Registry
	if dir := os.Getenv("LH_SKILLS_DIR"); dir != "" {
		fsys := os.DirFS(dir)
		skillReg, err = skills.LoadFromFS(fsys)
		if err != nil {
			log.Fatalf("load skills from %s: %v", dir, err)
		}
	} else {
		skillReg, err = skills.LoadFromFS(skills.EmbedFS())
		if err != nil {
			log.Fatalf("load embedded skills: %v", err)
		}
	}
	log.Printf("[boot] loaded %d skills", len(skillReg.List()))

	wikiHandler := handler.NewWikiHandler(db)
	aiHandler := handler.NewAIHandler(db, skillReg)
	queries := model.New(db)
	eng := engine.NewExecutionEngine(db, queries)

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

	// Skills catalog (outside /api route group for clarity)
	r.Get("/api/skills", (&handler.SkillsHandler{Registry: skillReg}).HandleList)

	// Public share SSR route. Reads the SPA's dist/index.html and injects
	// og:* meta tags so IM crawlers (WeChat, Twitter, etc.) can render a
	// link preview card. Reverse proxy must forward /share/* → :8080.
	r.Get("/share/{slug}", wikiHandler.GetShareSSRPage)

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

		// Public share API (token-gated). Registered under /api so the same
		// reverse-proxy rule (/api/* → :8080) covers it.
		r.Get("/share/{slug}", wikiHandler.GetPublicSharePage)

		// Wiki structure operations (no confirmation needed)
			r.Patch("/wiki/{id}/rename", wikiHandler.RenameWikiPage)
			r.Patch("/wiki/{id}/move", wikiHandler.MoveWikiPage)
			r.Post("/wiki/quick-create", wikiHandler.CreateEmptyWikiPage)

			// AI routes
		r.Route("/ai", func(r chi.Router) {
			r.Post("/chat", aiHandler.AIChat)
			r.Post("/upload", aiHandler.UploadFile)
			r.Post("/permission_response", aiHandler.HandlePermissionResponse)
			r.Post("/ask_user_response", aiHandler.HandleAskUserResponse)
			r.Get("/conversations", aiHandler.ListConversations)
			r.Post("/conversations", aiHandler.CreateConversation)
			r.Patch("/conversations/{id}", aiHandler.UpdateConversationTitle)
			r.Delete("/conversations/{id}", aiHandler.DeleteConversation)
			r.Get("/conversations/{id}/messages", aiHandler.GetConversationMessages)
			r.Get("/configs", aiHandler.GetAIConfigs)
			r.Post("/configs", aiHandler.UpsertAIConfig)
		})
	})

	// Catch-all SPA fallback: any GET that didn't match a more specific route.
	// Try the path as a static asset in the embedded dist first; if that
	// misses, serve index.html so the React Router takes over on the client.
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// 1) Try as a static asset from the embedded dist
		if body, err := fs.ReadFile(spaRoot, path); err == nil {
			if ctype := mime.TypeByExtension(filepath.Ext(path)); ctype != "" {
				w.Header().Set("Content-Type", ctype)
			} else {
				w.Header().Set("Content-Type", "application/octet-stream")
			}
			_, _ = w.Write(body)
			return
		}

		// 2) Fallback: serve index.html (client-side routing)
		body, err := handler.IndexHTML()
		if err != nil {
			http.Error(w, "SPA not loaded", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	})

	// --- Cron tasks: scheduler + HTTP API ---
	cronDB := cron.NewSQLDBAdapter(db)
	// *handler.AIHandler satisfies cron.RunnerHooks (its RunReAct signature
	// matches the interface). No adapter needed.
	cronRunner := cron.NewRunner(cronDB, aiHandler)
	cronHandler := cron.NewHandler(cronDB, cronRunner)
	cronScheduler := cron.NewScheduler(cronDB, cronRunner)

	// Mount cron HTTP routes
	r.Route("/api/cron", func(r chi.Router) {
		r.Get("/tasks", cronHandler.ListTasks)
		r.Post("/tasks", cronHandler.CreateTask)
		r.Get("/tasks/{id}", cronHandler.GetTask)
		r.Patch("/tasks/{id}", cronHandler.PatchTask)
		r.Delete("/tasks/{id}", cronHandler.DeleteTask)
		r.Post("/tasks/{id}/run-now", cronHandler.RunNow)
		r.Get("/tasks/{id}/runs", cronHandler.ListRuns)
		r.Get("/runs", cronHandler.ListAllRuns)
		r.Get("/runs/{id}", cronHandler.GetRun)
	})

	// Start the cron scheduler (background goroutine; tied to a context we
	// cancel on server shutdown).
	cronSchedulerCtx, cronSchedulerCancel := context.WithCancel(context.Background())
	defer cronSchedulerCancel()
	if os.Getenv("DISABLE_CRON_SCHEDULER") != "1" {
		go cronScheduler.Run(cronSchedulerCtx)
		log.Printf("[cron] scheduler started")
	} else {
		log.Printf("[cron] scheduler disabled via DISABLE_CRON_SCHEDULER=1")
	}

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

// recreatePlanActionsForPatchPage rebuilds plan_actions with the CHECK constraint
// updated to allow 'patch_page'. Uses a single connection + transaction so all
// steps succeed atomically or roll back together.
func recreatePlanActionsForPatchPage(db *sql.DB) error {
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	// foreign_keys PRAGMA cannot be set inside a transaction, and the rename
	// can briefly leave the FK in a state SQLite doesn't like. Keep it on the
	// same connection so it reverts cleanly.
	if _, err := conn.ExecContext(context.Background(), `PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`)

	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	steps := []string{
		`CREATE TABLE plan_actions_new (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
			type TEXT NOT NULL CHECK(type IN ('create_page', 'update_page', 'patch_page', 'delete_page', 'link_pages', 'move_page')),
			params TEXT NOT NULL,
			depends_on TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
			result TEXT,
			sort_order INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`INSERT INTO plan_actions_new SELECT * FROM plan_actions`,
		`DROP TABLE plan_actions`,
		`ALTER TABLE plan_actions_new RENAME TO plan_actions`,
		`CREATE INDEX IF NOT EXISTS idx_plan_actions_plan ON plan_actions(plan_id)`,
	}
	for _, s := range steps {
		if _, err := tx.ExecContext(context.Background(), s); err != nil {
			return fmt.Errorf("step %q: %w", firstLine(s), err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
