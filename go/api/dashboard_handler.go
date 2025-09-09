package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"server/db"
	"server/logs"
	"strconv"
	"strings"
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
	Timestamp    string `json:"timestamp"`
	Method       string `json:"method"`
	URL          string `json:"url"`
	StatusCode   int    `json:"status_code"`
	ResponseTime int64  `json:"response_time"`
	RequestSize  int64  `json:"request_size"`
	ResponseSize int64  `json:"response_size"`
	UserAgent    string `json:"user_agent"`
}

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logs.HTTPError(w, r, nil, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	var timeCondition string
	switch period {
	case "1h":
		timeCondition = "timestamp >= datetime('now', '-1 hour')"
	case "24h":
		timeCondition = "timestamp >= datetime('now', '-1 day')"
	case "7d":
		timeCondition = "timestamp >= datetime('now', '-7 days')"
	case "30d":
		timeCondition = "timestamp >= datetime('now', '-30 days')"
	default:
		timeCondition = "timestamp >= datetime('now', '-1 day')"
	}

	data := DashboardData{}

	stats, err := getDashboardStats(timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get dashboard stats")
		return
	}
	data.Stats = stats

	topIPs, err := getTopIPs(timeCondition, 100)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get top IPs")
		return
	}
	data.TopIPs = topIPs

	topRoutes, err := getTopRoutes(timeCondition, 100)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get top routes")
		return
	}
	data.TopRoutes = topRoutes

	bot404s, err := getBot404s(timeCondition, 100)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get bot 404s")
		return
	}
	data.Bot404s = bot404s

	errorCodes, err := getErrorCodes(timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get error codes")
		return
	}
	data.ErrorCodes = errorCodes

	userAgents, err := getTopUserAgents(timeCondition, 20)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get user agents")
		return
	}
	data.UserAgents = userAgents

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func getDashboardStats(timeCondition string) (DashboardStats, error) {
	var stats DashboardStats

	query := "SELECT COUNT(*) FROM access_logs WHERE " + timeCondition
	err := db.DB.QueryRow(query).Scan(&stats.TotalRequests)
	if err != nil {
		return stats, err
	}

	query = "SELECT COUNT(DISTINCT remote_addr) FROM access_logs WHERE " + timeCondition
	err = db.DB.QueryRow(query).Scan(&stats.UniqueIPs)
	if err != nil {
		return stats, err
	}

	var errorCount int
	query = "SELECT COUNT(*) FROM access_logs WHERE " + timeCondition + " AND status_code >= 400"
	err = db.DB.QueryRow(query).Scan(&errorCount)
	if err != nil {
		return stats, err
	}

	if stats.TotalRequests > 0 {
		stats.ErrorRate = (float64(errorCount) / float64(stats.TotalRequests)) * 100
	}

	query = "SELECT AVG(response_time) FROM access_logs WHERE " + timeCondition + " AND response_time > 0"
	var avgTime sql.NullFloat64
	err = db.DB.QueryRow(query).Scan(&avgTime)
	if err != nil {
		return stats, err
	}
	if avgTime.Valid {
		stats.AvgResponseTime = avgTime.Float64
	}

	return stats, nil
}

func getTopIPs(timeCondition string, limit int) ([]IPCount, error) {
	query := `
		SELECT remote_addr, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + `
		GROUP BY remote_addr
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []IPCount
	for rows.Next() {
		var ip IPCount
		err := rows.Scan(&ip.IP, &ip.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ip)
	}

	return results, nil
}

func getTopRoutes(timeCondition string, limit int) ([]RouteCount, error) {
	query := `
		SELECT url, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + `
		GROUP BY url
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RouteCount
	for rows.Next() {
		var route RouteCount
		err := rows.Scan(&route.URL, &route.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, route)
	}

	return results, nil
}

func getBot404s(timeCondition string, limit int) ([]IPCount, error) {
	query := `
		SELECT remote_addr, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + ` AND status_code = 404
		GROUP BY remote_addr
		HAVING count >= 5
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []IPCount
	for rows.Next() {
		var ip IPCount
		err := rows.Scan(&ip.IP, &ip.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ip)
	}

	return results, nil
}

