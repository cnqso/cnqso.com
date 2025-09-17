package types

import (
	"html/template"
	"net/http"
	"time"
)

type JSONResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type Route struct {
	Path    string
	Handler http.HandlerFunc
}

type BlogPost struct {
	Slug     string
	Title    string
	Date     time.Time
	Content  template.HTML
	FilePath string
}

type BlogData struct {
	Posts []BlogPost
}
