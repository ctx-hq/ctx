---
name: ctx
description: |
  Universal package manager for AI agent skills, MCP servers, and CLI tools.
  Search, install, update, and manage packages across all detected agents.
triggers:
  - ctx
  - /ctx
  - ctx install
  - ctx search
  - install skill
  - install mcp server
  - install cli tool
  - find a tool
  - find a skill
  - manage agent tools
  - what skills are available
  - search packages
  - update packages
  - remove package
invocable: true
argument-hint: "[command] [args...]"
---

# /ctx - AI Agent Package Manager

Universal package manager for skills, MCP servers, and CLI tools. One command to install and link to all detected AI agents.

## Bootstrap

If `ctx` is not installed, install it first:

**macOS / Linux:**
```sh
curl -fsSL https://getctx.org/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://getctx.org/install.ps1 | iex
```

Choose the command matching the current platform. Both are zero-interaction — no sudo prompts, no manual PATH setup.

## Agent Invariants

1. Use `--agent` flag for machine-parseable output (quiet JSON data, JSON errors)
2. Use `--type` to filter by package type: `skill`, `mcp`, `cli`
3. Installation auto-detects and links to all agents (Claude, Cursor, Windsurf, OpenCode, Codex)
4. Use `ctx dr` to diagnose environment issues
5. All operations are reversible — `ctx rm` cleanly removes all traces

## Quick Reference

| Command | Alias | Description |
|---------|-------|-------------|
| `ctx search <query>` | `ctx s` | Search registry for packages |
| `ctx install <pkg>` | `ctx i` | Install a package (auto-links to agents) |
| `ctx remove <pkg>` | `ctx rm` | Uninstall and clean all links |
| `ctx list` | `ctx ls` | List installed packages |
| `ctx update` | `ctx up` | Update all packages |
| `ctx outdated` | `ctx od` | Check for available updates |
| `ctx info <pkg>` | | View package details |
| `ctx use <pkg>@<ver>` | | Switch to a local version (instant) |
| `ctx prune` | | Remove old versions, free disk |
| `ctx doctor` | `ctx dr` | Diagnose environment and connectivity |
| `ctx link [agent]` | `ctx ln` | Link packages to a specific agent |
| `ctx validate` | `ctx val` | Validate a ctx.yaml manifest |
| `ctx init --type <t>` | | Scaffold a new package |
| `ctx publish` | | Publish to registry |
| `ctx skill install` | | Install ctx's own skill to agents |

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Full JSON envelope with breadcrumbs |
| `--quiet` | Raw JSON data only |
| `--agent` | Agent mode (quiet + JSON errors) |
| `--styled` | Force ANSI colors |
| `--md` | Markdown table output |
| `--ids-only` | One ID per line |
| `--count` | Count only |
| `--yes` | Skip confirmation prompts |
| `--type` | Filter by type (skill/mcp/cli) |

## Common Workflows

### Install Tools for Your Agent
```bash
ctx s ffmpeg --type skill     # Search for skill packages
ctx i @ctx/ffmpeg             # Install (auto-links to all agents)
ctx ls                        # Verify installation
```

### Batch Update
```bash
ctx od                        # Check what's outdated
ctx up                        # Update everything
```

### Version Management
```bash
ctx use @ctx/basecamp@0.7.2   # Rollback to older version (instant)
ctx prune --keep 2            # Clean old versions, keep 2 most recent
```

### Diagnose Issues
```bash
ctx dr                        # Check agents, registry, links
```

## Package Types

- **skill**: SKILL.md that teaches agents how to use a tool (lowest friction)
- **cli**: CLI binary with optional SKILL.md wrapper (sweet spot for most tools)
- **mcp**: MCP server with typed tools (highest capability)

## JSON Output Examples

```bash
# Search with agent mode
ctx s review --agent
[{"full_name":"@ctx/review","type":"skill","description":"Code review skill"}]

# Install with JSON envelope
ctx i @ctx/ffmpeg --json
{"ok":true,"data":{"full_name":"@ctx/ffmpeg","version":"1.0.0"},"summary":"installed","breadcrumbs":[...]}
```
