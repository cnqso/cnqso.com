package api

import (
	"net"
	"net/http"
	"net/url"
	"server/config"
	"server/internal/libraryauth"
	"strings"
)

type loginData struct{ Error, Next string }

// Only our own admin pages are valid post-login destinations.
func adminLoginTarget(target string) string {
	if target == "/dashboard" || target == "/dashboard/journeys" {
		return target
	}
	if strings.HasPrefix(target, "/dashboard/ip/") && net.ParseIP(strings.TrimPrefix(target, "/dashboard/ip/")) != nil {
		return target
	}
	if strings.HasPrefix(target, "/dashboard/journey/") && validVisitorID(strings.TrimPrefix(target, "/dashboard/journey/")) {
		return target
	}
	return libraryAdminPath
}

func requireAnalyticsAdmin(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Cookie")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// The existing dashboard uses inline scripts and styles; data is HTML-escaped.
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	hash, err := libraryauth.ReadHash(config.LibraryDir)
	if err != nil {
		http.Error(w, "Administration is not configured.", http.StatusServiceUnavailable)
		return false
	}
	if _, ok := currentLibrarySession(r, hash); ok {
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		http.Error(w, "Please sign in again.", http.StatusUnauthorized)
	} else {
		http.Redirect(w, r, libraryAdminPath+"login?next="+url.QueryEscape(adminLoginTarget(r.URL.Path)), http.StatusSeeOther)
	}
	return false
}
