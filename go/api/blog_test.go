package api

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupBlog(t *testing.T, posts map[string]string) {
	t.Helper()
	oldDir := blogDir
	blogDir = t.TempDir()
	t.Cleanup(func() { blogDir = oldDir })
	for name, content := range posts {
		if err := os.WriteFile(filepath.Join(blogDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParseMarkdownPost(t *testing.T) {
	post, err := parseMarkdownPost("# Hello World\r\n## 2024-03-01\r\n\r\nSome *text*.", "blog-posts/hello-world.md")
	if err != nil {
		t.Fatal(err)
	}
	if post.Slug != "hello-world" || post.Title != "Hello World" || post.Date.Format("2006-01-02") != "2024-03-01" {
		t.Fatalf("unexpected post metadata: %+v", post)
	}
	if !strings.Contains(string(post.Content), "<em>text</em>") {
		t.Fatalf("markdown not rendered: %q", post.Content)
	}
}

func TestParseMarkdownPostRejectsBadHeaders(t *testing.T) {
	for name, content := range map[string]string{
		"too short":    "# Title",
		"empty title":  "#\n## 2024-03-01\n\nbody",
		"empty date":   "# Title\n##\n\nbody",
		"invalid date": "# Title\n## March 1st\n\nbody",
	} {
		if _, err := parseMarkdownPost(content, "x.md"); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestLoadBlogPostsSortsNewestFirst(t *testing.T) {
	setupBlog(t, map[string]string{
		"old.md":     "# Old\n## 2020-01-01\n\nold",
		"new.md":     "# New\n## 2024-01-01\n\nnew",
		"ignore.txt": "not a post",
	})
	posts, err := loadBlogPosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 || posts[0].Slug != "new" || posts[1].Slug != "old" {
		t.Fatalf("unexpected posts: %+v", posts)
	}
}

func TestBlogHandlerRejectsUnsafeSlugs(t *testing.T) {
	setupBlog(t, map[string]string{"post.md": "# Post\n## 2024-01-01\n\nbody"})
	outside := filepath.Join(filepath.Dir(blogDir), "secret.md")
	if err := os.WriteFile(outside, []byte("# Secret\n## 2024-01-01\n\nsecret"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(outside) })

	for _, slug := range []string{"../secret", "..%2Fsecret", "Post", "missing", ""} {
		if _, err := loadBlogPost(slug); err == nil {
			t.Errorf("slug %q: expected error", slug)
		}
	}
	if _, err := loadBlogPost("post"); err != nil {
		t.Fatalf("valid slug rejected: %v", err)
	}

	rec := httptest.NewRecorder()
	BlogHandler(rec, httptest.NewRequest(http.MethodGet, "/blog/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSitemapHandler(t *testing.T) {
	setupBlog(t, map[string]string{
		"first.md":  "# First\n## 2023-05-01\n\nbody",
		"second.md": "# Second\n## 2024-06-02\n\nbody",
	})
	rec := httptest.NewRecorder()
	SitemapHandler(rec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Fatalf("unexpected content type %q", ct)
	}

	var set sitemapURLSet
	if err := xml.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatal(err)
	}
	lastMods := map[string]string{}
	for _, u := range set.URLs {
		if _, dup := lastMods[u.Loc]; dup {
			t.Errorf("duplicate URL %s", u.Loc)
		}
		lastMods[u.Loc] = u.LastMod
	}
	if len(set.URLs) != len(sitemapPages)+2 {
		t.Fatalf("expected %d URLs, got %d", len(sitemapPages)+2, len(set.URLs))
	}
	for loc, want := range map[string]string{
		"https://cnqso.com/":            "",
		"https://cnqso.com/blog/":       "2024-06-02",
		"https://cnqso.com/blog/first":  "2023-05-01",
		"https://cnqso.com/blog/second": "2024-06-02",
	} {
		got, ok := lastMods[loc]
		if !ok {
			t.Errorf("missing %s", loc)
		} else if got != want {
			t.Errorf("%s lastmod = %q, want %q", loc, got, want)
		}
	}
}
