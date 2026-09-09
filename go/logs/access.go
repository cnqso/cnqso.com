package logs

import (
	"net/http"
	"server/internal/requestlog"
	"time"
)

type responseCapture struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
	wroteHeader  bool
	beforeHeader func(int)
}

// Unwrap preserves ResponseController support, including upload read deadlines.
func (rc *responseCapture) Unwrap() http.ResponseWriter { return rc.ResponseWriter }

func (rc *responseCapture) WriteHeader(code int) {
	if code >= 100 && code < 200 {
		rc.ResponseWriter.WriteHeader(code)
		return
	}
	if rc.wroteHeader {
		return
	}
	if rc.beforeHeader != nil {
		rc.beforeHeader(code)
	}
	rc.wroteHeader = true
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	if !rc.wroteHeader {
		if rc.Header().Get("Content-Type") == "" {
			rc.Header().Set("Content-Type", http.DetectContentType(b))
		}
		rc.WriteHeader(http.StatusOK)
	}
	size, err := rc.ResponseWriter.Write(b)
	rc.responseSize += int64(size)
	return size, err
}

func AccessLogEntry(r *http.Request, statusCode int, responseTime float64, responseSize int64) {
	if r.URL.Path == "/health" {
		return
	}
	entry := AccessEntry{
		Timestamp:    time.Now().UTC(),
		Level:        LevelInfo,
		Message:      "HTTP Request",
		Method:       r.Method,
		URL:          requestlog.Path(r.URL.EscapedPath()),
		StatusCode:   statusCode,
		ResponseTime: responseTime,
		UserAgent:    r.UserAgent(),
		RemoteAddr:   getRemoteAddr(r),
		RequestSize:  r.ContentLength,
		ResponseSize: responseSize,
	}
	if meta, ok := r.Context().Value(journeyKey{}).(*journeyMetadata); ok {
		entry.Timestamp = meta.Started
		entry.VisitorID = meta.VisitorID
		entry.EventKind = meta.Kind
		entry.Referrer = meta.Referrer
		entry.Fingerprint = meta.Fingerprint
	}
	logToOutput(entry, LevelInfo)
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		capture := &responseCapture{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			responseSize:   0,
		}

		request, beforeHeader := prepareJourney(r, capture)
		capture.beforeHeader = beforeHeader
		next.ServeHTTP(capture, request)

		responseTime := float64(time.Since(start)) / float64(time.Millisecond)

		AccessLogEntry(request, capture.statusCode, responseTime, capture.responseSize)
	})
}

func Handler(handler http.HandlerFunc) http.HandlerFunc {
	return Middleware(http.HandlerFunc(handler)).ServeHTTP
}