func getErrorCodes(timeCondition string) ([]StatusCount, error) {
	query := `
		SELECT status_code, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + ` AND status_code >= 400
		GROUP BY status_code
		ORDER BY count DESC`

	rows, err := db.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StatusCount
	for rows.Next() {
		var status StatusCount
		err := rows.Scan(&status.StatusCode, &status.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, status)
	}

	return results, nil
}

func getTopUserAgents(timeCondition string, limit int) ([]UACount, error) {
	query := `
		SELECT COALESCE(user_agent, 'Unknown') as user_agent, COUNT(*) as count
		FROM access_logs
		WHERE ` + timeCondition + `
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT ?`

	rows, err := db.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UACount
	for rows.Next() {
		var ua UACount
		err := rows.Scan(&ua.UserAgent, &ua.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ua)
	}

	return results, nil
}

func DashboardPageHandler(w http.ResponseWriter, r *http.Request) {
	ServeTemplate(w, r, "dashboard.html", nil)
}

func IPAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
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

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	var timeCondition string
	switch period {
	case "1h":
		timeCondition = "timestamp >= datetime('now', '-1 hour')"
	case "24h":
		timeCondition = "timestamp >= datetime('now', '-1 day')"
	case "7d":
		timeCondition = "timestamp >= datetime('now', '-7 days')"
	case "30d":
		timeCondition = "timestamp >= datetime('now', '-30 days')"
	default:
		timeCondition = "timestamp >= datetime('now', '-1 day')"
	}

	data := IPAnalyticsData{}

	stats, err := getIPStats(ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP stats")
		return
	}
	data.Stats = stats

	topRoutes, err := getIPTopRoutes(ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP routes")
		return
	}
	data.TopRoutes = topRoutes

	statusCodes, err := getIPStatusCodes(ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP status codes")
		return
	}
	data.StatusCodes = statusCodes

	hourlyActivity, err := getIPHourlyActivity(ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP hourly activity")
		return
	}
	data.HourlyActivity = hourlyActivity

	userAgents, err := getIPUserAgents(ip, timeCondition)
	if err != nil {
		logs.HTTPError(w, r, err, http.StatusInternalServerError, "Failed to get IP user agents")
		return
	}
	data.UserAgents = userAgents

	accessLogs, err := getIPAccessLogs(ip, timeCondition)
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

func getIPStats(ip, timeCondition string) (IPStats, error) {
	var stats IPStats

	query := "SELECT COUNT(*) FROM access_logs WHERE remote_addr = ? AND " + timeCondition
	err := db.DB.QueryRow(query, ip).Scan(&stats.TotalRequests)
	if err != nil {
		return stats, err
	}

	query = "SELECT COUNT(DISTINCT url) FROM access_logs WHERE remote_addr = ? AND " + timeCondition
	err = db.DB.QueryRow(query, ip).Scan(&stats.UniqueRoutes)
	if err != nil {
		return stats, err
	}

	query = "SELECT COUNT(*) FROM access_logs WHERE remote_addr = ? AND " + timeCondition + " AND status_code >= 400"
	err = db.DB.QueryRow(query, ip).Scan(&stats.ErrorCount)
	if err != nil {
		return stats, err
	}

	query = "SELECT AVG(response_time) FROM access_logs WHERE remote_addr = ? AND " + timeCondition + " AND response_time > 0"
	var avgTime sql.NullFloat64
	err = db.DB.QueryRow(query, ip).Scan(&avgTime)
	if err != nil {
		return stats, err
	}
	if avgTime.Valid {
		stats.AvgResponseTime = avgTime.Float64
	}

	query = "SELECT MIN(timestamp), MAX(timestamp) FROM access_logs WHERE remote_addr = ?"
	var firstSeen, lastSeen sql.NullString
	err = db.DB.QueryRow(query, ip).Scan(&firstSeen, &lastSeen)
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

func getIPTopRoutes(ip, timeCondition string) ([]RouteCount, error) {
	query := `
		SELECT url, COUNT(*) as count
		FROM access_logs
		WHERE remote_addr = ? AND ` + timeCondition + `
		GROUP BY url
		ORDER BY count DESC
		LIMIT 50`

	rows, err := db.DB.Query(query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RouteCount
	for rows.Next() {
		var route RouteCount
		err := rows.Scan(&route.URL, &route.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, route)
	}

	return results, nil
}

func getIPStatusCodes(ip, timeCondition string) ([]StatusCount, error) {
	query := `
		SELECT status_code, COUNT(*) as count
		FROM access_logs
		WHERE remote_addr = ? AND ` + timeCondition + `
		GROUP BY status_code
		ORDER BY count DESC`

	rows, err := db.DB.Query(query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StatusCount
	for rows.Next() {
		var status StatusCount
		err := rows.Scan(&status.StatusCode, &status.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, status)
	}

	return results, nil
}

func getIPHourlyActivity(ip, timeCondition string) ([]HourCount, error) {
	query := `
		SELECT strftime('%H', timestamp) as hour, COUNT(*) as count
		FROM access_logs
		WHERE remote_addr = ? AND ` + timeCondition + `
		GROUP BY hour
		ORDER BY hour`

	rows, err := db.DB.Query(query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []HourCount
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

	return results, nil
}

func getIPUserAgents(ip, timeCondition string) ([]UACount, error) {
	query := `
		SELECT
			COALESCE(user_agent, 'Unknown') as user_agent,
			COUNT(*) as count
		FROM access_logs
		WHERE remote_addr = ? AND ` + timeCondition + `
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT 10`

	rows, err := db.DB.Query(query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UACount
	for rows.Next() {
		var ua UACount
		err := rows.Scan(&ua.UserAgent, &ua.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, ua)
	}

	return results, nil
}

func getIPAccessLogs(ip, timeCondition string) ([]AccessLog, error) {
	query := `
		SELECT timestamp, method, url, status_code, response_time,
			   request_size, response_size, COALESCE(user_agent, 'Unknown') as user_agent
		FROM access_logs
		WHERE remote_addr = ? AND ` + timeCondition + `
		ORDER BY timestamp DESC
		LIMIT 100`

	rows, err := db.DB.Query(query, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AccessLog
	for rows.Next() {
		var log AccessLog
		err := rows.Scan(&log.Timestamp, &log.Method, &log.URL, &log.StatusCode,
			&log.ResponseTime, &log.RequestSize, &log.ResponseSize, &log.UserAgent)
		if err != nil {
			return nil, err
		}
		results = append(results, log)
	}

	return results, nil
}
