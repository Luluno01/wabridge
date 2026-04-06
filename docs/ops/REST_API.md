# REST API

The bridge process exposes a REST API on `:8080` (configurable with `--addr`). In bridge+mcp mode, the MCP process uses this API to delegate actions that require a live WhatsApp connection — see [ARCHITECTURE.md](../dev/ARCHITECTURE.md) for the data flow. All endpoints return JSON with a standard envelope:

```json
{"success": true, "message": "...", "data": ...}
```

On error, `success` is `false` and `message` contains the error description.

---

## Access Levels

Action endpoints can be disabled at startup using the `--access-level` flag. Disabled endpoints are not registered and return 404. The health and features endpoints are always available. See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels) for the level table.

---

## Endpoints

### GET /health

Health check.

**Request:** no body.

**Response:**
```json
{"success": true, "message": "ok"}
```

**Example:**
```bash
curl http://localhost:8080/health
```

---

### GET /api/features

Returns the bridge's current feature configuration. Always available (not gated by access level). Used by the `mcp` subcommand to pull the bridge's feature flags at startup.

**Request:** no body.

**Response:**
```json
{"success": true, "data": {"send": true, "download": true, "history_sync": true}}
```

**Example:**
```bash
curl http://localhost:8080/api/features
```

---

### POST /api/send

Send a text message.

**Request body:**
```json
{
  "recipient": "1234567890@s.whatsapp.net",
  "message": "Hello!"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `recipient` | string | yes | Full JID or phone number |
| `message` | string | yes | Message text |

**Response:**
```json
{"success": true, "message": "sent"}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/send \
  -H 'Content-Type: application/json' \
  -d '{"recipient": "1234567890@s.whatsapp.net", "message": "Hello!"}'
```

---

### POST /api/send-file

Send a file as a media message.

**Request body:**
```json
{
  "recipient": "1234567890@s.whatsapp.net",
  "file_path": "/path/to/photo.jpg"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `recipient` | string | yes | Full JID or phone number |
| `file_path` | string | yes | Absolute path to the file (must be accessible to the bridge process) |

**Response:**
```json
{"success": true, "message": "file sent"}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/send-file \
  -H 'Content-Type: application/json' \
  -d '{"recipient": "1234567890@s.whatsapp.net", "file_path": "/tmp/photo.jpg"}'
```

---

### POST /api/send-audio

Send an Ogg Opus audio file as a voice message (PTT).

**Request body:**
```json
{
  "recipient": "1234567890@s.whatsapp.net",
  "file_path": "/path/to/voice.ogg"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `recipient` | string | yes | Full JID or phone number |
| `file_path` | string | yes | Path to Ogg Opus file |

**Response:**
```json
{"success": true, "message": "audio sent"}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/send-audio \
  -H 'Content-Type: application/json' \
  -d '{"recipient": "1234567890@s.whatsapp.net", "file_path": "/tmp/voice.ogg"}'
```

---

### POST /api/download

Download media from a stored message.

**Request body:**
```json
{
  "message_id": "3EB0ABC123",
  "chat_jid": "1234567890@s.whatsapp.net"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `message_id` | string | yes | Message ID |
| `chat_jid` | string | yes | Chat JID the message belongs to |

**Response:**
```json
{"success": true, "data": {"path": "/data/media/1234567890@s.whatsapp.net/3EB0ABC123.jpg"}}
```

The filename is the WhatsApp message ID plus the file extension. The media directory path depends on configuration (`--media-dir` flag or `WABRIDGE_DATA_DIR`).

**Example:**
```bash
curl -X POST http://localhost:8080/api/download \
  -H 'Content-Type: application/json' \
  -d '{"message_id": "3EB0ABC123", "chat_jid": "1234567890@s.whatsapp.net"}'
```

---

### POST /api/sync-history

Request additional message history from the primary WhatsApp device for a specific chat. Uses the oldest stored message in the chat as the cursor. The response arrives asynchronously as history sync events.

**Request body:**
```json
{
  "chat_jid": "120363012345678901@g.us"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `chat_jid` | string | yes | JID of the chat to request older history for |

Requires at least one stored message in the target chat to build a valid cursor.

**Response:**
```json
{"success": true, "message": "history sync requested"}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/sync-history \
  -H 'Content-Type: application/json' \
  -d '{"chat_jid": "120363012345678901@g.us"}'
```
