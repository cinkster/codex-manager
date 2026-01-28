# Codex Manager

Browse and render Codex session `.jsonl` files with a web UI, and optionally share rendered HTML via a dedicated share server (with optional Tailscale integration).

## Features
- Indexes `~/.codex/sessions/{year}/{month}/{day}` and lists sessions by date.
- Renders conversations as HTML with markdown support and dark theme.
- Shows only user/agent messages and reasoning; tool calls and other events are omitted.
- Consecutive messages are merged; for user groups, only the last message is kept.
- User messages can be trimmed to content after `## My request for Codex:` (default on).
- Share button saves a hard‑to‑guess HTML file to `~/.codex/shares` and copies its URL.
- Separate share server serves only exact filenames (no directory listing).
- Optional Tailscale `serve`/`funnel` integration for public share URLs.

## Usage
```bash
# Run the main UI (default: :8080) and share server (:8081)
go run ./cmd/codex-manager

# Show help
 go run ./cmd/codex-manager -h

# Disable trimming to the request marker
 go run ./cmd/codex-manager -full

# Enable Tailscale serve/funnel for share URLs
 go run ./cmd/codex-manager -ts
```

Visit:
- UI: http://localhost:8080/
- Share server: http://localhost:8081/

## Flags
- `--sessions-dir` (default `~/.codex/sessions`)
- `--addr` (default `:8080`)
- `--share-addr` (default `:8081`)
- `--share-dir` (default `~/.codex/shares`)
- `--rescan-interval` (default `2m`)
- `-ts` enable Tailscale serve/funnel
- `-full` disable trimming to `## My request for Codex:`
- `-h` / `--help`

## Tailscale notes
When `-ts` is enabled, Codex Manager:
- Runs `tailscale serve --bg --yes --http <share-port>`
- Runs `tailscale funnel --bg --yes <share-port>`
- Uses `tailscale status --json` to discover your Tailscale DNS name and builds share URLs like `https://<tailscale-host>/<uuid>.html`.

The binary is auto-detected at:
- `/Applications/Tailscale.app/Contents/MacOS/Tailscale` (macOS)
- `tailscale` on PATH

## Share behavior
Clicking “Share”:
- Renders the current session to a UUID-like filename: `~/.codex/shares/<uuid>.html`
- Copies the share URL to your clipboard
- Displays a banner showing the copied URL

## Development
```bash
go test ./...
```

## Directory layout
- `cmd/codex-manager/main.go` – server startup
- `internal/config` – CLI flags + config
- `internal/sessions` – scanning + parsing
- `internal/web` – HTTP handlers + share logic
- `internal/render` – templates + CSS

