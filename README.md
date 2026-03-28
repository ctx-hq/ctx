# ctx

[![Go Version](https://img.shields.io/github/go-mod/go-version/ctx-hq/ctx)](https://go.dev/)
[![CI](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml/badge.svg)](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ctx-hq/ctx)](https://github.com/ctx-hq/ctx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[中文](README.zh-CN.md) | English

The universal package manager for AI coding agents.

```
ctx install @openelf/code-review   # install a skill
ctx link claude                     # link to your agent — done
```

## Why ctx?

AI coding agents (Claude Code, Cursor, Windsurf, etc.) lack a shared way to discover and install extensions. Each agent has its own format, its own config directory, its own linking mechanism.

**ctx solves this.** One command to install. One command to link. Works across 18 agents.

```
                 ┌─────────────────────────────┐
                 │        ctx registry          │
                 │  skills · MCP · CLI tools    │
                 └──────────┬──────────────────┘
                            │
                      ctx install
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐      ┌──────────┐      ┌──────────┐
   │  Claude   │      │  Cursor  │      │ Windsurf │  ...
   │  Code     │      │          │      │          │
   └──────────┘      └──────────┘      └──────────┘
```

## Install

```bash
# macOS / Linux
curl -fsSL https://getctx.org/install.sh | sh

# Homebrew
brew install ctx-hq/tap/ctx

# Go
go install github.com/ctx-hq/ctx/cmd/ctx@latest

# Windows
irm https://getctx.org/install.ps1 | iex
```

<details>
<summary>More options (Scoop, deb, rpm)</summary>

```bash
# Debian / Ubuntu
dpkg -i ctx_*.deb

# Windows (Scoop)
scoop bucket add ctx https://github.com/ctx-hq/homebrew-tap
scoop install ctx
```

</details>

## Quick Start

```bash
# Search for packages
ctx search "code review"

# Install a skill — auto-detects your agents and links it
ctx install @openelf/code-review

# Install an MCP server with a specific version
ctx install @mcp/github@2.1.0

# Install a CLI tool (delegates to brew/cargo/binary as appropriate)
ctx install @community/ripgrep

# See what's installed
ctx list

# Check environment health
ctx doctor
```

## Package Types

| Type | What it is | Example |
|------|------------|---------|
| **skill** | Prompt files and workflows for agents | Code review, refactoring, test generation |
| **mcp** | Model Context Protocol servers | GitHub, database, filesystem access |
| **cli** | Command-line tools | ripgrep, jq, fzf |

## Supported Agents

ctx detects and links packages to **18 agents**:

`claude` · `cursor` · `windsurf` · `opencode` · `codex` · `copilot` · `cline` · `continue` · `zed` · `roo` · `goose` · `amp` · `trae` · `kilo` · `pear` · `junie` · `aider` · `generic`

```bash
ctx link                  # list detected agents on your system
ctx link claude           # link all packages to Claude Code
ctx link cursor           # link all packages to Cursor
```

## Commands

```bash
# Discovery
ctx search <query>                  # search the registry
ctx search "git" --type mcp        # filter by type
ctx info <package>                  # show package details

# Install / Remove
ctx install <package[@version]>     # install a package
ctx install github:user/repo       # install from GitHub directly
ctx remove <package>                # uninstall
ctx list                            # list installed packages

# Update
ctx update                          # update all packages
ctx update @scope/name              # update one package
ctx outdated                        # check for updates
ctx prune                           # remove old versions

# Agent Linking
ctx link                            # list detected agents
ctx link <agent>                    # link packages to an agent

# Publishing
ctx login                           # authenticate via GitHub
ctx init --type skill               # scaffold a ctx.yaml
ctx validate                        # validate your manifest
ctx publish                         # publish to the registry

# Configuration
ctx config list                     # show all settings
ctx config set <key> <value>        # change a setting
ctx config get <key>                # read a setting

# System
ctx version                         # print version
ctx doctor                          # diagnose environment
ctx upgrade                         # self-update ctx
ctx serve                           # run as MCP server
```

## MCP Server Mode

ctx can run as an MCP server, letting AI agents manage packages directly:

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

Available tools: `ctx_search`, `ctx_install`, `ctx_info`, `ctx_list`

## Package Manifest

Packages are defined by a `ctx.yaml` file. Run `ctx init` to scaffold one.

<details>
<summary>Skill example</summary>

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

</details>

<details>
<summary>MCP server example</summary>

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

</details>

<details>
<summary>CLI tool example</summary>

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

</details>

## Configuration

### Files

| Path | Purpose |
|------|---------|
| `~/.ctx/config.yaml` | Registry URL, username, privacy settings |
| `~/.ctx/packages/` | Installed packages |
| `~/.ctx/links.json` | Agent link registry |
| System keychain | Auth token (macOS Keychain / Linux secret-tool) |

### Settings

```bash
ctx config set update_check false      # disable update checks
ctx config set network_mode offline    # disable all network access
ctx config set registry https://...    # use a custom registry
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CTX_HOME` | Override config directory |
| `CTX_DATA_HOME` | Override data directory |
| `CTX_CACHE_HOME` | Override cache directory |
| `CTX_REGISTRY` | Override registry URL |

### Flags

| Flag | Description |
|------|-------------|
| `--offline` | Disable all network access for this command |
| `--json` | JSON output |
| `--quiet` / `-q` | Minimal output |
| `--yes` / `-y` | Skip confirmation prompts |

## Privacy

- **No telemetry.** ctx does not collect analytics or usage data.
- **Tokens in system keychain.** Auth tokens are stored in macOS Keychain or Linux `secret-tool`, not in config files.
- **Update checks are optional.** Disable with `ctx config set update_check false` or `--offline`.
- **All files 0600/0700.** Sensitive files are owner-readable only.
- **No data leaves your machine** unless you explicitly search, install, publish, or login.

## Architecture

```
cmd/ctx/              CLI commands (Cobra)
internal/
  ├── config/         Configuration + file permissions
  ├── auth/           GitHub OAuth + system keychain
  ├── registry/       HTTP client for getctx.org API
  ├── resolver/       Version + source resolution
  ├── installer/      Download, extract, link packages
  ├── adapter/        Bridge to native pkg managers (brew, npm, pip, cargo)
  ├── agent/          Agent detection + config linking (18 agents)
  ├── mcpserver/      MCP server mode (ctx serve)
  └── output/         Human + JSON output formatting
```

## Contributing

```bash
git clone https://github.com/ctx-hq/ctx.git
cd ctx
make build      # build binary
make test       # run tests
make lint       # run linter
make check      # vet + lint + test
```

### Release Process

Releases are automated via [release-please](https://github.com/googleapis/release-please). Push conventional commits to `main` and a release PR is created automatically.

The pipeline (GoReleaser + GitHub Actions) handles cross-compilation (Linux/macOS/Windows x AMD64/ARM64), shell completions, cosign signing, SBOM, Homebrew/Scoop/deb/rpm packages, and build provenance attestation.

## License

[MIT](LICENSE)
