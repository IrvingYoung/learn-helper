package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	schema := `
	PRAGMA foreign_keys = ON;
	CREATE TABLE IF NOT EXISTS conversations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_id INTEGER,
		exercise_id INTEGER,
		context_type TEXT,
		role TEXT,
		title TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
		role TEXT NOT NULL,
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
		config TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("exec schema: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})
	return db
}

func TestListConversations_Empty(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/conversations", nil)
	w := httptest.NewRecorder()
	h.ListConversations(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty array, got %v", result)
	}
}

func TestCreateConversation(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	body, _ := json.Marshal(map[string]interface{}{
		"role":         "knowledge_explain",
		"context_type": "topic",
		"topic_id":     1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/conversations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateConversation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["role"] != "knowledge_explain" {
		t.Errorf("expected role=knowledge_explain, got %v", result["role"])
	}
	if result["title"] == nil || result["title"] == "" {
		t.Error("expected default title to be generated")
	}
}

func TestCreateConversation_WithCustomTitle(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	body, _ := json.Marshal(map[string]interface{}{
		"role":         "problem_solving",
		"context_type": "exercise",
		"title":        "排序讨论",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/conversations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateConversation(w, req)

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["title"] != "排序讨论" {
		t.Errorf("expected custom title, got %v", result["title"])
	}
}

func TestCreateConversation_MissingRole(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	body, _ := json.Marshal(map[string]interface{}{
		"context_type": "topic",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/conversations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateConversation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateConversationTitle(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	// Create a conversation first
	db.Exec(`INSERT INTO conversations (context_type, role, title) VALUES ('topic', 'knowledge_explain', 'old title')`)

	body, _ := json.Marshal(map[string]interface{}{"title": "new title"})
	req := httptest.NewRequest(http.MethodPatch, "/api/ai/conversations/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.UpdateConversationTitle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["title"] != "new title" {
		t.Errorf("expected updated title, got %v", result["title"])
	}
}

func TestUpdateConversationTitle_NotFound(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	body, _ := json.Marshal(map[string]interface{}{"title": "test"})
	req := httptest.NewRequest(http.MethodPatch, "/api/ai/conversations/999", bytes.NewReader(body))
	req.SetPathValue("id", "999")
	w := httptest.NewRecorder()
	h.UpdateConversationTitle(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetConversationMessages(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	// Create conversation and messages
	db.Exec(`INSERT INTO conversations (context_type, role, title) VALUES ('topic', 'knowledge_explain', 'test')`)
	db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (1, 'user', 'hello', 'claude', 0)`)
	db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (1, 'assistant', 'hi there', 'claude', 0)`)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/conversations/1/messages", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.GetConversationMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0]["role"] != "user" {
		t.Errorf("expected first message role=user, got %v", result[0]["role"])
	}
}

func TestGetConversationMessages_NotFound(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/conversations/999/messages", nil)
	req.SetPathValue("id", "999")
	w := httptest.NewRecorder()
	h.GetConversationMessages(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteConversation(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	db.Exec(`INSERT INTO conversations (context_type, role, title) VALUES ('topic', 'knowledge_explain', 'test')`)
	db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (1, 'user', 'hello', 'claude', 0)`)

	req := httptest.NewRequest(http.MethodDelete, "/api/ai/conversations/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.DeleteConversation(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Verify messages are also deleted
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = 1`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", count)
	}
}

func TestDeleteConversation_NotFound(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	req := httptest.NewRequest(http.MethodDelete, "/api/ai/conversations/999", nil)
	req.SetPathValue("id", "999")
	w := httptest.NewRecorder()
	h.DeleteConversation(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAIChat_MissingConversationID(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	body, _ := json.Marshal(map[string]interface{}{
		"message": "hello",
		"role":    "knowledge_explain",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.AIChat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAIChat_ConversationNotFound(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	body, _ := json.Marshal(map[string]interface{}{
		"conversation_id": 999,
		"message":         "hello",
		"role":            "knowledge_explain",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.AIChat(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListConversations_WithData(t *testing.T) {
	db := setupTestDB(t)
	h := NewAIHandler(db)

	// Create conversations with messages
	db.Exec(`INSERT INTO conversations (context_type, role, title) VALUES ('topic', 'knowledge_explain', 'Conv A')`)
	db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (1, 'user', 'hello world this is a long message', 'claude', 0)`)
	db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (1, 'assistant', 'hi', 'claude', 0)`)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/conversations", nil)
	w := httptest.NewRecorder()
	h.ListConversations(w, req)

	var result []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(result))
	}
	conv := result[0]
	if conv["message_count"] == nil {
		t.Error("expected message_count field")
	}
	if conv["last_message_preview"] == nil {
		t.Error("expected last_message_preview field")
	}
}
