package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type State struct {
	LastProcessed string         `json:"lastProcessed"`
	ProcessedIDs  []string       `json:"processedIds"`
	Categories    map[string]int `json:"categories"`
}

type Tweet struct {
	ID     string
	Raw    string
	Text   string
	Thread string
}

type Config struct {
	ConfigPath   string
	StatePath    string
	ObsidianBase string
	PromptsDir   string
	BirdBin      string
	SummarizeBin string
	GeminiBin    string
	QuietStart   string
	QuietEnd     string
	Limit        int
	Parallel     bool
	Workers      int
}

type CategoryConfig struct {
	Description string   `yaml:"description"`
	Keywords    []string `yaml:"keywords"`
}

type RoutingConfig struct {
	Action string `yaml:"action"`
	Path   string `yaml:"path"`
	Notify bool   `yaml:"notify"`
}

type CategoriesConfig struct {
	Items map[string]CategoryConfig
	Order []string
}

func (c *CategoriesConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("categories must be a mapping")
	}

	c.Items = make(map[string]CategoryConfig)
	c.Order = make([]string, 0, len(node.Content)/2)

	for i := 0; i < len(node.Content); i += 2 {
		key := strings.TrimSpace(node.Content[i].Value)
		if key == "" {
			continue
		}

		var category CategoryConfig
		if err := node.Content[i+1].Decode(&category); err != nil {
			return err
		}

		c.Items[key] = category
		c.Order = append(c.Order, key)
	}

	return nil
}

type BookmarksConfig struct {
	Categories CategoriesConfig         `yaml:"categories"`
	Routing    map[string]RoutingConfig `yaml:"routing"`
}

func (c BookmarksConfig) CategoryOrder() []string {
	if len(c.Categories.Order) > 0 {
		return append([]string{}, c.Categories.Order...)
	}
	return sortedKeys(c.Categories.Items)
}

var (
	config          Config
	bookmarksConfig BookmarksConfig
)

const defaultConfigYAML = `categories:
  razor:
    description: "AI agents, automation, CLI tools, Clawdis ecosystem"
    keywords: [agent, automation, clawdis, "pi coding", "cli tool", terminal, workflow, skill, "claude code", cursor]
  codex:
    description: "Vibe coding, AI-assisted development, Codex CLI"
    keywords: ["vibe coding", codex, "ai-assisted", "ai development", "code generation", "llm coding", "prompt engineering"]
  readLater:
    description: "Articles, videos, long-form content"
    keywords: [article, blog, "http://", "https://", "youtube.com", "youtu.be", "spotify.com", podcast]
  other:
    description: "Unclear or uncategorized"
    keywords: []

routing:
  razor:
    action: razor_task
    path: "Razor-Clawdis/Twitter-Bookmarks"
    notify: true
  codex:
    action: codex_prompt
    path: "Codex-Vibe/Twitter-Bookmarks"
    notify: true
  readLater:
    action: summarize
    notify: true
  other:
    action: notify
    notify: true
`

