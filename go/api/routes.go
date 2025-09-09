package api

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"server/db"
	"server/logs"
	"strconv"
	"strings"
	"time"
)

var (
	Templates map[string]*template.Template
)

func ServeTemplate(w http.ResponseWriter, r *http.Request, templateName string, data any) {

	var err error
	if Templates[templateName] != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = Templates[templateName].Execute(w, data)
	} else {
		err = FourHundredHandler(w, r, 404)
	}
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error serving template")
	}
}

func isURLPathErrorCode(path string) bool {
	digits, err := strconv.Atoi(path[1:])
	if err != nil {
		return false
	}
	codeText := http.StatusText(digits)
	if codeText == "" {
		return false
	}
	return true
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path == "/" {
		homeHandler(w, r)
	} else if isURLPathErrorCode(r.URL.Path) {
		errorCode, err := strconv.Atoi(r.URL.Path[1:])
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error serving template")
		}
		err = FourHundredHandler(w, r, errorCode)
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error serving template")
		}
	} else {
		err := FourHundredHandler(w, r, 404)
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error serving template")
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	routes := []string{
		"/hexagons",
		"/splits",
		"/petrarchive/",
	}
	ServeTemplate(w, r, "index.html", struct {
		Routes []string
	}{
		Routes: routes,
	})
}

func FakeNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	FourHundredHandler(w, r, 200)
}

func FourHundredHandler(w http.ResponseWriter, r *http.Request, statusCode int) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	subtitle := http.StatusText(statusCode)
	data := struct {
		Title    string
		Subtitle string
	}{
		Title:    fmt.Sprintf("%d", statusCode),
		Subtitle: subtitle,
	}
	err := Templates["40X.html"].Execute(w, data)
	return err
}

func BlogHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/blog" {
		ServeTemplate(w, r, "blog.html", nil)
		return
	}
}

func HexagonsHandler(w http.ResponseWriter, r *http.Request) {
	ServeTemplate(w, r, "hexagons.html", nil)
}

func SplitsHandler(w http.ResponseWriter, r *http.Request) {
	ServeTemplate(w, r, "splits.html", nil)
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	filePath := r.URL.Path[len("/static/"):]

	if filepath.Ext(filePath) == "" && !strings.HasPrefix(filePath, "petrarchive/") {
		FourHundredHandler(w, r, 403)
		return
	}
	http.ServeFile(w, r, "static/"+filePath)
}

type ArchivePost struct {
	ID        string
	Date      time.Time
	Title     string
	Poster    string
	Contents  string
	IsOP      bool
	Thread    string
	Replies   int
	ImagePath string
	ImageURL  string
}

func (p ArchivePost) EST() time.Time {
	return p.Date.Add(-4 * time.Hour)
}

func ArchiveHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/petrarchive/" || r.URL.Path == "/petrarchive" {
		rows, err := db.DB.Query(`
			SELECT p.id,
				p.date,
				p.title,
				p.poster,
				p.contents,
				p.thread,
				p.replies,
				p.image_path
			FROM posts p
			JOIN (
				SELECT thread, MAX(date) AS latest_date
				FROM posts
				GROUP BY thread
			) t ON p.thread = t.thread
			WHERE p.thread_owner = 1
			ORDER BY t.latest_date DESC;
		`)
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error querying threads")
			return
		}
		defer rows.Close()

		var threads []ArchivePost
		for rows.Next() {
			var post ArchivePost
			var imagePath *string
			err := rows.Scan(&post.ID, &post.Date, &post.Title, &post.Poster,
				&post.Contents, &post.Thread, &post.Replies, &imagePath)
			if err != nil {
				logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error scanning thread")
				return
			}

			post.IsOP = true
			if imagePath != nil && *imagePath != "" {
				post.ImageURL = "/static/" + strings.TrimPrefix(*imagePath, "static/")
			}

			threads = append(threads, post)
		}

		ServeTemplate(w, r, "archive_catalog.html", struct {
			Threads []ArchivePost
		}{
			Threads: threads,
		})
		return
	}

	if strings.HasPrefix(r.URL.Path, "/petrarchive/thread/") {
		threadID := strings.TrimPrefix(r.URL.Path, "/petrarchive/thread/")
		if threadID == "" {
			FourHundredHandler(w, r, 404)
			return
		}

		ThreadHandler(w, r, threadID)
		return
	}

	FourHundredHandler(w, r, 404)
}

func ThreadHandler(w http.ResponseWriter, r *http.Request, threadID string) {
	if _, err := strconv.Atoi(threadID); err != nil {
		FourHundredHandler(w, r, 404)
		return
	}

	rows, err := db.DB.Query(`
		SELECT id, date, title, poster, contents, thread_owner, image_path
		FROM posts
		WHERE thread = ?
		ORDER BY date ASC
	`, threadID)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error querying thread")
		return
	}
	defer rows.Close()

	var posts []ArchivePost
	var threadTitle string
	for rows.Next() {
		var post ArchivePost
		var imagePath *string
		err := rows.Scan(&post.ID, &post.Date, &post.Title, &post.Poster,
			&post.Contents, &post.IsOP, &imagePath)
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error scanning post")
			return
		}

		post.Thread = threadID
		if imagePath != nil && *imagePath != "" {
			post.ImageURL = "/static/" + strings.TrimPrefix(*imagePath, "static/")
		}

		if post.IsOP && post.Title != "" {
			threadTitle = post.Title
		}

		posts = append(posts, post)
	}

	if len(posts) == 0 {
		FourHundredHandler(w, r, 404)
		return
	}

	ServeTemplate(w, r, "archive_thread.html", struct {
		ThreadID    string
		ThreadTitle string
		Posts       []ArchivePost
	}{
		ThreadID:    threadID,
		ThreadTitle: threadTitle,
		Posts:       posts,
	})
}
