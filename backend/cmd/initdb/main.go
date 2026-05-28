package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Get the backend directory
	exeDir, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to get executable path: %v", err)
	}
	backendDir := filepath.Dir(filepath.Dir(exeDir))

	// If running with `go run`, exeDir might be a temp directory, fall back to cwd
	if _, err := os.Stat(filepath.Join(backendDir, "go.mod")); os.IsNotExist(err) {
		backendDir = "."
	}

	dbPath := filepath.Join(backendDir, "learn-helper.db")
	schemaPath := filepath.Join(backendDir, "db", "migrations", "schema.sql")
	seedPath := filepath.Join(backendDir, "db", "seed", "seed.sql")

	// Check if database already exists
	if _, err := os.Stat(dbPath); err == nil {
		log.Println("Database already exists at", dbPath)
		return
	}

	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Fatalf("failed to read schema.sql: %v", err)
	}

	seedSQL, err := os.ReadFile(seedPath)
	if err != nil {
		log.Fatalf("failed to read seed.sql: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	log.Println("Creating schema...")
	if _, err := db.Exec(string(schemaSQL)); err != nil {
		log.Fatalf("failed to create schema: %v", err)
	}

	log.Println("Seeding data...")
	if _, err := db.Exec(string(seedSQL)); err != nil {
		log.Fatalf("failed to seed data: %v", err)
	}

	log.Println("Done! Database initialized at", dbPath)
}