func main() {
	rootCmd := &cobra.Command{
		Use:   "twitter-bookmarks",
		Short: "Process Twitter bookmarks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProcess(cmd, false)
		},
	}

	rootCmd.PersistentFlags().StringVar(&config.ConfigPath, "config", envOr("TWITTER_BOOKMARKS_CONFIG", filepath.Join(userHomeDir(), ".twitter-bookmarks-config.yaml")), "Path to YAML config")
	rootCmd.PersistentFlags().StringVar(&config.StatePath, "state", envOr("TWITTER_BOOKMARKS_STATE", filepath.Join(userHomeDir(), ".twitter-bookmarks-state.json")), "Path to state file")
	rootCmd.PersistentFlags().StringVar(&config.ObsidianBase, "obsidian", envOr("TWITTER_BOOKMARKS_OBSIDIAN", filepath.Join(userHomeDir(), "Library/CloudStorage/Dropbox/Vault/Private/obsidian/obs_vault_personal")), "Path to Obsidian vault")
	rootCmd.PersistentFlags().StringVar(&config.PromptsDir, "prompts", envOr("TWITTER_BOOKMARKS_PROMPTS", filepath.Join(userHomeDir(), ".codex-prompts")), "Path to Codex prompts directory")
	rootCmd.PersistentFlags().StringVar(&config.BirdBin, "bird", envOr("BIRD_BIN", "bird"), "Path to bird binary")
	rootCmd.PersistentFlags().StringVar(&config.SummarizeBin, "summarize", envOr("SUMMARIZE_BIN", "summarize"), "Path to summarize binary")
	rootCmd.PersistentFlags().StringVar(&config.GeminiBin, "gemini", envOr("GEMINI_BIN", "gemini"), "Path to gemini binary")
	rootCmd.PersistentFlags().StringVar(&config.QuietStart, "quiet-start", envOr("TWITTER_BOOKMARKS_QUIET_START", "23:00"), "Quiet hours start (HH:MM)")
	rootCmd.PersistentFlags().StringVar(&config.QuietEnd, "quiet-end", envOr("TWITTER_BOOKMARKS_QUIET_END", "08:00"), "Quiet hours end (HH:MM)")
	rootCmd.PersistentFlags().IntVar(&config.Limit, "limit", envOrInt("TWITTER_BOOKMARKS_LIMIT", 50), "Bookmarks fetch limit")
	rootCmd.PersistentFlags().BoolVar(&config.Parallel, "parallel", true, "Process bookmarks in parallel")
	rootCmd.PersistentFlags().IntVar(&config.Workers, "workers", 5, "Number of parallel workers")

	processCmd := &cobra.Command{
		Use:   "process",
		Short: "Process new bookmarks",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			return runProcess(cmd, force)
		},
	}
	processCmd.Flags().Bool("force", false, "Reprocess all bookmarks (dangerous)")
	rootCmd.AddCommand(processCmd)

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show processing stats",
		RunE:  runStatus,
	}
	rootCmd.AddCommand(statusCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	if err := loadBookmarksConfigFromFlags(); err != nil {
		return err
	}

	state, err := loadState(config.StatePath, bookmarksConfig.CategoryOrder())
	if err != nil {
		return err
	}

	fmt.Printf("Processed: %d bookmarks\n", len(state.ProcessedIDs))
	fmt.Printf("Last processed: %s\n", emptyIf(state.LastProcessed, "never"))
	fmt.Printf("Categories: %s\n", formatCategoryCounts(bookmarksConfig.CategoryOrder(), state.Categories))
	return nil
}

func runProcess(cmd *cobra.Command, force bool) error {
	if err := loadBookmarksConfigFromFlags(); err != nil {
		return err
	}

	quietStart, err := parseClock(config.QuietStart)
	if err != nil {
		return fmt.Errorf("invalid quiet-start: %w", err)
	}
	quietEnd, err := parseClock(config.QuietEnd)
	if err != nil {
		return fmt.Errorf("invalid quiet-end: %w", err)
	}

	birdPath, err := exec.LookPath(config.BirdBin)
	if err != nil {
		return fmt.Errorf("bird binary not found: %w", err)
	}
	config.BirdBin = birdPath

	if _, err := exec.LookPath(config.SummarizeBin); err != nil {
		config.SummarizeBin = ""
	}
	if _, err := exec.LookPath(config.GeminiBin); err != nil {
		config.GeminiBin = ""
	}

	state, err := loadState(config.StatePath, bookmarksConfig.CategoryOrder())
	if err != nil {
		return err
	}

	fmt.Printf("Processed so far: %d bookmarks\n", len(state.ProcessedIDs))
	fmt.Printf("Categories: %s\n", formatCategoryCounts(bookmarksConfig.CategoryOrder(), state.Categories))

	ids, err := getBookmarks(config.BirdBin, config.Limit)
	if err != nil {
		return err
	}

	processedSet := make(map[string]bool, len(state.ProcessedIDs))
	for _, id := range state.ProcessedIDs {
		processedSet[id] = true
	}

	pending := []string{}
	for _, id := range ids {
		if !force && processedSet[id] {
			continue
		}
		pending = append(pending, id)
	}

	var processedCount int
	if config.Parallel {
		processedCount = processBookmarksParallel(pending, &state, quietStart, quietEnd, processedSet)
	} else {
		processedCount = processBookmarksSequential(pending, &state, quietStart, quietEnd, processedSet)
	}

	if processedCount > 0 {
		state.LastProcessed = time.Now().UTC().Format(time.RFC3339)
		if err := saveState(config.StatePath, state); err != nil {
			return err
		}
	}

	fmt.Printf("Processed %d new bookmarks\n", processedCount)
	fmt.Printf("Updated categories: %s\n", formatCategoryCounts(bookmarksConfig.CategoryOrder(), state.Categories))

	return nil
}

