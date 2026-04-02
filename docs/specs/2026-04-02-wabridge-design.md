# wabridge Design Spec

WhatsApp MCP bridge — single Go binary that connects to WhatsApp via whatsmeow, stores messages in SQLite, and serves MCP tools over stdio.

## Modes

Three subcommands, one binary:

- **`wabridge standalone`** — all-in-one process. Connects to WhatsApp, stores messages, serves MCP over stdio. WhatsApp disconnects when the MCP client disconnects.
- **`wabridge bridge`** — persistent daemon. Connects to WhatsApp, stores messages, exposes REST API. Runs independently of MCP clients.
- **`wabridge mcp`** — ephemeral MCP stdio server. Reads SQLite directly for queries, calls the bridge REST API for actions (send, download, sync). Launched on demand by MCP clients.

### Why the split

MCP clients control the MCP process lifecycle. But WhatsApp should stay connected and keep receiving messages even when the MCP client isn't running. The bridge runs independently; the mcp process is disposable. Standalone mode is the simple option when you don't need persistence beyond the MCP session.

### Pairing

All modes auto-detect session state. If no valid session exists or the session has expired (~20 days), the QR code is displayed in the terminal. No separate pairing command needed.

## Architecture

```
standalone mode:
  ┌───────────┐ ┌───────┐ ┌───────────┐
  │ whatsapp  │→│ store │←│    mcp    │
  └───────────┘ └───────┘ └───────────┘

bridge + mcp mode:
  bridge process:              mcp process:
  ┌───────────┐ ┌─────┐       ┌───────┐ ┌─────────┐
  │ whatsapp  │→│store│       │ store │ │   mcp   │
  └───────────┘ └─────┘       └───────┘ └─────────┘
  ┌───────────┐               ┌─────────┐
  │ api server│←─────────────→│api client│
  └───────────┘               └─────────┘
```

- **Shared SQLite file** — bridge writes, mcp reads (same filesystem / Docker volume)
- **REST API** — mcp calls bridge over HTTP for actions requiring a live WhatsApp connection

## Database Schema

GORM with SQLite. Swappable to Postgres later by changing the driver.

### chats

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| jid | TEXT | PRIMARY KEY | `@s.whatsapp.net`, `@lid`, or `@g.us` |
| name | TEXT | nullable | Cached name for groups only. NULL for 1:1 chats. |
| last_message_time | TIMESTAMP | indexed | For recency sorting |

### contacts

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| jid | TEXT | PRIMARY KEY | Phone JID or LID JID |
| phone_jid | TEXT | indexed, nullable | Cross-reference: LID entry points to phone JID |
| push_name | TEXT | nullable | From message push notifications (transient) |
| full_name | TEXT | nullable | From address book sync (stable) |
| updated_at | TIMESTAMP | | |

**Dual-entry strategy:** Each contact gets two rows — one keyed by phone JID, one by LID JID. The LID row's `phone_jid` points back for cross-referencing.

### messages

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| id | TEXT | PK (composite) | WhatsApp message ID |
| chat_jid | TEXT | PK (composite), indexed | |
| sender | TEXT | NOT NULL, indexed | Always `ToNonAD().String()` — canonical JID, never a display name |
| content | TEXT | | |
| timestamp | TIMESTAMP | NOT NULL, indexed | |
| is_from_me | BOOLEAN | NOT NULL | |
| media_type | TEXT | nullable | "image", "video", "audio", "document" |
| mime_type | TEXT | nullable | e.g. "image/jpeg" |
| filename | TEXT | nullable | Original filename when available |
| url | TEXT | nullable | WhatsApp CDN URL |
| media_key | BLOB | nullable | |
| file_sha256 | BLOB | nullable | |
| file_enc_sha256 | BLOB | nullable | |
| file_length | INTEGER | nullable | |
| mentioned_jids | TEXT | nullable | JSON array of JIDs from ContextInfo.MentionedJid |

### Name resolution (query time)

```sql
COALESCE(NULLIF(contacts.full_name, ''), NULLIF(contacts.push_name, ''), messages.sender)
```

For group messages, dual JOIN — one on chat JID for chat name, one on sender JID for sender name.

### Contact upsert

Explicit Go logic: only overwrite a field if the new value is non-empty. Separate method available to explicitly clear fields when needed. Avoids the original's `COALESCE(NULLIF(...))` pattern that couldn't clear values back to empty.

## MCP Tools

All tools served over stdio. Same set as the original.

### Query tools (read SQLite directly)

| Tool | Description |
|------|-------------|
| `search_contacts` | Search contacts by name or phone number |
| `list_messages` | Query messages with filters: chat, sender, date range, text search, pagination |
| `list_chats` | List chats sorted by recency or name |
| `get_chat` | Get single chat metadata |
| `get_direct_chat_by_contact` | Find 1:1 chat by phone number |
| `get_contact_chats` | Get all chats involving a contact |
| `get_last_interaction` | Most recent message with a contact |
| `get_message_context` | Messages surrounding a target message |

### Action tools (route through ActionBackend interface)

| Tool | Description |
|------|-------------|
| `send_message` | Send text message |
| `send_file` | Send image/video/document |
| `send_audio_message` | Send audio as voice message (Opus OGG) |
| `download_media` | Download media attachment to local filesystem |
| `request_history_sync` | Trigger older message sync from WhatsApp |

