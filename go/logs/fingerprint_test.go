package logs

import (
	"net/http"
	"net/http/httptest"
	"server/config"
	"testing"
	"time"
)

func TestPassiveFingerprintScopeAndOptOut(t *testing.T) {
	old := config.Fingerprinting
	config.Fingerprinting = true
	t.Cleanup(func() { config.Fingerprinting = old })
	at := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	makeRequest := func() *http.Request {
		r := httptest.NewRequest("GET", "https://example.test/odir/", nil)
		r.Header.Set("User-Agent", "ExampleBrowser/1.0")
		r.Header.Set("Accept-Language", "en-US,en;q=0.9")
		return r
	}
	a := makeRequest()
	a.AddCookie(&http.Cookie{Name: visitorCookie, Value: "0123456789abcdef0123456789abcdef"})
	a.RemoteAddr = "192.0.2.1:1234"
	first := passiveFingerprint(a, at)
	if !validVisitorToken(first) {
		t.Fatal("missing fingerprint")
	}
	b := makeRequest()
	b.AddCookie(&http.Cookie{Name: visitorCookie, Value: "ffffffffffffffffffffffffffffffff"})
	b.RemoteAddr = "198.51.100.1:5678"
	if passiveFingerprint(b, at) != first {
		t.Fatal("IP or cookie affected browser signature")
	}
	for _, change := range []func(*http.Request){
		func(r *http.Request) { r.Header.Set("Accept-Language", "fr-FR") },
		func(r *http.Request) { r.Header.Set("User-Agent", "DifferentBrowser/2.0") },
		func(r *http.Request) { r.Host = "other.example" },
	} {
		r := makeRequest()
		change(r)
		if passiveFingerprint(r, at) == first {
			t.Fatal("changed inputs did not change signature")
		}
	}
	if passiveFingerprint(a, at.Add(24*time.Hour)) == first {
		t.Fatal("daily rotation failed")
	}
	for _, path := range []string{"/admin/odir/", "/dashboard/journeys", "/api/dashboard"} {
		r := makeRequest()
		r.URL.Path = path
		if passiveFingerprint(r, at) != "" {
			t.Fatal("fingerprinted admin")
		}
	}
	for _, header := range []string{"DNT", "Sec-GPC"} {
		r := makeRequest()
		r.Header.Set(header, "1")
		if passiveFingerprint(r, at) != "" {
			t.Fatal("ignored opt out")
		}
	}
	config.Fingerprinting = false
	if passiveFingerprint(a, at) != "" {
		t.Fatal("ignored off switch")
	}
}

func TestFingerprintCollectedOnlyForJourneyEvents(t *testing.T) {
	r := httptest.NewRequest("GET", "https://example.test/odir/", nil)
	r.Header.Set("User-Agent", "Browser/1.0")
	w := httptest.NewRecorder()
	request, before := prepareJourney(r, w)
	w.Header().Set("Content-Type", "text/html")
	before(200)
	meta := request.Context().Value(journeyKey{}).(*journeyMetadata)
	if meta.Fingerprint == "" || meta.VisitorID == "" {
		t.Fatal("page missing recognition metadata")
	}
	w = httptest.NewRecorder()
	request, before = prepareJourney(r, w)
	w.Header().Set("Content-Type", "text/css")
	before(200)
	if request.Context().Value(journeyKey{}).(*journeyMetadata).Fingerprint != "" {
		t.Fatal("fingerprinted asset")
	}
}
