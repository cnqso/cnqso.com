package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"server/config"
	"server/internal/libraryauth"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const libraryAdminPath = "/admin/odir/"
const libraryCookie = "cnqso_admin"
const librarySessionLifetime = 12 * time.Hour

type librarySession struct {
	CSRF            string
	Expires         time.Time
	PasswordVersion [32]byte
}

var libraryAuth = struct {
	sync.Mutex
	Sessions map[string]librarySession
	Window   time.Time
	Attempts int
}{Sessions: make(map[string]librarySession)}

func randomLibraryToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func libraryCookieSecure(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return r.TLS != nil || (host != "localhost" && host != "127.0.0.1" && host != "[::1]" && host != "::1")
}

func setLibraryCookie(w http.ResponseWriter, r *http.Request, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: libraryCookie, Value: token, Path: "/",
		MaxAge: age, HttpOnly: true, Secure: libraryCookieSecure(r), SameSite: http.SameSiteStrictMode})
}

func currentLibrarySession(r *http.Request, hash []byte) (librarySession, bool) {
	cookie, err := r.Cookie(libraryCookie)
	if err != nil {
		return librarySession{}, false
	}
	libraryAuth.Lock()
	defer libraryAuth.Unlock()
	session, ok := libraryAuth.Sessions[cookie.Value]
	if !ok || time.Now().After(session.Expires) || session.PasswordVersion != sha256.Sum256(hash) {
		delete(libraryAuth.Sessions, cookie.Value)
		return librarySession{}, false
	}
	return session, true
}

func librarySameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host && (u.Scheme == "https" || (!libraryCookieSecure(r) && u.Scheme == "http"))
}

func libraryLogin(w http.ResponseWriter, r *http.Request, hash []byte) {
	if r.Method == http.MethodGet {
		ServeTemplate(w, r, "library_login.html", loginData{Next: adminLoginTarget(r.URL.Query().Get("next"))})
		return
	}
	if r.Method != http.MethodPost {
		libraryMethodNotAllowed(w, "GET, POST")
		return
	}
	if !librarySameOrigin(r) {
		http.Error(w, "Please sign in from this site.", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid sign-in form.", http.StatusBadRequest)
		return
	}
	libraryAuth.Lock()
	now := time.Now()
	if now.Sub(libraryAuth.Window) >= time.Minute {
		libraryAuth.Window = now
		libraryAuth.Attempts = 0
	}
	allowed := libraryAuth.Attempts < 10
	if allowed {
		libraryAuth.Attempts++
	}
	libraryAuth.Unlock()
	if !allowed {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Too many sign-in attempts. Try again in a minute.", http.StatusTooManyRequests)
		return
	}
	password := r.PostForm.Get("password")
	if len(password) > 72 || bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		ServeTemplate(w, r, "library_login.html", loginData{Error: "Incorrect password.", Next: adminLoginTarget(r.PostForm.Get("next"))})
		return
	}
	token := randomLibraryToken()
	session := librarySession{CSRF: randomLibraryToken(), Expires: now.Add(librarySessionLifetime), PasswordVersion: sha256.Sum256(hash)}
	libraryAuth.Lock()
	for key, s := range libraryAuth.Sessions {
		if now.After(s.Expires) {
			delete(libraryAuth.Sessions, key)
		}
	}
	// Bound memory even if many browsers sign in successfully.
	if len(libraryAuth.Sessions) >= 16 {
		var oldestKey string
		oldest := now.Add(librarySessionLifetime)
		for key, s := range libraryAuth.Sessions {
			if !s.Expires.After(oldest) {
				oldestKey, oldest = key, s.Expires
			}
		}
		delete(libraryAuth.Sessions, oldestKey)
	}
	libraryAuth.Sessions[token] = session
	libraryAuth.Unlock()
	setLibraryCookie(w, r, token, int(librarySessionLifetime.Seconds()))
	http.Redirect(w, r, adminLoginTarget(r.PostForm.Get("next")), http.StatusSeeOther)
}

func LibraryAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Cookie")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; font-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	hash, err := libraryauth.ReadHash(config.LibraryDir)
	if err != nil {
		http.Error(w, "Library administration is not configured.", http.StatusServiceUnavailable)
		return
	}
	if r.URL.Path == libraryAdminPath+"login" {
		libraryLogin(w, r, hash)
		return
	}
	session, ok := currentLibrarySession(r, hash)
	if !ok {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Please sign in again.", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, libraryAdminPath+"login", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		if !librarySameOrigin(r) {
			http.Error(w, "Invalid request origin.", http.StatusForbidden)
			return
		}
		libraryMutation(w, r, session)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		libraryMethodNotAllowed(w, "GET, HEAD, POST")
		return
	}
	// Forms have fixed endpoints; all browsing lives under /browse/.
	if r.URL.Path == libraryAdminPath {
		libraryBrowse(w, r, true, "", session.CSRF)
		return
	}
	if strings.HasPrefix(r.URL.Path, libraryAdminPath+"browse/") {
		libraryBrowse(w, r, true, strings.TrimPrefix(r.URL.Path, libraryAdminPath+"browse/"), session.CSRF)
		return
	}
	http.NotFound(w, r)
}

func libraryValidCSRF(got, want string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