type ProcessResult struct {
	Category  string
	Processed bool
}

func processBookmarksSequential(ids []string, state *State, quietStart, quietEnd int, processedSet map[string]bool) int {
	processedCount := 0
	for _, id := range ids {
		result, err := processBookmark(id, quietStart, quietEnd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to process %s: %v\n", id, err)
			continue
		}
		if result.Processed {
			processedSet[id] = true
			state.ProcessedIDs = appendUnique(state.ProcessedIDs, id)
			state.Categories[result.Category]++
			processedCount++
		}
	}
	return processedCount
}

func processBookmarksParallel(ids []string, state *State, quietStart, quietEnd int, processedSet map[string]bool) int {
	if len(ids) == 0 {
		return 0
	}

	workers := config.Workers
	if workers < 1 {
		workers = 1
	}
	if workers > len(ids) {
		workers = len(ids)
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	processedCount := 0

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				result, err := processBookmark(id, quietStart, quietEnd)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to process %s: %v\n", id, err)
					continue
				}
				if result.Processed {
					mu.Lock()
					processedSet[id] = true
					state.ProcessedIDs = appendUnique(state.ProcessedIDs, id)
					state.Categories[result.Category]++
					processedCount++
					mu.Unlock()
				}
			}
		}()
	}

	for _, id := range ids {
		jobs <- id
	}
	close(jobs)
	wg.Wait()

	return processedCount
}

func loadBookmarksConfigFromFlags() error {
	cfg, err := loadBookmarksConfig(config.ConfigPath)
	if err != nil {
		return err
	}
	bookmarksConfig = cfg
	return nil
}

func loadBookmarksConfig(path string) (BookmarksConfig, error) {
	if strings.TrimSpace(path) == "" {
		return BookmarksConfig{}, errors.New("config path is required")
	}

	if err := ensureDefaultConfig(path); err != nil {
		return BookmarksConfig{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return BookmarksConfig{}, err
	}

	var cfg BookmarksConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return BookmarksConfig{}, err
	}
	if len(cfg.Categories.Items) == 0 {
		return BookmarksConfig{}, errors.New("config must define at least one category")
	}
	if cfg.Routing == nil {
		cfg.Routing = map[string]RoutingConfig{}
	}
	if len(cfg.Categories.Order) == 0 {
		cfg.Categories.Order = sortedKeys(cfg.Categories.Items)
	}

	return cfg, nil
}

func ensureDefaultConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultConfigYAML), 0o644)
}

func formatCategoryCounts(categories []string, counts map[string]int) string {
	if len(categories) == 0 {
		return ""
	}
	parts := make([]string, 0, len(categories))
	for _, category := range categories {
		parts = append(parts, fmt.Sprintf("%s=%d", category, counts[category]))
	}
	return strings.Join(parts, " ")
}

