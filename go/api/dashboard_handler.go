package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"server/db"
	"server/internal/requestlog"
	"server/logs"
	"strconv"
	"strings"
	"time"
)

type DashboardData struct {
	Stats      DashboardStats `json:"stats"`
	TopIPs     []IPCount      `json:"topIPs"`
	TopRoutes  []RouteCount   `json:"topRoutes"`
	Bot404s    []IPCount      `json:"bot404s"`
	ErrorCodes []StatusCount  `json:"errorCodes"`
	UserAgents []UACount      `json:"userAgents"`
}

type DashboardStats struct {
	TotalRequests   int     `json:"totalRequests"`
	UniqueIPs       int     `json:"uniqueIPs"`
	ErrorRate       float64 `json:"errorRate"`
	AvgResponseTime float64 `json:"avgResponseTime"`
}

type IPCount struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

type RouteCount struct {
	URL   string `json:"url"`
	Count int    `json:"count"`
}

type StatusCount struct {
	StatusCode int `json:"status_code"`
	Count      int `json:"count"`
}

type IPAnalyticsData struct {
	Stats          IPStats       `json:"stats"`
	TopRoutes      []RouteCount  `json:"topRoutes"`
	StatusCodes    []StatusCount `json:"statusCodes"`
	HourlyActivity []HourCount   `json:"hourlyActivity"`
	UserAgents     []UACount     `json:"userAgents"`
	AccessLogs     []AccessLog   `json:"accessLogs"`
}

type IPStats struct {
	TotalRequests   int     `json:"totalRequests"`
	UniqueRoutes    int     `json:"uniqueRoutes"`
	ErrorCount      int     `json:"errorCount"`
	AvgResponseTime float64 `json:"avgResponseTime"`
	FirstSeen       string  `json:"firstSeen"`
	LastSeen        string  `json:"lastSeen"`
}

type HourCount struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

type UACount struct {
	UserAgent string `json:"user_agent"`
	Count     int    `json:"count"`
}

type AccessLog struct {
	Timestamp    string  `json:"timestamp"`
	Method       string  `json:"method"`
	URL          string  `json:"url"`
	StatusCode   int     `json:"status_code"`
	ResponseTime float64 `json:"response_time"`
	RequestSize  int64   `json:"request_size"`
	ResponseSize int64   `json:"response_size"`
	UserAgent    string  `json:"user_agent"`
}

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAnalyticsAdmin(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if r.Method != http.MethodGet {
		logs.HTTPError(w, r, nil, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	timeCondition := dashboardCondition(r.URL.Query().Get("period"))

	data := DashboardData{}

	stats, err := getDashboardStats(ctx, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get dashboard stats")
		return
	}
	data.Stats = stats

	topIPs, err := getTopIPs(ctx, timeCondition, 100)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get top IPs")
		return
	}
	data.TopIPs = topIPs

	topRoutes, err := getTopRoutes(ctx, timeCondition, 100)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get top routes")
		return
	}
	data.TopRoutes = topRoutes

	bot404s, err := getBot404s(ctx, timeCondition, 100)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get bot 404s")
		return
	}
	data.Bot404s = bot404s

	errorCodes, err := getErrorCodes(ctx, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get error codes")
		return
	}
	data.ErrorCodes = errorCodes

	userAgents, err := getTopUserAgents(ctx, timeCondition, 20)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get user agents")
		return
	}
	data.UserAgents = userAgents

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func getDashboardStats(ctx context.Context, timeCondition string) (DashboardStats, error) {
	var stats DashboardStats

	query := `SELECT COUNT(*), COUNT(DISTINCT client_ip),
 COALESCE(100.0 * SUM(status_code >= 500) / NULLIF(COUNT(*), 0), 0),
 COALESCE(AVG(response_time), 0) FROM access_logs WHERE ` + timeCondition
	err := db.DB.QueryRowContext(ctx, query).Scan(&stats.TotalRequests, &stats.UniqueIPs, &stats.ErrorRate, &stats.AvgResponseTime)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func getTopIPs(ctx context.Context, timeCondition string, limit int) ([]IPCount, error) {
	query := `
		SELECT client_ip, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + `
		GROUP BY client_ip
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]IPCount, 0)
	for rows.Next() {
		var ip IPCount
		err := rows.Scan(&ip.IP, &ip.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ip)
	}

	return results, rows.Err()
}

