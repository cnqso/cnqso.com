package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestStaticHandlerCachesVersionedFilesForAYear(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("static/css", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("static/css/site.css", []byte("body{}"), 0644); err != nil {
		t.Fatal(err)
	}
	for url, want := range map[string]string{
		"/static/css/site.css?v=3":    "public, max-age=31536000, immutable",
		"/static/css/site.css":        "public, max-age=86400",
		"/static/css/missing.css?v=3": "", // errors are never cached
	} {
		recorder := httptest.NewRecorder()
		StaticHandler(recorder, httptest.NewRequest(http.MethodGet, url, nil))
		if got := recorder.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control %q, want %q", url, got, want)
		}
	}
}
