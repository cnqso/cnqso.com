package logs

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"server/db"
	"server/internal/requestlog"
	"server/types"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     Level     `json:"level"`
	Message   string    `json:"message"`
	Data      any       `json:"data,omitempty"`
}

type AccessEntry struct {
	Fingerprint  string    `json:"fingerprint,omitempty"`
	VisitorID    string    `json:"visitor_id,omitempty"`
	EventKind    string    `json:"event_kind,omitempty"`
	Referrer     string    `json:"referrer,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	Level        Level     `json:"level"`
	Message      string    `json:"message"`
	Method       string    `json:"method"`
	URL          string    `json:"url"`
	StatusCode   int       `json:"status_code,omitempty"`
	ResponseTime float64   `json:"response_time_ms,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	RemoteAddr   string    `json:"remote_addr,omitempty"`
	RequestSize  int64     `json:"request_size,omitempty"`
	ResponseSize int64     `json:"response_size,omitempty"`
	Data         any       `json:"data,omitempty"`
}

func logToOutput(entry any, level Level) {
	var output *os.File
	if level == LevelError {
		output = os.Stderr
	} else {
		output = os.Stdout
	}

	switch e := entry.(type) {
	case Entry:
		fmt.Fprintf(output, "[%s] %s: %s\n", e.Level, e.Timestamp.Format("15:04:05"), e.Message)
		// Capture caller-owned maps before the worker can outlive or race them.
		if e.Data != nil {
			data, err := json.Marshal(e.Data)
			if err != nil {
				log.Printf("Error encoding log data: %v", err)
				e.Data = nil
			} else {
				e.Data = json.RawMessage(data)
			}
			entry = e
		}
	case AccessEntry:
		fmt.Fprintf(output, "[ACCESS] %s: %s %s %d (%.3fms) %s\n",
			e.Timestamp.Format("15:04:05"), e.Method, e.URL, e.StatusCode, e.ResponseTime, e.RemoteAddr)
	}

	if writer != nil {
		writer.enqueue(entry)
	} else if db.DB != nil {
		if err := persistLog(db.DB, entry); err != nil {
			log.Printf("Error inserting log entry: %v", err)
		}
	}
}

func DEBUG(message string, data ...any) {
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     LevelDebug,
		Message:   message,
	}
	if len(data) > 0 {
		entry.Data = data[0]
	}
	logToOutput(entry, LevelDebug)
}

func INFO(message string, data ...any) {
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     LevelInfo,
		Message:   message,
	}
	if len(data) > 0 {
		entry.Data = data[0]
	}
	logToOutput(entry, LevelInfo)
}

func WARN(message string, data ...any) {
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     LevelWarn,
		Message:   message,
	}
	if len(data) > 0 {
		entry.Data = data[0]
	}
	logToOutput(entry, LevelWarn)
}

func ERROR(message string, data ...any) {
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     LevelError,
		Message:   message,
	}
	if len(data) > 0 {
		entry.Data = data[0]
	}
	logToOutput(entry, LevelError)
}

func HTTPSuccess(w http.ResponseWriter, r *http.Request, message string) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(types.JSONResponse{
		Success: true,
		Message: message,
	})
}

func HTTPError(w http.ResponseWriter, r *http.Request, err error, status int, message string) {
	errorText := ""
	if err != nil {
		errorText = err.Error()
	}
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     LevelError,
		Message:   message,
		Data:      map[string]any{"error": errorText, "status": status, "method": r.Method, "route": requestlog.Path(r.URL.EscapedPath()), "UserAgent": r.UserAgent(), "RemoteAddr": getRemoteAddr(r), "RequestSize": r.ContentLength},
	}
	logToOutput(entry, LevelError)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(types.JSONResponse{
		Success: false,
		Message: message,
	})
}
