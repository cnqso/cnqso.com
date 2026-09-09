package logs

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"server/db"
	"sync"
	"time"
)

// One bounded worker keeps analytics writes off the HTTP response path.
// Overflow is reported rather than consuming unlimited memory or stalling visitors.
type logWriter struct {
	mu      sync.Mutex
	queue   chan any
	done    chan struct{}
	closed  bool
	dropped uint64
}

var writer *logWriter // Initialized before jobs and HTTP handlers start.

func newLogWriter(write func(any) error) *logWriter {
	w := &logWriter{queue: make(chan any, 64), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		for entry := range w.queue {
			if err := write(entry); err != nil {
				log.Printf("Error inserting log entry: %v", err)
			}
		}
	}()
	return w
}

func StartWriter() {
	writer = newLogWriter(func(entry any) error { return persistLog(db.LogDB, entry) })
}

func (w *logWriter) enqueue(entry any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	select {
	case w.queue <- entry:
	default:
		w.dropped++
		if w.dropped == 1 || w.dropped%1000 == 0 {
			log.Printf("Analytics queue full: %d records dropped", w.dropped)
		}
	}
}

func (w *logWriter) stop() {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.queue)
	}
	w.mu.Unlock()
	<-w.done
}

func StopWriter() {
	if writer != nil {
		writer.stop()
	}
}

func persistLog(handle *sql.DB, entry any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	switch e := entry.(type) {
	case Entry:
		data, err := json.Marshal(e.Data)
		if err != nil {
			return err
		}
		_, err = handle.ExecContext(ctx, "INSERT INTO dev_logs (timestamp, level, message, data) VALUES (?, ?, ?, ?)", e.Timestamp, e.Level, e.Message, string(data))
		return err
	case AccessEntry:
		_, err := handle.ExecContext(ctx, "INSERT INTO access_logs (timestamp, method, url, status_code, response_time, remote_addr, client_ip, request_size, response_size, user_agent, data, visitor_id, event_kind, referrer, fingerprint) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", e.Timestamp, e.Method, e.URL, e.StatusCode, e.ResponseTime, e.RemoteAddr, e.RemoteAddr, e.RequestSize, e.ResponseSize, e.UserAgent, "", e.VisitorID, e.EventKind, e.Referrer, e.Fingerprint)
		return err
	}
	return nil
}
