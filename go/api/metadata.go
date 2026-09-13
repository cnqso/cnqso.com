package api

import (
	"encoding/xml"
	"net/http"
	"server/logs"
)

const siteOrigin = "https://cnqso.com"

// sitemapPages lists public pages that should be crawled, in addition to blog posts.
var sitemapPages = []string{
	"/",
	"/blog/",
	"/bloonsbench/",
	"/esotericbench/",
	"/spirals/",
	"/hexagons",
	"/l8",
	"/splits",
	"/reverse-wordle-solver",
	"/piano-flashcards",
	"/odir/",
	"/petrarchive/",
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	filePath := r.URL.Path[len("/favicon.ico"):]
	http.ServeFile(w, r, "static/favicon.ico"+filePath)
}

func RobotsHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/meta/robots.txt")
}

func SitemapHandler(w http.ResponseWriter, r *http.Request) {
	body, err := buildSitemap()
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error building sitemap")
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(body)
}

func buildSitemap() ([]byte, error) {
	posts, err := loadBlogPosts()
	if err != nil {
		return nil, err
	}

	set := sitemapURLSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, page := range sitemapPages {
		entry := sitemapURL{Loc: siteOrigin + page}
		if page == "/blog/" && len(posts) > 0 {
			entry.LastMod = posts[0].Date.Format("2006-01-02")
		}
		set.URLs = append(set.URLs, entry)
	}
	for _, post := range posts {
		set.URLs = append(set.URLs, sitemapURL{
			Loc:     siteOrigin + "/blog/" + post.Slug,
			LastMod: post.Date.Format("2006-01-02"),
		})
	}

	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(body, '\n')...), nil
}

func SecurityTxtHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/meta/security.txt")
}
