package types

import "net/http"

type JSONResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type Route struct {
	Path    string
	Handler http.HandlerFunc
}
