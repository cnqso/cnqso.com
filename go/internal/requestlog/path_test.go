package requestlog

import "testing"

func TestPath(t *testing.T) {
	for raw, want := range map[string]string{
		"/Admin/config.php?password=secret":                "/admin/",
		"/admin/../public":                                 "/admin/",
		"/dashboard/../../public":                          "/dashboard",
		"/api/dashboard/../../public":                      "/api/dashboard",
		"/public/../admin/odir/private.epub":               "/admin/odir/",
		"/book?token=secret#section":                       "/book",
		"https://user:password@example.test/book?q=secret": "/book",
		"/odir/papers/A%20Book.pdf":                        "/odir/papers/A%20Book.pdf",
		"/book%3Ftitle.pdf?token=secret":                   "/book%3Ftitle.pdf",
		"/admin/odir/browse/private.epub":                  "/admin/odir/",
		"/%61dmin/odir/browse/private.epub":                "/admin/odir/",
		"/admin/settings?secret=1":                         "/admin/",
		"/dashboard/journey/0123456789abcdef":              "/dashboard",
		"/api/dashboard/ip/192.0.2.1?period=30d":           "/api/dashboard",
		"/dashboard-public":                                "/dashboard-public",
		"[malformed?token=secret":                          "[redacted]",
		"[redacted]":                                       "[redacted]",
	} {
		if got := Path(raw); got != want {
			t.Errorf("Path(%q) = %q, want %q", raw, got, want)
		}
	}
}
