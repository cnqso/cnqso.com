package jobs

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"server/db"
	"server/logs"
	"strconv"
	"strings"
	"time"

	_ "image/gif"

	"github.com/gocolly/colly"
	"github.com/nfnt/resize"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var KnownPostIDs map[string]bool

var Scraper = colly.NewCollector(
	colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"),
)

func ScrapePetrarchan() {
	startTime := time.Now()
	// logs.INFO("Starting catalog scrape...")

	err := os.MkdirAll("static/petrarchive", 0755)
	if err != nil {
		logs.ERROR(fmt.Sprintf("Error creating archive directory: %v", err))
	}

	loadKnownPostIDs()
	// logs.INFO(fmt.Sprintf("Loaded %d known post IDs from database", len(KnownPostIDs)))

	threads, err := fetchPetrarchanCatalog()
	if err != nil {
		log.Fatal(err)
	}

	threadsToScrape, newCount, updatedCount, err := processCatalogUpdates(threads)
	if err != nil {
		log.Fatal(err)
	}
	logs.INFO(fmt.Sprintf("Identified %d new threads and %d updated threads to scrape", newCount, updatedCount))

	maxThreads := 61 // No limit - catalog only holds 60
	if len(threadsToScrape) > maxThreads {
		threadsToScrape = threadsToScrape[:maxThreads]
	}

	if len(threadsToScrape) > 0 {
		logs.INFO(fmt.Sprintf("Scraping %d threads", len(threadsToScrape)))
		for _, threadID := range threadsToScrape {
			logs.INFO(fmt.Sprintf("Scraping thread %s", threadID))
			err := ScrapePost(threadID)
			if err != nil {
				logs.INFO(fmt.Sprintf("Error scraping thread %s: %v", threadID, err))
			}
		}
	}

	auditReplyCounts()
	logs.INFO(fmt.Sprintf("Scrape complete in %s", time.Since(startTime)))
}

func loadKnownPostIDs() {
	KnownPostIDs = make(map[string]bool)

	rows, err := db.DB.Query("SELECT id FROM posts")
	if err != nil {
		logs.ERROR(fmt.Sprintf("Error querying post IDs: %v", err))
		return
	}
	defer rows.Close()

	var id string
	for rows.Next() {
		if err := rows.Scan(&id); err != nil {
			logs.ERROR(fmt.Sprintf("Error scanning ID: %v", err))
			continue
		}
		KnownPostIDs[id] = true
	}

	auditReplyCounts()
}

func auditReplyCounts() {

	rows, err := db.DB.Query("SELECT id FROM posts WHERE thread_owner = true")
	if err != nil {
		logs.ERROR(fmt.Sprintf("Error querying thread IDs: %v", err))
		return
	}
	defer rows.Close()

	var threadIDs []string
	var threadID string
	for rows.Next() {
		if err := rows.Scan(&threadID); err != nil {
			logs.ERROR(fmt.Sprintf("Error scanning thread ID: %v", err))
			continue
		}
		threadIDs = append(threadIDs, threadID)
	}

	// logs.INFO(fmt.Sprintf("Auditing reply counts for %d threads", len(threadIDs)))
	var updatedCount int

	for _, threadID := range threadIDs {
		var currentReplyCount int
		err := db.DB.QueryRow("SELECT replies FROM posts WHERE id = ?", threadID).Scan(&currentReplyCount)
		if err != nil {
			logs.ERROR(fmt.Sprintf("Error getting current reply count for thread %s: %v", threadID, err))
			continue
		}

		var actualReplyCount int
		err = db.DB.QueryRow("SELECT COUNT(*) FROM posts WHERE thread = ? AND thread_owner = false", threadID).Scan(&actualReplyCount)
		if err != nil {
			logs.INFO(fmt.Sprintf("Error counting actual replies for thread %s: %v", threadID, err))
			continue
		}

		if currentReplyCount != actualReplyCount {
			_, err := db.DB.Exec("UPDATE posts SET replies = ? WHERE id = ?", actualReplyCount, threadID)
			if err != nil {
				logs.INFO(fmt.Sprintf("Error updating reply count for thread %s: %v", threadID, err))
				continue
			}
			logs.INFO(fmt.Sprintf("Thread %s: Updated reply count from %d to %d", threadID, currentReplyCount, actualReplyCount))
			updatedCount++
		}
	}

	logs.INFO(fmt.Sprintf("Reply count audit complete: Updated %d threads", updatedCount))
}

type ThreadInfo struct {
	URL     string
	Replies int
}

