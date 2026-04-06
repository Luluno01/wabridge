# Architecture

wabridge is a single Go binary with three run modes. All modes use the same SQLite database and the same MCP tool definitions; they differ in how the WhatsApp connection and MCP server relate to each other.

## Three Modes

| Mode | Command | Description |
|------|---------|-------------|
| **Standalone** | `wabridge standalone` | All-in-one: WhatsApp client + MCP stdio server in a single process. |
| **Bridge** | `wabridge bridge` | Long-running daemon: WhatsApp client + REST API, no MCP. |
| **MCP** | `wabridge mcp` | Ephemeral MCP stdio server: reads SQLite directly, delegates actions to bridge via REST. |

### Why the split?

MCP clients (Claude Desktop, etc.) control the MCP server's lifecycle -- they start and stop it at will. WhatsApp, however, needs a persistent connection to receive messages and maintain session state. The bridge+mcp split decouples these lifecycles: the bridge stays connected to WhatsApp while MCP servers come and go.

## Data Flow

### Standalone Mode

```
WhatsApp Cloud
      |
      v
+------------------------------+
|         wabridge standalone  |
|                              |
|  whatsapp.Client             |
|    |-> event handlers        |
|    |     |-> store.Store ----+--> messages.db (SQLite)
|    |                         |
|  mcp.Server                  |
|    |-> query tools ----reads-+--> messages.db
|    |-> action tools -------->|
|         |-> DirectBackend    |
|              |-> WAClient    |
|                              |
|  stdin/stdout  <-- MCP stdio |
+------------------------------+
```

### Bridge + MCP Mode

```
WhatsApp Cloud
      |
      v
+-------------------------+         +------------------------+
|     wabridge bridge     |         |     wabridge mcp       |
|                         |         |                        |
|  whatsapp.Client        |         |  mcp.Server            |
|    |-> event handlers   |         |    |-> query tools ----+--reads--> messages.db
|    |     |-> store -----+-> messages.db                    |
|    |                    |         |    |-> action tools     |
|  api.APIServer          |  HTTP   |         |-> APIClient -+--POST--> bridge:8080
|    POST /api/send  <----+---------+---                     |
|    POST /api/download   |         |  stdin/stdout <-- MCP  |
|    ...                  |         +------------------------+
+-------------------------+
```

Both processes mount the same SQLite database. Queries (list chats, search messages) go directly to SQLite. Actions (send message, download media) require a live WhatsApp connection, so the MCP process forwards them to the bridge over HTTP.

## Package Map

```
cmd/
  root.go          Cobra root command, global flags (--db, --log-level)
  runtime.go       Shared startup: newRuntime (store + whatsapp + backend), signal handling
  standalone.go    Wires runtime + mcp.Server
  bridge.go        Wires runtime + api.APIServer
  mcp.go           Wires api.APIClient + mcp.Server (no WhatsApp)

internal/
  action/          Backend interface — abstracts actions requiring a live WhatsApp connection
  store/           GORM-backed SQLite: models, migrations, queries
  whatsapp/        whatsmeow wrapper: connection, event handlers, media utils
  mcp/             MCP server, tool registration, DirectBackend (implements action.Backend)
  api/             REST API server (bridge side) and client (MCP side, implements action.Backend)
  mention/         @mention JID-to-name resolution in message text
```

## Key Interfaces

**action.Backend** (defined in `internal/action/action.go`):

```go
type Backend interface {
    SendMessage(ctx, recipient, text) error
    SendFile(ctx, recipient, filePath) error
    SendAudioMessage(ctx, recipient, filePath) error
    DownloadMedia(ctx, messageID, chatJID) (string, error)
    RequestHistorySync(ctx, chatJID) error
}
```

Two implementations:
- **mcp.DirectBackend** -- calls whatsmeow directly (standalone and bridge modes)
- **api.APIClient** -- proxies calls over HTTP to the bridge (MCP mode)

## Docker

```yaml
services:
  bridge:       # persistent, restarts unless-stopped
  mcp:          # ephemeral, started on demand with docker compose run
  standalone:   # all-in-one alternative
```

All services bind-mount `WABRIDGE_DATA_DIR` to the same path inside the container (identity mount) so media paths work on both host and container. Services run as `WABRIDGE_UID:WABRIDGE_GID` with `umask 077` for strict file permissions. See `.env.example` for configuration.
