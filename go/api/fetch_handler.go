package api

import (
	"net/http"
	"os"
	"path/filepath"
	"server/logs"
	"server/middleware"
	"strconv"
	"strings"
)

func FetchHandler(w http.ResponseWriter, r *http.Request) {
	middleware.SetCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		logs.HTTPError(w, r, nil, http.StatusMethodNotAllowed, "Invalid request method")
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		logs.HTTPError(w, r, nil, http.StatusBadRequest, "filename parameter is required")
		return
	}

	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		logs.HTTPError(w, r, nil, http.StatusBadRequest, "Invalid filename (Potential directory traversal attempt)")
		return
	}

	lower := strings.ToLower(filename)
	if !strings.HasSuffix(lower, ".webm") {
		logs.HTTPError(w, r, nil, http.StatusBadRequest, "Only .webm files are allowed")
		return
	}

	var saveDir string
	if _, err := os.Stat("/app/uploads"); err == nil {
		saveDir = "/app/uploads"
	} else {
		saveDir = filepath.Join(os.Getenv("HOME"), "server", "webm")
	}

	filePath := filepath.Join(saveDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logs.HTTPError(w, r, nil, http.StatusNotFound, "File not found")
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Could not open file")
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Could not get file info")
		return
	}

	w.Header().Set("Content-Type", "video/webm")
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")

	http.ServeFile(w, r, filePath)

	logs.INFO("File fetched successfully", map[string]any{
		"filename":    filename,
		"file_path":   filePath,
		"file_size":   fileInfo.Size(),
		"remote_addr": r.Header.Get("X-Forwarded-For"),
		"user_agent":  r.UserAgent(),
	})
}
