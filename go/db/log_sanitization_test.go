package db

import (
	"path/filepath"
	"testing"
)

func TestHistoricalURLCleanupPreservesEventsAndIsRepeatable(t *testing.T) {
	handle, err := openDatabase(filepath.Join(t.TempDir(), "history.db"), 100)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if _, err = handle.Exec(`CREATE TABLE access_logs(id INTEGER PRIMARY KEY, url TEXT, user_agent TEXT);
 INSERT INTO access_logs VALUES(1, '/book?token=credential#fragment', 'keep agent'),
 (2, '/admin/odir/browse/private.epub?x=y', 'keep agent'),
 (3, '/dashboard/ip/192.0.2.1', 'keep agent'),
 (4, '/api/dashboard/ip/192.0.2.1', 'keep agent'),
 (5, '/odir/papers/A%20Book.pdf', 'keep agent'), (6, NULL, 'keep agent');
 CREATE TABLE posts(title TEXT); INSERT INTO posts VALUES('keep post');`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = sanitizeHistoricalURLs(handle); err != nil {
			t.Fatal(err)
		}
	}
	for id, want := range []string{"/book", "/admin/odir/", "/dashboard", "/api/dashboard", "/odir/papers/A%20Book.pdf"} {
		var got, agent string
		if err = handle.QueryRow("SELECT url,user_agent FROM access_logs WHERE id=?", id+1).Scan(&got, &agent); err != nil {
			t.Fatal(err)
		}
		if got != want || agent != "keep agent" {
			t.Fatalf("row %d: %q %q", id+1, got, agent)
		}
	}
	var count int
	if err = handle.QueryRow("SELECT count(*) FROM access_logs").Scan(&count); err != nil || count != 6 {
		t.Fatalf("event count changed: %d %v", count, err)
	}
	var title string
	if err = handle.QueryRow("SELECT title FROM posts").Scan(&title); err != nil || title != "keep post" {
		t.Fatal("post changed", err)
	}
	// An old binary can add unsafe rows during rollback. A later startup cleans them.
	if _, err = handle.Exec("INSERT INTO access_logs VALUES(7, '/new?password=credential', 'keep agent')"); err != nil {
		t.Fatal(err)
	}
	if err = sanitizeHistoricalURLs(handle); err != nil {
		t.Fatal(err)
	}
	var got string
	if err = handle.QueryRow("SELECT url FROM access_logs WHERE id=7").Scan(&got); err != nil || got != "/new" {
		t.Fatalf("rollback recovery: %q %v", got, err)
	}
}
