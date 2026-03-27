# ctx CLI

Go CLI for the getctx.org package manager. Manages skills, MCP servers, and CLI tools for LLM agents.

## Build & Test

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linter
```

## Architecture

- `cmd/ctx/` — Cobra command definitions
- `internal/manifest/` — ctx.yaml parsing and validation
- `internal/registry/` — HTTP client for getctx.org API
- `internal/resolver/` — Version and source resolution
- `internal/installer/` — Download, place, and link packages
- `internal/adapter/` — Bridge adapters (brew, npm, pip, cargo, binary)
- `internal/agent/` — Agent detection and config linking (Claude, Cursor, etc.)
- `internal/mcpserver/` — MCP server mode (`ctx serve`)
- `internal/config/` — CLI configuration (~/.ctx/)
- `internal/output/` — Human and JSON output formatting
- `internal/auth/` — GitHub OAuth device flow
