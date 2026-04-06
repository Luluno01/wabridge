# Read-only mode via tiered access levels

## Problem

wabridge exposes both read (query) and write (action) MCP tools. Some deployments — automated dashboards, daily briefings, monitoring — only need to read messages and should not have the ability to send. Running with write tools enabled in an unattended automation context creates risk: a misbehaving agent could send unintended messages to real contacts.

There is no way to disable action tools without modifying code.

## Design

### Feature config

A new package `internal/feature` with a `Config` struct:

```go
type Config struct {
    Send        bool // send_message, send_file, send_audio_message
    Download    bool // download_media
    HistorySync bool // request_history_sync
}
```

### Access levels

Tiered presets control which tool categories are enabled:

| Level | Send | Download | HistorySync | Description |
|-------|------|----------|-------------|-------------|
| 0     | false | false   | false       | Strict read-only: query tools only |
| 1     | false | true    | false       | Read + media download |
| 2     | false | true    | true        | Read + download + history sync |
| 3     | true  | true    | true        | Full access (default) |

### Per-feature overrides

A single `--features` flag accepts `+`/`-` prefixed feature names applied on top of the access level preset:

```
--features=+download,-send
```

Feature names: `send`, `download`, `history-sync`.

Overrides can both grant tools above the preset level and revoke tools below it.

### CLI flags & environment variables

Two new persistent flags on `rootCmd`:

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--access-level=N` | `WABRIDGE_ACCESS_LEVEL` | `3` | Preset access level (0-3) |
| `--features=+foo,-bar` | `WABRIDGE_FEATURES` | `""` | Per-feature overrides |

Default is level 3 (full access) for backward compatibility.

### Constructor: `NewConfig`

```go
func NewConfig(level int, overrides string) (Config, error)
```

1. Look up the preset for the given level.
2. Parse the overrides string into `+`/`-` feature toggles.
3. Apply toggles on top of the preset.
4. Return the resulting config.

Invalid level or unrecognized feature names return an error.

### MCP server integration

`mcp.NewServer` gains a `feature.Config` parameter. In `registerTools()`, query tools are always registered. Action tools are gated:

- `features.Send`: `send_message`, `send_file`, `send_audio_message`
- `features.Download`: `download_media`
- `features.HistorySync`: `request_history_sync`

Disabled tools are not registered — they do not appear in the MCP tool list.

### REST API integration

`api.NewAPIServer` gains a `feature.Config` parameter. In `Handler()`, action routes are conditionally registered:

- `features.Send`: `POST /api/send`, `POST /api/send-file`, `POST /api/send-audio`
- `features.Download`: `POST /api/download`
- `features.HistorySync`: `POST /api/sync-history`

Unregistered routes 404 naturally. `GET /health` is always registered.

### Bridge feature endpoint & MCP pull

The bridge exposes a new metadata endpoint `GET /api/features` that returns its feature config as JSON. This endpoint is always registered (not gated).

`api.APIClient` gets a new method `GetFeatures() (feature.Config, error)`.

In `cmd/mcp.go` at startup:

1. Pull bridge features via `apiClient.GetFeatures()`.
2. Compute local config from local flags.
3. Effective config = `min(bridge, local)` — each field is `bridge.X && local.X`.

This ensures the MCP process never registers a tool the bridge has disabled. Local flags can further restrict but not grant beyond the bridge.

If the `mcp` subcommand cannot reach the bridge (connection refused, timeout), it falls back to using only its local config. This is safe because the bridge will still enforce its own restrictions on incoming API calls.

### Docker configuration

Both `WABRIDGE_ACCESS_LEVEL` and `WABRIDGE_FEATURES` are added to `.env.example` and `docker-compose.yml`.

## Affected files

| File | Change |
|------|--------|
| `internal/feature/feature.go` | New — `Config` struct, `NewConfig`, level presets, override parsing |
| `internal/feature/feature_test.go` | New — unit tests for config construction and overrides |
| `cmd/root.go` | Add `--access-level` and `--features` persistent flags |
| `cmd/standalone.go` | Pass `feature.Config` to `mcp.NewServer` |
| `cmd/bridge.go` | Pass `feature.Config` to `api.NewAPIServer` |
| `cmd/mcp.go` | Pull bridge features, compute effective config, pass to `mcp.NewServer` |
| `internal/mcp/server.go` | Accept `feature.Config` in `NewServer` |
| `internal/mcp/tools.go` | Gate action tool registration on feature flags |
| `internal/api/server.go` | Accept `feature.Config`, gate routes, add `GET /api/features` |
| `internal/api/client.go` | Add `GetFeatures()` method |
| `internal/api/server_test.go` | Extend with tests for feature-gated routes |
| `.env.example` | Add `WABRIDGE_ACCESS_LEVEL` and `WABRIDGE_FEATURES` |
| `docker-compose.yml` | Pass through the new env vars |

## Testing

- **`feature.NewConfig` unit tests**: each access level produces correct preset; overrides grant and revoke correctly; invalid names rejected
- **`min(bridge, local)` unit test**: verify AND behavior across configs
- **`api/server_test.go`**: disabled routes 404 at different access levels
- **MCP tool registration**: `registerTools()` with different configs produces expected tool count
