package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"server/config"
	"server/db"
	"server/internal/libraryauth"
	"strings"
	"testing"
	"time"
)

func analyticsDatabase(t *testing.T) {
	t.Helper()
	previous := db.DB
	var err error
	db.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.DB.SetMaxOpenConns(1)
	t.Cleanup(func() { db.DB.Close(); db.DB = previous })
	_, err = db.DB.Exec(`CREATE TABLE access_logs(timestamp DATETIME, method TEXT, url TEXT, status_code INTEGER, response_time REAL, client_ip TEXT, request_size INTEGER, response_size INTEGER, user_agent TEXT); CREATE TABLE dev_logs(timestamp,level,message,data);`)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		path   string
		status int
		ms     float64
	}{
		{"/book?token=secret", 200, 0}, {"/book?sort=title", 200, 0.25}, {"/missing", 404, 4}, {"/fail", 500, 8},
		{"/health", 200, 999}, {"/health?old=1", 200, 999}, {"/dashboard", 200, 999}, {"/api/dashboard?period=1h", 200, 999},
		{"/dashboard/ip/192.0.2.1", 200, 999}, {"/api/dashboard/ip/192.0.2.1", 200, 999}, {"/admin/odir/browse/private.epub", 200, 999},
		{"/dashboard/journeys", 200, 999}, {"/dashboard/journey/0123456789abcdef", 200, 999},
	} {
		_, err = db.DB.Exec("INSERT INTO access_logs VALUES(?,?,?,?,?,?,?,?,?)", time.Now().UTC(), "GET", row.path, row.status, row.ms, "192.0.2.1", 0, 12, "test")
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnalyticsMetricsAndHistoricalFiltering(t *testing.T) {
	analyticsDatabase(t)
	ctx := context.Background()
	condition := dashboardCondition("24h")
	stats, err := getDashboardStats(ctx, condition)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRequests != 4 || stats.UniqueIPs != 1 || stats.ErrorRate != 25 || math.Abs(stats.AvgResponseTime-3.0625) > 0.00001 {
		t.Fatalf("wrong stats: %+v", stats)
	}
	routes, err := getTopRoutes(ctx, condition, 100)
	if err != nil || len(routes) != 3 || routes[0].URL != "/book" || routes[0].Count != 2 {
		t.Fatalf("routes: %+v %v", routes, err)
	}
	ip, err := getIPStats(ctx, "192.0.2.1", condition)
	if err != nil || ip.TotalRequests != 4 || ip.UniqueRoutes != 3 || ip.ErrorCount != 1 || ip.AvgResponseTime != stats.AvgResponseTime {
		t.Fatalf("IP: %+v %v", ip, err)
	}
	entries, err := getIPAccessLogs(ctx, "192.0.2.1", condition)
	if err != nil || len(entries) != 4 {
		t.Fatalf("entries: %+v %v", entries, err)
	}
	for _, entry := range entries {
		if strings.ContainsAny(entry.URL, "?#") {
			t.Fatal("legacy query string leaked through history API")
		}
	}
	empty, err := getIPStats(ctx, "192.0.2.2", condition)
	if err != nil || empty.TotalRequests != 0 || empty.AvgResponseTime != 0 {
		t.Fatalf("empty: %+v %v", empty, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = getDashboardStats(canceled, condition); err == nil {
		t.Fatal("canceled request still queried")
	}
}

func TestDashboardSharesLibraryGate(t *testing.T) {
	setupLibrary(t)
	analyticsDatabase(t)
	for _, name := range []string{"dashboard.html", "ip_analytics.html"} {
		Templates[name] = template.Must(template.ParseFiles("../templates/" + name))
	}
	handlers := map[string]http.HandlerFunc{"/dashboard": DashboardPageHandler, "/dashboard/ip/192.0.2.1": IPAnalyticsPageHandler, "/api/dashboard": DashboardHandler, "/api/dashboard/ip/192.0.2.1": IPAnalyticsHandler}
	cookie, csrf := loginLibrary(t)
	if cookie.Path != "/" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("bad shared cookie: %+v", cookie)
	}
	for path, handler := range handlers {
		w := httptest.NewRecorder()
		handler(w, libraryRequest("GET", path, nil))
		expected := 303
		if strings.HasPrefix(path, "/api/") {
			expected = 401
		}
		if w.Code != expected || strings.Contains(w.Body.String(), "192.0.2.1\",\"count") {
			t.Fatalf("unguarded %s: %d", path, w.Code)
		}
		r := libraryRequest("GET", path, nil)
		r.AddCookie(cookie)
		w = httptest.NewRecorder()
		handler(w, r)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("signed-in %s: %d %s", path, w.Code, w.Body.String())
		}
		if path == "/api/dashboard" {
			var d DashboardData
			if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || d.Stats.TotalRequests != 4 {
				t.Fatalf("API: %s", w.Body.String())
			}
		}
	}
	r := libraryRequest("POST", "/api/dashboard", nil)
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	DashboardHandler(w, r)
	if w.Code != 405 {
		t.Fatalf("invalid method: %d", w.Code)
	}
	r = libraryRequest("POST", libraryAdminPath+"logout", strings.NewReader(url.Values{"csrf": {csrf}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	r = libraryRequest("GET", "/api/dashboard", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	DashboardHandler(w, r)
	if w.Code != 401 {
		t.Fatal("logout did not revoke dashboard access")
	}
	if err := os.Remove(filepath.Join(config.LibraryDir, libraryauth.PasswordFile)); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	DashboardHandler(w, r)
	if w.Code != 503 {
		t.Fatal("unconfigured dashboard did not fail closed")
	}
}

func TestDashboardLoginDestination(t *testing.T) {
	setupLibrary(t)
	for _, target := range []string{"/dashboard", "/dashboard/ip/192.0.2.1", "https://evil.example/", "//evil.example/", "/dashboard?next=https://evil.example/"} {
		r := libraryRequest("POST", libraryAdminPath+"login", strings.NewReader(url.Values{"password": {testLibraryPassword}, "next": {target}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		LibraryAdminHandler(w, r)
		expected := libraryAdminPath
		if target == "/dashboard" || target == "/dashboard/ip/192.0.2.1" {
			expected = target
		}
		if w.Code != 303 || w.Header().Get("Location") != expected {
			t.Fatalf("target %s: %d %s", target, w.Code, w.Header().Get("Location"))
		}
	}
}
