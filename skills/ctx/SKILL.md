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
  - ctx logout
  - who am i
  - check auth
  - check login status
  - ctx workspace
  - workspace init
  - monorepo skills
  - publish all skills
  - install collection
  - multi-skill repo
  - ctx token
  - api token
  - create token
  - ctx star
  - star a package
  - ctx artifact
  - upload artifact
  - ctx rename
  - rename package
  - ctx transfer
  - transfer ownership
  - ctx unpublish
  - ctx yank
  - ctx wrap
  - wrap cli tool
  - ctx tui
  - browse packages
  - ctx serve
  - mcp server mode
  - ctx config
  - ctx upgrade
  - ctx notifications
  - ctx mcp test
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
| `ctx publish <file.md>` | | Scaffold + publish a single .md skill |
| `ctx push` | | Push as private package (zero friction) |
| `ctx push <file.md>` | | Scaffold + push a single .md skill |
| `ctx push --bump patch` | | Bump version and push |
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
| `ctx org add <org> <user>` | | Add member directly (--role admin) |
| `ctx org invite <org> <user>` | | Invite member (--role admin) |
| `ctx org invitations <org>` | | List pending invitations |
| `ctx org cancel-invite <org> <id>` | | Cancel a pending invitation |
| `ctx org remove <org> <user>` | | Remove member (cascades access) |
| `ctx org delete <name>` | | Delete org (0 packages required) |
| `ctx invitations` | `ctx inv` | List your pending invitations |
| `ctx invitations accept <id>` | | Accept an org invitation |
| `ctx invitations decline <id>` | | Decline an org invitation |
| `ctx access <package>` | | List users with access to a package |
| `ctx access grant <pkg> <user...>` | | Grant access to users |
| `ctx access revoke <pkg> <user...>` | | Revoke access from users |
| `ctx login` | | Authenticate via GitHub |
| `ctx whoami` | | Show current authenticated user |
| `ctx skill install` | | Install ctx's own skill to agents |
| `ctx workspace list` | `ctx ws ls` | List workspace members |
| `ctx workspace init` | `ctx ws init` | Initialize workspace from existing repo |
| `ctx workspace validate` | `ctx ws val` | Validate all workspace members |
| `ctx publish --all` | | Publish all workspace members |
| `ctx publish --filter <glob>` | | Publish matching workspace members |
| `ctx install <collection>` | | Install all skills in a collection |
| `ctx install <collection> --pick` | | Interactive skill selection from collection |
| `ctx install --from-list <list>` | | Batch install from a star list |
| `ctx token create` | | Create an API token (scoped permissions) |
| `ctx token list` | `ctx token ls` | List your API tokens |
| `ctx token revoke <id>` | | Revoke an API token |
| `ctx star <pkg>` | | Star/unstar a package |
| `ctx star list` | `ctx star ls` | List your starred packages |
| `ctx star create <name>` | | Create a star list (curated collection) |
| `ctx star show` | | Show your star lists |
| `ctx artifact upload` | | Upload a platform-specific artifact |
| `ctx artifact list` | `ctx artifact ls` | List artifacts for a version |
| `ctx rename <pkg> <new>` | | Rename a package (old name redirects) |
| `ctx transfer <pkg> <scope>` | | Transfer ownership to another scope |
| `ctx transfers` | `ctx xfer` | List incoming transfer requests |
| `ctx transfers accept <id>` | | Accept a transfer request |
| `ctx transfers decline <id>` | | Decline a transfer request |
| `ctx unpublish <pkg>` | | Permanently delete a package or version |
| `ctx yank <pkg>@<ver>` | | Retract a published version (reversible) |
| `ctx wrap <binary>` | | Package a CLI tool as a ctx skill |
| `ctx tui` | | Interactive terminal package browser |
| `ctx serve` | | Run ctx as an MCP server over stdio |
| `ctx config list` | | List all configuration settings |
| `ctx config get <key>` | | Get a configuration value |
| `ctx config set <key> <val>` | | Set a configuration value |
| `ctx upgrade` | | Upgrade ctx to the latest version |
| `ctx logout` | | Log out and clear credentials |
| `ctx mcp test` | | Test an MCP server connection |
| `ctx mcp list` | `ctx mcp ls` | List installed MCP servers |
| `ctx mcp env` | | Manage MCP server env variables |
| `ctx notifications` | `ctx notif` | List notifications |
| `ctx notifications read <id>` | | Mark notification as read |
| `ctx notifications dismiss <id>` | | Dismiss a notification |

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
| `--caller` | Identify calling agent (e.g., `--caller claude`) |
| `--offline` | Disable all network access |
| `-v, --verbose` | Show verbose diagnostic output |

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

