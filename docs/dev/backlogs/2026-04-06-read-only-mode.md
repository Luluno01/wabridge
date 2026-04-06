# Read-only mode via feature switches

## Problem

wabridge exposes both read (query) and write (action) MCP tools. Some deployments — automated dashboards, daily briefings, monitoring — only need to read messages and should not have the ability to send. There is no way to disable action tools without modifying code.

Running with write tools enabled in an unattended automation context creates risk: a misbehaving agent could send unintended messages to real contacts.

## Proposed Solution

Add feature switches that allow the user to disable selected tool categories at startup. When a tool category is disabled, the corresponding MCP tools are not registered and the REST API endpoints return 403.

### Configuration

Environment variables or CLI flags:

```
--enable-send=false      # disables send_message, send_file, send_audio_message
--enable-history-sync=false  # disables request_history_sync
```

Or a single flag for convenience:

```
--read-only              # disables all action tools (equivalent to --enable-send=false --enable-history-sync=false)
```

`download_media` is a grey area — it requires a live WhatsApp connection (action) but is read-oriented (fetching an attachment). It should probably remain enabled in read-only mode, but could have its own switch.

### Scope

- `cmd/root.go` or per-subcommand flags — parse feature switches
- `internal/mcp/tools.go` — conditionally register tools based on switches
- `internal/api/server.go` — conditionally register REST endpoints or return 403
- Docker: expose as environment variables in `.env.example` and `docker-compose.yml`

### Open Questions

- Should `download_media` be considered a read or write operation?
- Should the switches be per-tool granularity, or category-based (read vs write)?
- Should disabled tools still appear in the MCP tool list (with a note) or be completely hidden?
