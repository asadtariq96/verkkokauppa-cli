# Verkkokauppa CLI

Search, filter, price, and availability data for Verkkokauppa.com, Finland's largest consumer-electronics retailer. Three public JSON services power the web storefront: search (query + filters + facets + sort + pagination, with prices embedded), availability (per-store stock), and product info (brand, category, price, discount). No API key is required; the search service only needs a random UUID sessionId.

Created by [@asadtariq96](https://github.com/asadtariq96).

## Install

The recommended path installs both the `verkkokauppa-pp-cli` binary and the `pp-verkkokauppa` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install verkkokauppa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install verkkokauppa --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install verkkokauppa --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install verkkokauppa --agent claude-code
npx -y @mvanhorn/printing-press-library install verkkokauppa --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/verkkokauppa/cmd/verkkokauppa-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/verkkokauppa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install verkkokauppa --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-verkkokauppa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-verkkokauppa --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install verkkokauppa --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/verkkokauppa-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/verkkokauppa/cmd/verkkokauppa-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "verkkokauppa": {
      "command": "verkkokauppa-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
verkkokauppa-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

No API key is required. The search endpoint just needs a random UUID `sessionId`
(any UUID works):

```bash
# Search (return JSON, prices embedded)
verkkokauppa-pp-cli fi search-products \
  --filter-q "rtx 5090" \
  --session-id 550e8400-e29b-41d4-a716-446655440000 \
  --page-size 10 --json

# Stock availability for specific product ids
verkkokauppa-pp-cli fi get-availability --pids 975146,1011404 --json

# Price / brand / category info for product ids
verkkokauppa-pp-cli info --pids 975146 --json

# One-shot search + price + stock table (helper, see Cookbook)
bash bin/vk-stock "iphone 17" --size 10
```

## Usage

Run `verkkokauppa-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `VERKKOKAUPPA_CONFIG_DIR`, `VERKKOKAUPPA_DATA_DIR`, `VERKKOKAUPPA_STATE_DIR`, or `VERKKOKAUPPA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `VERKKOKAUPPA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export VERKKOKAUPPA_HOME=/srv/verkkokauppa
verkkokauppa-pp-cli doctor
```

Under `VERKKOKAUPPA_HOME=/srv/verkkokauppa`, the four dirs resolve to `/srv/verkkokauppa/config`, `/srv/verkkokauppa/data`, `/srv/verkkokauppa/state`, and `/srv/verkkokauppa/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "verkkokauppa": {
      "command": "verkkokauppa-pp-mcp",
      "env": {
        "VERKKOKAUPPA_HOME": "/srv/verkkokauppa"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `VERKKOKAUPPA_DATA_DIR` overrides an explicit `--home` for that kind. Use `VERKKOKAUPPA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `VERKKOKAUPPA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `verkkokauppa-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### fi

Manage fi

- **`verkkokauppa-pp-cli fi get-availability`** - Returns per-store and per-channel stock, purchase eligibility and flags (sold out, preorder, in campaign) for one or more product ids.
- **`verkkokauppa-pp-cli fi search-products`** - Returns product search results. Prices and basic availability flags are embedded in each product; facets (category, price range, brand, etc.) are returned in `included`. A `sessionId` (any UUID) is required.

### info

Manage info

- **`verkkokauppa-pp-cli info`** - Returns price, discount, brand and category breadcrumbs for one or more product ids.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`verkkokauppa-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`verkkokauppa-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`verkkokauppa-pp-cli learnings list`** - Inspect taught rows
- **`verkkokauppa-pp-cli learnings forget <query>`** - Undo a teach
- **`verkkokauppa-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`verkkokauppa-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`verkkokauppa-pp-cli teach-pattern`** - Install a query/resource template up front
- **`verkkokauppa-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `VERKKOKAUPPA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `verkkokauppa-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
verkkokauppa-pp-cli fi get-availability --pids example-value

# JSON for scripting and agents
verkkokauppa-pp-cli fi get-availability --pids example-value --json

# Filter to specific fields
verkkokauppa-pp-cli fi get-availability --pids example-value --json --select id,name,status

# Dry run — show the request without sending
verkkokauppa-pp-cli fi get-availability --pids example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
verkkokauppa-pp-cli fi get-availability --pids example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
verkkokauppa-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `verkkokauppa-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/verkkokauppa-com-product-pp-cli/config.toml`; `--home`, `VERKKOKAUPPA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Cookbook

These workflows are tested against the live Verkkokauppa.com API.

**Search with filters, sorting and facets** — prices are embedded, and facets
(category tree, price ranges, brands) come back in `included`:

```bash
# Full-text search, cheapest first
verkkokauppa-pp-cli fi search-products --filter-q "näytönohjain" \
  --session-id <uuid> --sort price --page-size 20 --json

# Narrow by category facet + price range
verkkokauppa-pp-cli fi search-products --filter-q "rtx 5090" \
  --filter-category information_technology --filter-price 1000-3000 \
  --session-id <uuid> --page-size 10 --json

# Show only the name + price of each hit
verkkokauppa-pp-cli fi search-products --filter-q "iphone 17" \
  --session-id <uuid> --page-size 5 --json --select id,attributes.name,attributes.articles
```

**Stock availability + flags** — sold out / preorder / campaign / store stock:

```bash
verkkokauppa-pp-cli fi get-availability --pids 975146,1011404 --json \
  --select pid,status.schema,flags.isSoldOut,flags.isPreorder
```

**Price + brand + category** — analytics-style one row per pid:

```bash
verkkokauppa-pp-cli info --pids 975146,1011404 --json
```

**One-shot `vk-stock` helper** (search + price + stock in a single table).
Wraps `search-products` then `get-availability`. Lives in `bin/vk-stock` /
`bin/vk-stock.py`; set `VK_SESSION_ID` to reuse a session:

```bash
python bin/vk-stock.py "iphone 17" --size 10
python bin/vk-stock.py "rtx 5090" --sort -price --category information_technology
python bin/vk-stock.py "airpods pro" --json          # raw JSON instead of table
```

Example output:

```
PID      NAME                                                         PRICE        WAS  STOCK
1011533  Apple AirPods Pro 3 nappikuulokkeet (MFHP4)                   249.00             InStock
979763   ASUS ROG Zephyrus G16 16" pelikannettava, Windows 11           4 799.99   5 579.99  InStock
975146   ASUS ROG Astral LC GeForce RTX 5090 32GB GDDR7 OC Edition     4 799.00             OutOfStock
```

**Pagination** — page through a full result set (`--all` fetches every page):

```bash
verkkokauppa-pp-cli fi search-products --filter-q "ssd" --session-id <uuid> \
  --page-number 2 --page-size 48 --json
```

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
