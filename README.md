# wabridge

WhatsApp MCP bridge — a single Go binary that connects to WhatsApp via [whatsmeow](https://github.com/tulir/whatsmeow) and exposes your messages as [MCP](https://modelcontextprotocol.io/) tools.

## Features

- Read chats, messages, and contacts from WhatsApp
- Send text messages, files, and voice messages
- Download media attachments
- Search messages with substring search
- @mention resolution (JIDs to display names)
- Three operating modes: standalone, bridge+mcp, Docker

## Quick Start

### Build

```bash
go build -o wabridge .
```

### Standalone Mode (simplest)

Connects to WhatsApp and serves MCP in a single process.

```bash
./wabridge standalone
```

On first run, scan the QR code with WhatsApp (Settings > Linked Devices > Link a Device). Messages sync automatically.

### Bridge + MCP Mode (persistent)

The bridge stays connected to WhatsApp even when Claude isn't running.

```bash
# Terminal 1: start the bridge (stays running)
./wabridge bridge

# Terminal 2: start the MCP server (ephemeral, launched by your MCP client)
./wabridge mcp --bridge-url http://localhost:8080
```

## MCP Client Configuration

### Standalone mode

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "/path/to/wabridge",
      "args": ["standalone"]
    }
  }
}
```

### Bridge + MCP mode

Start the bridge first (`./wabridge bridge`), then configure your MCP client:

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "/path/to/wabridge",
      "args": ["mcp", "--db", "/path/to/messages.db", "--bridge-url", "http://localhost:8080"]
    }
  }
}
```

### Docker

```bash
# Setup (one-time)
cp .env.example .env
# Edit .env: set WABRIDGE_DATA_DIR, WABRIDGE_UID, WABRIDGE_GID
mkdir -p "$WABRIDGE_DATA_DIR"  # must exist before first run

# Start bridge (on first run, check logs for QR code: docker compose logs bridge)
docker compose up -d bridge

# Configure MCP client
docker compose run --rm -T mcp
```

## MCP Tools

wabridge exposes 13 MCP tools for reading chats, contacts, and messages, and for sending messages and media. See [docs/ops/MCP_TOOLS.md](docs/ops/MCP_TOOLS.md) for the full parameter reference.

## Documentation

See [docs/README.md](docs/README.md) for the full index.

| Topic | Document |
|-------|----------|
| Getting started | [docs/ops/GETTING_STARTED.md](docs/ops/GETTING_STARTED.md) |
| MCP tools reference | [docs/ops/MCP_TOOLS.md](docs/ops/MCP_TOOLS.md) |
| Cookbook | [docs/ops/COOKBOOK.md](docs/ops/COOKBOOK.md) |
| Automation safety | [docs/ops/AUTOMATION_SAFETY.md](docs/ops/AUTOMATION_SAFETY.md) |
| Architecture | [docs/dev/ARCHITECTURE.md](docs/dev/ARCHITECTURE.md) |
| Database schema | [docs/dev/SCHEMA.md](docs/dev/SCHEMA.md) |
| REST API | [docs/ops/REST_API.md](docs/ops/REST_API.md) |
| WhatsApp quirks | [docs/dev/WHATSAPP_QUIRKS.md](docs/dev/WHATSAPP_QUIRKS.md) |