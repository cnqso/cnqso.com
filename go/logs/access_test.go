package logs

import (
	"database/sql"
	"net/http/httptest"
	"server/db"
	"testing"
)

func TestPrivateLibraryPathsAreRedactedFromAnalytics(t *testing.T) {
	original := db.DB
	var err error
	db.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.DB.Close(); db.DB = original })
	_, err = db.DB.Exec("CREATE TABLE access_logs (timestamp,method,url,status_code,response_time,remote_addr,client_ip,request_size,response_size,user_agent,data,visitor_id,event_kind,referrer,fingerprint)")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "http://localhost/admin/odir/browse/Private%20title.epub?anything=secret", nil)
	AccessLogEntry(request, 200, 1, 1)
	var logged string
	if err := db.DB.QueryRow("SELECT url FROM access_logs").Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != "/admin/odir/" {
		t.Fatalf("private URL was logged: %s", logged)
	}
}
