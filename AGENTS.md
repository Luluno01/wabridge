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

# Docker
docker compose up bridge           # persistent bridge
docker compose run --rm -T mcp     # ephemeral MCP server
```

## Documentation

| Topic             | Document                 |
|-------------------|--------------------------|
| Architecture      | docs/ARCHITECTURE.md     |
| Database schema   | docs/SCHEMA.md           |
| MCP tools         | docs/MCP_TOOLS.md        |
| REST API          | docs/REST_API.md         |
| WhatsApp quirks   | docs/WHATSAPP_QUIRKS.md  |
| Design spec       | docs/specs/2026-04-02-wabridge-design.md |

## Project Layout

```
cmd/           CLI subcommands (standalone, bridge, mcp)
internal/
  store/       GORM models and database queries
  whatsapp/    whatsmeow connection, events, media
  mcp/         MCP tool definitions and server
  api/         REST API server and client
  mention/     @mention resolution
```
