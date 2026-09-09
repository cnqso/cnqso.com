// Package requestlog defines the URL information safe to retain in request logs.
package requestlog

import (
	"net/url"
	"path"
	"strings"
)

// Path omits query strings, fragments, origins, and private admin identifiers.
// The same rule applies to new requests, historical records, and API responses.
func Path(raw string) string {
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return "[redacted]"
	}
	// Historical scanner requests can contain traversal segments. Redact both
	// the requested private prefix and a private destination after normalization.
	for _, p := range []string{u.Path, path.Clean(u.Path)} {
		p = strings.ToLower(p)
		switch {
		case p == "/admin/odir" || strings.HasPrefix(p, "/admin/odir/"):
			return "/admin/odir/"
		case p == "/admin" || strings.HasPrefix(p, "/admin/"):
			return "/admin/"
		case p == "/dashboard" || strings.HasPrefix(p, "/dashboard/"):
			return "/dashboard"
		case p == "/api/dashboard" || strings.HasPrefix(p, "/api/dashboard/"):
			return "/api/dashboard"
		}
	}
	if u.Path == "" {
		return "/"
	}
	return u.EscapedPath()
}
