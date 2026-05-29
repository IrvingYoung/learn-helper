package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		exeDir, _ := os.Getwd()
		dbPath = filepath.Join(exeDir, "learn-helper.db")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check if topics table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='topics'").Scan(&tableName)
	if err == sql.ErrNoRows {
		log.Println("No topics table found, nothing to migrate")
		return
	}
	if err != nil {
		log.Fatalf("Failed to check for topics table: %v", err)
	}

	// Check if wiki_pages already has data (excluding overview)
	var wikiCount int
	err = db.QueryRow("SELECT COUNT(*) FROM wiki_pages WHERE page_type != 'overview'").Scan(&wikiCount)
	if err != nil {
		log.Fatalf("Failed to check wiki_pages: %v", err)
	}
	if wikiCount > 0 {
		log.Println("wiki_pages already has data, skipping migration")
		return
	}

	rows, err := db.Query(`
		SELECT id, COALESCE(parent_id, 0), name, slug,
		       COALESCE(description, ''), COALESCE(content, ''),
		       COALESCE(difficulty, 'beginner'),
		       COALESCE(sort_order, 0)
		FROM topics
		ORDER BY sort_order, id
	`)
	if err != nil {
		log.Fatalf("Failed to query topics: %v", err)
	}
	defer rows.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}

	idMap := make(map[int64]int64)
	var sortIdx int64 = 1

	for rows.Next() {
		var oldID, parentID int64
		var name, slug, description, content, difficulty string
		var sortOrder int64
		if err := rows.Scan(&oldID, &parentID, &name, &slug, &description, &content, &difficulty, &sortOrder); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to scan topic: %v", err)
		}

		contentStatus := "empty"
		if content != "" {
			contentStatus = "published"
		} else if description != "" {
			contentStatus = "draft"
		}

		pageType := "entity"
		if parentID == 0 {
			pageType = "concept"
		}

		tags := "[]"
		if difficulty != "" {
			tags = fmt.Sprintf(`["%s"]`, difficulty)
		}

		fullContent := content
		if fullContent == "" && description != "" {
			fullContent = fmt.Sprintf("# %s\n\n%s", name, description)
		}

		var newParentID sql.NullInt64
		if newParent, ok := idMap[parentID]; ok && parentID != 0 {
			newParentID = sql.NullInt64{Int64: newParent, Valid: true}
		}

		result, err := tx.Exec(`
			INSERT INTO wiki_pages (title, slug, page_type, content, tags, parent_id, content_status, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, name, slug, pageType, fullContent, tags, newParentID, contentStatus, sortIdx)
		if err != nil {
			tx.Rollback()
			log.Fatalf("Failed to insert wiki_page: %v", err)
		}

		newID, _ := result.LastInsertId()
		idMap[oldID] = newID
		sortIdx++

		log.Printf("Migrated: %s (topic_id=%d -> wiki_id=%d)", name, oldID, newID)
	}

	if err := rows.Err(); err != nil {
		tx.Rollback()
		log.Fatalf("Row iteration error: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	log.Printf("Migration complete: %d topics migrated to wiki_pages", len(idMap))

	log.Println("Dropping old tables...")
	for _, t := range []string{"learning_records", "exercises", "topics"} {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
		if err != nil {
			log.Printf("Warning: failed to drop table %s: %v", t, err)
		} else {
			log.Printf("Dropped table: %s", t)
		}
	}

	log.Println("Migration finished successfully")
}
