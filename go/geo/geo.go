// Package geo labels IP addresses with a coarse location from a local copy of
// DB-IP's free City Lite database, so visitor IPs are never sent to a lookup service.
// DB-IP licenses it under CC BY 4.0; pages that show its data must credit it.
package geo

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"server/config"
	"server/logs"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const maxAge = 35 * 24 * time.Hour

var (
	mu     sync.RWMutex
	reader *maxminddb.Reader
)

type record struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// Init opens the database if present and fetches a fresh one in the background
// when it is missing or a month stale. Lookups return "" until one is loaded.
func Init() {
	if err := open(config.GeoDB); err != nil && !os.IsNotExist(err) {
		logs.ERROR("Failed to open geolocation database", map[string]any{"error": err.Error()})
	}
	if info, err := os.Stat(config.GeoDB); err != nil || time.Since(info.ModTime()) > maxAge {
		go Refresh()
	}
}

// Refresh downloads this month's database, falling back to last month's early in
// the month before DB-IP publishes, and swaps it in once it opens cleanly.
func Refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	now := time.Now().UTC()
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var err error
	for _, month := range []time.Time{thisMonth, thisMonth.AddDate(0, -1, 0)} {
		url := "https://download.db-ip.com/free/dbip-city-lite-" + month.Format("2006-01") + ".mmdb.gz"
		if err = download(ctx, url, config.GeoDB); err == nil {
			if err = open(config.GeoDB); err == nil {
				logs.INFO("Loaded geolocation database", map[string]any{"source": url})
				return nil
			}
		}
	}
	logs.ERROR("Failed to refresh geolocation database", map[string]any{"error": err.Error()})
	return err
}

func download(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, res.Status)
	}
	body, err := gzip.NewReader(res.Body)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".geo-*.mmdb")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = io.Copy(tmp, body); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	// Validate before replacing a working copy.
	check, err := maxminddb.Open(tmp.Name())
	if err != nil {
		return err
	}
	check.Close()
	return os.Rename(tmp.Name(), path)
}

func open(path string) error {
	next, err := maxminddb.Open(path)
	if err != nil {
		return err
	}
	mu.Lock()
	previous := reader
	reader = next
	mu.Unlock()
	if previous != nil {
		previous.Close()
	}
	return nil
}

// Lookup returns a label such as "Ann Arbor, Michigan, US", or "" when the IP is
// private, unknown, or no database is loaded.
func Lookup(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.IsPrivate() || addr.IsLoopback() {
		return ""
	}
	var r record
	mu.RLock()
	if reader != nil {
		err = reader.Lookup(addr.Unmap()).Decode(&r)
	}
	mu.RUnlock()
	if err != nil {
		return ""
	}
	var parts []string
	for _, part := range []string{r.City.Names["en"], firstName(r.Subdivisions), r.Country.ISOCode} {
		if part != "" && (len(parts) == 0 || parts[len(parts)-1] != part) {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ", ")
}

func firstName(subdivisions []struct {
	Names map[string]string `maxminddb:"names"`
}) string {
	if len(subdivisions) == 0 {
		return ""
	}
	return subdivisions[0].Names["en"]
}
