package api

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
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

func BlogHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	fmt.Println(path)
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

	slug := strings.TrimPrefix(path, "/blog/")
	slug = strings.TrimSuffix(slug, "/")

	if slug != "" {
		post, err := loadBlogPost(slug)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		ServeTemplate(w, r, "blog-post.html", post)
		return
	}

	http.NotFound(w, r)
}

func loadBlogPosts() ([]types.BlogPost, error) {
	var posts []types.BlogPost

	md := goldmark.New(
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

	blogDir := "blog-posts"

	err := filepath.Walk(blogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		post, err := parseMarkdownPost(string(content), path, md)
		if err != nil {
			return fmt.Errorf("failed to parse post %s: %w", path, err)
		}

		posts = append(posts, post)
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	return posts, nil
}

func loadBlogPost(slug string) (types.BlogPost, error) {
	md := goldmark.New(
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

	filePath := filepath.Join("blog-posts", slug+".md")
	fmt.Println(filePath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return types.BlogPost{}, fmt.Errorf("blog post not found: %s", slug)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return types.BlogPost{}, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	post, err := parseMarkdownPost(string(content), filePath, md)
	if err != nil {
		return types.BlogPost{}, fmt.Errorf("failed to parse post %s: %w", filePath, err)
	}

	return post, nil
}

func parseMarkdownPost(content, filePath string, md goldmark.Markdown) (types.BlogPost, error) {
	lines := strings.Split(content, "\n")

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

	slug := strings.TrimSuffix(filepath.Base(filePath), ".md")

	contentLines := lines
	if len(contentLines) > 3 {
		contentLines = contentLines[3:]
	} else {
		contentLines = []string{}
	}
	contentWithoutHeader := strings.Join(contentLines, "\n")

	var buf bytes.Buffer
	if err := md.Convert([]byte(contentWithoutHeader), &buf); err != nil {
		return types.BlogPost{}, fmt.Errorf("failed to convert markdown: %w", err)
	}

	return types.BlogPost{
		Slug:     slug,
		Title:    title,
		Date:     date,
		Content:  template.HTML(buf.String()),
		FilePath: filePath,
	}, nil
}
