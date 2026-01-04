# Twitter Bookmarks Processor

Automated Twitter bookmark processor with YAML-configurable categories, LLM analysis, and smart routing.

## How It Works

**The Classifier:**
1. Fetches new bookmarks via `bird` CLI
2. Reads full tweet + thread context
3. Sends tweet to Gemini with your category definitions
4. Routes based on category → action mapping in YAML config
5. Tracks state to avoid reprocessing

**Key Feature:** Instead of hardcoding categories, you define them in `~/.twitter-bookmarks-config.yaml` with keywords and routing actions.

## Example YAML Config

```yaml
categories:
  # Define your categories with keywords for LLM guidance
  razor:
    description: "AI agents, automation, CLI tools, Clawdis ecosystem"
    keywords: [agent, automation, clawdis, cli, workflow, cursor, codex]
    
  pottery:
    description: "Ceramics, pottery business, kiln, glazes"
    keywords: [ceramic, pottery, kiln, glaze, clay, wheel]
    
  readLater:
    description: "Articles, videos, long-form content to read/watch later"
    keywords: [article, blog, youtube, podcast, tutorial]
    
  politics:
    description: "Political content, news, debate"
    keywords: [election, policy, government, senate]
    
  recipes:
    description: "Cooking recipes, food techniques"
    keywords: [recipe, cook, bake, ingredient]
    
  discard:
    description: "Spam, noise, irrelevant content"
    keywords: [polymarket, crypto trading, nft, spam]

routing:
  # Define what happens to each category
  
  razor:
    action: save_obsidian  # Save to Obsidian vault
    path: "AI-Research/Twitter-Bookmarks"  # Relative to vault root
    notify: true  # Send Telegram notification
    
  pottery:
    action: save_obsidian
    path: "Business/Pottery-Ideas"
    notify: false
    
  readLater:
    action: summarize  # Summarize URLs and send via Telegram
    notify: true
    
  politics:
    action: notify  # Just send notification, don't save
    
  recipes:
    action: save_file  # Save to plain text file
    path: "/Users/you/recipes/twitter-recipes.md"
    notify: false
    
  discard:
    action: unbookmark  # Remove bookmark automatically
```

## Actions Reference

**Available routing actions:**

- **`save_obsidian`** - Save to Obsidian vault at specified path
- **`summarize`** - Extract & summarize URLs, send via Telegram
- **`notify`** - Send Telegram notification only (no saving)
- **`save_file`** - Append to specified file path
- **`unbookmark`** - Remove bookmark after processing
- **`codex`** - Create ready-to-paste Codex prompt in `~/.codex-prompts/`
- **`razor`** - Auto-implement if possible, or save to Obsidian

## Creating Custom Flows

### Example 1: Political content → Telegram only
```yaml
categories:
  politics:
    description: "Political news and commentary"
    keywords: [politics, election, senate, congress]

routing:
  politics:
    action: notify  # Just send to Telegram
```

### Example 2: Long recipes → Append to recipe file
```yaml
categories:
  recipes:
    description: "Cooking recipes and techniques"
    keywords: [recipe, cook, bake, ingredient, cuisine]

routing:
  recipes:
    action: save_file
    path: "/Users/you/Documents/recipes.md"
    notify: false  # Silent processing
```

### Example 3: Auto-discard crypto spam
```yaml
categories:
  spam:
    description: "Unwanted crypto/trading content"
    keywords: [polymarket, nft, airdrop, presale, moonshot]

routing:
  spam:
    action: unbookmark  # Remove immediately
```

### Example 4: Work research → Save + Notify
```yaml
categories:
  work:
    description: "Work-related research and tools"
    keywords: [enterprise, b2b, saas, productivity]

routing:
  work:
    action: save_obsidian
    path: "Work/Research"
    notify: true  # Alert me when saved
```

## Build

```bash
go build -o twitter-bookmarks
```

## Usage

```bash
# Process new bookmarks
./twitter-bookmarks process

# Check status and category breakdown
./twitter-bookmarks status

# Use custom config file
./twitter-bookmarks process --config ~/my-config.yaml

# Force reprocess all (dangerous - will reprocess everything!)
./twitter-bookmarks process --force
```

## Configuration

### Environment Variables

- `TWITTER_BOOKMARKS_CONFIG` - Path to YAML config (default: `~/.twitter-bookmarks-config.yaml`)
- `TWITTER_BOOKMARKS_STATE` - State file path (default: `~/.twitter-bookmarks-state.json`)
- `TWITTER_BOOKMARKS_OBSIDIAN` - Obsidian vault path (required if using `save_obsidian`)
- `TWITTER_BOOKMARKS_PROMPTS` - Codex prompts directory (default: `~/.codex-prompts/`)
- `BIRD_BIN` - Path to bird CLI (default: `bird` in PATH)
- `SUMMARIZE_BIN` - Path to summarize CLI (optional)
- `TWITTER_BOOKMARKS_QUIET_START` - Quiet hours start (default: `23:00`)
- `TWITTER_BOOKMARKS_QUIET_END` - Quiet hours end (default: `08:00`)
- `TWITTER_BOOKMARKS_LIMIT` - Max bookmarks per run (default: 50)

### Command-line Flags

```bash
./twitter-bookmarks process --help
```

All env vars can be overridden via flags (e.g., `--config`, `--state`, `--obsidian`, etc.)

## Automated Processing (Cron)

Run every 20 minutes:

```bash
*/20 * * * * cd /path/to/twitter-bookmarks && ./twitter-bookmarks process
```

Or use Clawdis cron if available:

```bash
python3 -c "
from clawdis_cron import add_job
add_job({
    'id': 'twitter-bookmarks',
    'name': 'Twitter Bookmarks Processor',
    'schedule': '*/20 * * * *',
    'command': 'cd /path/to/twitter-bookmarks && ./twitter-bookmarks process',
    'enabled': True
})
"
```

## Requirements

- **bird CLI** - Twitter/X CLI tool (https://github.com/steipete/bird)
  - Must be authenticated and in PATH
- **summarize CLI** (optional) - For URL content extraction
- **Obsidian vault** (optional) - Only if using `save_obsidian` action
- **Gemini API key** - Set in environment or bird config
- **Telegram** (optional) - For notifications

## Default Config

If `~/.twitter-bookmarks-config.yaml` doesn't exist, the tool creates a default with these categories:
- `razor` - AI agents, automation, CLI tools
- `codex` - Vibe coding, AI-assisted development
- `readLater` - Articles, videos, long-form content
- `other` - Catch-all for uncategorized

## Files Created

- `~/.twitter-bookmarks-state.json` - Tracks processed bookmarks
- `~/.twitter-bookmarks-config.yaml` - Category definitions & routing (auto-created)
- `~/.codex-prompts/{tweet-id}.txt` - Codex prompts (if using `codex` action)
- Obsidian notes at configured paths (if using `save_obsidian`)

## Quiet Hours

During quiet hours (default 23:00-08:00), Telegram notifications are suppressed unless the content is urgent (configurable per category).

---

*Inspired by Alex Hillman's JFDI system*
*Built with Codex + Razor 🥷*
