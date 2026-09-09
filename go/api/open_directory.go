package api

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"server/config"
	"server/logs"
	"sort"
	"strings"
)

const openDirectoryPapersPath = "/odir/papers/"

type openDirectoryDocument struct {
	Name string
	URL  string
}

func OpenDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/odir/":
		ServeTemplate(w, r, "open_directory.html", nil)
	case "/odir/papers":
		http.Redirect(w, r, openDirectoryPapersPath, http.StatusMovedPermanently)
	case openDirectoryPapersPath:
		documents, err := loadOpenDirectoryDocuments()
		if err != nil {
			logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error reading Open Directory")
			return
		}
		ServeTemplate(w, r, "open_directory_papers.html", struct {
			Documents []openDirectoryDocument
		}{
			Documents: documents,
		})
	case "/odir/FSEX300.ttf":
		serveOpenDirectoryFile(w, r, filepath.Join(config.OpenDirectoryDir, "FSEX300.ttf"))
	default:
		serveOpenDirectoryDocument(w, r)
	}
}

func loadOpenDirectoryDocuments() ([]openDirectoryDocument, error) {
	entries, err := os.ReadDir(filepath.Join(config.OpenDirectoryDir, "papers"))
	if err != nil {
		return nil, err
	}

	documents := make([]openDirectoryDocument, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			continue
		}
		documents = append(documents, openDirectoryDocument{
			Name: entry.Name(),
			URL:  openDirectoryPapersPath + url.PathEscape(entry.Name()),
		})
	}

	sort.Slice(documents, func(i, j int) bool {
		return strings.ToLower(documents[i].Name) < strings.ToLower(documents[j].Name)
	})
	return documents, nil
}

func serveOpenDirectoryDocument(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, openDirectoryPapersPath) {
		FourHundredHandler(w, r, http.StatusNotFound)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, openDirectoryPapersPath)
	if name == "" || filepath.Base(name) != name || !strings.EqualFold(filepath.Ext(name), ".pdf") {
		FourHundredHandler(w, r, http.StatusNotFound)
		return
	}

	serveOpenDirectoryFile(w, r, filepath.Join(config.OpenDirectoryDir, "papers", name))
}

func serveOpenDirectoryFile(w http.ResponseWriter, r *http.Request, path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		FourHundredHandler(w, r, http.StatusNotFound)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, path)
}
