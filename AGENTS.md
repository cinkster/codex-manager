# AGENTS.md

## Project at a glance
- `codex-manager` is a Go 1.21 web app for browsing Codex session `.jsonl` files.
- It runs two HTTP servers:
  - Main UI server (default `:8080`)
  - Share server (default `:8081`) that serves generated static HTML share files
- Primary source data is `~/.codex/sessions/{YYYY}/{MM}/{DD}/*.jsonl`.
- Shares are written to `~/.codex/shares` by default.

## Runtime flow
1. `cmd/codex-manager/main.go` parses flags from `internal/config`.
2. Builds `sessions.Index` from disk and refreshes it periodically (`--rescan-interval`, default `2m`).
3. Builds/refreshes `search.Index` from the sessions index.
4. Creates renderer (`internal/render`) and HTTP handlers (`internal/web`).
5. Optionally configures Tailscale share host when `-ts` is enabled.

## Key packages
- `internal/config`
  - CLI parsing and validation.
  - Expands `~` in `--sessions-dir` and `--share-dir`.
  - Supports `--theme` values `1..6`.
- `internal/sessions`
  - Filesystem index by date/name/cwd (`index.go`).
  - Session parser for multiple JSONL shapes (`parser.go`, `meta.go`).
  - CWD normalization (`(unknown)` sentinel).
- `internal/search`
  - Incremental-ish rebuild: reuses unchanged files by `(size, modTime)`.
  - Searches parsed content, case-insensitive, returns preview snippets + line numbers.
- `internal/web`
  - All main routes and view models (`server.go`).
  - Share-only static file server with strict filename checks (`share.go`).
  - Tailscale integration (`tailscale.go`).
- `internal/render`
  - Embedded Go templates (`templates/*.html`) and shared CSS in `style.html`.

## HTTP routes (main UI server)
- `GET /` index page (`view=date|dir`, default is directory heatmap mode)
- `GET /dir?cwd=...` directory-specific date listing
- `GET /search?query=...&limit=...` JSON search endpoint
- `GET /raw/{yyyy}/{mm}/{dd}/{file}` download raw session JSONL
- `GET /{yyyy}/{mm}/{dd}/` day page
- `GET /{yyyy}/{mm}/{dd}/{file}` session page
- `POST /share/{yyyy}/{mm}/{dd}/{file}` render and persist share HTML, return JSON `{url}`

## Parsing/rendering behavior to preserve
- The UI only shows user/assistant message content and reasoning summaries.
- Tool calls/tool outputs are intentionally omitted from rendered items.
- Consecutive items with same `(type, subtype, role)` are merged, except:
  - User message groups keep only the last message in each consecutive run.
- User content is trimmed to text after `## My request for Codex:` by default.
  - `-full` disables this trimming.
- Auto-injected context user messages are detected and preserved as collapsible “Auto context” blocks.
- Markdown is rendered with Goldmark + GFM.

## UI/template notes
- Templates are embedded at build time (`go:embed templates/*.html`).
- `session.html` includes:
  - Copy Markdown per message + full thread
  - Copy deep-link to line anchors
  - Share button (POST to `/share/...`)
  - Jump-to-previous/next user message controls
- `index.html` includes in-page JS for search and directory heat filter behavior.
- Theme handling is class-based (`theme-noir-blue`, etc.) set from `--theme`.

## Dev commands
- Build: `make build` or `go build -o bin/codex-manager ./cmd/codex-manager`
- Run: `go run ./cmd/codex-manager`
- Test: `go test ./...` or `make test`
- Useful flags:
  - `--open-browser`
  - `--sessions-dir`
  - `--share-dir`
  - `--rescan-interval`
  - `--theme`
  - `-ts` (Tailscale), `-full` (disable user prompt trimming)

## Test coverage hotspots
- `internal/sessions/parser_test.go`
  - Metadata extraction, request trimming, direct-format parsing, user-message merge behavior
- `internal/sessions/index_test.go`
  - Date indexing and lookup
- `internal/search/index_test.go`
  - Search correctness across file updates

When changing parsing/indexing/search semantics, update or extend these tests first.

## Operational/security details
- Share filenames are random-token UUID-like strings ending with `.html`.
- Share server serves only exact filenames (no directory traversal, no listing).
- Raw and session routes validate date path segments and reject unsafe filenames.

## Current repo state assumptions
- `scripts/` and `service/` directories are present but currently contain no files.
- `bin/codex-manager` may exist as a locally built artifact.

## Change guidelines for future agents
- Keep parser behavior changes deliberate; small tweaks can alter search, rendering, and CWD grouping simultaneously.
- If adding a flag:
  1. Update `internal/config/config.go`
  2. Wire behavior in `cmd/codex-manager/main.go` or handlers
  3. Update `README.md`
- If adding a page route, update `internal/web/server.go` and corresponding templates.
- After meaningful code changes, run `go test ./...` before handoff.
