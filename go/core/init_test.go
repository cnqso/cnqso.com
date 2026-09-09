package core

import (
	"database/sql"
	"server/db"
	"strings"
	"testing"
)

func TestArchiveTextIsEscapedAndReferencesStillWork(t *testing.T) {
	original := db.DB
	var err error
	db.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.DB.Close(); db.DB = original })
	if _, err := db.DB.Exec("CREATE TABLE posts (id INTEGER, thread INTEGER); INSERT INTO posts VALUES (123, 1)"); err != nil {
		t.Fatal(err)
	}
	result := string(processPostContent("<img src=x onerror=alert(1)>\n>>123", "1"))
	if strings.Contains(result, "<img") {
		t.Fatal("untrusted HTML was not escaped")
	}
	if !strings.Contains(result, "&lt;img") || !strings.Contains(result, `href="#post-123"`) {
		t.Fatalf("escaped text or reference lost: %s", result)
	}
}
