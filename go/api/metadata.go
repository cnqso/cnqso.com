package api

import (
	"net/http"
)

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	filePath := r.URL.Path[len("/favicon.ico"):]
	http.ServeFile(w, r, "static/favicon.ico"+filePath)
}

func RobotsHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/meta/robots.txt")
}

func SitemapHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/meta/sitemap.xml")
}

func SecurityTxtHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/meta/security.txt")
}
