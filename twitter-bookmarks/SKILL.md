---
name: twitter-bookmarks
description: Automated Twitter bookmark processor - checks every 20 minutes, categorizes, and routes bookmarks
homepage: https://x.com/alexhillman/status/2006420618091094104
metadata: {"clawdis":{"emoji":"🔖","requires":{"bins":["bird"]}}}
---

# Twitter Bookmarks Processor

Automated workflow inspired by Alex Hillman's system. Checks Twitter bookmarks every 20 minutes, processes new ones, and routes them based on content.

## How It Works

1. **Every 20 minutes**: Check for new Twitter bookmarks via `bird bookmarks`
2. **Process each new bookmark**: 
   - Read full tweet + thread context
   - Extract links, videos, podcasts, transcripts
   - Categorize based on content
3. **Route to destinations**:
   - **A) Read Later** → Summarize links & send to Mariano via Telegram
   - **B) Razor/Clawdis Relevant** → **Auto-implement** if possible, or save to Obsidian + notify
   - **C) Codex/Vibe Coding** → **Create ready-to-paste Codex prompt**, save to `~/.codex-prompts/`
   - **D) Other/Uncertain** → Ask Mariano via Telegram

## State Tracking

File: `~/.twitter-bookmarks-state.json`

```json
{
  "lastProcessed": "2026-01-03T02:35:00Z",
  "processedIds": ["1234567890", "..."],
  "categories": {
    "readLater": 0,
    "razor": 0,
    "codex": 0,
    "other": 0
  }
}
```

## Usage

### Manual Run
```bash
# Process new bookmarks now
python -m twitter_bookmarks process

# Check status
python -m twitter_bookmarks status

# Force reprocess all (dangerous - will spam!)
python -m twitter_bookmarks process --force
```

### Automated (via cron)
Set up via clawdis_cron - runs every 20 minutes

## Categories & Routing

### A) Read Later
- News articles, blog posts, long-form content
- Podcasts, YouTube videos
- Action: Summarize key points + send to Telegram
- Storage: Optional - keep summary in `read-later/` folder

### B) Razor/Clawdis Relevant
- AI agent development, automation
- CLI tools, terminal workflows
- Pi/Clawdis tips, skills, improvements
- **Action**: 
  - Auto-implement if it's a new skill/tool/workflow
  - Save full content + summaries to `obs_vault_personal/Razor-Clawdis/`
  - Notify via Telegram with status

### C) Codex/Vibe Coding
- Vibe coding methodology, examples
- AI-assisted development patterns
- Codex CLI usage, tips
- **Action**: 
  - Create ready-to-paste Codex prompt with full context
  - Save prompt to `~/.codex-prompts/{tweet-id}.txt`
  - Save reference to `obs_vault_personal/Codex-Vibe/`
  - Send prompt preview via Telegram

### D) Other
- Doesn't fit A/B/C clearly
- Action: Send to Telegram with question: "Where should this go?"

## Processing Logic

1. **Fetch new bookmarks** (since last run)
2. **For each bookmark:**
   - Get full tweet content via `bird read <id>`
   - If tweet is part of thread: `bird thread <id>`
   - Extract all URLs from tweet
   - For each URL:
     - If YouTube/podcast: extract title, description
     - If article: use `summarize` skill to get content
   - **Categorize** using LLM prompt:
     ```
     Categorize this tweet/content:
     - readLater: general interest, articles, videos, podcasts
     - razor: AI agents, automation, Clawdis, Pi, CLI tools
     - codex: vibe coding, AI-assisted dev, Codex CLI
     - other: unclear/uncertain
     
     Content: {tweet_text}
     Links: {urls}
     Thread: {thread_context}
     ```
3. **Route** based on category
4. **Update state** file

## Implementation Notes

- Use `bird bookmarks -n 50` to get recent bookmarks
- Track processed IDs to avoid duplicates
- Store state file to persist across runs
- Use `summarize` skill for link content extraction
- Telegram sends should be concise (max 2-3 paragraphs per bookmark)

## Obsidian Storage Structure

```
obs_vault_personal/
├── Razor-Clawdis/
│   ├── Twitter-Bookmarks/
│   │   ├── 2026-01-03-agent-workflows.md
│   │   └── 2026-01-02-cli-tools.md
└── Codex-Vibe/
    ├── Twitter-Bookmarks/
    │   ├── 2026-01-03-vibe-coding-examples.md
    │   └── 2026-01-01-codex-tips.md
```

Each saved bookmark becomes a markdown note with:
- Original tweet text
- Author info
- Links + summaries
- Date bookmarked
- Tags

## Example Flow

**Bookmark:** Tweet about new Claude Code feature
1. Bird fetches bookmark
2. Read full tweet + thread
3. Categorize → "razor" (Clawdis relevant)
4. Save to `obs_vault_personal/Razor-Clawdis/Twitter-Bookmarks/2026-01-03-claude-code-update.md`
5. Mark as processed

**Bookmark:** Long article about productivity
1. Fetch bookmark
2. Summarize article via `summarize` skill
3. Categorize → "readLater"
4. Send summary to Telegram: "📖 Read Later: [Article Title] - Key points: ..."
5. Mark as processed

---

*Inspired by Alex Hillman's JFDI system*
