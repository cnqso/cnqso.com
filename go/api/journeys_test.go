package api

import (
	"database/sql"
	"html/template"
	"net/http/httptest"
	"server/db"
	"strings"
	"testing"
	"time"
)

func TestJourneyVisitsAndPDFChunks(t *testing.T) {
	start := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	events := []journeyEvent{
		{At: start.Add(time.Hour), Kind: "page", URL: "/next", Status: 200},
		{At: start.Add(2 * time.Minute), Kind: "download", URL: "/book.pdf", Status: 206},
		{At: start.Add(time.Minute), Kind: "download", URL: "/book.pdf", Status: 206},
		{At: start, Kind: "page", URL: "/", Status: 200},
	}
	visits := groupJourney(events, false)
	if len(visits) != 2 || len(visits[0].Events) != 2 || visits[0].Events[1].Repeat != 2 || visits[0].Events[0].URL != "/" {
		t.Fatalf("visits %+v", visits)
	}
	raw := groupJourney(events, true)
	if len(raw[0].Events) != 3 {
		t.Fatal("raw requests collapsed")
	}
}

func TestJourneysArePrivateAndPaginateWithoutMixingBrowsers(t *testing.T) {
	setupLibrary(t)
	old := db.DB
	var err error
	db.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.DB.SetMaxOpenConns(1)
	t.Cleanup(func() { db.DB.Close(); db.DB = old })
	_, err = db.DB.Exec(`CREATE TABLE access_logs(id INTEGER PRIMARY KEY,timestamp DATETIME,visitor_id TEXT,event_kind TEXT,url TEXT,referrer TEXT,method TEXT,status_code INTEGER,fingerprint TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"journey.html", "journeys.html"} {
		Templates[name] = template.Must(template.ParseFiles("../templates/" + name))
	}
	id := "0123456789abcdef0123456789abcdef"
	for i := 0; i < 205; i++ {
		_, err = db.DB.Exec("INSERT INTO access_logs(timestamp,visitor_id,event_kind,url,referrer,method,status_code) VALUES(?,?,?,?,?,?,?)", time.Now().UTC().Add(time.Duration(i-300)*time.Second), id, "page", "/page", "<script>bad</script>", "GET", 200)
		if err != nil {
			t.Fatal(err)
		}
	}
	db.DB.Exec("INSERT INTO access_logs(timestamp,visitor_id,event_kind,url,referrer,method,status_code) VALUES(?,?,?,?,?,?,?)", time.Now().UTC(), "ffffffffffffffffffffffffffffffff", "page", "/other-browser", "", "GET", 200)
	w := httptest.NewRecorder()
	JourneyHandler(w, libraryRequest("GET", "/dashboard/journey/"+id, nil))
	if w.Code != 303 {
		t.Fatal("unguarded journey")
	}
	cookie, _ := loginLibrary(t)
	for _, path := range []string{"/dashboard/journeys", "/dashboard/journey/" + id, "/dashboard/journey/" + id + "?before=6"} {
		r := libraryRequest("GET", path, nil)
		r.AddCookie(cookie)
		w = httptest.NewRecorder()
		if path == "/dashboard/journeys" {
			JourneysHandler(w, r)
		} else {
			JourneyHandler(w, r)
		}
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<script>bad</script>") {
			t.Fatal("unescaped event data")
		}
		if path == "/dashboard/journey/"+id {
			if strings.Contains(w.Body.String(), "/other-browser") || !strings.Contains(w.Body.String(), "before=6") {
				t.Fatal("wrong pagination or mixed browsers")
			}
		}
		if strings.Contains(path, "before=6") && strings.Count(w.Body.String(), "GET /page") != 5 {
			t.Fatal("older page did not contain remaining events")
		}
	}
}
