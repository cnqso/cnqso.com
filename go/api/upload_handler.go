package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"server/logs"
	"server/middleware"
	"strings"
)

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	middleware.SetCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		logs.HTTPError(w, r, nil, http.StatusMethodNotAllowed, "Invalid request method")
		return
	}

	err := r.ParseMultipartForm(20 << 20)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusBadRequest, "Could not parse multipart form")
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusBadRequest, "Error retrieving the file")
		return
	}
	defer file.Close()

	lower := strings.ToLower(handler.Filename)
	if !(strings.HasSuffix(lower, ".webm")) {
		logs.HTTPError(w, r, nil, http.StatusBadRequest, "Only .webm files are allowed")
		return
	}

	var saveDir string
	if _, err := os.Stat("/app/uploads"); err == nil {
		saveDir = "/app/uploads"
	} else {
		saveDir = filepath.Join(os.Getenv("HOME"), "code", "server", "webm")
	}
	err = os.MkdirAll(saveDir, 0755)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Could not create directory")
		return
	}

	savePath := filepath.Join(saveDir, handler.Filename)
	dst, err := os.Create(savePath)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Could not save file")
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Error writing file")
		return
	}

	logs.HTTPSuccess(w, r, "File uploaded successfully: "+handler.Filename)
}