func sortedKeys(values map[string]CategoryConfig) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func loadState(path string, categories []string) (State, error) {
	state := State{
		LastProcessed: "",
		ProcessedIDs:  []string{},
		Categories:    map[string]int{},
	}

	for _, category := range categories {
		state.Categories[category] = 0
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return state, err
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}

	if state.Categories == nil {
		state.Categories = map[string]int{}
	}
	for _, category := range categories {
		ensureCategory(&state, category)
	}

	return state, nil
}

func saveState(path string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func ensureCategory(state *State, key string) {
	if _, ok := state.Categories[key]; !ok {
		state.Categories[key] = 0
	}
}

func getBookmarks(birdBin string, limit int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, birdBin, "bookmarks", "-n", fmt.Sprintf("%d", limit))
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bird bookmarks failed: %w", err)
	}

	ids := []string{}
	seen := map[string]bool{}
	regex := regexp.MustCompile(`status/(\d+)`)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		matches := regex.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		id := matches[1]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

func unbookmarkTweet(birdBin, tweetID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, birdBin, "unbookmark", tweetID)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bird unbookmark failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func readTweet(birdBin, tweetID string) (Tweet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, birdBin, "read", tweetID)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return Tweet{}, fmt.Errorf("bird read failed: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return Tweet{}, errors.New("empty tweet output")
	}

	lines := strings.Split(raw, "\n")
	text := strings.TrimSpace(lines[0])

	thread, _ := readThread(birdBin, tweetID)

	return Tweet{
		ID:     tweetID,
		Raw:    raw,
		Text:   text,
		Thread: thread,
	}, nil
}

func readThread(birdBin, tweetID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, birdBin, "thread", tweetID)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	thread := strings.TrimSpace(string(output))
	if thread == "" {
		return "", nil
	}

	return thread, nil
}

func categorizeTweet(tweet Tweet, categories CategoriesConfig) string {
	textLower := strings.ToLower(tweet.Raw + "\n" + tweet.Thread)

	for _, name := range categories.Order {
		category, ok := categories.Items[name]
		if !ok {
			continue
		}
		for _, kw := range category.Keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(textLower, strings.ToLower(kw)) {
				return name
			}
		}
	}

	return ""
}

type LLMAnalysis struct {
	Category        string `json:"category"`
	NeedsURLContent bool   `json:"needsUrlContent"`
}

func buildGeminiPrompt(tweet Tweet, categories CategoriesConfig) string {
	var builder strings.Builder
	builder.WriteString("Categorize this tweet into ONE category:\n\n")

	for _, name := range categories.Order {
		category, ok := categories.Items[name]
		if !ok {
			continue
		}
		description := strings.TrimSpace(category.Description)
		if description == "" {
			description = "No description provided"
		}
		builder.WriteString("- ")
		builder.WriteString(name)
		builder.WriteString(": ")
		builder.WriteString(description)
		builder.WriteString("\n")
	}

	urlCount := len(extractURLs(tweet.Raw + "\n" + tweet.Thread))
	builder.WriteString("\nTweet: ")
	builder.WriteString(emptyIf(tweet.Text, "(no tweet text)"))
	builder.WriteString("\nThread: ")
	builder.WriteString(emptyIf(tweet.Thread, "(no thread)"))
	builder.WriteString("\nURLs: ")
	builder.WriteString(fmt.Sprintf("%d", urlCount))
	builder.WriteString("\n\nReturn JSON: {\"category\": \"")
	builder.WriteString(strings.Join(categories.Order, "|"))
	builder.WriteString("\", \"needsUrlContent\": true/false}\n")

	return builder.String()
}

func analyzeWithLLM(tweet Tweet, categories CategoriesConfig) (LLMAnalysis, error) {
	if config.GeminiBin == "" {
		return LLMAnalysis{}, errors.New("gemini CLI not available")
	}

	prompt := buildGeminiPrompt(tweet, categories)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.GeminiBin, prompt)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		return LLMAnalysis{}, fmt.Errorf("gemini failed: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return LLMAnalysis{}, errors.New("empty gemini response")
	}

	jsonText := raw
	if extracted, ok := extractFirstJSON(raw); ok {
		jsonText = extracted
	}

	var analysis LLMAnalysis
	if err := json.Unmarshal([]byte(jsonText), &analysis); err != nil {
		return LLMAnalysis{}, fmt.Errorf("invalid gemini JSON: %w", err)
	}

	analysis.Category = normalizeCategory(analysis.Category, categories)
	return analysis, nil
}

