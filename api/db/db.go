package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"path/filepath"
)

const Schema = `
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    stream_key TEXT UNIQUE NOT NULL,
    is_live BOOLEAN DEFAULT 0,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
-- Streams table
CREATE TABLE IF NOT EXISTS streams (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    category TEXT,
    is_live BOOLEAN DEFAULT 0,
    viewer_count INTEGER DEFAULT 0,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    thumbnail_url TEXT,
    FOREIGN KEY (user_id) REFERENCES users (id)
);
-- Messages table
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    stream_id TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    content TEXT NOT NULL,
    type TEXT DEFAULT 'text',
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (stream_id) REFERENCES streams (id),
    FOREIGN KEY (user_id) REFERENCES users (id)
);
-- Followers table
CREATE TABLE IF NOT EXISTS followers (
    follower_id INTEGER NOT NULL,
    followed_id INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (follower_id, followed_id),
    FOREIGN KEY (follower_id) REFERENCES users (id),
    FOREIGN KEY (followed_id) REFERENCES users (id)
);
-- Create indexes
CREATE INDEX IF NOT EXISTS idx_streams_user_id ON streams (user_id);
CREATE INDEX IF NOT EXISTS idx_streams_is_live ON streams (is_live);
CREATE INDEX IF NOT EXISTS idx_messages_stream_id ON messages (stream_id);
CREATE INDEX IF NOT EXISTS idx_followers_follower_id ON followers (follower_id);
CREATE INDEX IF NOT EXISTS idx_followers_followed_id ON followers (followed_id);
`

func InitDB(dbPath string) (*sql.DB, error) {
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := db.Exec(Schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}
	log.Println("Database initialized successfully")
	return db, nil
}