### ActionBackend interface

```go
type ActionBackend interface {
    SendMessage(ctx context.Context, recipient, text string) error
    SendFile(ctx context.Context, recipient, path string) error
    SendAudioMessage(ctx context.Context, recipient, path string) error
    DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error)
    RequestHistorySync(ctx context.Context) error
}
```

Standalone mode: implemented by the whatsapp package (direct whatsmeow calls).
MCP mode: implemented by the api client package (REST calls to bridge).

### Mention resolution

- `mentioned_jids` stored at write time from protobuf `ContextInfo.MentionedJid`
- At query time, `@<number>` patterns in message content are resolved to display names using the mentioned JIDs + contacts table lookup
- All query tools accept a `raw` boolean parameter — when true, returns unresolved original text

## WhatsApp Event Handling

### Events

| Event | Action |
|-------|--------|
| `events.Message` | Extract text/media, store message, update chat, store sender push name |
| `events.HistorySync` | Process batches via `ParseWebMessage`, store messages with proper sender JIDs |
| `events.PushName` | Update contact push name |
| `events.Contact` | Update contact full name |
| `events.Connected` | Dump all contacts via `GetAllContacts()`, map LIDs via `GetLIDForPN()` |

### Text content extraction order

1. `msg.Conversation` — simple text
2. `msg.ExtendedTextMessage.GetText()` — text with link preview/formatting
3. Empty string — media-only

### History sync

- Uses `ParseWebMessage` to correctly resolve sender JIDs in group messages
- Completion detected by settling period (15 seconds of no new events)
- After settling, triggers full contact dump
- `BuildHistorySyncRequest` wrapped in panic recovery

### Fixes from original

- All WhatsApp API calls use contexts with timeouts
- Sender always stored as `ToNonAD().String()` from the start — no migration path needed
- Thread-safe waveform generation: `rand.New(rand.NewSource(...))` instead of global `rand.Seed`

## REST API (bridge mode)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health check |
| `/api/send` | POST | Send text message |
| `/api/send-file` | POST | Send media file |
| `/api/send-audio` | POST | Send voice message |
| `/api/download` | POST | Download media by message ID + chat JID |
| `/api/sync-history` | POST | Trigger history sync |

## Docker

Single Dockerfile, single image. Different subcommands for each mode.

```yaml
services:
  bridge:
    build: .
    command: ["wabridge", "bridge"]
    restart: unless-stopped
    volumes:
      - store:/app/store
    networks:
      - wabridge-net

  mcp:
    build: .
    command: ["wabridge", "mcp"]
    stdin_open: true
    volumes:
      - store:/app/store
    networks:
      - wabridge-net
    environment:
      - WABRIDGE_API_URL=http://bridge:8080
    depends_on:
      - bridge
    profiles:
      - mcp

  standalone:
    build: .
    command: ["wabridge", "standalone"]
    stdin_open: true
    volumes:
      - store:/app/store
    profiles:
      - standalone

volumes:
  store:

networks:
  wabridge-net:
```

- `docker compose up bridge` — persistent bridge
- `docker compose run --rm -T mcp` — ephemeral MCP client
- `docker compose run --rm -T standalone` — all-in-one

## Project Structure

```
wabridge/
├── AGENTS.md
├── docs/
│   ├── specs/                # design specs (source of truth)
│   ├── ARCHITECTURE.md       # modes, components, data flow
│   ├── SCHEMA.md             # tables, JID formats, name resolution
│   ├── MCP_TOOLS.md          # tool catalog with params and examples
│   ├── REST_API.md           # endpoints, request/response formats
│   └── WHATSAPP_QUIRKS.md    # platform gotchas (JID migration, ParseWebMessage, etc.)
├── cmd/
│   ├── root.go               # Cobra root command, global flags
│   ├── standalone.go
│   ├── bridge.go
│   └── mcp.go
├── internal/
│   ├── store/
│   │   ├── models.go         # GORM models
│   │   ├── store.go          # DB init, migrations
│   │   ├── messages.go       # message queries
│   │   ├── contacts.go       # contact queries, upsert
│   │   └── chats.go          # chat queries
│   ├── whatsapp/
│   │   ├── client.go         # whatsmeow connection, QR pairing
│   │   ├── handlers.go       # event handlers
│   │   └── media.go          # media handling, ogg analysis, waveform
│   ├── mcp/
│   │   ├── server.go         # MCP stdio server setup
│   │   └── tools.go          # tool definitions and handlers
│   ├── api/
│   │   ├── server.go         # REST API server (bridge mode)
│   │   └── client.go         # REST API client (mcp mode)
│   └── mention/
│       └── resolve.go        # @JID -> @DisplayName resolution
├── go.mod
├── Dockerfile
├── docker-compose.yml
└── mise.toml
```

## Dependencies

- `go.mau.fi/whatsmeow` — WhatsApp protocol
- `gorm.io/gorm` + `gorm.io/driver/sqlite` — ORM
- `github.com/spf13/cobra` — CLI
- `github.com/modelcontextprotocol/go-sdk` — official Go MCP SDK (stdio transport)
- `github.com/mdp/qrterminal` — QR code display
