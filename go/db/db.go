package db

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB
var LogDB *sql.DB

func InitDatabase() error {
	var err error
	DB, err = openDatabase("./db/db.db", 5000)
	if err != nil {
		return err
	}

	DB.SetMaxOpenConns(4)

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS access_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME,
			method TEXT,
			url TEXT,
			status_code INTEGER,
			response_time REAL,
			remote_addr TEXT,
            client_ip TEXT,
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
	`)

	if err != nil {
		DB.Close()
		return err
	}
	if err = migrateClientIPs(DB); err != nil {
		DB.Close()
		return err
	}
	_, err = DB.Exec(`CREATE INDEX IF NOT EXISTS access_logs_timestamp ON access_logs(timestamp);
 CREATE INDEX IF NOT EXISTS access_logs_client_ip_timestamp ON access_logs(client_ip, timestamp);
 DROP INDEX IF EXISTS access_logs_ip_timestamp;`)
	if err != nil {
		DB.Close()
		return err
	}
	if err = migrateJourneys(DB); err != nil {
		DB.Close()
		return err
	}
	if err = sanitizeHistoricalURLs(DB); err != nil {
		DB.Close()
		return err
	}
	LogDB, err = openDatabase("./db/db.db", 100)
	if err != nil {
		DB.Close()
	}
	return err
}

func openDatabase(path string, busyMS int) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: absolute}
	q := u.Query()
	q.Set("_journal_mode", "WAL")
	q.Set("_busy_timeout", strconv.Itoa(busyMS))
	u.RawQuery = q.Encode()
	handle, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	handle.SetMaxOpenConns(1)
	if err = handle.Ping(); err != nil {
		handle.Close()
		return nil, err
	}
	return handle, nil
}
