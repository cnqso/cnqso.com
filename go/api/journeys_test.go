package api

import (
	"database/sql"
	"html/template"
	"net/http/httptest"
	"regexp"
	"server/db"
	"strconv"
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
	_, err = db.DB.Exec(`CREATE TABLE access_logs(id INTEGER PRIMARY KEY,timestamp DATETIME,visitor_id TEXT,event_kind TEXT,url TEXT,referrer TEXT,method TEXT,status_code INTEGER,fingerprint TEXT,user_agent TEXT,client_ip TEXT)`)
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

func TestJourneysFilterBotsAndSinglePageBrowsers(t *testing.T) {
	setupLibrary(t)
	old := db.DB
	var err error
	db.DB, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.DB.SetMaxOpenConns(1)
	t.Cleanup(func() { db.DB.Close(); db.DB = old })
	_, err = db.DB.Exec(`CREATE TABLE access_logs(id INTEGER PRIMARY KEY,timestamp DATETIME,visitor_id TEXT,event_kind TEXT,url TEXT,referrer TEXT,method TEXT,status_code INTEGER,fingerprint TEXT,user_agent TEXT,client_ip TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	Templates["journeys.html"] = template.Must(template.ParseFiles("../templates/journeys.html"))
	browser := "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0"
	visits := []struct {
		id, agent string
		urls      []string
		status    int
	}{
		{"11111111111111111111111111111111", browser, []string{"/", "/blog/"}, 200},
		{"22222222222222222222222222222222", browser, []string{"/"}, 200},
		{"33333333333333333333333333333333", browser, []string{"/", "/.env.local"}, 200},
		{"44444444444444444444444444444444", "Mozilla/5.0 (compatible; Googlebot/2.1)", []string{"/", "/odir/"}, 200},
		{"55555555555555555555555555555555", browser, []string{"/a1", "/a2", "/a3", "/"}, 404},
		{"66666666666666666666666666666666", browser, []string{"/wp-content/x.PHP", "/"}, 200},
		{"77777777777777777777777777777777", browser, []string{"/", "/"}, 200},
		{"88888888888888888888888888888888", browser, []string{"/", "/", "asset:/static/images/secretdoor.png", "asset:/.well-known/appspecific/com.chrome.devtools.json"}, 200},
		{"99999999999999999999999999999999", browser, []string{"/", "/blog/"}, 200},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", browser, []string{"/", "/odir/", "asset:/robots.txt"}, 200},
	}
	insert := func(visitor, kind, u string, status int, agent, ip string) {
		if _, err := db.DB.Exec("INSERT INTO access_logs(timestamp,visitor_id,event_kind,url,method,status_code,user_agent,client_ip) VALUES(?,?,?,?,?,?,?,?)", time.Now().UTC(), visitor, kind, u, "GET", status, agent, ip); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range visits {
		for _, u := range v.urls {
			kind, status := "page", v.status
			if after, ok := strings.CutPrefix(u, "asset:"); ok {
				kind, u = "", after
			}
			if u == "/" || kind == "" {
				status = 200
			}
			insert(v.id, kind, u, status, v.agent, "203.0.113."+v.id[:1])
		}
	}
	// A Chrome-like crawler that fetched every page within a second.
	for i := 0; i < 12; i++ {
		insert("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "page", "/blog/"+strconv.Itoa(i), 200, browser, "198.51.100.1")
	}
	insert("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "", "/static/js/hexagons.js", 200, browser, "198.51.100.1")
	// A crawler without the cookie fetched robots.txt from browser 9's IP today.
	insert("", "", "/robots.txt", 200, browser, "203.0.113.9")
	cookie, _ := loginLibrary(t)
	for query, want := range map[string]string{
		"":                    "1278",
		"?multi=1":            "178",
		"?rendered=1":         "8",
		"?multi=1&rendered=1": "8",
		"?bots=show":          "123456789ab",
		"?bots=show&multi=1":  "13456789ab",
	} {
		r := libraryRequest("GET", "/dashboard/journeys"+query, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		JourneysHandler(w, r)
		got := ""
		for _, c := range "123456789ab" {
			if strings.Contains(w.Body.String(), "Browser "+strings.Repeat(string(c), 8)) {
				got += string(c)
			}
		}
		if got != want {
			t.Fatalf("%q: got browsers %q, want %q", query, got, want)
		}
		if query == "" && (!strings.Contains(w.Body.String(), `href="/dashboard/ip/203.0.113.1"`) || !regexp.MustCompile(` [AP]M E[SD]T`).MatchString(w.Body.String())) {
			t.Fatalf("summary lacks IP or Eastern time: %s", w.Body.String())
		}
	}
}