func normalizeCategory(value string, categories CategoriesConfig) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	for name := range categories.Items {
		if strings.ToLower(name) == lower {
			return name
		}
	}

	normalized := normalizeCategoryKey(lower)
	for name := range categories.Items {
		if normalizeCategoryKey(strings.ToLower(name)) == normalized {
			return name
		}
	}

	return ""
}

func normalizeCategoryKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	return normalized
}

func fallbackCategory(categories CategoriesConfig) string {
	if other := findCategoryByNormalizedKey(categories, "other"); other != "" {
		return other
	}
	if len(categories.Order) > 0 {
		return categories.Order[0]
	}
	return ""
}

func findCategoryByNormalizedKey(categories CategoriesConfig, target string) string {
	normalized := normalizeCategoryKey(target)
	for name := range categories.Items {
		if normalizeCategoryKey(name) == normalized {
			return name
		}
	}
	return ""
}

func tweetHasEnoughContext(tweet Tweet) bool {
	textLen := len(strings.TrimSpace(tweet.Text))
	threadLen := len(strings.TrimSpace(tweet.Thread))
	return textLen >= 200 || threadLen >= 200
}

func extractURLs(text string) []string {
	regex := regexp.MustCompile(`https?://[^\s<>"{}|\\^\` + "`" + `\[\]]+`)
	matches := regex.FindAllString(text, -1)
	return uniqueStrings(matches)
}

func summarizeContent(url string) string {
	if config.SummarizeBin == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.SummarizeBin, url)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func saveToObsidian(tweet Tweet, category string, allowSummaries bool, folderPath string) error {
	folder := resolveObsidianPath(folderPath)
	if folder == "" {
		return errors.New("obsidian path is required")
	}

	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%s.md", time.Now().Format("2006-01-02"), tweet.ID)
	path := filepath.Join(folder, filename)

	urls := extractURLs(tweet.Raw + "\n" + tweet.Thread)
	var builder strings.Builder
	if allowSummaries {
		for _, url := range urls {
			summary := summarizeContent(url)
			if summary == "" {
				continue
			}
			builder.WriteString("### ")
			builder.WriteString(url)
			builder.WriteString("\n\n")
			builder.WriteString(summary)
			builder.WriteString("\n\n")
		}
	}

	linkedContent := builder.String()
	if linkedContent == "" {
		if allowSummaries {
			linkedContent = "(no links or summaries available)"
		} else {
			linkedContent = "(summaries skipped)"
		}
	}

	content := fmt.Sprintf(`# Twitter Bookmark

**ID:** %s
**Saved:** %s
**Category:** %s

## Content

%s

## Thread

%s

## Linked Content

%s

---
*Auto-saved via twitter-bookmarks*
`, tweet.ID, time.Now().Format(time.RFC3339), category, tweet.Raw, emptyIf(tweet.Thread, "(no thread)"), linkedContent)

	return os.WriteFile(path, []byte(content), 0o644)
}

func resolveObsidianPath(folderPath string) string {
	trimmed := strings.TrimSpace(folderPath)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(config.ObsidianBase, filepath.FromSlash(trimmed))
}

func sendTelegram(message string, quietStart, quietEnd int) {
	if isQuietHours(time.Now(), quietStart, quietEnd) {
		fmt.Println("Quiet hours active; skipping Telegram notification")
		return
	}

	fmt.Printf("TELEGRAM: %s\n", message)
}

func routeTweet(tweet Tweet, category string, route RoutingConfig, urls []string, quietStart, quietEnd int, allowSummaries bool) error {
	action := strings.ToLower(strings.TrimSpace(route.Action))
	if action == "" {
		action = "notify"
	}

	switch action {
	case "summarize":
		if route.Notify {
			sendTelegram(buildSummaryMessage(category, tweet, urls, allowSummaries), quietStart, quietEnd)
		}
		return nil
	case "razor_task":
		implementRazorTask(tweet, quietStart, quietEnd, allowSummaries, route.Path, route.Notify)
		return nil
	case "codex_prompt":
		if err := saveToObsidian(tweet, category, allowSummaries, route.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save to Obsidian: %v\n", err)
		}

		prompt := createCodexPrompt(tweet, urls, allowSummaries)
		if err := os.MkdirAll(config.PromptsDir, 0o755); err != nil {
			return err
		}
		promptPath := filepath.Join(config.PromptsDir, fmt.Sprintf("%s.txt", tweet.ID))
		if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
			return err
		}

		if route.Notify {
			message := fmt.Sprintf("Codex task ready. Prompt saved to %s. Preview: %s", promptPath, truncate(prompt, 400))
			sendTelegram(message, quietStart, quietEnd)
		}
		return nil
	case "save_obsidian":
		if err := saveToObsidian(tweet, category, allowSummaries, route.Path); err != nil {
			return err
		}
		if route.Notify {
			message := fmt.Sprintf("Saved bookmark (%s) to Obsidian. Tweet: %s", category, truncate(tweet.Text, 200))
			sendTelegram(message, quietStart, quietEnd)
		}
		return nil
	case "unbookmark":
		if err := unbookmarkTweet(config.BirdBin, tweet.ID); err != nil {
			return err
		}
		if route.Notify {
			message := fmt.Sprintf("Removed bookmark (%s): %s", category, truncate(tweet.Text, 200))
			sendTelegram(message, quietStart, quietEnd)
		}
		return nil
	case "notify":
		if route.Notify {
			sendTelegram(buildNotifyMessage(category, tweet, bookmarksConfig.Categories), quietStart, quietEnd)
		}
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown routing action %q for category %q; defaulting to notify\n", route.Action, category)
		if route.Notify {
			sendTelegram(buildNotifyMessage(category, tweet, bookmarksConfig.Categories), quietStart, quietEnd)
		}
		return nil
	}
}

