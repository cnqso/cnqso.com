package api

import (
	"bytes"
	"crypto/sha256"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"server/config"
	"server/internal/libraryauth"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const testLibraryPassword = "test-only-library-password"

func setupLibrary(t *testing.T) {
	t.Helper()
	oldDir, oldTemplates := config.LibraryDir, Templates
	config.LibraryDir = t.TempDir()
	t.Cleanup(func() { config.LibraryDir = oldDir; Templates = oldTemplates })
	for _, dir := range []string{"public/papers", "Fiction"} {
		if err := os.MkdirAll(filepath.Join(config.LibraryDir, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range map[string]string{
		"private.epub": "private book", "public/papers/A & B #1 + 100%.PDF": "%PDF-1.4\n0123456789",
		"public/papers/book.epub": "public ebook", "public/papers/notes.txt": "public notes",
		"public/papers/.hidden": "hidden", "public/papers/active.html": "<script>alert(1)</script>",
	} {
		if err := os.WriteFile(filepath.Join(config.LibraryDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(testLibraryPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.LibraryDir, libraryauth.PasswordFile), hash, 0600); err != nil {
		t.Fatal(err)
	}
	Templates = map[string]*template.Template{}
	for _, name := range []string{"open_directory.html", "library_login.html"} {
		Templates[name] = template.Must(template.ParseFiles("../templates/" + name))
	}
	libraryAuth.Lock()
	libraryAuth.Sessions = make(map[string]librarySession)
	libraryAuth.Attempts = 0
	libraryAuth.Window = time.Time{}
	libraryAuth.Unlock()
}

func libraryRequest(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, "http://localhost"+target, body)
	r.Header.Set("Origin", "http://localhost")
	return r
}
func loginLibrary(t *testing.T) (*http.Cookie, string) {
	t.Helper()
	r := libraryRequest("POST", libraryAdminPath+"login", strings.NewReader(url.Values{"password": {testLibraryPassword}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if w.Code != 303 {
		t.Fatalf("login %d: %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing session cookie")
	}
	r = libraryRequest("GET", libraryAdminPath, nil)
	r.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	match := regexp.MustCompile("name=\"csrf\" value=\"([^\"]+)\"").FindStringSubmatch(w.Body.String())
	if len(match) != 2 {
		t.Fatalf("missing CSRF: %d %s", w.Code, w.Body.String())
	}
	return cookies[0], match[1]
}
func uploadRequest(t *testing.T, cookie *http.Cookie, csrf, destination, name, body string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("csrf", csrf)
	writer.WriteField("destination", destination)
	f, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(f, body)
	writer.Close()
	r := libraryRequest("POST", libraryAdminPath+"upload", &buf)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	if cookie != nil {
		r.AddCookie(cookie)
	}
	return r
}

func TestLibraryPublicBoundary(t *testing.T) {
	setupLibrary(t)
	os.Symlink("../../private.epub", filepath.Join(config.LibraryDir, "public/papers/leak.epub"))
	os.Symlink(t.TempDir(), filepath.Join(config.LibraryDir, "public/outside"))
	for _, target := range []string{
		"/odir/private.epub", "/odir/papers/.hidden", "/odir/papers/leak.epub",
		"/odir/outside/file", "/odir/../private.epub", "/odir/%2e%2e/private.epub",
		"/odir/%2e%2e%2fprivate.epub", "/odir/public/../private.epub", "/odir/.admin-password",
		"/odir/papers/..%5cprivate.epub",
	} {
		t.Run(target, func(t *testing.T) {
			w := httptest.NewRecorder()
			OpenDirectoryHandler(w, libraryRequest("GET", target, nil))
			if w.Code != 404 {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "private book") {
				t.Fatal("private content leaked")
			}
		})
	}
	w := httptest.NewRecorder()
	OpenDirectoryHandler(w, libraryRequest("GET", "/odir/papers/", nil))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "book.epub") || !strings.Contains(body, "notes.txt") {
		t.Fatalf("listing: %d %s", w.Code, body)
	}
	for _, hidden := range []string{".hidden", "leak.epub", "private.epub", ".admin-password"} {
		if strings.Contains(body, hidden) {
			t.Fatalf("listed %s", hidden)
		}
	}
}

func TestLibraryDownloadBehavior(t *testing.T) {
	setupLibrary(t)
	name := url.PathEscape("A & B #1 + 100%.PDF")
	for _, tc := range []struct {
		method, target, rangeHeader string
		status                      int
	}{
		{"GET", "/odir/papers", "", 301},
		{"GET", "/odir/papers/" + name, "", 200},
		{"HEAD", "/odir/papers/" + name, "", 200},
		{"GET", "/odir/papers/" + name, "bytes=0-3", 206},
		{"POST", "/odir/papers/book.epub", "", 405},
		{"GET", "/odir/papers/book.epub/", "", 404},
	} {
		r := libraryRequest(tc.method, tc.target, nil)
		r.Header.Set("Range", tc.rangeHeader)
		w := httptest.NewRecorder()
		OpenDirectoryHandler(w, r)
		if w.Code != tc.status {
			t.Errorf("%s %s: %d", tc.method, tc.target, w.Code)
		}
		if tc.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD has body")
		}
		if tc.status == 206 && w.Body.String() != "%PDF" {
			t.Fatal("invalid range")
		}
	}
	w := httptest.NewRecorder()
	OpenDirectoryHandler(w, libraryRequest("GET", "/odir/papers/active.html", nil))
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("active content not isolated")
	}
}

func TestLibraryAuthentication(t *testing.T) {
	setupLibrary(t)
	w := httptest.NewRecorder()
	LibraryAdminHandler(w, libraryRequest("GET", libraryAdminPath+"browse/private.epub", nil))
	if w.Code != 303 || strings.Contains(w.Body.String(), "private book") {
		t.Fatal("private download did not require login")
	}
	cookie, csrf := loginLibrary(t)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatal("incorrect cookie policy")
	}
	r := libraryRequest("GET", libraryAdminPath+"browse/private.epub", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if w.Code != 200 || w.Body.String() != "private book" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private download failed")
	}
	r = libraryRequest("POST", libraryAdminPath+"logout", strings.NewReader(url.Values{"csrf": {csrf}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if w.Code != 303 {
		t.Fatal("logout failed")
	}
	r = libraryRequest("GET", libraryAdminPath, nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if w.Code != 303 {
		t.Fatal("logged-out session accepted")
	}
}

func TestLibraryPasswordChangeAndExpiry(t *testing.T) {
	setupLibrary(t)
	cookie, _ := loginLibrary(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("a different password"), bcrypt.MinCost)
	os.WriteFile(filepath.Join(config.LibraryDir, libraryauth.PasswordFile), hash, 0600)
	if _, ok := currentLibrarySession(func() *http.Request { r := libraryRequest("GET", libraryAdminPath, nil); r.AddCookie(cookie); return r }(), hash); ok {
		t.Fatal("password change did not revoke session")
	}
	libraryAuth.Lock()
	libraryAuth.Sessions[cookie.Value] = librarySession{Expires: time.Now().Add(-time.Minute), PasswordVersion: sha256.Sum256(hash)}
	libraryAuth.Unlock()
	r := libraryRequest("GET", libraryAdminPath, nil)
	r.AddCookie(cookie)
	if _, ok := currentLibrarySession(r, hash); ok {
		t.Fatal("expired session accepted")
	}
}

func TestLibraryLoginRateLimitAndOrigin(t *testing.T) {
	setupLibrary(t)
	for i := 0; i < 11; i++ {
		r := libraryRequest("POST", libraryAdminPath+"login", strings.NewReader("password=wrong"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		LibraryAdminHandler(w, r)
		expected := 401
		if i == 10 {
			expected = 429
		}
		if w.Code != expected {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	r := libraryRequest("POST", libraryAdminPath+"login", strings.NewReader("password="+testLibraryPassword))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://elsewhere.example")
	w := httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin login accepted")
	}
	r = httptest.NewRequest("GET", "https://cnqso.com/admin/odir/", nil)
	w = httptest.NewRecorder()
	setLibraryCookie(w, r, "test", 60)
	if !w.Result().Cookies()[0].Secure {
		t.Fatal("production cookie is not secure")
	}
}

func TestLibraryUploadAndFolders(t *testing.T) {
	setupLibrary(t)
	cookie, csrf := loginLibrary(t)
	for _, destination := range []string{"Fiction", "public/papers"} {
		w := httptest.NewRecorder()
		LibraryAdminHandler(w, uploadRequest(t, cookie, csrf, destination, "Novel.epub", "new ebook"))
		if w.Code != 303 {
			t.Fatalf("upload %s: %d %s", destination, w.Code, w.Body.String())
		}
		data, err := os.ReadFile(filepath.Join(config.LibraryDir, destination, "Novel.epub"))
		if err != nil || string(data) != "new ebook" {
			t.Fatal("file missing")
		}
		w = httptest.NewRecorder()
		LibraryAdminHandler(w, uploadRequest(t, cookie, csrf, destination, "Novel.epub", "overwrite"))
		if w.Code != 409 {
			t.Fatal("duplicate did not conflict")
		}
		data, _ = os.ReadFile(filepath.Join(config.LibraryDir, destination, "Novel.epub"))
		if string(data) != "new ebook" {
			t.Fatal("existing file overwritten")
		}
	}
	r := libraryRequest("POST", libraryAdminPath+"mkdir", strings.NewReader(url.Values{"csrf": {csrf}, "parent": {"public"}, "name": {"Sci Fi"}}.Encode()))
	r.AddCookie(cookie)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if w.Code != 303 {
		t.Fatalf("mkdir: %d %s", w.Code, w.Body.String())
	}
	if info, err := os.Stat(filepath.Join(config.LibraryDir, "public/Sci Fi")); err != nil || !info.IsDir() {
		t.Fatal("folder missing")
	}
	r = libraryRequest("GET", libraryAdminPath+"browse/public/", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if !strings.Contains(w.Body.String(), `value="" selected`) {
		t.Fatal("upload did not default to private")
	}
}

func TestLibraryRejectsUnsafeWrites(t *testing.T) {
	setupLibrary(t)
	cookie, csrf := loginLibrary(t)
	os.Symlink(t.TempDir(), filepath.Join(config.LibraryDir, "escape"))
	for _, tc := range []struct {
		cookie                          *http.Cookie
		csrf, destination, name, origin string
		status                          int
	}{
		{nil, csrf, "", "x.epub", "", 401},
		{cookie, "bad", "", "x.epub", "", 403},
		{cookie, csrf, "../", "x.epub", "", 400},
		{cookie, csrf, "", "../x.epub", "", 400},
		{cookie, csrf, "", ".admin-password", "", 400},
		{cookie, csrf, "escape", "x.epub", "", 404},
		{cookie, csrf, "", "x.epub", "https://elsewhere.example", 403},
	} {
		r := uploadRequest(t, tc.cookie, tc.csrf, tc.destination, tc.name, "content")
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		LibraryAdminHandler(w, r)
		if w.Code != tc.status {
			t.Errorf("name=%s destination=%s: got %d want %d (%s)", tc.name, tc.destination, w.Code, tc.status, w.Body.String())
		}
	}
}

type libraryZeroReader struct{}

func (libraryZeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
func TestLibraryOversizedUploadLeavesNoFile(t *testing.T) {
	setupLibrary(t)
	cookie, csrf := loginLibrary(t)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("csrf", csrf)
	writer.WriteField("destination", "")
	writer.CreateFormFile("file", "oversized.epub")
	prefix := buf.String()
	writer.Close()
	suffix := buf.String()[len(prefix):]
	r := libraryRequest("POST", libraryAdminPath+"upload", io.MultiReader(strings.NewReader(prefix), io.LimitReader(libraryZeroReader{}, libraryUploadLimit+1), strings.NewReader(suffix)))
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	LibraryAdminHandler(w, r)
	if w.Code != 413 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	entries, _ := os.ReadDir(config.LibraryDir)
	for _, entry := range entries {
		if entry.Name() == "oversized.epub" || strings.HasPrefix(entry.Name(), ".upload-") {
			t.Fatal("partial file left behind")
		}
	}
}
