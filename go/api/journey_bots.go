package api

import "strings"

// Lowercased GLOB patterns for paths that only scanners request. This site serves
// no PHP, WordPress, or dotfiles, so one hit marks the whole browser as automated.
var journeyProbePaths = []string{
	"*/.env*", "*/.git*", "*/.svn*", "*/.hg/*", "*/.aws*", "*/.ssh*", "*/.docker*",
	"*/.config/*", "*/.claude*", "*/.vscode*", "*/.idea*", "*/.npmrc*", "*/.s3cfg*",
	"*/.boto*", "*/.ds_store*", "*/.ht*", "*/.github/*", "*/.gitlab*", "*/@fs/*", "*/@vite*",
	"*.php*", "*.asp*", "*.jsp*", "*.cgi*", "*.sql*", "*.bak*", "*.old", "*.swp", "*~",
	"*/wp-*", "*/wp/*", "*/wordpress*", "*/xmlrpc*", "*/phpmyadmin*", "*/pma/*", "*/cgi-bin*",
	"*/vendor/*", "*/node_modules*", "*/_profiler*", "*/phpinfo*", "*/actuator*", "*/server-status*",
	"*/administrator*", "*joomla*", "*credentials*", "*secret*", "*service-account*",
	"*/appsettings*", "*/docker-compose*", "*/config.json*", "*/config.js", "*/config.y*ml",
	"*/webpack-stats*", "*/asset-manifest*", "/manifest.json*", "*/assets/manifest.json*",
	"*/dist/manifest.json*", "/sitemap_index.xml*", "*debug-trigger*",
	"/login", "/register", "/admin", "/admin.*", "/user/login*",
}

// Lowercased GLOB patterns for self-identified crawlers and HTTP libraries.
var journeyBotAgents = []string{
	"", "*bot*", "*crawl*", "*spider*", "*slurp*", "*scan*", "*curl/*", "*wget*", "*python*",
	"*go-http*", "*httpclient*", "*java/*", "*okhttp*", "*axios*", "*node-fetch*", "*libwww*",
	"*scrapy*", "*headless*", "*facebookexternalhit*", "*preview*", "*zgrab*", "*masscan*",
}

func globAny(column string, patterns []string) (string, []any) {
	clauses := make([]string, len(patterns))
	args := make([]any, len(patterns))
	for i, p := range patterns {
		clauses[i] = column + " GLOB ?"
		args[i] = p
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

// journeyBotHaving is a HAVING clause over one browser's page and download events.
// Besides known probes, a browser whose pages are mostly errors is guessing URLs.
func journeyBotHaving() (string, []any) {
	paths, pathArgs := globAny("LOWER(url)", journeyProbePaths)
	agents, agentArgs := globAny("LOWER(COALESCE(user_agent,''))", journeyBotAgents)
	clause := "SUM(" + paths + ")=0 AND SUM(" + agents + ")=0" +
		" AND NOT (SUM(event_kind='page' AND status_code>=400)>=3 AND SUM(event_kind='page' AND status_code>=400)*2>SUM(event_kind='page'))"
	return clause, append(pathArgs, agentArgs...)
}
