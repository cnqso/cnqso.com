package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, handler http.HandlerFunc, header map[string]string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/file", nil)
	for name, value := range header {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	Gzip(handler)(recorder, request)
	return recorder.Result()
}

func text(contentType, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		io.WriteString(w, body)
	}
}

func TestGzipCompressesText(t *testing.T) {
	body := strings.Repeat("let x = 1;\n", 200)
	response := serve(t, text("text/javascript; charset=utf-8", body), map[string]string{"Accept-Encoding": "br, gzip"})
	if response.Header.Get("Content-Encoding") != "gzip" || response.Header.Get("Vary") != "Accept-Encoding" {
		t.Fatalf("unexpected headers: %v", response.Header)
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := io.ReadAll(reader)
	if string(decoded) != body {
		t.Fatal("decoded body does not match")
	}
}

func TestGzipLeavesOtherResponsesAlone(t *testing.T) {
	for name, test := range map[string]struct {
		handler http.HandlerFunc
		header  map[string]string
	}{
		"no gzip accepted": {text("text/css", "body{}"), map[string]string{"Accept-Encoding": "br"}},
		"gzip refused":     {text("text/css", "body{}"), map[string]string{"Accept-Encoding": "gzip;q=0"}},
		"image":            {text("image/png", "\x89PNG"), map[string]string{"Accept-Encoding": "gzip"}},
		"not ok": {func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotModified)
		}, map[string]string{"Accept-Encoding": "gzip"}},
	} {
		if response := serve(t, test.handler, test.header); response.Header.Get("Content-Encoding") != "" {
			t.Errorf("%s: compressed unexpectedly", name)
		}
	}
}

func TestGzipKeepsRangesExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("0123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	serveFile := func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "notes.txt", time.Time{}, file)
	}
	response := serve(t, serveFile, map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=2-4"})
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusPartialContent || response.Header.Get("Content-Encoding") != "" || string(body) != "234" {
		t.Fatalf("range response changed: %d %q %v", response.StatusCode, body, response.Header)
	}
}