func buildSummaryMessage(category string, tweet Tweet, urls []string, allowSummaries bool) string {
	var summaries []string
	if allowSummaries {
		for _, url := range urls {
			if len(summaries) >= 3 {
				break
			}
			summary := summarizeContent(url)
			if summary == "" {
				continue
			}
			summaries = append(summaries, fmt.Sprintf("%s\n%s", url, truncate(summary, 200)))
		}
	}

	prefix := "Read later"
	if normalizeCategoryKey(category) != normalizeCategoryKey("readlater") {
		prefix = fmt.Sprintf("Summary (%s)", category)
	}
	message := fmt.Sprintf("%s: %s", prefix, truncate(tweet.Text, 200))
	if len(summaries) > 0 {
		message = fmt.Sprintf("%s\n\nSummaries:\n%s", message, strings.Join(summaries, "\n\n"))
	}
	return message
}

func buildNotifyMessage(category string, tweet Tweet, categories CategoriesConfig) string {
	other := findCategoryByNormalizedKey(categories, "other")
	if other != "" && category == other {
		return fmt.Sprintf("Unclear bookmark. Where should this go? Tweet: %s", truncate(tweet.Text, 300))
	}

	return fmt.Sprintf("Bookmark categorized as %s. Tweet: %s", category, truncate(tweet.Text, 300))
}

