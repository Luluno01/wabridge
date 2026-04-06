# Getting Started

This guide covers first-run setup, WhatsApp pairing, and what to expect during initial sync. Choose the mode that fits your deployment.

## Choosing a Mode

| Mode | Best for | Trade-off |
|------|----------|-----------|
| **Bridge + MCP** | Production, automation, daily use | Two processes, but the bridge stays connected when MCP restarts |
| **Standalone** | Quick testing, one-off queries | Single process, but WhatsApp disconnects when the process stops |
| **Docker** | Reproducible deployments, servers | Requires Docker, but handles UID/permissions cleanly |

**Important:** WhatsApp allows only one active session per linked device. If you run two instances of the same client (e.g., two standalone processes sharing the same session database), the older one gets disconnected. For this reason, **standalone mode is not recommended for automation** that may run concurrently with other MCP clients — use bridge+mcp mode instead, where a single bridge holds the WhatsApp connection and multiple MCP servers can come and go.

## Prerequisites

- Go 1.22+ (for building from source)
- A WhatsApp account with a phone that can scan QR codes
- Docker and Docker Compose (for Docker mode only)

## Bridge + MCP Mode (recommended)

This is the recommended setup for daily use and automation. The bridge maintains a persistent WhatsApp connection; MCP servers are ephemeral and launched on demand by your MCP client.

### 1. Build

From the repository root:

```bash
go build -o wabridge .
```

### 2. Start the bridge

```bash
./wabridge bridge
```

On first run, you will see a QR code printed to the terminal. Scan it with WhatsApp:
1. Open WhatsApp on your phone
2. Go to **Settings > Linked Devices > Link a Device**
3. Scan the QR code displayed in the terminal

Multiple QR codes are generated in sequence while you scan (each lasts ~60 seconds). If the entire pairing session times out before you scan, the process exits with an error — restart it and try again.

After scanning, you will see log output like:
```
Login successful
History sync: received 42 conversations
History sync: stored 1523 messages
History sync settled, dumping contacts
Dumped 150 contacts from device store
```

The bridge continues running and receiving messages. Leave this terminal open.

### 3. Configure your MCP client

Add this to your MCP client configuration (e.g., Claude Desktop, Claude Code):

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

**Optional: restrict access level.** To run in read-only mode (no action tools):

```bash
./wabridge bridge --access-level=0
```

See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels) for access level details and per-feature overrides.

The `--db` path must point to the same database the bridge writes to. The `--bridge-url` must reach the bridge's REST API (default `:8080`).

### 4. Verify

Once your MCP client connects, try a simple query:
- `list_chats` — should return your WhatsApp chats
- `search_contacts` with your own name — should find your contact

## Standalone Mode

All-in-one: WhatsApp client and MCP server in a single process. Simple but stops receiving messages when the process exits.

### 1. Build

From the repository root:

```bash
go build -o wabridge .
```

### 2. Pair first (important)

**You must complete pairing before configuring your MCP client.** When an MCP client launches wabridge, it captures stdout for JSON-RPC — the QR code is printed to stderr and may not be visible in the MCP client's UI.

Run the binary directly to pair:

```bash
./wabridge standalone --db /path/to/messages.db --session-db /path/to/whatsapp.db
```

Same QR code flow as bridge mode — scan with your phone. Once paired, press Ctrl+C to stop.

### 3. Configure MCP client

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "/path/to/wabridge",
      "args": ["standalone", "--db", "/path/to/messages.db", "--session-db", "/path/to/whatsapp.db"]
    }
  }
}
```

**Optional: restrict access level.** To run in read-only mode:

```bash
./wabridge standalone --access-level=0
```

See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels) for access level details and per-feature overrides.

Use absolute paths for `--db` and `--session-db` — without them, the defaults are relative paths that resolve to whatever working directory the MCP client uses.

In standalone mode, the MCP client starts and stops the process. WhatsApp messages are only received while the process is running.

## Docker Mode

### 1. Configure environment

```bash
cp .env.example .env
```

Edit `.env`:
```bash
WABRIDGE_DATA_DIR=/home/youruser/.wabridge   # absolute path, will be bind-mounted
WABRIDGE_UID=1000                             # run: id -u
WABRIDGE_GID=1000                             # run: id -g
```

Create the data directory:
```bash
mkdir -p "$WABRIDGE_DATA_DIR"
```

**Optional: restrict access level.** Set `WABRIDGE_ACCESS_LEVEL=0` in `.env` for read-only mode. See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels).

### 2. Start the bridge

```bash
docker compose up bridge
```

**Keep `-d` off for first run** so you can see the QR code in the terminal output. Scan it with your phone (same flow as above).

After pairing succeeds, you can restart in detached mode:
```bash
docker compose up -d bridge
```

On subsequent runs, the bridge reconnects automatically using the saved session — no QR code needed.

### 3. View logs (detached mode)

```bash
docker compose logs -f bridge
```

### 4. Configure MCP client

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "docker",
      "args": ["compose", "-f", "/path/to/docker-compose.yml", "run", "--rm", "-T", "mcp"]
    }
  }
}
```

The `-T` flag disables pseudo-TTY allocation, which is required for MCP stdio communication.

## Initial Sync

When you first pair a device, WhatsApp sends a batch of recent message history. This happens automatically — you do not need to request it.

**What to expect:**
- The sync typically takes 10-60 seconds depending on how many chats you have
- You will see log lines like `History sync: received N conversations` and `History sync: stored N messages`
- After ~15 seconds of no new sync events, wabridge considers the sync settled and dumps contacts from the device store
- The initial sync usually provides the most recent ~3 months of messages (varies by account)

**What not to expect:**
- Full message history — WhatsApp only sends recent messages during initial pairing
- The `request_history_sync` tool can ask the phone for older messages, but the phone rarely responds (see [MCP_TOOLS.md](MCP_TOOLS.md#request_history_sync) for details)
- To re-trigger the initial sync, unlink the device and re-pair

## Reconnection

On subsequent starts (after the first pairing), the bridge reconnects automatically using the saved session. No QR code is needed unless the device has been unlinked.

If WhatsApp logs you out (e.g., you unlink from your phone, or WhatsApp forces a logout), the bridge logs a warning:
```
Device logged out, please scan QR code to log in again
```

Restart the bridge in foreground mode to scan a new QR code.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| QR code not visible | Running in detached Docker mode | Run `docker compose up bridge` (no `-d`) for first pairing |
| QR code times out | Pairing session expired before scan | Restart the process and scan promptly |
| `connection lost` errors from MCP tools | Bridge is not running or not reachable | Check that the bridge process is running and `--bridge-url` is correct |
| No messages after pairing | Sync still in progress | Wait 30-60 seconds, check bridge logs for sync progress |
| `Device logged out` warning | Unlinked from phone or WhatsApp forced logout | Restart bridge in foreground, scan new QR code |
| Old MCP binary after rebuild | Docker `restart` reuses the old container | Use `docker compose up -d bridge` instead of `restart` to recreate |