func getTopRoutes(ctx context.Context, timeCondition string, limit int) ([]RouteCount, error) {
	query := `
		SELECT ` + analyticsRoute + ` AS route, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + `
		GROUP BY route
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]RouteCount, 0)
	for rows.Next() {
		var route RouteCount
		err := rows.Scan(&route.URL, &route.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, route)
	}

	return results, rows.Err()
}

func getBot404s(ctx context.Context, timeCondition string, limit int) ([]IPCount, error) {
	query := `
		SELECT client_ip, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + ` AND status_code = 404
		GROUP BY client_ip
		HAVING count >= 5
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]IPCount, 0)
	for rows.Next() {
		var ip IPCount
		err := rows.Scan(&ip.IP, &ip.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ip)
	}

	return results, rows.Err()
}

func getErrorCodes(ctx context.Context, timeCondition string) ([]StatusCount, error) {
	query := `
		SELECT status_code, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + ` AND status_code >= 400
		GROUP BY status_code
		ORDER BY count DESC`

	rows, err := db.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]StatusCount, 0)
	for rows.Next() {
		var status StatusCount
		err := rows.Scan(&status.StatusCode, &status.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, status)
	}

	return results, rows.Err()
}

func getTopUserAgents(ctx context.Context, timeCondition string, limit int) ([]UACount, error) {
	query := `
		SELECT COALESCE(user_agent, 'Unknown') as user_agent, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + `
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]UACount, 0)
	for rows.Next() {
		var ua UACount
		err := rows.Scan(&ua.UserAgent, &ua.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ua)
	}

	return results, rows.Err()
}

func DashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAnalyticsAdmin(w, r) {
		return
	}
	ServeTemplate(w, r, "dashboard.html", nil)
}

func IPAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAnalyticsAdmin(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if r.Method != http.MethodGet {
		logs.HTTPError(w, r, nil, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/dashboard/ip/") {
		logs.HTTPError(w, r, nil, http.StatusBadRequest, "Invalid IP path")
		return
	}

	ip := strings.TrimPrefix(path, "/api/dashboard/ip/")
	if ip == "" {
		logs.HTTPError(w, r, nil, http.StatusBadRequest, "IP address required")
		return
	}

	timeCondition := dashboardCondition(r.URL.Query().Get("period"))

	data := IPAnalyticsData{}

	stats, err := getIPStats(ctx, ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP stats")
		return
	}
	data.Stats = stats

	topRoutes, err := getIPTopRoutes(ctx, ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP routes")
		return
	}
	data.TopRoutes = topRoutes

	statusCodes, err := getIPStatusCodes(ctx, ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP status codes")
		return
	}
	data.StatusCodes = statusCodes

	hourlyActivity, err := getIPHourlyActivity(ctx, ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP hourly activity")
		return
	}
	data.HourlyActivity = hourlyActivity

	userAgents, err := getIPUserAgents(ctx, ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP user agents")
		return
	}
	data.UserAgents = userAgents

	accessLogs, err := getIPAccessLogs(ctx, ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP access logs")
		return
	}
	data.AccessLogs = accessLogs

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func IPAnalyticsPageHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAnalyticsAdmin(w, r) {
		return
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, "/dashboard/ip/") {
		FourHundredHandler(w, r, 404)
		return
	}

	ip := strings.TrimPrefix(path, "/dashboard/ip/")
	if ip == "" {
		FourHundredHandler(w, r, 404)
		return
	}

	data := struct {
		IP string
	}{
		IP: ip,
	}

	ServeTemplate(w, r, "ip_analytics.html", data)
}

func getIPStats(ctx context.Context, ip, timeCondition string) (IPStats, error) {
	var stats IPStats

	query := `SELECT COUNT(*), COUNT(DISTINCT ` + analyticsRoute + `),
 COALESCE(SUM(status_code >= 500), 0), COALESCE(AVG(response_time), 0)
 FROM access_logs WHERE client_ip = ? AND ` + timeCondition
	err := db.DB.QueryRowContext(ctx, query, ip).Scan(&stats.TotalRequests, &stats.UniqueRoutes, &stats.ErrorCount, &stats.AvgResponseTime)
	if err != nil {
		return stats, err
	}

	query = "SELECT MIN(timestamp), MAX(timestamp) FROM access_logs WHERE client_ip = ? AND " + analyticsTraffic
	var firstSeen, lastSeen sql.NullString
	err = db.DB.QueryRowContext(ctx, query, ip).Scan(&firstSeen, &lastSeen)
	if err != nil {
		return stats, err
	}
	if firstSeen.Valid {
		stats.FirstSeen = firstSeen.String
	}
	if lastSeen.Valid {
		stats.LastSeen = lastSeen.String
	}

	return stats, nil
}

func getIPTopRoutes(ctx context.Context, ip, timeCondition string) ([]RouteCount, error) {
	query := `
		SELECT ` + analyticsRoute + ` AS route, COUNT(*) as count
		FROM access_logs
		WHERE client_ip = ? AND ` + timeCondition + `
		GROUP BY route
		ORDER BY count DESC
		LIMIT 50`

	rows, err := db.DB.QueryContext(ctx, query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]RouteCount, 0)
	for rows.Next() {
		var route RouteCount
		err := rows.Scan(&route.URL, &route.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, route)
	}

	return results, rows.Err()
}

func getIPStatusCodes(ctx context.Context, ip, timeCondition string) ([]StatusCount, error) {
	query := `
		SELECT status_code, COUNT(*) as count
		FROM access_logs
		WHERE client_ip = ? AND ` + timeCondition + `
		GROUP BY status_code
		ORDER BY count DESC`

	rows, err := db.DB.QueryContext(ctx, query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]StatusCount, 0)
	for rows.Next() {
		var status StatusCount
		err := rows.Scan(&status.StatusCode, &status.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, status)
	}

	return results, rows.Err()
}

func getIPHourlyActivity(ctx context.Context, ip, timeCondition string) ([]HourCount, error) {
	query := `
		SELECT strftime('%H', timestamp) as hour, COUNT(*) as count
		FROM access_logs
		WHERE client_ip = ? AND ` + timeCondition + `
		GROUP BY hour
		ORDER BY hour`

	rows, err := db.DB.QueryContext(ctx, query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]HourCount, 0)
	for rows.Next() {
		var hourCount HourCount
		var hourStr string
		err := rows.Scan(&hourStr, &hourCount.Count)
		if err != nil {
			return nil, err
		}

		hour, err := strconv.Atoi(hourStr)
		if err != nil {
			return nil, err
		}
		hourCount.Hour = hour

		results = append(results, hourCount)
	}

	return results, rows.Err()
}

func getIPUserAgents(ctx context.Context, ip, timeCondition string) ([]UACount, error) {
	query := `
		SELECT
			COALESCE(user_agent, 'Unknown') as user_agent,
			COUNT(*) as count
		FROM access_logs
		WHERE client_ip = ? AND ` + timeCondition + `
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT 10`

	rows, err := db.DB.QueryContext(ctx, query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]UACount, 0)
	for rows.Next() {
		var ua UACount
		err := rows.Scan(&ua.UserAgent, &ua.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ua)
	}

	return results, rows.Err()
}

func getIPAccessLogs(ctx context.Context, ip, timeCondition string) ([]AccessLog, error) {
	query := `
		SELECT timestamp, method, url, status_code, response_time,
			   request_size, response_size, COALESCE(user_agent, 'Unknown') as user_agent
		FROM access_logs
		WHERE client_ip = ? AND ` + timeCondition + `
		ORDER BY timestamp DESC
		LIMIT 100`

	rows, err := db.DB.QueryContext(ctx, query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]AccessLog, 0)
	for rows.Next() {
		var log AccessLog
		err := rows.Scan(&log.Timestamp, &log.Method, &log.URL, &log.StatusCode,
			&log.ResponseTime, &log.RequestSize, &log.ResponseSize, &log.UserAgent)
		if err != nil {
			return nil, err
		}
		log.URL = requestlog.Path(log.URL)
		results = append(results, log)
	}

	return results, rows.Err()
}
