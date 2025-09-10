package main

import (
	"net/http"
	"server/api"
	"server/config"
	"server/core"
	"server/logs"
	"server/types"
)

var routes = []types.Route{
	{Path: "/", Handler: api.IndexHandler},
	{Path: "/health", Handler: api.HealthHandler},
	{Path: "/upload", Handler: api.UploadHandler},
	{Path: "/fetch", Handler: api.FetchHandler},
	{Path: "/blog", Handler: api.BlogHandler},
	{Path: "/splits", Handler: api.SplitsHandler},
	{Path: "/spirals/", Handler: api.SpiralsHandler},
	{Path: "/reverse-wordle-solver", Handler: api.ReverseWordleHandler},

	{Path: "/dashboard", Handler: api.DashboardPageHandler},
	{Path: "/api/dashboard", Handler: api.DashboardHandler},
	{Path: "/dashboard/ip/", Handler: api.IPAnalyticsPageHandler},
	{Path: "/api/dashboard/ip/", Handler: api.IPAnalyticsHandler},
	{Path: "/static/", Handler: api.StaticHandler},
	{Path: "/petrarchive/", Handler: api.ArchiveHandler},
	{Path: "/hexagons", Handler: api.HexagonsHandler},

	{Path: "/favicon.ico/", Handler: api.FaviconHandler},
	{Path: "/robots.txt", Handler: api.RobotsHandler},
	// {Path: "/sitemap.xml", Handler: api.SitemapHandler},
	{Path: "/security.txt", Handler: api.SecurityTxtHandler},
	{Path: "/.well-known/security.txt", Handler: api.SecurityTxtHandler},
}

func main() {
	core.Init()

	for _, route := range routes {
		http.HandleFunc(route.Path, logs.Handler(route.Handler))
	}

	logs.INFO("Starting server", map[string]any{"port": config.Port})

	if err := http.ListenAndServe(config.Port, nil); err != nil {
		logs.ERROR("Server failed to start", map[string]any{"error": err.Error()})
	}
}
