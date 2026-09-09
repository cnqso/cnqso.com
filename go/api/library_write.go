package api

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const libraryUploadLimit = 100 << 20

var libraryUploads = make(chan struct{}, 2)

func libraryMutation(w http.ResponseWriter, r *http.Request, session librarySession) {
	switch r.URL.Path {
	case libraryAdminPath + "upload":
		libraryUpload(w, r, session)
	case libraryAdminPath + "mkdir", libraryAdminPath + "logout":
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form.", http.StatusBadRequest)
			return
		}
		if !libraryValidCSRF(r.PostForm.Get("csrf"), session.CSRF) {
			http.Error(w, "This form has expired. Reload the library and try again.", http.StatusForbidden)
			return
		}
		if r.URL.Path == libraryAdminPath+"logout" {
			if cookie, err := r.Cookie(libraryCookie); err == nil {
				libraryAuth.Lock()
				delete(libraryAuth.Sessions, cookie.Value)
				libraryAuth.Unlock()
			}
			setLibraryCookie(w, r, "", -1)
			http.Redirect(w, r, libraryAdminPath+"login", http.StatusSeeOther)
			return
		}
		parent, name := r.PostForm.Get("parent"), r.PostForm.Get("name")
		if !validLibraryPath(parent) || name == "" || !validLibraryPath(name) || strings.Contains(name, "/") {
			http.Error(w, "Use a folder name without slashes or a leading dot.", http.StatusBadRequest)
			return
		}
		root, err := openLibraryRoot(false)
		if err != nil {
			libraryWriteError(w, err)
			return
		}
		defer root.Close()
		if err := libraryNoSymlinks(root, parent); err != nil {
			libraryWriteError(w, err)
			return
		}
		folder, err := root.OpenRoot(libraryDiskPath(parent))
		if err != nil {
			libraryWriteError(w, err)
			return
		}
		defer folder.Close()
		if err := folder.Mkdir(name, 0755); err != nil {
			libraryWriteError(w, err)
			return
		}
		http.Redirect(w, r, libraryURL(true, path.Join(parent, name), true)+"?done=folder", http.StatusSeeOther)
	default:
		http.NotFound(w, r)
	}
}

func libraryUpload(w http.ResponseWriter, r *http.Request, session librarySession) {
	select {
	case libraryUploads <- struct{}{}:
		defer func() { <-libraryUploads }()
	default:
		http.Error(w, "Two uploads are already in progress. Try again shortly.", http.StatusTooManyRequests)
		return
	}
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(10 * time.Minute))
	defer controller.SetReadDeadline(time.Time{})
	r.Body = http.MaxBytesReader(w, r.Body, libraryUploadLimit+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Choose a file to upload.", http.StatusBadRequest)
		return
	}
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err != nil {
			http.Error(w, "The upload was incomplete.", http.StatusBadRequest)
			return
		}
		if part.FormName() == "file" {
			if !libraryValidCSRF(fields["csrf"], session.CSRF) {
				http.Error(w, "This form has expired. Reload the library and try again.", http.StatusForbidden)
				return
			}
			// Inspect the raw filename too: multipart.FileName intentionally strips directories.
			_, parameters, parseErr := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
			name := parameters["filename"]
			if parseErr != nil {
				http.Error(w, "Invalid filename.", http.StatusBadRequest)
				return
			}
			if name == "" || !validLibraryPath(name) || strings.ContainsAny(name, "/\\") {
				http.Error(w, "Choose a file with a valid name.", http.StatusBadRequest)
				return
			}
			destination, ok := fields["destination"]
			if !ok || !validLibraryPath(destination) {
				http.Error(w, "Choose a library folder.", http.StatusBadRequest)
				return
			}
			root, err := openLibraryRoot(false)
			if err != nil {
				libraryWriteError(w, err)
				return
			}
			defer root.Close()
			if err := libraryNoSymlinks(root, destination); err != nil {
				libraryWriteError(w, err)
				return
			}
			folder, err := root.OpenRoot(libraryDiskPath(destination))
			if err != nil {
				libraryWriteError(w, err)
				return
			}
			defer folder.Close()
			temporary := ".upload-" + randomLibraryToken()
			file, err := folder.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				libraryWriteError(w, err)
				return
			}
			defer folder.Remove(temporary)
			defer file.Close()
			size, err := io.Copy(file, io.LimitReader(part, libraryUploadLimit+1))
			var tooLarge *http.MaxBytesError
			if size > libraryUploadLimit || errors.As(err, &tooLarge) {
				http.Error(w, "Files must be 100 MB or smaller.", http.StatusRequestEntityTooLarge)
				return
			}
			if err != nil {
				http.Error(w, "The upload was interrupted. No file was saved.", http.StatusBadRequest)
				return
			}
			if _, err := reader.NextPart(); err != io.EOF {
				http.Error(w, "Upload one file at a time, after the destination and form token.", http.StatusBadRequest)
				return
			}
			if err := file.Sync(); err != nil {
				libraryWriteError(w, err)
				return
			}
			if err := file.Close(); err != nil {
				libraryWriteError(w, err)
				return
			}
			// Publish only a completed file. Link is atomic and refuses to replace an existing name.
			if err := folder.Link(temporary, name); err != nil {
				libraryWriteError(w, err)
				return
			}
			http.Redirect(w, r, libraryURL(true, destination, true)+"?done=upload", http.StatusSeeOther)
			return
		}
		key := part.FormName()
		if key != "csrf" && key != "destination" {
			http.Error(w, "Invalid upload form.", http.StatusBadRequest)
			return
		}
		if _, exists := fields[key]; exists {
			http.Error(w, "Duplicate form field.", http.StatusBadRequest)
			return
		}
		value, err := io.ReadAll(io.LimitReader(part, 4097))
		if err != nil || len(value) > 4096 {
			http.Error(w, "Invalid upload form.", http.StatusBadRequest)
			return
		}
		fields[key] = string(value)
	}
}
