package api

// Also normalize legacy query-string variants in databases not yet migrated.
const analyticsRoute = `CASE WHEN instr(url, '?') > 0 THEN substr(url, 1, instr(url, '?') - 1) ELSE url END`
const analyticsTraffic = `(` + analyticsRoute + `) NOT IN ('/health', '/admin', '/dashboard', '/api/dashboard') AND url NOT LIKE '/admin/%' AND url NOT LIKE '/dashboard/%' AND url NOT LIKE '/api/dashboard/%'`

func dashboardCondition(period string) string {
	window := "-1 day"
	switch period {
	case "1h":
		window = "-1 hour"
	case "7d":
		window = "-7 days"
	case "30d":
		window = "-30 days"
	}
	return "timestamp >= datetime('now', '" + window + "') AND " + analyticsTraffic
}
