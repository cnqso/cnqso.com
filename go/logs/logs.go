package logs

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"server/db"
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
	Timestamp    time.Time `json:"timestamp"`
	Level        Level     `json:"level"`
	Message      string    `json:"message"`
	Method       string    `json:"method"`
	URL          string    `json:"url"`
	StatusCode   int       `json:"status_code,omitempty"`
	ResponseTime int64     `json:"response_time_ms,omitempty"`
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
	case AccessEntry:
		fmt.Fprintf(output, "[ACCESS] %s: %s %s %d (%dms) %s\n",
			e.Timestamp.Format("15:04:05"), e.Method, e.URL, e.StatusCode, e.ResponseTime, e.RemoteAddr)
	}

	if db.DB != nil {
		switch e := entry.(type) {
		case Entry:
			var dataJSON string
			if e.Data != nil {
				if jsonData, err := json.Marshal(e.Data); err == nil {
					dataJSON = string(jsonData)
				}
			}
			_, err := db.DB.Exec("INSERT INTO dev_logs (timestamp, level, message, data) VALUES (?, ?, ?, ?)",
				e.Timestamp, e.Level, e.Message, dataJSON)
			if err != nil {
				log.Printf("Error inserting log entry: %v", err)
			}
		case AccessEntry:
			var dataJSON string
			if e.Data != nil {
				if jsonData, err := json.Marshal(e.Data); err == nil {
					dataJSON = string(jsonData)
				}
			}
			db.DB.Exec("INSERT INTO access_logs (timestamp, method, url, status_code, response_time, remote_addr, request_size, response_size, user_agent, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
				e.Timestamp, e.Method, e.URL, e.StatusCode, e.ResponseTime, e.RemoteAddr, e.RequestSize, e.ResponseSize, e.UserAgent, dataJSON)
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
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     LevelError,
		Message:   message,
		Data:      map[string]any{"error": err.Error(), "status": status, "method": r.Method, "route": r.URL.Path, "UserAgent": r.UserAgent(), "RemoteAddr": getRemoteAddr(r), "RequestSize": r.ContentLength},
	}
	logToOutput(entry, LevelError)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(types.JSONResponse{
		Success: false,
		Message: message,
	})
}

func getRemoteAddr(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}
