package logs

import (
	"net/http"
	"strings"
	"time"
)

type responseCapture struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
}

// Unwrap preserves ResponseController support, including upload read deadlines.
func (rc *responseCapture) Unwrap() http.ResponseWriter { return rc.ResponseWriter }

func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	size, err := rc.ResponseWriter.Write(b)
	rc.responseSize += int64(size)
	return size, err
}

func AccessLogEntry(r *http.Request, statusCode int, responseTime int64, responseSize int64) {
	url := r.URL.String()
	// The traffic dashboard is public; private library names must never enter it.
	if strings.HasPrefix(r.URL.Path, "/admin/odir/") {
		url = "/admin/odir/"
	}
	entry := AccessEntry{
		Timestamp:    time.Now().UTC(),
		Level:        LevelInfo,
		Message:      "HTTP Request",
		Method:       r.Method,
		URL:          url,
		StatusCode:   statusCode,
		ResponseTime: responseTime,
		UserAgent:    r.UserAgent(),
		RemoteAddr:   getRemoteAddr(r),
		RequestSize:  r.ContentLength,
		ResponseSize: responseSize,
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

		next.ServeHTTP(capture, r)

		responseTime := time.Since(start).Milliseconds()

		AccessLogEntry(r, capture.statusCode, responseTime, capture.responseSize)
	})
}

func Handler(handler http.HandlerFunc) http.HandlerFunc {
	return Middleware(http.HandlerFunc(handler)).ServeHTTP
}
