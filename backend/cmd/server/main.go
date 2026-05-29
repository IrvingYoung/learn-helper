package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"

	"learn-helper/internal/handler"
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
    context_type TEXT DEFAULT 'topic',
    role TEXT DEFAULT 'knowledge_explain',
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
    provider_type TEXT NOT NULL DEFAULT 'claude',
    model_name TEXT NOT NULL DEFAULT 'claude-sonnet-4-20250514',
    api_key TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_slug ON wiki_pages(slug);
`

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "learn-helper.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(schemaSQL); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	// Insert overview page if not exists
	db.Exec(`INSERT OR IGNORE INTO wiki_pages (title, slug, page_type, content_status, sort_order) VALUES ('概览', 'overview', 'overview', 'published', 0)`)

	wikiHandler := handler.NewWikiHandler(db)
	aiHandler := handler.NewAIHandler(db)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		// Wiki routes
		r.Get("/wiki", wikiHandler.GetWikiTree)
		r.Get("/wiki/overview", wikiHandler.GetOverviewPage)
		r.Get("/wiki/{slug}", wikiHandler.GetWikiPageBySlug)
		r.Post("/wiki", wikiHandler.CreateWikiPage)
		r.Put("/wiki/{id}", wikiHandler.UpdateWikiPage)
		r.Delete("/wiki/{id}", wikiHandler.DeleteWikiPage)

		// AI routes
		r.Route("/ai", func(r chi.Router) {
			r.Post("/chat", aiHandler.AIChat)
			r.Get("/conversations", aiHandler.ListConversations)
			r.Post("/conversations", aiHandler.CreateConversation)
			r.Patch("/conversations/{id}", aiHandler.UpdateConversationTitle)
			r.Delete("/conversations/{id}", aiHandler.DeleteConversation)
			r.Get("/conversations/{id}/messages", aiHandler.GetConversationMessages)
			r.Get("/configs", aiHandler.GetAIConfigs)
			r.Post("/configs", aiHandler.UpsertAIConfig)
		})
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
