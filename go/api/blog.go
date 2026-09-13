package api

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"server/types"
	"sort"
	"strings"
	"time"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var blogDir = "blog-posts"

var blogSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var blogMarkdown = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Footnote,
		highlighting.NewHighlighting(
			highlighting.WithStyle("solarized-light"),
			highlighting.WithFormatOptions(
				chromahtml.WithLineNumbers(true),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
	),
)

func BlogHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/blog" || path == "/blog/" {
		posts, err := loadBlogPosts()
		if err != nil {
			http.Error(w, "Failed to load blog posts", http.StatusInternalServerError)
			return
		}

		data := types.BlogData{Posts: posts}
		ServeTemplate(w, r, "blog.html", data)
		return
	}

	slug := strings.TrimSuffix(strings.TrimPrefix(path, "/blog/"), "/")
	post, err := loadBlogPost(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ServeTemplate(w, r, "blog-post.html", post)
}

func loadBlogPosts() ([]types.BlogPost, error) {
	paths, err := filepath.Glob(filepath.Join(blogDir, "*.md"))
	if err != nil {
		return nil, err
	}

	posts := make([]types.BlogPost, 0, len(paths))
	for _, path := range paths {
		post, err := readBlogPost(path)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	return posts, nil
}

func loadBlogPost(slug string) (types.BlogPost, error) {
	if !blogSlugPattern.MatchString(slug) {
		return types.BlogPost{}, fmt.Errorf("invalid blog slug: %q", slug)
	}
	return readBlogPost(filepath.Join(blogDir, slug+".md"))
}

func readBlogPost(filePath string) (types.BlogPost, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return types.BlogPost{}, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	post, err := parseMarkdownPost(string(content), filePath)
	if err != nil {
		return types.BlogPost{}, fmt.Errorf("failed to parse post %s: %w", filePath, err)
	}

	return post, nil
}

func parseMarkdownPost(content, filePath string) (types.BlogPost, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	if len(lines) < 2 {
		return types.BlogPost{}, fmt.Errorf("post must have at least title and date lines")
	}

	title := strings.TrimSpace(strings.TrimPrefix(lines[0], "#"))
	if title == "" {
		return types.BlogPost{}, fmt.Errorf("post must have a title on the first line")
	}

	dateStr := strings.TrimSpace(strings.TrimPrefix(lines[1], "##"))
	if dateStr == "" {
		return types.BlogPost{}, fmt.Errorf("post must have a date on the second line")
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return types.BlogPost{}, fmt.Errorf("invalid date format: %w", err)
	}

	var body string
	if len(lines) > 3 {
		body = strings.Join(lines[3:], "\n")
	}

	var buf bytes.Buffer
	if err := blogMarkdown.Convert([]byte(body), &buf); err != nil {
		return types.BlogPost{}, fmt.Errorf("failed to convert markdown: %w", err)
	}

	return types.BlogPost{
		Slug:     strings.TrimSuffix(filepath.Base(filePath), ".md"),
		Title:    title,
		Date:     date,
		Content:  template.HTML(buf.String()),
		FilePath: filePath,
	}, nil
}
