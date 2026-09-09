package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseUpgradeIsIdempotentAndPreservesData(t *testing.T) {
	t.Chdir(t.TempDir())
	// The production directory is a persistent volume.
	if err := os.Mkdir("db", 0755); err != nil {
		t.Fatal(err)
	}
	old, oldLog := DB, LogDB
	t.Cleanup(func() {
		if DB != nil {
			DB.Close()
		}
		if LogDB != nil {
			LogDB.Close()
		}
		DB, LogDB = old, oldLog
	})
	if err := InitDatabase(); err != nil {
		t.Fatal(err)
	}
	if _, err := DB.Exec("INSERT INTO posts(id,title) VALUES(1,'keep me')"); err != nil {
		t.Fatal(err)
	}
	DB.Close()
	LogDB.Close()
	if err := InitDatabase(); err != nil {
		t.Fatal(err)
	}
	var mode, title string
	var indexes int
	if err := DB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("mode %s %v", mode, err)
	}
	if err := DB.QueryRow("SELECT title FROM posts WHERE id=1").Scan(&title); err != nil || title != "keep me" {
		t.Fatalf("lost data: %s %v", title, err)
	}
	if err := DB.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND tbl_name='access_logs'").Scan(&indexes); err != nil || indexes != 5 {
		t.Fatalf("indexes %d %v", indexes, err)
	}
	for _, query := range []string{"SELECT * FROM access_logs WHERE timestamp>'2026-01-01'", "SELECT * FROM access_logs WHERE client_ip='192.0.2.1' AND timestamp>'2026-01-01' ORDER BY timestamp"} {
		var a, b, c int
		var plan string
		if err := DB.QueryRow("EXPLAIN QUERY PLAN "+query).Scan(&a, &b, &c, &plan); err != nil || !strings.Contains(plan, "USING INDEX") {
			t.Fatalf("plan %s %v", plan, err)
		}
	}
	// A reader continues while the separate connection has an uncommitted write.
	tx, err := LogDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO posts(id) VALUES(2)"); err != nil {
		t.Fatal(err)
	}
	if err = DB.QueryRow("SELECT title FROM posts WHERE id=1").Scan(&title); err != nil {
		t.Fatal(err)
	}
}

func TestHistoricalAddressesAreNormalizedWithoutLosingOriginals(t *testing.T) {
	handle, err := openDatabase(filepath.Join(t.TempDir(), "legacy.db"), 100)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if _, err = handle.Exec("CREATE TABLE access_logs(remote_addr TEXT)"); err != nil {
		t.Fatal(err)
	}
	originals := []string{"127.0.0.1:12345", "127.0.0.1:56789", "[::1]:56789", "spoofed, 192.0.2.1", "::ffff:192.0.2.1", "invalid"}
	expected := []string{"127.0.0.1", "127.0.0.1", "::1", "192.0.2.1", "192.0.2.1", "unknown"}
	for _, raw := range originals {
		if _, err = handle.Exec("INSERT INTO access_logs VALUES(?)", raw); err != nil {
			t.Fatal(err)
		}
	}
	if err = migrateClientIPs(handle); err != nil {
		t.Fatal(err)
	}
	if err = migrateClientIPs(handle); err != nil {
		t.Fatal(err)
	}

	// Rows written by a temporarily rolled-back server are picked up on restart.
	if _, err = handle.Exec("INSERT INTO access_logs(remote_addr) VALUES('192.0.2.9:54321')"); err != nil {
		t.Fatal(err)
	}
	if err = migrateClientIPs(handle); err != nil {
		t.Fatal(err)
	}
	var recovered string
	if err = handle.QueryRow("SELECT client_ip FROM access_logs WHERE remote_addr='192.0.2.9:54321'").Scan(&recovered); err != nil || recovered != "192.0.2.9" {
		t.Fatalf("rollback recovery: %s %v", recovered, err)
	}
	for i, raw := range originals {
		var stored, normalized string
		if err = handle.QueryRow("SELECT remote_addr,client_ip FROM access_logs WHERE rowid=?", i+1).Scan(&stored, &normalized); err != nil || stored != raw || normalized != expected[i] {
			t.Fatalf("row %d: %s %s %v", i, stored, normalized, err)
		}
	}
}
