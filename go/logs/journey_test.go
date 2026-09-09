package logs

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJourneyRecognitionAndPrivacy(t *testing.T) {
	for _, tt := range []struct {
		name, path, media, method, signal string
		cookie                            bool
		wantKind                          string
		wantCookie                        bool
	}{
		{"document", "/", "text/html", "GET", "", false, "page", true},
		{"returning", "/blog/", "text/html", "GET", "", true, "page", false},
		{"asset", "/static/app.js", "text/javascript", "GET", "", false, "", false},
		{"admin", "/admin/odir/", "text/html", "GET", "", true, "", false},
		{"dashboard", "/dashboard/journeys", "text/html", "GET", "", true, "", false},
		{"pdf", "/odir/book.pdf", "application/pdf", "GET", "", false, "download", true},
		{"head", "/", "text/html", "HEAD", "", false, "", false},
		{"GPC", "/", "text/html", "GET", "Sec-GPC", false, "", false},
		{"DNT", "/", "text/html", "GET", "DNT", true, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "https://example.test"+tt.path, nil)
			if tt.cookie {
				r.AddCookie(&http.Cookie{Name: visitorCookie, Value: "0123456789abcdef0123456789abcdef"})
			}
			if tt.signal != "" {
				r.Header.Set(tt.signal, "1")
			}
			w := httptest.NewRecorder()
			request, before := prepareJourney(r, w)
			w.Header().Set("Content-Type", tt.media)
			before(200)
			meta := request.Context().Value(journeyKey{}).(*journeyMetadata)
			if meta.Kind != tt.wantKind {
				t.Fatalf("kind %s", meta.Kind)
			}
			cookies := w.Result().Cookies()
			if (len(cookies) > 0) != tt.wantCookie {
				t.Fatalf("cookie count %d", len(cookies))
			}
			if tt.wantKind != "" && !validVisitorToken(meta.VisitorID) {
				t.Fatal("missing valid visitor")
			}
			if (tt.signal != "" || privateAnalyticsPath(tt.path)) && meta.VisitorID != "" {
				t.Fatal("recognized opted-out/admin browser")
			}
			if len(cookies) > 0 && cookies[0].MaxAge > 0 {
				c := cookies[0]
				if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 2592000 {
					t.Fatalf("cookie policy %+v", c)
				}
			}
		})
	}
}

func TestReferrerAndDownloadClassification(t *testing.T) {
	for _, tt := range []struct{ raw, want string }{
		{"https://search.example/find?q=private", "https://search.example"},
		{"https://example.test/blog/post?token=secret#section", "/blog/post"},
		{"https://example.test/admin/odir/browse/Private.epub", ""},
		{"javascript:alert(1)", ""},
	} {
		r := httptest.NewRequest("GET", "https://example.test/", nil)
		r.Header.Set("Referer", tt.raw)
		if got := journeyReferrer(r); got != tt.want {
			t.Fatalf("referrer %s", got)
		}
	}
	r := httptest.NewRequest("GET", "https://example.test/book", nil)
	h := http.Header{"Content-Type": {"application/octet-stream"}, "Content-Disposition": {"attachment; filename=book.epub"}}
	if journeyKind(r, h, 206) != "download" || journeyKind(r, h, 404) != "" {
		t.Fatal("download classification")
	}
}
