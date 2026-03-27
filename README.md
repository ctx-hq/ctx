# ctx

[![Go Version](https://img.shields.io/github/go-mod/go-version/ctx-hq/ctx)](https://go.dev/)
[![CI](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml/badge.svg)](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ctx-hq/ctx)](https://github.com/ctx-hq/ctx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[中文](README.zh-CN.md) | English

The universal package manager for LLM agent skills, MCP servers, and CLI tools.

**ctx** makes it easy to discover, install, and manage packages that extend AI coding agents like Claude Code, Cursor, and Windsurf.

## Install

```bash
curl -fsSL https://getctx.org/install.sh | sh
```

Or build from source:

```bash
go install github.com/getctx/ctx/cmd/ctx@latest
```

## Quick Start

```bash
# Search for packages
ctx search "code review"

# Install a skill
ctx install @hong/my-skill

# Install an MCP server
ctx install @mcp/github@2.1.0

# Install a CLI tool
ctx install @community/ripgrep

# Link all packages to your agent
ctx link claude
```

## Package Types

ctx manages three types of packages:

| Type | Description | Example |
|------|-------------|---------|
| **skill** | Agent skills and commands | Code review prompts, refactoring workflows |
| **mcp** | MCP (Model Context Protocol) servers | GitHub, database, file system servers |
| **cli** | Command-line tools | ripgrep, jq, fzf |

## Commands

### Discovery & Installation

```bash
ctx search <query>                  # Search the registry
ctx search "git" --type mcp         # Filter by type
ctx install <package[@version]>     # Install a package
ctx install github:user/repo        # Install from GitHub directly
ctx info <package>                  # Show package details
ctx list                            # List installed packages
ctx remove <package>                # Uninstall a package
```

### Updates

```bash
ctx update                          # Update all packages
ctx update @hong/my-skill           # Update a specific package
ctx outdated                        # Check for available updates
```

### Agent Linking

```bash
ctx link                            # List detected agents
ctx link claude                     # Link packages to Claude Code
ctx link cursor                     # Link packages to Cursor
```

Supported agents: `claude`, `cursor`, `windsurf`, `generic`

### Publishing

```bash
ctx login                           # Authenticate via GitHub
ctx init --type skill               # Scaffold a new ctx.yaml
ctx validate                        # Validate your manifest
ctx publish                         # Publish to the registry
```

### Organization

```bash
ctx org create <name>               # Create an organization
ctx org info <name>                 # Show organization details
ctx org add <org> <user>            # Add a member
ctx org remove <org> <user>         # Remove a member
```

### Diagnostics

```bash
ctx version                         # Print version info
ctx doctor                          # Diagnose environment & connectivity
```

## MCP Server Mode

ctx can run as an MCP server, letting AI agents manage packages directly:

```bash
ctx serve
```

Add to your agent's MCP configuration:

```json
{
  "mcpServers": {
    "ctx": {
      "command": "ctx",
      "args": ["serve"]
    }
  }
}
```

Exposed tools: `ctx_search`, `ctx_install`, `ctx_info`, `ctx_list`

## Package Manifest

Packages are defined by a `ctx.yaml` file:

### Skill

```yaml
name: "@scope/my-skill"
version: "1.0.0"
type: skill
description: "Code review skill for AI agents"

skill:
  entry: SKILL.md
  tags: [review, code-quality]
  compatibility: "claude-code, cursor"
```

### MCP Server

```yaml
name: "@scope/github-mcp"
version: "2.1.0"
type: mcp
description: "GitHub MCP server"

mcp:
  transport: stdio
  command: npx
  args: ["-y", "@modelcontextprotocol/server-github"]
  env:
    - name: GITHUB_TOKEN
      required: true
      description: "GitHub personal access token"
```

### CLI Tool

```yaml
name: "@community/ripgrep"
version: "14.1.0"
type: cli
description: "Fast regex search tool"

cli:
  binary: rg
  verify: "rg --version"

install:
  brew: ripgrep
  cargo: ripgrep
  platforms:
    darwin-arm64:
      binary: "https://github.com/.../ripgrep-14.1.0-aarch64-apple-darwin.tar.gz"
    linux-amd64:
      binary: "https://github.com/.../ripgrep-14.1.0-x86_64-unknown-linux-musl.tar.gz"
```

## Configuration

| Path | Purpose |
|------|---------|
| `~/.ctx/config.yaml` | Registry URL, auth token |
| `~/.ctx/packages/` | Installed packages |
| `ctx.lock` | Lockfile (project-level) |

Environment variables:

| Variable | Description |
|----------|-------------|
| `CTX_HOME` | Override config directory |
| `CTX_DATA_HOME` | Override data directory |
| `CTX_CACHE_HOME` | Override cache directory |
| `CTX_REGISTRY` | Override registry URL |

## Development

```bash
make build          # Build binary
make test           # Run tests
make lint           # Run linter
make build-all      # Cross-compile for all platforms
```

## License

[MIT](LICENSE)
