package logs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const visitorCookie = "cnqso_visitor"
const visitorLifetime = 30 * 24 * time.Hour

type journeyKey struct{}
type journeyMetadata struct {
	VisitorID, Kind, Referrer, Fingerprint string
	Started                                time.Time
}

func validVisitorToken(token string) bool {
	if len(token) != 32 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
func privateAnalyticsPath(path string) bool {
	return path == "/health" || path == "/dashboard" || strings.HasPrefix(path, "/dashboard/") || strings.HasPrefix(path, "/admin/") || path == "/api/dashboard" || strings.HasPrefix(path, "/api/dashboard/")
}
func visitorSecure(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return r.TLS != nil || (host != "localhost" && host != "127.0.0.1" && host != "::1" && host != "[::1]")
}

func prepareJourney(r *http.Request, w http.ResponseWriter) (*http.Request, func(int)) {
	meta := &journeyMetadata{Started: time.Now().UTC()}
	request := r.WithContext(context.WithValue(r.Context(), journeyKey{}, meta))
	// These signals disable browser recognition; ordinary operational logs remain.
	optedOut := r.Header.Get("Sec-GPC") == "1" || r.Header.Get("DNT") == "1"
	if privateAnalyticsPath(r.URL.Path) {
		return request, func(int) {}
	}
	if optedOut {
		if _, err := r.Cookie(visitorCookie); err == nil {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Add("Vary", "Cookie")
			http.SetCookie(w, &http.Cookie{Name: visitorCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: visitorSecure(r), SameSite: http.SameSiteLaxMode})
		}
		return request, func(int) {}
	}
	if cookie, err := r.Cookie(visitorCookie); err == nil && validVisitorToken(cookie.Value) {
		meta.VisitorID = cookie.Value
	}
	return request, func(status int) {
		meta.Kind = journeyKind(r, w.Header(), status)
		if meta.Kind == "" {
			return
		}
		meta.Referrer = journeyReferrer(r)
		meta.Fingerprint = passiveFingerprint(r, meta.Started)
		if meta.VisitorID != "" {
			return
		}
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return
		}
		meta.VisitorID = hex.EncodeToString(token[:])
		http.SetCookie(w, &http.Cookie{Name: visitorCookie, Value: meta.VisitorID, Path: "/", MaxAge: int(visitorLifetime.Seconds()), Expires: time.Now().Add(visitorLifetime), HttpOnly: true, Secure: visitorSecure(r), SameSite: http.SameSiteLaxMode})
		// A response assigning an ID must never be reused for another browser.
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Add("Vary", "Cookie")
	}
}

func journeyKind(r *http.Request, h http.Header, status int) string {
	if r.Method != "GET" || privateAnalyticsPath(r.URL.Path) || (status >= 300 && status < 400 && status != 304) {
		return ""
	}
	media, _, _ := mime.ParseMediaType(h.Get("Content-Type"))
	if media == "text/html" || media == "application/xhtml+xml" {
		return "page"
	}
	if status >= 400 {
		return ""
	}
	disposition, _, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	if disposition == "attachment" || media == "application/pdf" || media == "application/epub+zip" {
		return "download"
	}
	return ""
}

func journeyReferrer(r *http.Request) string {
	raw := r.Referer()
	if len(raw) > 2048 {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	if !strings.EqualFold(u.Host, r.Host) {
		return u.Scheme + "://" + u.Host
	}
	if privateAnalyticsPath(u.Path) {
		return ""
	}
	return u.EscapedPath()
}
