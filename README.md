# AI Explains

An AI-powered explanation tool. Submit a topic and Claude generates a structured, multi-section HTML explanation stored locally. Each section can be further explored through follow-up questions, extended with new sections, reordered, or deleted. All content is versioned so you can navigate between explanations over time.

## Features

- Generate multi-section explanations on any topic
- Ask follow-up questions per section (creates a new version with a richer answer)
- Add new sections after any existing section
- Reorder sections with up/down arrows
- Soft-delete sections (moved to a collapsed area, restorable)
- Version navigation within each section (← →)
- Editable titles (click the title to edit)
- All state persisted to disk — reload and pick up where you left off

## Requirements

- Go 1.22+
- Node.js 20+
- The `claude` CLI installed and authenticated (default), **or** an `ANTHROPIC_API_KEY` for SDK mode

## Build & Run

```sh
# Build frontend + backend, then start the server (uses local claude CLI)
make run

# Use the Anthropic SDK instead of the local CLI
ANTHROPIC_API_KEY=sk-ant-... make run LOCALEXEC=

# Start server only (after a previous build)
./aiexplains serve --localexec

# SDK mode
ANTHROPIC_API_KEY=sk-ant-... ./aiexplains serve
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

## Development

Run the backend and frontend dev servers in separate terminals for hot reload:

```sh
# Terminal 1 — Go backend with live reload (uses local claude CLI)
make dev-backend

# Terminal 2 — Vite dev server (proxies /api to :3000)
make dev-frontend
```

The Vite dev server runs on [http://localhost:5173](http://localhost:5173) and proxies API requests to the Go backend on port 3000.

## Data

All data is stored in `~/.aiexplains/`:

| Path | Contents |
|------|----------|
| `~/.aiexplains/database.sqlite` | Explanation metadata and conversation threads |
| `~/.aiexplains/explanations/{id}.html` | HTML files — the source of truth for section content and version history |

## Docker

A Docker Compose setup is included for running with SDK mode:

```sh
ANTHROPIC_API_KEY=sk-ant-... docker compose up
```

The container binds to port 3000 and mounts `~/.aiexplains` as a named volume so data persists across restarts.

## CLI Options

```
./aiexplains serve [flags]

Flags:
  --port int           Port to listen on (default 3000)
  --host string        Host to bind to (default "127.0.0.1")
  --frontend-dir string  Path to frontend dist directory
  --localexec          Use local claude CLI instead of Anthropic SDK
```
