package logs

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"server/config"
	"server/db"
	"sync/atomic"
	"testing"
)

func TestCanonicalTrustedProxyAddress(t *testing.T) {
	old := config.TrustedProxies
	config.TrustedProxies = "127.0.0.1/32,::1/128,172.18.0.1/32"
	t.Cleanup(func() { config.TrustedProxies = old })
	for _, tt := range []struct{ peer, real, forwarded, want string }{
		{"[::1]:12345", "", "", "::1"}, {"127.0.0.1:12345", "", "", "127.0.0.1"},
		{"198.51.100.1:12345", "192.0.2.1", "192.0.2.2", "198.51.100.1"},
		{"172.18.0.1:12345", "192.0.2.1", "spoof, 192.0.2.1", "192.0.2.1"},
		{"127.0.0.1:12345", "invalid", "192.0.2.1", "127.0.0.1"},
		{"[::1]:12345", "::ffff:192.0.2.1", "", "192.0.2.1"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tt.peer
		r.Header.Set("X-Real-IP", tt.real)
		r.Header.Set("X-Forwarded-For", tt.forwarded)
		if got := getRemoteAddr(r); got != tt.want {
			t.Errorf("%+v got %s", tt, got)
		}
	}
}

func TestAnalyticsPrivacyAndPrecision(t *testing.T) {
	old := db.DB
	var err error
	db.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.DB.Close(); db.DB = old })
	_, err = db.DB.Exec("CREATE TABLE access_logs(timestamp,method,url,status_code,response_time,remote_addr,client_ip,request_size,response_size,user_agent,data,visitor_id,event_kind,referrer,fingerprint)")
	if err != nil {
		t.Fatal(err)
	}
	AccessLogEntry(httptest.NewRequest("GET", "http://localhost/health?secret=1", nil), 200, 0.25, 1)
	AccessLogEntry(httptest.NewRequest("GET", "http://localhost/book?token=secret", nil), 200, 0.25, 1)
	var count int
	var path string
	var ms float64
	if err = db.DB.QueryRow("SELECT count(*),url,response_time FROM access_logs").Scan(&count, &path, &ms); err != nil {
		t.Fatal(err)
	}
	if count != 1 || path != "/book" || ms != 0.25 {
		t.Fatalf("count=%d path=%s ms=%f", count, path, ms)
	}
}

func TestErrorWithoutCauseAndFirstResponseStatus(t *testing.T) {
	w := httptest.NewRecorder()
	HTTPError(w, httptest.NewRequest("POST", "/api/dashboard", nil), nil, 405, "Method not allowed")
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
	for _, implicit := range []bool{true, false} {
		raw := httptest.NewRecorder()
		w := &responseCapture{ResponseWriter: raw, statusCode: 200}
		if implicit {
			w.Write([]byte("ok"))
		} else {
			w.WriteHeader(201)
		}
		w.WriteHeader(500)
		if w.statusCode != raw.Code || w.statusCode == 500 {
			t.Fatal("recorded status differs from response")
		}
	}
}

func TestWriterIsBoundedAndDrainsOnStop(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var writes atomic.Int64
	w := newLogWriter(func(any) error {
		if writes.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	})
	w.enqueue(1)
	<-started
	for i := 0; i < 80; i++ {
		w.enqueue(i)
	}
	if w.dropped != 16 {
		t.Fatalf("dropped %d", w.dropped)
	}
	close(release)
	w.stop()
	w.stop()
	if writes.Load() != 65 {
		t.Fatalf("did not drain: %d", writes.Load())
	}
	w.enqueue(1)
}

var _ http.ResponseWriter = (*responseCapture)(nil)
