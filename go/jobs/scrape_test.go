package jobs

import "testing"

func TestScrapePetrarchanSkipsWhileAnotherScrapeRuns(t *testing.T) {
	scraping.Lock()
	defer scraping.Unlock()

	// With no database set up, a scrape that ran anyway would panic here.
	ScrapePetrarchan()
}
