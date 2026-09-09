package api

import (
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"server/config"
	"sort"
	"strings"
	"unicode"
)

type libraryEntry struct {
	Name, URL, Size string
	IsDir, Public   bool
}
type libraryLocation struct {
	Label, URL, Path string
	Selected         bool
}
type libraryPage struct {
	Admin, Public              bool
	Path, Label, CSRF, Message string
	Entries                    []libraryEntry
	Breadcrumbs, Destinations  []libraryLocation
}

func libraryMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
}

// os.Root independently confines filesystem operations, including symlink races.
func validLibraryPath(name string) bool {
	if name == "" {
		return true
	}
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || strings.TrimSpace(part) != part || len(part) > 255 {
			return false
		}
		for _, c := range part {
			if unicode.IsControl(c) {
				return false
			}
		}
	}
	return true
}
func libraryDiskPath(name string) string {
	if name == "" {
		return "."
	}
	return name
}
func libraryIsPublic(name string) bool { return name == "public" || strings.HasPrefix(name, "public/") }

func openLibraryRoot(public bool) (*os.Root, error) {
	root, err := os.OpenRoot(config.LibraryDir)
	if err != nil {
		return nil, err
	}
	if !public {
		return root, nil
	}
	defer root.Close()
	info, err := root.Lstat("public")
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fs.ErrNotExist
	}
	return root.OpenRoot("public")
}

func libraryNoSymlinks(root *os.Root, name string) error {
	if name == "" {
		return nil
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fs.ErrNotExist
		}
	}
	return nil
}

func libraryURL(admin bool, name string, directory bool) string {
	prefix := "/odir/"
	if admin {
		prefix = libraryAdminPath + "browse/"
		if name == "" {
			return libraryAdminPath
		}
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	result := prefix + strings.Join(parts, "/")
	if directory && !strings.HasSuffix(result, "/") {
		result += "/"
	}
	return result
}

func OpenDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		libraryMethodNotAllowed(w, "GET, HEAD")
		return
	}
	if r.URL.Path == "/odir/FSEX300.ttf" {
		http.Redirect(w, r, "/static/fonts/FSEX300.ttf", http.StatusMovedPermanently)
		return
	}
	libraryBrowse(w, r, false, strings.TrimPrefix(r.URL.Path, "/odir/"), "")
}

func libraryBrowse(w http.ResponseWriter, r *http.Request, admin bool, relative, csrf string) {
	name := strings.TrimSuffix(relative, "/")
	if !validLibraryPath(name) {
		http.NotFound(w, r)
		return
	}
	root, err := openLibraryRoot(!admin)
	if err != nil {
		http.Error(w, "The library is unavailable.", http.StatusServiceUnavailable)
		return
	}
	defer root.Close()
	if err := libraryNoSymlinks(root, name); err != nil {
		http.NotFound(w, r)
		return
	}
	file, err := root.Open(libraryDiskPath(name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.Mode().IsRegular() {
		if strings.HasSuffix(relative, "/") {
			http.NotFound(w, r)
			return
		}
		// Uploaded HTML, SVG and scripts cannot execute in the site's origin.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Cache-Control", "no-store")
		disposition := "attachment"
		if strings.EqualFold(path.Ext(info.Name()), ".pdf") {
			disposition = "inline"
			w.Header().Set("Content-Type", "application/pdf")
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": info.Name()}))
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
		return
	}
	if !info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/") {
		http.Redirect(w, r, libraryURL(admin, name, true), http.StatusMovedPermanently)
		return
	}
	entries, err := file.ReadDir(-1)
	if err != nil {
		http.Error(w, "Could not read this folder.", http.StatusInternalServerError)
		return
	}
	page := libraryPage{Admin: admin, Public: !admin || libraryIsPublic(name), Path: name, CSRF: csrf}
	page.Label = libraryLabel(admin, name)
	rootLabel := "Public"
	if admin {
		rootLabel = "Library"
	}
	page.Breadcrumbs = []libraryLocation{{Label: rootLabel, URL: libraryURL(admin, "", true)}}
	if name != "" {
		parts := strings.Split(name, "/")
		for i, part := range parts {
			page.Breadcrumbs = append(page.Breadcrumbs, libraryLocation{Label: part, URL: libraryURL(admin, strings.Join(parts[:i+1], "/"), true)})
		}
	}
	for _, entry := range entries {
		child := path.Join(name, entry.Name())
		if !validLibraryPath(child) || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		stat, err := root.Lstat(child)
		if err != nil || (!stat.IsDir() && !stat.Mode().IsRegular()) {
			continue
		}
		size := ""
		if !stat.IsDir() {
			size = libraryFileSize(stat.Size())
		}
		page.Entries = append(page.Entries, libraryEntry{Name: entry.Name(), URL: libraryURL(admin, child, stat.IsDir()), Size: size, IsDir: stat.IsDir(), Public: !admin || libraryIsPublic(child)})
	}
	sort.Slice(page.Entries, func(i, j int) bool {
		a, b := page.Entries[i], page.Entries[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		if strings.EqualFold(a.Name, b.Name) {
			return a.Name < b.Name
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	if admin {
		page.Destinations, err = libraryDestinations(root, name)
		if err != nil {
			http.Error(w, "Could not read library folders.", http.StatusInternalServerError)
			return
		}
		switch r.URL.Query().Get("done") {
		case "upload":
			page.Message = "File uploaded."
		case "folder":
			page.Message = "Folder created."
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	ServeTemplate(w, r, "open_directory.html", page)
}

func libraryLabel(admin bool, name string) string {
	if !admin {
		if name == "" {
			return "Public"
		}
		return "Public / " + name
	}
	if libraryIsPublic(name) {
		if name == "public" {
			return "Public"
		}
		return "Public / " + strings.TrimPrefix(name, "public/")
	}
	if name == "" {
		return "Private"
	}
	return "Private / " + name
}

func libraryDestinations(root *os.Root, current string) ([]libraryLocation, error) {
	// Default to private storage, including when browsing public files.
	selected := current
	if libraryIsPublic(selected) {
		selected = ""
	}
	locations := []libraryLocation{{Label: "Private / Library", Path: "", Selected: selected == ""}}
	err := fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if !validLibraryPath(name) || d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			locations = append(locations, libraryLocation{Label: libraryLabel(true, name), Path: name, Selected: name == selected})
		}
		return nil
	})
	return locations, err
}

func libraryFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func libraryWriteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fs.ErrExist):
		http.Error(w, "That name already exists. Choose another name; existing files are never overwritten.", http.StatusConflict)
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "That folder no longer exists.", http.StatusNotFound)
	default:
		http.Error(w, "Could not save to this folder.", http.StatusInternalServerError)
	}
}