### Publish a Single-File Skill
```bash
# From a standalone .md file (e.g., a Claude Code slash command)
ctx push ~/.claude/commands/gc.md   # Interactive: scaffolds → publishes → links back

# After first push, edit and update from the skills dir
ctx push ~/.ctx/skills/biao29/gc/   # Push updates
ctx push ~/.ctx/skills/biao29/gc/ --bump patch  # Bump version + push

# Public release
ctx publish ~/.ctx/skills/biao29/gc/
```

### Cross-device Sync
```bash
ctx sync push                       # Upload installed state (Machine A)
ctx sync pull                       # Restore everything (Machine B)
ctx sync status                     # Check last sync time
```

### Organization Workflow
```bash
ctx org create myteam                          # Create org
ctx org invite myteam alice --role admin        # Invite member (they must accept)
ctx invitations                                # (as alice) See pending invitations
ctx invitations accept <id>                    # (as alice) Accept invitation
ctx access grant @myteam/pkg alice bob              # Grant access to specific users
ctx org packages myteam                        # List team's packages
ctx org remove myteam alice                    # Remove member (cascades access cleanup)
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

### API Tokens (CI/CD)
```bash
ctx token create --name "ci" --scope publish  # Create scoped token
ctx token ls                                  # List tokens
ctx token revoke <id>                         # Revoke a token
```

### Star & Curate
```bash
ctx star @ctx/ffmpeg              # Star a package
ctx star ls                       # List starred packages
ctx star create "my-toolkit"      # Create a star list
ctx install --from-list my-toolkit  # Batch install from star list
```

### Artifact Management
```bash
ctx artifact upload --platform darwin-arm64 ./my-binary  # Upload artifact
ctx artifact ls @scope/pkg@1.0.0                         # List artifacts
```

### Package Lifecycle
```bash
ctx rename @me/old-name new-name   # Rename (old redirects)
ctx transfer @me/pkg @org          # Transfer ownership
ctx yank @me/pkg@1.0.0             # Retract a version (reversible)
ctx unpublish @me/pkg --yes        # Permanently delete
```

### Wrap a CLI Tool
```bash
ctx wrap ./my-binary               # Introspect + generate ctx.yaml & SKILL.md
ctx publish                        # Then publish to registry
```

### MCP Server Management
```bash
ctx mcp ls                         # List installed MCP servers
ctx mcp test                       # Test all connections
ctx mcp env set MY_KEY val         # Set env variable
```

### Diagnose Issues
```bash
ctx dr                        # Check agents, registry, links
```

### Workspace (Monorepo) Management
```bash
# Initialize workspace from existing multi-skill repo
cd my-skills-repo/
ctx workspace init --scan "skills/*" --scope "@myname"
ctx workspace init --scan "*" --exclude "docs,scripts" --scope "@team"  # Root-level skills
ctx workspace init --scan "marketing/*,engineering/*" --scope "@org"     # Nested hierarchy
ctx workspace list                        # List discovered skills
ctx workspace validate                    # Check all members

# Publish all skills + auto-create collections
ctx publish --all                         # Publish all members
ctx publish --filter "baoyu-trans*"       # Publish matching only
ctx publish --all --continue-on-error     # Skip failures

# Consumer: install individual or collection
ctx install @myname/translate             # Single skill
ctx install @myname/skills                # All skills in collection
ctx install @myname/skills --pick         # Choose interactively
```

## Package Types

- **skill**: SKILL.md that teaches agents how to use a tool (lowest friction)
- **cli**: CLI binary with optional SKILL.md wrapper (sweet spot for most tools)
- **mcp**: MCP server with typed tools (highest capability)

## Visibility

- **public**: Discoverable via search, anyone can install
- **unlisted**: Not in search, but installable with full name
- **private**: Only the owner (or org members) can install, requires auth
- **private + access grant**: Only specified users can access (`ctx access grant` for per-user ACL)

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
