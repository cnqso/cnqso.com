package api

import (
	"net/url"
	"os"
	"path/filepath"
	"server/config"
	"testing"
)

func TestLoadOpenDirectoryDocuments(t *testing.T) {
	originalDir := config.OpenDirectoryDir
	config.OpenDirectoryDir = t.TempDir()
	t.Cleanup(func() {
		config.OpenDirectoryDir = originalDir
	})

	papersDir := filepath.Join(config.OpenDirectoryDir, "papers")
	if err := os.Mkdir(papersDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Zine.pdf", "book with spaces.PDF", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(papersDir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	documents, err := loadOpenDirectoryDocuments()
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 {
		t.Fatalf("got %d documents, want 2", len(documents))
	}
	if documents[0].Name != "book with spaces.PDF" || documents[1].Name != "Zine.pdf" {
		t.Fatalf("documents are not sorted by name: %#v", documents)
	}

	for _, document := range documents {
		name, err := url.PathUnescape(document.URL[len(openDirectoryPapersPath):])
		if err != nil || name != document.Name {
			t.Fatalf("document URL %q does not round-trip to %q", document.URL, document.Name)
		}
	}
}
