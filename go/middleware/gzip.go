package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Content types worth compressing. Images, video, woff2 and archives are already compressed.
var compressible = []string{
	"text/",
	"application/javascript",
	"application/json",
	"application/xml",
	"image/svg+xml",
	"model/gltf-binary",
	"font/ttf",
	"font/otf",
}

var gzipWriters = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

type gzipResponse struct {
	http.ResponseWriter
	acceptsGzip bool
	decided     bool
	gzip        *gzip.Writer
}

// Unwrap preserves ResponseController support, including upload read deadlines.
func (gr *gzipResponse) Unwrap() http.ResponseWriter { return gr.ResponseWriter }

// WriteHeader decides once, from the final headers, whether this response is compressed.
func (gr *gzipResponse) WriteHeader(code int) {
	if code >= 100 && code < 200 {
		gr.ResponseWriter.WriteHeader(code)
		return
	}
	if !gr.decided {
		gr.decided = true
		gr.start(code)
	}
	gr.ResponseWriter.WriteHeader(code)
}

func (gr *gzipResponse) start(code int) {
	header := gr.Header()
	if !isCompressible(header.Get("Content-Type")) {
		return
	}
	header.Add("Vary", "Accept-Encoding")
	// Only whole, unencoded bodies: partial content (206) must stay byte-exact.
	if !gr.acceptsGzip || code != http.StatusOK || header.Get("Content-Encoding") != "" {
		return
	}
	header.Set("Content-Encoding", "gzip")
	header.Del("Content-Length")
	header.Del("Accept-Ranges")
	gr.gzip = gzipWriters.Get().(*gzip.Writer)
	gr.gzip.Reset(gr.ResponseWriter)
}

func (gr *gzipResponse) Write(b []byte) (int, error) {
	if !gr.decided {
		if gr.Header().Get("Content-Type") == "" {
			gr.Header().Set("Content-Type", http.DetectContentType(b))
		}
		gr.WriteHeader(http.StatusOK)
	}
	if gr.gzip != nil {
		return gr.gzip.Write(b)
	}
	return gr.ResponseWriter.Write(b)
}

func (gr *gzipResponse) finish() {
	if gr.gzip == nil {
		return
	}
	gr.gzip.Close()
	gzipWriters.Put(gr.gzip)
}

func isCompressible(contentType string) bool {
	for _, prefix := range compressible {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

func acceptsGzip(r *http.Request) bool {
	for _, coding := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(coding), ";")
		if strings.TrimSpace(name) == "gzip" && strings.ReplaceAll(params, " ", "") != "q=0" {
			return true
		}
	}
	return false
}

// Gzip compresses text-like responses for clients that accept it.
func Gzip(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gr := &gzipResponse{ResponseWriter: w, acceptsGzip: acceptsGzip(r) && r.Method != http.MethodHead}
		defer gr.finish()
		next(gr, r)
	}
}
