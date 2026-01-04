# Twitter Bookmarks Processor (Go)

Automates Twitter bookmarks processing every 20 minutes:

A) Read Later -> Summarize links and send via Telegram
B) Razor/Clawdis -> Save to Obsidian and notify
C) Codex/Vibe -> Create ready-to-paste Codex prompt in `~/.codex-prompts/`
D) Other -> Ask via Telegram where it should go

## Build

```bash
go build -o twitter-bookmarks
```

## Usage

```bash
./twitter-bookmarks process
./twitter-bookmarks status
./twitter-bookmarks process --force
```

## Configuration

Flags (also supported via env vars):

- `--state` (`TWITTER_BOOKMARKS_STATE`)
- `--obsidian` (`TWITTER_BOOKMARKS_OBSIDIAN`)
- `--prompts` (`TWITTER_BOOKMARKS_PROMPTS`)
- `--bird` (`BIRD_BIN`)
- `--summarize` (`SUMMARIZE_BIN`)
- `--quiet-start` (`TWITTER_BOOKMARKS_QUIET_START`)
- `--quiet-end` (`TWITTER_BOOKMARKS_QUIET_END`)
- `--limit` (`TWITTER_BOOKMARKS_LIMIT`)

## Cron (every 20 minutes)

```bash
# Run this once to add the cron job
python3 -c "
from clawdis_cron import add_job
add_job({
    'id': 'twitter-bookmarks',
    'name': 'Twitter Bookmarks Processor',
    'schedule': '*/20 * * * *',
    'command': 'cd /Users/mariano/Coding/razor/workspace/skills/twitter-bookmarks && ./twitter-bookmarks process',
    'enabled': True
})
"
```

## Files Created

- `~/.twitter-bookmarks-state.json` - Tracks processed bookmarks
- `~/.codex-prompts/{tweet-id}.txt` - Codex prompts (category C)

## Obsidian Structure

```
obs_vault_personal/
├── Razor-Clawdis/
│   └── Twitter-Bookmarks/
│       └── 2026-01-03-{tweet-id}.md
└── Codex-Vibe/
    └── Twitter-Bookmarks/
        └── 2026-01-03-{tweet-id}.md
```

## Notes

- Requires `bird` CLI in PATH and authenticated in your environment.
- `summarize` is optional; if missing, link summaries are skipped.
- Quiet hours (default 23:00-08:00) suppress Telegram notifications.

Inspired by Alex Hillman's JFDI system.
