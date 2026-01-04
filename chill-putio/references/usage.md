# Chill → put.io tooling reference

## CLI
- Script: `/Users/mariano/Coding/infra/chillinstitute/chillput.py`
- Symlink: `~/.local/bin/chillput`

## Env
- `/Users/mariano/Coding/infra/chillinstitute/.env`
  - `PUTIO_OAUTH_TOKEN`
  - `PUTIO_CLIENT_ID`
  - `PUTIO_CLIENT_SECRET`
  - `CHILL_COOKIE_DOMAIN` (default `chill.institute`)
  - `CHILL_COOKIE_PROFILE` (default `Default`)

## Cookie extraction
- CLI: `sweetcookie` (SweetCookieKit)
- Typical Chrome profile id: `Default` / `Profile 1`
- Example: `sweetcookie --browser chrome --profile "Default" --domain "chill.institute" --format cookie-header`

## Troubleshooting
- **Keychain denied Chrome Safe Storage**: run `sweetcookie ...` once in a GUI session on mb-server and approve the keychain prompt.
- **Profile mismatch**: run `sweetcookie --list-stores --browser chrome` and update `CHILL_COOKIE_PROFILE`.
- **No links found**: check that the URL contains magnet/torrent links (view HTML, or test with `--dry-run`).
