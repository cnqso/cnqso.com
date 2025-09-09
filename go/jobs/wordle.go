package jobs

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"server/db"
	"server/logs"
	"sort"
	"strings"
	"time"
)

type WordleResponse struct {
	Solution        string `json:"solution"`
	PrintDate       string `json:"print_date"`
	DaysSinceLaunch int    `json:"days_since_launch"`
}
type WordleEntry struct {
	ID   int    `json:"number"`
	Date string `json:"date"`
	Word string `json:"word"`
}

func fetchTodaysWordle() (*WordleResponse, error) {
	today := time.Now()
	url := fmt.Sprintf("https://www.nytimes.com/svc/wordle/v2/%s.json", today.Format("2006-01-02"))
	return fetchWordleData(url)
}

func fetchWordleByID(id int) (*WordleResponse, error) {
	start := time.Date(2021, 6, 19, 0, 0, 0, 0, time.UTC)
	target := start.AddDate(0, 0, id)
	url := fmt.Sprintf("https://www.nytimes.com/svc/wordle/v2/%s.json", target.Format("2006-01-02"))
	return fetchWordleData(url)
}

func fetchWordleData(url string) (*WordleResponse, error) {
	logs.INFO("Fetching wordle from NYT at URL: " + url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var wordleResp WordleResponse
	if err := json.Unmarshal(body, &wordleResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	wordleResp.Solution = strings.ToUpper(wordleResp.Solution)

	return &wordleResp, nil
}

func insertWordleEntry(db *sql.DB, entry WordleEntry) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM wordle WHERE id = ?", entry.ID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing entry: %v", err)
	}

	if count > 0 {
		_, err = db.Exec("UPDATE wordle SET date = ?, word = ? WHERE id = ?", entry.Date, strings.ToUpper(entry.Word), entry.ID)
		if err != nil {
			return fmt.Errorf("failed to update entry: %v", err)
		}
	} else {
		_, err = db.Exec("INSERT INTO wordle (id, date, word) VALUES (?, ?, ?)", entry.ID, entry.Date, strings.ToUpper(entry.Word))
		if err != nil {
			return fmt.Errorf("failed to insert entry: %v", err)
		}
	}

	return nil
}

func getAllEntries(db *sql.DB) ([]WordleEntry, error) {
	rows, err := db.Query("SELECT id, date, word FROM wordle ORDER BY date DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %v", err)
	}
	defer rows.Close()

	var entries []WordleEntry
	for rows.Next() {
		var entry WordleEntry
		err := rows.Scan(&entry.ID, &entry.Date, &entry.Word)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func writeToTextFile(entries []WordleEntry, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create text file: %v", err)
	}
	defer file.Close()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date > entries[j].Date
	})

	for _, entry := range entries {
		date := entry.Date[0:10]
		_, err = fmt.Fprintf(file, "%s %d %s\n", date, entry.ID, entry.Word)

		if err != nil {
			return fmt.Errorf("failed to write to text file: %v", err)
		}
	}
	return nil
}

type JSONEntry struct {
	ID   string `json:"id"`
	Word string `json:"word"`
}

func writeToJSONFile(entries []WordleEntry, filename string) error {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID > entries[j].ID // make sure it's descending
	})

	jsonEntries := make([][2]string, 0, len(entries))
	for _, entry := range entries {
		jsonEntries = append(jsonEntries, [2]string{
			fmt.Sprintf("%d", entry.ID),
			entry.Word,
		})
	}

	buf := &bytes.Buffer{}
	buf.WriteString("{\n")
	for i, kv := range jsonEntries {
		fmt.Fprintf(buf, "  %q: %q", kv[0], kv[1])
		if i < len(jsonEntries)-1 {
			buf.WriteString(",\n")
		} else {
			buf.WriteString("\n")
		}
	}
	buf.WriteString("}")

	return os.WriteFile(filename, buf.Bytes(), 0644)
}

func updateWordleDB() {

	todaysWordle, err := fetchTodaysWordle()
	maxID := todaysWordle.DaysSinceLaunch
	entries, err := getAllEntries(db.DB)
	if err != nil {
		logs.ERROR("Failed to fetch wordle data: %v", err)
		return
	}
	expected := make(map[int]bool, maxID+2)
	for i, _ := range entries {
		expected[entries[i].ID] = true
	}

	var idsToFetch []int
	for i := 0; i <= maxID+7; i++ {
		if !expected[i] {
			idsToFetch = append(idsToFetch, i)
		}
	}
	logs.INFO(fmt.Sprintf("Missing %d wordle answers", len(idsToFetch)))
	for _, id := range idsToFetch {
		fmt.Println(id)
	}

	for _, id := range idsToFetch {
		wordleData, err := fetchWordleByID(id)
		if err != nil {
			logs.ERROR("Failed to fetch wordle for id", id)
			continue
		}
		entry := WordleEntry{
			ID:   wordleData.DaysSinceLaunch,
			Date: wordleData.PrintDate,
			Word: wordleData.Solution,
		}
		if err := insertWordleEntry(db.DB, entry); err != nil {
			logs.ERROR("Failed to insert wordle entry: %v", err)
		} else {
			logs.INFO("Successfully processed Wordle #%d: %s (%s)\n", entry.ID, entry.Word, entry.Date)
		}
	}
}

func pushToGithub() error {
	repoPath := "../wordle-data"

	commands := [][]string{
		{"git", "-C", repoPath, "commit", "-am", "Auto-update wordle data"},
		{"git", "-C", repoPath, "push"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s failed: %v\n%s", args[1], err, string(out))
		}
	}
	logs.INFO("Pushed updated wordle data to github")
	return nil
}

func GetWordle() {
	updateWordleDB()

	entries, err := getAllEntries(db.DB)
	if err != nil {
		log.Fatal("Failed to get entries:", err)
	}

	if err := writeToTextFile(entries, "../wordle-data/answers.txt"); err != nil {
		logs.ERROR("Failed to write text file: %v", err)
	} else {
		logs.INFO("Successfully wrote answers.txt")
	}

	if err := writeToJSONFile(entries, "../wordle-data/answers.json"); err != nil {
		logs.ERROR("Failed to write JSON file: %v", err)
	} else {
		logs.INFO("Successfully wrote answers.json")
	}

	pushToGithub()
}
