package api

import (
	"context"
	"database/sql"
	"server/db"
	"testing"
	"time"
)

func TestFingerprintCandidatesRemainSeparateAndExpire(t *testing.T) {
	old := db.DB
	var err error
	db.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.DB.SetMaxOpenConns(1)
	t.Cleanup(func() { db.DB.Close(); db.DB = old })
	if _, err = db.DB.Exec("CREATE TABLE access_logs(visitor_id,timestamp DATETIME,fingerprint)"); err != nil {
		t.Fatal(err)
	}
	ids := []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccccccccc", "dddddddddddddddddddddddddddddddd"}
	now := time.Now().UTC()
	for _, row := range []struct {
		id, signature string
		at            time.Time
	}{
		{ids[0], "shared", now}, {ids[0], "shared", now}, {ids[1], "shared", now},
		{ids[2], "shared", now.Add(-48 * time.Hour)}, {ids[3], "different", now},
	} {
		if _, err = db.DB.Exec("INSERT INTO access_logs VALUES(?,?,?)", row.id, row.at, row.signature); err != nil {
			t.Fatal(err)
		}
	}
	signatures, matches, err := journeyFingerprintMatches(context.Background(), ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(signatures) != 1 || len(matches) != 1 || matches[0].ID != ids[1] || matches[0].Shared != 1 {
		t.Fatalf("signatures=%v matches=%v", signatures, matches)
	}
	var distinct int
	if err = db.DB.QueryRow("SELECT COUNT(DISTINCT visitor_id) FROM access_logs").Scan(&distinct); err != nil || distinct != 4 {
		t.Fatal("cookie identities were merged")
	}
}
