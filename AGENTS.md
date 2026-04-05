# wabridge

WhatsApp MCP bridge -- single Go binary that connects to WhatsApp and serves MCP tools.

## Quick Start

```bash
# Build
go build -o wabridge .

# Run standalone (all-in-one)
./wabridge standalone

# Run as bridge + mcp (two-process)
./wabridge bridge &
./wabridge mcp

# Docker (requires .env — see .env.example)
cp .env.example .env               # edit WABRIDGE_DATA_DIR, UID, GID
mkdir -p "$WABRIDGE_DATA_DIR"
docker compose up bridge           # persistent bridge
docker compose run --rm -T mcp     # ephemeral MCP server
```

## Documentation

| Topic             | Document                 |
|-------------------|--------------------------|
| Architecture      | docs/dev/ARCHITECTURE.md     |
| Database schema   | docs/dev/SCHEMA.md           |
| MCP tools         | docs/ops/MCP_TOOLS.md        |
| REST API          | docs/ops/REST_API.md         |
| WhatsApp quirks   | docs/dev/WHATSAPP_QUIRKS.md  |
| Design spec       | docs/dev/specs/2026-04-02-wabridge-design.md (archived — historical reference only) |

## Project Layout

```
cmd/           CLI subcommands (standalone, bridge, mcp) + shared runtime
internal/
  action/      Backend interface for WhatsApp actions
  store/       GORM models and database queries
  whatsapp/    whatsmeow connection, events, media
  mcp/         MCP tool definitions, server, DirectBackend
  api/         REST API server and client
  mention/     @mention resolution
```
