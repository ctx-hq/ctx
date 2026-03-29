---
name: ctx
description: |
  Universal package manager for AI agent skills, MCP servers, and CLI tools.
  Search, install, update, publish, sync, and manage packages across all detected agents.
triggers:
  - ctx
  - /ctx
  - ctx install
  - ctx search
  - ctx push
  - ctx sync
  - ctx org
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
  - push private skill
  - sync packages across devices
  - manage organization
  - create organization
  - ctx whoami
  - ctx login
  - who am i
  - check auth
  - check login status
invocable: true
argument-hint: "[command] [args...]"
---

# /ctx - AI Agent Package Manager

Universal package manager for skills, MCP servers, and CLI tools. One command to install and link to all detected AI agents. Supports both public registry (hub) and private resource management (push/sync).

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
2. Use `--caller <name>` on install to identify yourself (e.g., `--caller claude`). The caller agent is linked first.
3. Use `--type` to filter by package type: `skill`, `mcp`, `cli`
4. Installation auto-detects and links to all agents (Claude, Cursor, Windsurf, OpenCode, Codex)
5. Use `ctx dr` to diagnose environment issues
6. All operations are reversible — `ctx rm` cleanly removes all traces

## Quick Reference

| Command | Alias | Description |
|---------|-------|-------------|
| `ctx search <query>` | `ctx s` | Search registry for packages |
| `ctx install <pkg>` | `ctx i` | Install a package (auto-links to agents) |
| `ctx install <pkg>@beta` | | Install by dist-tag |
| `ctx remove <pkg>` | `ctx rm` | Uninstall and clean all links |
| `ctx list` | `ctx ls` | List installed packages |
| `ctx update` | `ctx up` | Update all packages |
| `ctx outdated` | `ctx od` | Check for available updates |
| `ctx info <pkg>` | | View package details (trust, tags, stats) |
| `ctx use <pkg>@<ver>` | | Switch to a local version (instant) |
| `ctx prune` | | Remove old versions, free disk |
| `ctx doctor` | `ctx dr` | Diagnose environment and connectivity |
| `ctx link [agent]` | `ctx ln` | Link packages to a specific agent |
| `ctx validate` | `ctx val` | Validate a ctx.yaml manifest |
| `ctx init` | | Scaffold a new package (interactive) |
| `ctx publish` | | Publish to registry (public) |
| `ctx push` | | Push as private package (zero friction) |
| `ctx dist-tag ls <pkg>` | `ctx tag ls` | List dist-tags |
| `ctx dist-tag add <pkg> <tag> <ver>` | | Set a dist-tag |
| `ctx dist-tag rm <pkg> <tag>` | | Remove a dist-tag |
| `ctx visibility <pkg> [vis]` | | View/change package visibility |
| `ctx enrich <pkg>` | | View/manage SKILL.md enrichment |
| `ctx sync export` | | Export installed state to local file |
| `ctx sync push` | | Upload sync profile to registry |
| `ctx sync pull` | | Restore packages on new device |
| `ctx sync status` | | View sync status + last sync time |
| `ctx org create <name>` | | Create an organization |
| `ctx org list` | `ctx org ls` | List your organizations |
| `ctx org info <name>` | | Show org details |
| `ctx org packages <name>` | | List org's packages |
| `ctx org add <org> <user>` | | Add member (--role admin) |
| `ctx org remove <org> <user>` | | Remove member |
| `ctx org delete <name>` | | Delete org (0 packages required) |
| `ctx login` | | Authenticate via GitHub |
| `ctx whoami` | | Show current authenticated user |
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
| `--offline` | Disable all network access |

## Common Workflows

### Install Tools for Your Agent
```bash
ctx s ffmpeg --type skill           # Search for skill packages
ctx i @ctx/ffmpeg --caller claude   # Install (caller agent gets priority)
ctx ls                              # Verify installation
```

### Push a Private Skill
```bash
cd my-custom-skill/
ctx push                            # Auto-detects SKILL.md, pushes as private
ctx install @me/my-custom-skill     # Install on another device
```

### Cross-device Sync
```bash
ctx sync push                       # Upload installed state (Machine A)
ctx sync pull                       # Restore everything (Machine B)
ctx sync status                     # Check last sync time
```

### Organization Workflow
```bash
ctx org create myteam               # Create org
ctx org add myteam alice --role admin  # Add team member
ctx publish                         # Publish to @myteam scope
ctx org packages myteam             # List team's packages
```

### Dist-tag Management
```bash
ctx publish                         # v2.0.0-beta.1 → auto-tags "beta"
ctx install @scope/pkg@beta         # Install by tag
ctx dist-tag add @scope/pkg stable 1.0.0  # Manual tag
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

## Visibility

- **public**: Discoverable via search, anyone can install
- **unlisted**: Not in search, but installable with full name
- **private**: Only publisher (or org members) can install, requires auth

## Trust Tiers

Packages go through progressive verification:
- **unverified** → just published
- **structural** → manifest valid, SHA256 matches
- **source_linked** → GitHub repo verified
- **reviewed** → AI security review passed
- **verified** → admin-verified

## JSON Output Examples

```bash
# Search with agent mode
ctx s review --agent
[{"full_name":"@ctx/review","type":"skill","description":"Code review skill","trust_tier":"structural"}]

# Install with JSON envelope
ctx i @ctx/ffmpeg --json
{"ok":true,"data":{"full_name":"@ctx/ffmpeg","version":"1.0.0"},"summary":"installed","breadcrumbs":[...]}

# Sync status
ctx sync status --json
{"ok":true,"data":{"package_count":12,"syncable_count":11,"last_push_at":"2026-03-29T12:00:00Z"}}
```