func fetchPetrarchanCatalog() ([]ThreadInfo, error) {
	var threads []ThreadInfo

	c := colly.NewCollector()
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*petrarchan.com*",
		Parallelism: 1,
		RandomDelay: 1 * time.Second,
	})

	var scrapeError error
	c.OnError(func(r *colly.Response, err error) {
		scrapeError = err
	})

	c.OnHTML(".preview", func(e *colly.HTMLElement) {
		href := e.ChildAttr("a", "href")
		if href == "" {
			return
		}

		absoluteURL := e.Request.AbsoluteURL(href)

		repliesText := e.ChildText(".counts .count:first-child")
		replies := 0

		if repliesText != "" {
			repliesStr := strings.Fields(repliesText)[0]
			if parsedReplies, err := strconv.Atoi(repliesStr); err == nil {
				replies = parsedReplies
			}
		}

		threads = append(threads, ThreadInfo{
			URL:     absoluteURL,
			Replies: replies,
		})
	})

	catalogURL := "https://petrarchan.com/pt/catalog"
	logs.INFO(fmt.Sprintf("Visiting catalog URL: %s", catalogURL))
	err := c.Visit(catalogURL)
	if err != nil {
		return nil, fmt.Errorf("error visiting catalog: %v", err)
	}

	if scrapeError != nil {
		return nil, scrapeError
	}

	logs.INFO(fmt.Sprintf("Successfully scraped catalog, found %d threads", len(threads)))
	return threads, nil
}

func processCatalogUpdates(threads []ThreadInfo) ([]string, int, int, error) {
	var threadsToScrape []string
	var newCount, updatedCount int
	var skippedCount int = 0
	for _, thread := range threads {
		parts := strings.Split(thread.URL, "/")
		if len(parts) == 0 {
			continue
		}
		threadID := parts[len(parts)-1]

		if !KnownPostIDs[threadID] {
			threadsToScrape = append(threadsToScrape, threadID)
			newCount++
			logs.INFO(fmt.Sprintf("New thread found: %s (%d replies)", threadID, thread.Replies))
			continue
		}

		var currentReplies int
		err := db.DB.QueryRow("SELECT replies FROM posts WHERE id = ? AND thread_owner = true", threadID).Scan(&currentReplies)

		if err != nil {
			logs.ERROR(fmt.Sprintf("Error checking replies for thread %s: %v", threadID, err))
			continue
		}

		if thread.Replies > currentReplies {
			threadsToScrape = append(threadsToScrape, threadID)
			updatedCount++
			logs.INFO(fmt.Sprintf("Thread updated: %s (%d -> %d replies)", threadID, currentReplies, thread.Replies))
		} else {
			// logs.INFO(fmt.Sprintf("Skipping thread %s: no new replies (%d replies)", threadID, currentReplies))
			skippedCount++
		}
	}

	// logs.INFO(fmt.Sprintf("Skipping %d stale threads", skippedCount))

	return threadsToScrape, newCount, updatedCount, nil
}

type Post struct {
	ID        string
	Title     string
	Poster    string
	Timestamp string
	Body      string
	ImageURL  string
	IsOP      bool
	Replies   int // (Only relevant for OP posts)
}