func createCodexPrompt(tweet Tweet, urls []string, allowSummaries bool) string {
	var summaries strings.Builder
	if allowSummaries {
		for _, url := range urls {
			summary := summarizeContent(url)
			if summary == "" {
				continue
			}
			summaries.WriteString("Link: ")
			summaries.WriteString(url)
			summaries.WriteString("\n")
			summaries.WriteString(summary)
			summaries.WriteString("\n\n")
		}
	}

	linked := summaries.String()
	if linked == "" {
		if allowSummaries {
			linked = "No links available"
		} else if len(urls) > 0 {
			linked = "Summaries skipped"
		} else {
			linked = "No links available"
		}
	}

	return fmt.Sprintf(`# Implementation Task from Twitter Bookmark

## Context
%s

## Thread
%s

## Linked Resources
%s

## Your Task
Implement this based on the context above. Follow vibe coding principles:
1. Read and understand the full context
2. Implement the solution
3. Test it works
4. Provide usage examples

Start by asking any clarifying questions, then proceed with implementation.
`, tweet.Raw, emptyIf(tweet.Thread, "(no thread)"), linked)
}

func implementRazorTask(tweet Tweet, quietStart, quietEnd int, allowSummaries bool, folderPath string, notify bool) {
	if err := saveToObsidian(tweet, "razor", allowSummaries, folderPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save to Obsidian: %v\n", err)
	}

	if !notify {
		return
	}

	textLower := strings.ToLower(tweet.Raw + "\n" + tweet.Thread)
	if containsAny(textLower, []string{"skill", "cli", "tool", "workflow"}) {
		message := fmt.Sprintf("Razor task queued for implementation. Tweet: %s", truncate(tweet.Text, 200))
		sendTelegram(message, quietStart, quietEnd)
		return
	}

	message := fmt.Sprintf("Razor tip saved. Tweet: %s", truncate(tweet.Text, 200))
	sendTelegram(message, quietStart, quietEnd)
}

func processBookmark(tweetID string, quietStart, quietEnd int) (ProcessResult, error) {
	tweet, err := readTweet(config.BirdBin, tweetID)
	if err != nil {
		return ProcessResult{}, err
	}

	category := ""
	allowSummaries := false
	if analysis, err := analyzeWithLLM(tweet, bookmarksConfig.Categories); err == nil {
		category = normalizeCategory(analysis.Category, bookmarksConfig.Categories)
		allowSummaries = analysis.NeedsURLContent && !tweetHasEnoughContext(tweet)
	}
	if category == "" {
		category = categorizeTweet(tweet, bookmarksConfig.Categories)
	}
	if category == "" {
		category = fallbackCategory(bookmarksConfig.Categories)
	}
	if category == "" {
		return ProcessResult{}, errors.New("no categories configured")
	}

	urls := extractURLs(tweet.Raw + "\n" + tweet.Thread)

	route, ok := bookmarksConfig.Routing[category]
	if !ok {
		fmt.Fprintf(os.Stderr, "No routing configured for category %q; defaulting to notify\n", category)
		route = RoutingConfig{Action: "notify", Notify: true}
	}

	if err := routeTweet(tweet, category, route, urls, quietStart, quietEnd, allowSummaries); err != nil {
		return ProcessResult{}, err
	}

	return ProcessResult{Category: category, Processed: true}, nil
}

func parseClock(value string) (int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM")
	}
	var hour, minute int
	_, err := fmt.Sscanf(value, "%02d:%02d", &hour, &minute)
	if err != nil {
		return 0, err
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("invalid time")
	}
	return hour*60 + minute, nil
}

func isQuietHours(now time.Time, quietStart, quietEnd int) bool {
	if quietStart == quietEnd {
		return false
	}

	nowMinutes := now.Hour()*60 + now.Minute()
	if quietStart < quietEnd {
		return nowMinutes >= quietStart && nowMinutes < quietEnd
	}

	return nowMinutes >= quietStart || nowMinutes < quietEnd
}

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func extractFirstJSON(input string) (string, bool) {
	start := -1
	depth := 0
	inString := false
	escape := false

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
			continue
		}
		if ch == '}' && depth > 0 {
			depth--
			if depth == 0 && start >= 0 {
				return input[start : i+1], true
			}
		}
	}

	return "", false
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func emptyIf(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
