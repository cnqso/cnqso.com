package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDatabase() error {
	var err error
	DB, err = sql.Open("sqlite3", "./db/db.db")
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS access_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME,
			method TEXT,
			url TEXT,
			status_code INTEGER,
			response_time INTEGER,
			remote_addr TEXT,
			request_size INTEGER,
			response_size INTEGER,
			user_agent TEXT,
			data TEXT
		);
		CREATE TABLE IF NOT EXISTS dev_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME,
			level TEXT,
			message TEXT,
			data TEXT
		);
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY,
			date DATETIME,
			title TEXT,
			poster TEXT,
			contents TEXT,
			thread_owner BOOLEAN,
			thread INTEGER,
			replies INTEGER DEFAULT 0,
			image_path TEXT
		);
		CREATE TABLE IF NOT EXISTS wordle (
			id INTEGER PRIMARY KEY,
			date DATETIME,
			word CHAR(5)
		);
		CREATE TABLE IF NOT EXISTS ebwg_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			pin TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS ebwg_games (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			cover_url TEXT,
			user_id INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES ebwg_users(id)
		);
	`)

	return err
}