func ScrapePost(threadID string) error {
	var posts []Post
	threadURL := "https://petrarchan.com/pt/thread/" + threadID

	logs.INFO(fmt.Sprintf("Visiting thread URL: %s", threadURL))

	c := colly.NewCollector()
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*petrarchan.com*",
		Parallelism: 1,
		RandomDelay: 2 * time.Second,
	})

	var scrapeError error
	c.OnError(func(r *colly.Response, err error) {
		scrapeError = err
		logs.ERROR(fmt.Sprintf("Scrape error: %v", err))
	})

	err := os.MkdirAll("static/petrarchive", 0755)
	if err != nil {
		logs.ERROR(fmt.Sprintf("Error creating directory: %v", err))
	}

	c.OnHTML(".post.orig", func(e *colly.HTMLElement) {
		post := Post{}

		postNumText := e.ChildText("a.subtle-link")

		post.ID, _ = strings.CutPrefix(postNumText, "No.")

		skipContentExtraction := KnownPostIDs[post.ID]
		if skipContentExtraction {
			logs.INFO(fmt.Sprintf("OP %s already in database, updating reply count only", post.ID))
		}

		if !skipContentExtraction {
			post.Title = e.ChildText("span.post-title")

			post.Poster = e.ChildText("span.post-nick")

			post.Timestamp = e.ChildAttr("span.post-time", "title")

			post.Body = extractCleanPostBody(e, "p.post-body")

			post.ImageURL = e.ChildAttr(".post-image-frame a", "href")
			if post.ImageURL != "" {
				post.ImageURL = e.Request.AbsoluteURL(post.ImageURL)
			}
		}

		post.IsOP = true
		posts = append(posts, post)

		logs.INFO(fmt.Sprintf("Found OP: ID=%s, Title=%s, Poster=%s, Timestamp=%s",
			post.ID, post.Title, post.Poster, post.Timestamp))
	})

	c.OnHTML(".post.reply", func(e *colly.HTMLElement) {
		post := Post{}

		postNumText := e.ChildText("a.subtle-link")
		post.ID, _ = strings.CutPrefix(postNumText, "No.")

		if KnownPostIDs[post.ID] {
			// logs.INFO(fmt.Sprintf("Skipping reply %s: already in database", post.ID))
			return
		}

		post.Title = ""

		post.Poster = e.ChildText("span.post-nick")

		post.Timestamp = e.ChildAttr("span.post-time", "title")

		post.Body = extractCleanPostBody(e, "p.post-body")
		if post.Body == "" {
			post.Body = extractCleanPostBody(e, "div.post-body")
		}

		post.ImageURL = e.ChildAttr(".post-image-frame a", "href")
		if post.ImageURL != "" {
			post.ImageURL = e.Request.AbsoluteURL(post.ImageURL)
		}

		post.IsOP = false
		posts = append(posts, post)
	})

	err = c.Visit(threadURL)
	if err != nil {
		return err
	}

	if scrapeError != nil {
		return scrapeError
	}

	logs.INFO(fmt.Sprintf("Thread %s: Found %d posts total", threadID, len(posts)))

	for _, post := range posts {

		if !post.IsOP && KnownPostIDs[post.ID] {
			logs.INFO(fmt.Sprintf("Skipping known post %s", post.ID))
			continue
		}

		if post.IsOP && KnownPostIDs[post.ID] {
			_, err := db.DB.Exec("UPDATE posts SET replies = ? WHERE id = ?",
				len(posts)-1, post.ID)
			if err != nil {
				logs.ERROR(fmt.Sprintf("Error updating reply count for post %s: %v", post.ID, err))
			} else {
				logs.INFO(fmt.Sprintf("Updated reply count for post %s to %d", post.ID, len(posts)-1))
			}
			continue
		}

		var localImagePath string
		if post.ImageURL != "" {
			localPath, err := downloadImage(post.ImageURL, post.ID)
			if err != nil {
				logs.ERROR(fmt.Sprintf("Error downloading image: %v", err))
			} else {
				localImagePath = localPath
				logs.INFO(fmt.Sprintf("Downloaded image to: %s", localImagePath))
			}
		}

		timestamp, err := time.Parse("2006-01-02 15:04:05 UTC", post.Timestamp)
		if err != nil {
			logs.ERROR(fmt.Sprintf("Error parsing timestamp '%s': %v", post.Timestamp, err))
			timestamp = time.Now() // fallback
		}

		if post.IsOP {
			post.Replies = len(posts) - 1
		}

		err = storePost(post, threadID, timestamp, localImagePath)
		if err != nil {
			logs.ERROR(fmt.Sprintf("Error storing post %s: %v", post.ID, err))
		} else {
			logs.INFO(fmt.Sprintf("Successfully stored post %s", post.ID))
			KnownPostIDs[post.ID] = true
		}
	}

	return nil
}

func extractCleanPostBody(e *colly.HTMLElement, selector string) string {
	postBodyElement := e.DOM.Find(selector).First()
	if postBodyElement.Length() == 0 {
		return ""
	}

	postBodyElement.Find(".fwd-links").Remove()
	postBodyElement.Find(".floating-preview").Remove()

	return strings.TrimSpace(postBodyElement.Text())
}

func downloadImage(imageURL, postID string) (string, error) {

	localPath := filepath.Join("static", "petrarchive", postID)

	resp, err := http.Get(imageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	_, err = file.Write(imageData)
	if err != nil {
		return "", err
	}

	err = createThumbnail(localPath)
	if err != nil {
		logs.ERROR(fmt.Sprintf("Warning: Failed to create thumbnail for %s: %v", localPath, err))
	}

	return localPath, nil
}

func createThumbnail(imagePath string) error {
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return err
	}

	thumbnail := resize.Thumbnail(150, 150, img, resize.Lanczos3)

	ext := filepath.Ext(imagePath)
	thumbPath := strings.TrimSuffix(imagePath, ext) + "_thumb" + ext

	thumbFile, err := os.Create(thumbPath)
	if err != nil {
		return err
	}
	defer thumbFile.Close()

	switch format {
	case "jpeg":
		err = jpeg.Encode(thumbFile, thumbnail, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(thumbFile, thumbnail)
	default:
		err = jpeg.Encode(thumbFile, thumbnail, &jpeg.Options{Quality: 85})
	}

	return err
}

func storePost(post Post, threadID string, timestamp time.Time, imagePath string) error {
	_, err := db.DB.Exec(`
		INSERT OR REPLACE INTO posts (id, date, title, poster, contents, thread_owner, thread, replies, image_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		post.ID,
		timestamp,
		post.Title,
		post.Poster,
		post.Body,
		post.IsOP,
		threadID,
		post.Replies,
		imagePath,
	)
	return err
}

func ForceRegenerateThumbnails() error {
	logs.INFO("Force regenerating all thumbnails...")

	files, err := filepath.Glob("static/petrarchive/*")
	if err != nil {
		return err
	}

	var processed, errors int

	for _, file := range files {
		if strings.Contains(file, "_thumb") {
			continue
		}

		err := createThumbnail(file)
		if err != nil {
			logs.ERROR(fmt.Sprintf("Error creating thumbnail for %s: %v", file, err))
			errors++
		} else {
			processed++
		}
	}

	logs.INFO(fmt.Sprintf("Thumbnail regeneration complete: %d processed, %d errors", processed, errors))
	return nil
}
