# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JCP (韭菜盘) is an AI-powered stock analysis application with a **dual-mode architecture**: Wails desktop app and standalone HTTP web server, sharing the same Go backend services. Fork of `run-bigpig/jcp` with added web server mode, Docker support, and mobile-responsive frontend.

## Build & Run Commands

### Desktop (Wails)
```bash
wails dev                                    # Development with hot reload
wails build                                  # Build for current platform
wails build -platform windows/amd64          # Cross-compile
```

### Web Server
```bash
cd frontend && npm run build && cd ..
cp -r frontend/dist/* cmd/server/static/
cd cmd/server && go build -o ../../jcp-server .
JCP_PASSWORD=xxx PORT=8080 ./jcp-server
```

### Frontend Only
```bash
cd frontend
npm install
npm run dev       # Vite dev server
npm run build     # tsc + vite build
```

### Docker
```bash
docker build -t jcp:latest .
docker run -d -p 8080:8080 -v $(pwd)/data:/app/data -e JCP_PASSWORD=xxx jcp:latest
```

### Tests
```bash
go test ./...                                    # All tests
go test ./internal/memory/...                    # Single package
go test -run TestTokenizer ./internal/memory/    # Single test
```

Note: `internal/services` tests hit external APIs (Sina Finance, EastMoney) and may fail due to network/rate-limiting. `internal/memory` tests are reliable unit tests.

### Build Check
```bash
go build ./...    # Verify compilation without producing binaries
```

## Architecture

### Dual-Mode Frontend Detection

Frontend checks `window.go` existence to determine mode:
- **Wails mode**: Calls Go methods via generated bindings in `frontend/wailsjs/go/main/App`. Real-time data via `runtime.EventsEmit`/`EventsOn`.
- **Web mode**: Calls REST API at `/api/*`. Meeting responses via SSE. Market data via polling (stock 3s, orderbook 1s, news 30s).

Every frontend service file (e.g., `services/watchlistService.ts`) has dual implementations branching on `isWailsEnv()`.

### Backend Entry Points

| Mode | Entry | Role |
|------|-------|------|
| Desktop | `main.go` → `app.go` | Wails lifecycle, all Go↔JS bindings via `App` struct methods |
| Web | `cmd/server/main.go` | HTTP routes, SSE streaming, session-based auth |

Both initialize the **same service stack** in the same order: ConfigService → MarketService → ToolRegistry → MCPManager → MeetingService → MemoryManager → SessionService → StrategyService → AgentContainer.

### Core Service Layer (`internal/services/`)

- **ConfigService**: JSON file-based config + watchlist management (`data/config.json`, `data/watchlist.json`)
- **MarketService**: Stock data from Sina Finance API (`hq.sinajs.cn`), includes K-line, indices, order book
- **MarketDataPusher**: Goroutine-based real-time push to frontend (Wails events or polling cache)
- **MeetingService** (`internal/meeting/`): AI discussion orchestration with two modes:
  - **Smart mode** (no `@`): Moderator (小韭菜) analyzes intent → selects agents → multi-round discussion
  - **Direct mode** (`@agent`): Named agents respond directly
- **StrategyService**: Strategy + agent config persistence (`data/strategies.json`)
- **SessionService**: Per-stock chat history (`data/sessions/<code>.json`)

### AI Stack (`internal/adk/`)

- **ModelFactory**: Creates `model.LLM` instances based on provider (OpenAI Chat Completions, OpenAI Responses API, Gemini, VertexAI)
- **OpenAI model** (`adk/openai/`): Two implementations — `model.go` (Chat Completions) and `responses_model.go` (Responses API). Both support thinking/reasoning models, vendor tool call parsing (3 XML formats), and `NoSystemRole` fallback.
- **ToolRegistry** (`adk/tools/`): Registers tools (stock query, news, K-line, etc.) that agents can invoke during meetings
- **MCPManager** (`adk/mcp/`): MCP server connections (HTTP, SSE, Command transports)

### Memory System (`internal/memory/`)

Per-stock isolated memory with TF-IDF relevance retrieval. Uses GSE for Chinese text segmentation. Stores key facts, recent rounds, and LLM-generated summaries. Auto-compresses when round count exceeds threshold.

### Data Storage

All runtime data is JSON files under `data/`:
- `config.json` — app configuration
- `strategies.json` — strategy and agent configs
- `watchlist.json` — watched stock codes
- `sessions/<stockCode>.json` — per-stock chat history
- `memory/<stockCode>.json` — per-stock AI memories
- `logs/` — structured application logs

## Key Conventions

- **Go module**: `github.com/run-bigpig/jcp`, Go 1.24.4
- **No DI framework**: Services are plain structs with `New*Service()` constructors, dependencies passed explicitly
- **Thread safety**: Services use `sync.RWMutex` for concurrent access
- **Proxy support**: All HTTP clients use `proxy.GetManager().GetTransport()` for unified proxy handling
- **Comments**: Chinese for code comments, English for variable/function names
- **Config fields**: `SendParams` (local feature — controls whether to send temperature/top_p/max_tokens), `NoSystemRole` (upstream feature — auto-detected system role support)
- **OpenAI BaseURL normalization**: `normalizeOpenAIBaseURL()` auto-appends `/v1` if missing

## Web Server Auth

Controlled by `JCP_PASSWORD` env var. Token-based sessions stored in memory, 24-hour expiry. Token via `jcp_token` cookie or `Authorization: Bearer` header.

## CI/CD

- `.github/workflows/build.yml` — Wails desktop builds on tag push → GitHub Releases
- `.github/workflows/docker.yml` — Docker multi-arch (amd64/arm64) → GHCR on push to main/master or tags
