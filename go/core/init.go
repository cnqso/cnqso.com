package core

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"server/api"
	"server/config"
	"server/db"
	"server/jobs"
	"server/logs"
	"strings"
	"time"
)

func Init() {
	if err := db.InitDatabase(); err != nil {
		logs.ERROR("Failed to initialize logging database", map[string]any{
			"error": err.Error(),
		})
		panic("Failed to initialize logging database: " + err.Error())
	}

	if config.CompileTypeScript {
		go compileTypeScript()
	}
	if err := initTemplates(); err != nil {
		logs.ERROR("Failed to load templates", map[string]any{
			"error": err.Error(),
		})
		panic("Failed to load templates: " + err.Error())
	}

	if err := jobs.Init(); err != nil {
		logs.ERROR("Failed to schedule jobs", map[string]any{
			"error": err.Error(),
		})
		panic("Failed to schedule jobs: " + err.Error())
	}
}

func initTemplates() error {
	api.Templates = make(map[string]*template.Template)

	funcMap := template.FuncMap{
		"thumbnailURL": func(url string) string {
			// From "/static/petrarchive/12345.jpg" to "/static/petrarchive/12345_thumb.jpg"
			ext := filepath.Ext(url)
			return strings.TrimSuffix(url, ext) + "_thumb" + ext
		},
		"processContent": processPostContent,
		"getBacklinks":   getBacklinks,
	}

	templatesDir := "templates"

	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(templatesDir, path)
		if err != nil {
			return err
		}

		key := filepath.ToSlash(relPath)

		tmpl, err := template.New(filepath.Base(path)).Funcs(funcMap).ParseFiles(path)
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", path, err)
		}

		api.Templates[key] = tmpl

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	return nil
}

func processPostContent(content, currentThreadID string) template.HTML {
	re := regexp.MustCompile(`>>\d+`)

	processed := re.ReplaceAllStringFunc(content, func(match string) string {
		postID := strings.TrimPrefix(match, ">>")

		var referencedThreadID string
		err := db.DB.QueryRow("SELECT thread FROM posts WHERE id = ?", postID).Scan(&referencedThreadID)

		if err != nil {
			return fmt.Sprintf(`<span class="post-ref dead-link" title="Post not found">%s</span>`, match)
		}

		var linkClass string
		var href string
		if referencedThreadID == currentThreadID {
			linkClass = "post-ref same-thread"
			href = fmt.Sprintf("#post-%s", postID)
		} else {
			linkClass = "post-ref cross-thread"
			href = fmt.Sprintf("/petrarchive/thread/%s#post-%s", referencedThreadID, postID)
		}

		return fmt.Sprintf(`<a href="%s" class="%s">%s</a>`, href, linkClass, match)
	})

	processed = strings.ReplaceAll(processed, "\n", "<br>")

	processed = strings.TrimSpace(processed)
	processed = strings.TrimPrefix(processed, "<br>")
	processed = strings.TrimSuffix(processed, "<br>")

	return template.HTML(processed)
}

func getBacklinks(postID, threadID string) template.HTML {
	rows, err := db.DB.Query(`
		SELECT id, thread, contents
		FROM posts
		WHERE contents LIKE ?
	`, "%>>"+postID+"%")

	if err != nil {
		return template.HTML("")
	}
	defer rows.Close()

	var backlinks []string
	re := regexp.MustCompile(`>>\d+`)

	for rows.Next() {
		var refPostID, refThreadID, contents string
		if err := rows.Scan(&refPostID, &refThreadID, &contents); err != nil {
			continue
		}

		matches := re.FindAllString(contents, -1)
		for _, match := range matches {
			if strings.TrimPrefix(match, ">>") == postID {
				var linkClass string
				var href string
				if refThreadID == threadID {
					linkClass = "backlink same-thread"
					href = fmt.Sprintf("#post-%s", refPostID)
				} else {
					linkClass = "backlink cross-thread"
					href = fmt.Sprintf("/petrarchive/thread/%s#post-%s", refThreadID, refPostID)
				}

				backlinks = append(backlinks, fmt.Sprintf(
					`<a href="%s" class="%s">&gt;&gt;%s</a>`,
					href, linkClass, refPostID,
				))
				break
			}
		}
	}

	if len(backlinks) == 0 {
		return template.HTML("")
	}

	return template.HTML(fmt.Sprintf(
		`<span class="backlinks">%s</span>`,
		strings.Join(backlinks, " "),
	))
}

func compileTypeScript() {
	start := time.Now()

	cwd, err := os.Getwd()
	if err != nil {
		logs.WARN("Failed to find working directory during TypeScript transpilation", map[string]any{
			"error": err.Error(),
		})
		return
	}

	rootDir := cwd + "/.."

	cmd := exec.Command("npx", config.TypeScriptCompiler)
	cmd.Dir = rootDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.WARN("TypeScript transpilation failed", map[string]any{
			"error":  err.Error(),
			"output": string(output),
		})
		return
	}

	logs.INFO("TypeScript transpilation completed", map[string]any{
		"duration": time.Since(start),
		"output":   string(output),
		"tool":     config.TypeScriptCompiler,
	})
}
