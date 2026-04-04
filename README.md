# wabridge

WhatsApp MCP bridge — a single Go binary that connects to WhatsApp via [whatsmeow](https://github.com/tulir/whatsmeow) and exposes your messages as [MCP](https://modelcontextprotocol.io/) tools.

## Features

- Read chats, messages, and contacts from WhatsApp
- Send text messages, files, and voice messages
- Download media attachments
- Search messages with full-text search
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

# Start bridge
docker compose up -d bridge

# Configure MCP client
docker compose run --rm -T mcp
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `list_chats` | List chats sorted by most recent message |
| `list_messages` | Query messages with filters (chat, sender, date range, search) |
| `search_contacts` | Search contacts by name or phone number |
| `get_chat` | Get single chat metadata |
| `get_direct_chat_by_contact` | Find a 1:1 chat by phone number |
| `get_contact_chats` | Get all chats involving a contact |
| `get_last_interaction` | Most recent message with a contact |
| `get_message_context` | Messages surrounding a specific message |
| `send_message` | Send a text message |
| `send_file` | Send an image, video, or document |
| `send_audio_message` | Send a voice message (Ogg Opus) |
| `download_media` | Download a media attachment |
| `request_history_sync` | Request older message history |

## Documentation

See [docs/](docs/) for detailed reference:

| Topic | Document |
|-------|----------|
| Architecture | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Database schema | [docs/SCHEMA.md](docs/SCHEMA.md) |
| MCP tools reference | [docs/MCP_TOOLS.md](docs/MCP_TOOLS.md) |
| REST API | [docs/REST_API.md](docs/REST_API.md) |
| WhatsApp quirks | [docs/WHATSAPP_QUIRKS.md](docs/WHATSAPP_QUIRKS.md) |