package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"server/api"
	"server/config"
	"server/core"
	"server/db"
	"server/logs"
	"server/types"
	"syscall"
	"time"
)

var routes = []types.Route{
	{Path: "/", Handler: api.IndexHandler},
	{Path: "/health", Handler: api.HealthHandler},
	{Path: "/upload", Handler: api.UploadHandler},
	{Path: "/fetch", Handler: api.FetchHandler},
	{Path: "/blog/", Handler: api.BlogHandler},
	{Path: "/splits", Handler: api.SplitsHandler},
	{Path: "/spirals/", Handler: api.SpiralsHandler},
	{Path: "/reverse-wordle-solver", Handler: api.ReverseWordleHandler},
	{Path: "/piano-flashcards", Handler: api.PianoFlashcardsHandler},
	{Path: "/bloonsbench/", Handler: api.BloonsBenchHandler},
	{Path: "/esotericbench/", Handler: api.EsotericBenchHandler},
	{Path: "/odir/", Handler: api.OpenDirectoryHandler},
	{Path: "/admin/odir/", Handler: api.LibraryAdminHandler},

	{Path: "/dashboard", Handler: api.DashboardPageHandler},
	{Path: "/dashboard/journeys", Handler: api.JourneysHandler},
	{Path: "/dashboard/journey/", Handler: api.JourneyHandler},
	{Path: "/api/dashboard", Handler: api.DashboardHandler},
	{Path: "/dashboard/ip/", Handler: api.IPAnalyticsPageHandler},
	{Path: "/api/dashboard/ip/", Handler: api.IPAnalyticsHandler},
	{Path: "/static/", Handler: api.StaticHandler},
	{Path: "/petrarchive/", Handler: api.ArchiveHandler},
	{Path: "/hexagons", Handler: api.HexagonsHandler},
	{Path: "/l8", Handler: api.L8Handler},
	{Path: "/favicon.ico/", Handler: api.FaviconHandler},
	{Path: "/robots.txt", Handler: api.RobotsHandler},
	{Path: "/sitemap.xml", Handler: api.SitemapHandler},
	{Path: "/security.txt", Handler: api.SecurityTxtHandler},
	{Path: "/.well-known/security.txt", Handler: api.SecurityTxtHandler},
}

func main() {
	core.Init()
	defer db.DB.Close()
	defer db.LogDB.Close()
	defer logs.StopWriter()

	for _, route := range routes {
		http.HandleFunc(route.Path, logs.Handler(route.Handler))
	}

	logs.INFO("Starting server", map[string]any{"port": config.Port})

	server := &http.Server{Addr: config.Port, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	stop, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- server.ListenAndServe() }()
	select {
	case err := <-finished:
		if !errors.Is(err, http.ErrServerClosed) {
			logs.ERROR("Server stopped", map[string]any{"error": err.Error()})
		}
	case <-stop.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			server.Close()
		}
	}
}
