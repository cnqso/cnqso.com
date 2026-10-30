package api

import "strings"

// Lowercased GLOB patterns for paths that only scanners request. This site serves
// no PHP, WordPress, or dotfiles, so one hit marks the whole browser as automated.
// Every request a browser made is checked, so patterns must not match real assets.
var journeyProbePaths = []string{
	"*/.env*", "*/.git*", "*/.svn*", "*/.hg/*", "*/.aws*", "*/.ssh*", "*/.docker*",
	"*/.config/*", "*/.claude*", "*/.vscode*", "*/.idea*", "*/.npmrc*", "*/.s3cfg*",
	"*/.boto*", "*/.ds_store*", "*/.ht*", "*/.github/*", "*/.gitlab*", "*/@fs/*", "*/@vite*",
	"*.php*", "*.asp*", "*.jsp*", "*.cgi*", "*.sql*", "*.bak*", "*.old", "*.swp", "*~",
	"*/wp-*", "*/wp/*", "*/wordpress*", "*/xmlrpc*", "*/phpmyadmin*", "*/pma/*", "*/cgi-bin*",
	"*/vendor/*", "*/node_modules*", "*/_profiler*", "*/phpinfo*", "*/actuator*", "*/server-status*",
	"*/administrator*", "*joomla*", "*credentials*", "*secrets*", "*client_secret*", "*service-account*",
	"*/appsettings*", "*/docker-compose*", "*/config.json*", "*/config.js", "*/config.y*ml",
	"*/webpack-stats*", "/asset-manifest*", "/manifest.json*", "*/assets/manifest.json*",
	"*/dist/manifest.json*", "*debug-trigger*",
	"/login", "/register", "/admin", "/admin.*", "/user/login*",
}

// Lowercased GLOB patterns for files that crawlers and agents fetch but people
// don't open. Crawlers rarely send cookies, so these also match by IP and day.
var journeyMetaPaths = []string{
	"/robots.txt*", "/llms*.txt*", "/ai.txt*", "/ads.txt*", "/app-ads.txt*", "/sellers.json*",
	"/.well-known/*", "/security.txt*", "/humans.txt*", "/sitemap*", "/sitemaps/*",
	"/crossdomain.xml*", "/clientaccesspolicy.xml*", "/feed*", "/rss*", "/atom.xml*", "/index.xml*",
}

// Chrome DevTools requests this whenever it is open, including for the site owner.
const journeyMetaExempt = "/.well-known/appspecific/*"

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

// journeyBotHaving is a HAVING clause over every request one browser made.
// Besides known probes, a browser whose pages are mostly errors is guessing URLs,
// and one averaging over a page per second across ten or more is crawling.
func journeyBotHaving() (string, []any) {
	paths, args := globAny("LOWER(url)", append(append([]string{}, journeyProbePaths...), journeyMetaPaths...))
	agents, agentArgs := globAny("LOWER(COALESCE(user_agent,''))", journeyBotAgents)
	meta, metaArgs := globAny("LOWER(url)", journeyMetaPaths)
	args = append(append(append(args, journeyMetaExempt), agentArgs...), metaArgs...)
	args = append(args, journeyMetaExempt)
	clause := "SUM(" + paths + " AND LOWER(url) NOT GLOB ?)=0 AND SUM(" + agents + ")=0" +
		" AND NOT (SUM(event_kind='page' AND status_code>=400)>=3 AND SUM(event_kind='page' AND status_code>=400)*2>SUM(event_kind='page'))" +
		" AND NOT (SUM(event_kind IN ('page','download'))>=10 AND (JULIANDAY(MAX(timestamp))-JULIANDAY(MIN(timestamp)))*86400<SUM(event_kind IN ('page','download')))" +
		" AND SUM(client_ip||' '||DATE(timestamp) IN (SELECT client_ip||' '||DATE(timestamp) FROM access_logs" +
		" WHERE timestamp>=datetime('now','-31 days') AND " + meta + " AND LOWER(url) NOT GLOB ?))=0"
	return clause, args
}

// journeyRenderedHaving keeps browsers that fetched a stylesheet, script, or icon,
// or downloaded a file. Clients that reload / or walk links without assets aren't
// rendering pages, though a returning browser with a warm cache can look the same.
const journeyRenderedHaving = "(SUM(COALESCE(event_kind,'')='' AND status_code<400)>0 OR SUM(event_kind='download')>0)"
