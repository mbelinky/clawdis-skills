package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomCategoryFromConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := `categories:
  razor:
    description: "AI agents"
    keywords: [agent]
  pottery:
    description: "Ceramics, pottery business, kiln, glazes"
    keywords: [ceramic, pottery, kiln, glaze, clay]

routing:
  razor:
    action: notify
    notify: false
  pottery:
    action: save_obsidian
    path: "Pottery/Twitter-Bookmarks"
    notify: false
`
	if err := os.WriteFile(configPath, []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadBookmarksConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	tweet := Tweet{
		ID:   "123",
		Raw:  "New kiln and glaze setup for the pottery studio",
		Text: "New kiln and glaze setup for the pottery studio",
	}

	category := categorizeTweet(tweet, cfg.Categories)
	if category != "pottery" {
		t.Fatalf("expected pottery category, got %q", category)
	}

	prompt := buildGeminiPrompt(tweet, cfg.Categories)
	if !strings.Contains(prompt, "- pottery:") {
		t.Fatalf("prompt missing pottery category: %s", prompt)
	}
}
