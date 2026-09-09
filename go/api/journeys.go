package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"server/config"
	"server/db"
	"strconv"
	"time"
)

const journeyPageSize = 200

type journeyVisitor struct {
	ID, Label, LastSeen string
	Pages, Downloads    int
}
type journeyOverview struct {
	Visitors []journeyVisitor
	Older    string
}
type journeyEvent struct {
	ID                                int64
	At                                time.Time
	Time, Kind, URL, Referrer, Method string
	Status                            int
	Repeat                            int
}
type journeyVisit struct {
	Start  string
	Events []journeyEvent
}
type journeyDetail struct {
	Fingerprinting   bool
	Signatures       []string
	Matches          []fingerprintMatch
	ID, Label, Older string
	Raw              bool
	Visits           []journeyVisit
}

func validVisitorID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func journeyBefore(r *http.Request) (int64, error) {
	s := r.URL.Query().Get("before")
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return v, nil
}

func JourneysHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAnalyticsAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	before, err := journeyBefore(r)
	if err != nil {
		http.Error(w, "Invalid cursor", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := db.DB.QueryContext(ctx, `SELECT visitor_id,MAX(id),MAX(timestamp),SUM(event_kind='page'),SUM(event_kind='download') FROM access_logs
 WHERE visitor_id IS NOT NULL AND visitor_id!='' AND event_kind IN ('page','download') AND timestamp>=datetime('now','-30 days')
 GROUP BY visitor_id HAVING (?=0 OR MAX(id)<?) ORDER BY MAX(id) DESC LIMIT 51`, before, before)
	if err != nil {
		http.Error(w, "Unable to load journeys", 500)
		return
	}
	defer rows.Close()
	data := journeyOverview{}
	var last int64
	for rows.Next() {
		var v journeyVisitor
		var at string
		var id int64
		if err = rows.Scan(&v.ID, &id, &at, &v.Pages, &v.Downloads); err != nil {
			http.Error(w, "Unable to load journeys", 500)
			return
		}
		if len(data.Visitors) == 50 {
			data.Older = "/dashboard/journeys?before=" + strconv.FormatInt(last, 10)
			break
		}
		if !validVisitorID(v.ID) {
			continue
		}
		v.Label = "Browser " + v.ID[:8]
		v.LastSeen = at
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00"} {
			if parsed, err := time.Parse(layout, at); err == nil {
				v.LastSeen = parsed.UTC().Format("Jan 2, 15:04 UTC")
				break
			}
		}
		data.Visitors = append(data.Visitors, v)
		last = id
	}
	if err = rows.Err(); err != nil {
		http.Error(w, "Unable to load journeys", 500)
		return
	}
	ServeTemplate(w, r, "journeys.html", data)
}

func JourneyHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAnalyticsAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := r.URL.Path[len("/dashboard/journey/"):]
	if !validVisitorID(id) {
		http.NotFound(w, r)
		return
	}
	before, err := journeyBefore(r)
	if err != nil {
		http.Error(w, "Invalid cursor", 400)
		return
	}
	raw := r.URL.Query().Get("raw") == "1"
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	query := `SELECT id,timestamp,COALESCE(event_kind,''),url,COALESCE(referrer,''),method,status_code FROM access_logs WHERE visitor_id=? AND timestamp>=datetime('now','-30 days') AND (?=0 OR (timestamp,id)<(SELECT timestamp,id FROM access_logs WHERE id=? AND visitor_id=?))`
	if !raw {
		query += " AND event_kind IN ('page','download')"
	}
	query += " ORDER BY timestamp DESC,id DESC LIMIT ?"
	rows, err := db.DB.QueryContext(ctx, query, id, before, before, id, journeyPageSize+1)
	if err != nil {
		http.Error(w, "Unable to load journey", 500)
		return
	}
	defer rows.Close()
	var events []journeyEvent
	for rows.Next() {
		var e journeyEvent
		if err = rows.Scan(&e.ID, &e.At, &e.Kind, &e.URL, &e.Referrer, &e.Method, &e.Status); err != nil {
			http.Error(w, "Unable to load journey", 500)
			return
		}
		events = append(events, e)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, "Unable to load journey", 500)
		return
	}
	data := journeyDetail{ID: id, Label: "Browser " + id[:8], Raw: raw}
	if len(events) > journeyPageSize {
		events = events[:journeyPageSize]
		params := url.Values{"before": {strconv.FormatInt(events[len(events)-1].ID, 10)}}
		if raw {
			params.Set("raw", "1")
		}
		data.Older = "/dashboard/journey/" + id + "?" + params.Encode()
	}
	rows.Close()
	data.Fingerprinting = config.Fingerprinting
	if data.Fingerprinting {
		data.Signatures, data.Matches, err = journeyFingerprintMatches(ctx, id)
		if err != nil {
			http.Error(w, "Unable to load fingerprint matches", 500)
			return
		}
	}
	data.Visits = groupJourney(events, raw)
	ServeTemplate(w, r, "journey.html", data)
}

// Queries fetch newest first; each page is displayed chronologically. A 30-minute
// gap starts another visit. Pagination can split a visit, so the UI calls it a segment.
func groupJourney(events []journeyEvent, raw bool) []journeyVisit {
	var visits []journeyVisit
	var previous time.Time
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		e.At = e.At.UTC()
		e.Time = e.At.Format("15:04:05")
		e.Repeat = 1
		if len(visits) == 0 || e.At.Sub(previous) > 30*time.Minute {
			visits = append(visits, journeyVisit{Start: e.At.Format("Jan 2, 2006 · 15:04 UTC")})
		}
		visit := &visits[len(visits)-1]
		// Browser PDF readers request many chunks. Collapse adjacent successful reads.
		if !raw && e.Kind == "download" && e.Status < 400 && len(visit.Events) > 0 {
			last := &visit.Events[len(visit.Events)-1]
			if last.Kind == e.Kind && last.URL == e.URL && last.Status < 400 && e.At.Sub(previous) < 5*time.Minute {
				last.Repeat++
				previous = e.At
				continue
			}
		}
		visit.Events = append(visit.Events, e)
		previous = e.At
	}
	return visits
}
