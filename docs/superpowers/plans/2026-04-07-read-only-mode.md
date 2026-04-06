# Read-only Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add tiered access levels (0-3) with per-feature overrides so deployments can disable action tools at startup.

**Architecture:** A `feature.Config` struct computed from `--access-level` + `--features` flags is threaded into `mcp.NewServer` and `api.NewAPIServer`. Disabled tools are never registered. The bridge exposes `GET /api/features` so the `mcp` subcommand can pull and intersect with local config.

**Tech Stack:** Go, Cobra (CLI), mcp-go, net/http, testify

**Spec:** `docs/superpowers/specs/2026-04-07-read-only-mode-design.md`

---

### Task 1: Feature config package

**Files:**
- Create: `internal/feature/feature.go`
- Create: `internal/feature/feature_test.go`

- [ ] **Step 1: Write failing tests for access level presets**

```go
package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_Level0(t *testing.T) {
	cfg, err := NewConfig(0, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: false, HistorySync: false}, cfg)
}

func TestNewConfig_Level1(t *testing.T) {
	cfg, err := NewConfig(1, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, cfg)
}

func TestNewConfig_Level2(t *testing.T) {
	cfg, err := NewConfig(2, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_Level3(t *testing.T) {
	cfg, err := NewConfig(3, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: true, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_InvalidLevel(t *testing.T) {
	_, err := NewConfig(4, "")
	assert.Error(t, err)

	_, err = NewConfig(-1, "")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/feature/ -v`
Expected: compilation error — package does not exist yet.

- [ ] **Step 3: Implement Config struct and NewConfig with level presets**

```go
package feature

import "fmt"

// Config controls which action tool categories are enabled.
type Config struct {
	Send        bool `json:"send"`
	Download    bool `json:"download"`
	HistorySync bool `json:"history_sync"`
}

// presets maps access level (0-3) to a Config.
var presets = map[int]Config{
	0: {Send: false, Download: false, HistorySync: false},
	1: {Send: false, Download: true, HistorySync: false},
	2: {Send: false, Download: true, HistorySync: true},
	3: {Send: true, Download: true, HistorySync: true},
}

// NewConfig builds a Config from an access level and an override string.
// The override string is a comma-separated list of "+feature" or "-feature"
// toggles applied on top of the level preset.
// Valid feature names: send, download, history-sync.
func NewConfig(level int, overrides string) (Config, error) {
	cfg, ok := presets[level]
	if !ok {
		return Config{}, fmt.Errorf("invalid access level %d (must be 0-3)", level)
	}

	if overrides == "" {
		return cfg, nil
	}

	return applyOverrides(cfg, overrides)
}
```

- [ ] **Step 4: Run tests to verify presets pass**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/feature/ -v`
Expected: preset tests PASS, but `applyOverrides` is undefined — compilation error.

- [ ] **Step 5: Write failing tests for override parsing**

Add to `internal/feature/feature_test.go`:

```go
func TestNewConfig_OverrideGrant(t *testing.T) {
	// Level 0 + grant download
	cfg, err := NewConfig(0, "+download")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, cfg)
}

func TestNewConfig_OverrideRevoke(t *testing.T) {
	// Level 3 + revoke send
	cfg, err := NewConfig(3, "-send")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_MultipleOverrides(t *testing.T) {
	// Level 0 + grant download and history-sync
	cfg, err := NewConfig(0, "+download,+history-sync")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_MixedOverrides(t *testing.T) {
	// Level 3 + revoke send, revoke history-sync
	cfg, err := NewConfig(3, "-send,-history-sync")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, cfg)
}

func TestNewConfig_InvalidFeatureName(t *testing.T) {
	_, err := NewConfig(3, "+invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown feature")
}

func TestNewConfig_MissingPrefix(t *testing.T) {
	_, err := NewConfig(3, "send")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must start with")
}
```

- [ ] **Step 6: Implement applyOverrides**

Add to `internal/feature/feature.go`:

```go
import (
	"fmt"
	"strings"
)

// applyOverrides parses a comma-separated override string and applies
// each toggle to the config. Each entry must be "+feature" or "-feature".
func applyOverrides(cfg Config, overrides string) (Config, error) {
	for _, entry := range strings.Split(overrides, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if len(entry) < 2 || (entry[0] != '+' && entry[0] != '-') {
			return Config{}, fmt.Errorf("override %q must start with + or -", entry)
		}

		enable := entry[0] == '+'
		name := entry[1:]

		switch name {
		case "send":
			cfg.Send = enable
		case "download":
			cfg.Download = enable
		case "history-sync":
			cfg.HistorySync = enable
		default:
			return Config{}, fmt.Errorf("unknown feature %q (valid: send, download, history-sync)", name)
		}
	}
	return cfg, nil
}
```

- [ ] **Step 7: Run all feature tests**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/feature/ -v`
Expected: all PASS.

- [ ] **Step 8: Write and run test for Intersect (min logic)**

Add to `internal/feature/feature_test.go`:

```go
func TestIntersect(t *testing.T) {
	a := Config{Send: true, Download: true, HistorySync: false}
	b := Config{Send: false, Download: true, HistorySync: true}
	got := Intersect(a, b)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, got)
}

func TestIntersect_BothFull(t *testing.T) {
	a := Config{Send: true, Download: true, HistorySync: true}
	b := Config{Send: true, Download: true, HistorySync: true}
	got := Intersect(a, b)
	assert.Equal(t, Config{Send: true, Download: true, HistorySync: true}, got)
}

func TestIntersect_BothEmpty(t *testing.T) {
	a := Config{}
	b := Config{}
	got := Intersect(a, b)
	assert.Equal(t, Config{}, got)
}
```

Add to `internal/feature/feature.go`:

```go
// Intersect returns a Config where each feature is enabled only if it is
// enabled in both a and b. Used to combine bridge and local configs.
func Intersect(a, b Config) Config {
	return Config{
		Send:        a.Send && b.Send,
		Download:    a.Download && b.Download,
		HistorySync: a.HistorySync && b.HistorySync,
	}
}
```

Run: `cd /home/untitled/personal/wabridge && go test ./internal/feature/ -v`
Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/feature/feature.go internal/feature/feature_test.go
git commit -m "feat: add feature config package with access levels and overrides"
```

---

### Task 2: CLI flags on root command

**Files:**
- Modify: `cmd/root.go:10-23`

- [ ] **Step 1: Add access-level and features flags**

In `cmd/root.go`, add two new package-level vars and register them as persistent flags:

```go
var (
	dbPath      string
	logLevel    string
	accessLevel int
	features    string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "messages.db", "path to SQLite database")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().IntVar(&accessLevel, "access-level", 3, "access level 0-3 (0=read-only, 3=full)")
	rootCmd.PersistentFlags().StringVar(&features, "features", "", "per-feature overrides (+send,-download,+history-sync)")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/untitled/personal/wabridge && go build ./...`
Expected: success with no errors.

- [ ] **Step 3: Verify flag appears in help**

Run: `cd /home/untitled/personal/wabridge && go run . --help`
Expected: `--access-level` and `--features` appear in the output.

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat: add --access-level and --features persistent flags"
```

---

### Task 3: Wire feature config into MCP server

**Files:**
- Modify: `internal/mcp/server.go:12-29`
- Modify: `internal/mcp/tools.go:44-58`
- Modify: `cmd/standalone.go:26-37`
- Modify: `cmd/mcp.go:26-42`

- [ ] **Step 1: Add features field to mcp.Server and update NewServer**

In `internal/mcp/server.go`, add `feature.Config` to the struct and constructor:

```go
import (
	"wabridge/internal/action"
	"wabridge/internal/feature"
	appstore "wabridge/internal/store"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcp      *mcpserver.MCPServer
	store    *appstore.Store
	backend  action.Backend
	features feature.Config
}

func NewServer(store *appstore.Store, backend action.Backend, features feature.Config) *Server {
	s := &Server{
		mcp:      mcpserver.NewMCPServer("wabridge", "1.0.0"),
		store:    store,
		backend:  backend,
		features: features,
	}

	s.registerTools()

	return s
}
```

- [ ] **Step 2: Gate action tool registration in registerTools**

In `internal/mcp/tools.go`, replace the `registerTools` function:

```go
// registerTools registers MCP tools on the server.
// Query tools are always registered. Action tools are gated by feature config.
func (s *Server) registerTools() {
	// Query tools — always registered
	s.registerSearchContacts()
	s.registerListChats()
	s.registerGetChat()
	s.registerGetDirectChatByContact()
	s.registerGetContactChats()
	s.registerListMessages()
	s.registerGetLastInteraction()
	s.registerGetMessageContext()

	// Action tools — conditional
	if s.features.Send {
		s.registerSendMessage()
		s.registerSendFile()
		s.registerSendAudioMessage()
	}
	if s.features.Download {
		s.registerDownloadMedia()
	}
	if s.features.HistorySync {
		s.registerRequestHistorySync()
	}
}
```

- [ ] **Step 3: Update cmd/standalone.go to build and pass feature.Config**

```go
package cmd

import (
	"wabridge/internal/feature"
	"wabridge/internal/mcp"

	"github.com/spf13/cobra"
)

var (
	standaloneSessionDB string
	standaloneMediaDir  string
)

var standaloneCmd = &cobra.Command{
	Use:   "standalone",
	Short: "All-in-one mode: WhatsApp connection + MCP server in one process",
	RunE:  runStandalone,
}

func init() {
	standaloneCmd.Flags().StringVar(&standaloneSessionDB, "session-db", "whatsapp.db", "path to WhatsApp session database")
	standaloneCmd.Flags().StringVar(&standaloneMediaDir, "media-dir", "media", "directory for downloaded media files")
	rootCmd.AddCommand(standaloneCmd)
}

func runStandalone(cmd *cobra.Command, args []string) error {
	featureCfg, err := feature.NewConfig(accessLevel, features)
	if err != nil {
		return err
	}

	rt, err := newRuntime(standaloneSessionDB, standaloneMediaDir)
	if err != nil {
		return err
	}
	defer rt.Store.Close()

	rt.handleShutdown()

	mcpServer := mcp.NewServer(rt.Store, rt.Backend, featureCfg)
	return mcpServer.ServeStdio()
}
```

- [ ] **Step 4: Update cmd/mcp.go to build and pass feature.Config**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/feature"
	"wabridge/internal/mcp"
	"wabridge/internal/store"
)

var bridgeURL string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Ephemeral MCP stdio server (reads SQLite, calls bridge REST API for actions)",
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&bridgeURL, "bridge-url", "http://localhost:8080", "URL of the bridge REST API")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	localCfg, err := feature.NewConfig(accessLevel, features)
	if err != nil {
		return err
	}

	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open app store: %w", err)
	}
	defer appStore.Close()

	apiClient := api.NewAPIClient(bridgeURL)

	// Pull feature config from the bridge and intersect with local config.
	// On failure (bridge unreachable), fall back to local config only.
	remoteCfg, err := apiClient.GetFeatures()
	effectiveCfg := localCfg
	if err == nil {
		effectiveCfg = feature.Intersect(remoteCfg, localCfg)
	}

	mcpServer := mcp.NewServer(appStore, apiClient, effectiveCfg)
	return mcpServer.ServeStdio()
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /home/untitled/personal/wabridge && go build ./...`
Expected: compilation error — `apiClient.GetFeatures()` does not exist yet. That's expected; we'll add it in Task 4. For now, comment out the `GetFeatures` call in `cmd/mcp.go` (just use `localCfg` directly) and verify the rest compiles.

Temporary in `cmd/mcp.go`:
```go
	mcpServer := mcp.NewServer(appStore, apiClient, localCfg)
```

Run: `cd /home/untitled/personal/wabridge && go build ./...`
Expected: success.

- [ ] **Step 6: Run existing tests to confirm no regressions**

Run: `cd /home/untitled/personal/wabridge && go test ./...`
Expected: all PASS. (The `api/server_test.go` tests may fail because `NewAPIServer` signature changed — that's addressed in Task 4.)

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/server.go internal/mcp/tools.go cmd/standalone.go cmd/mcp.go
git commit -m "feat: wire feature config into MCP server and gate action tools"
```

---

### Task 4: Wire feature config into REST API server + bridge features endpoint

**Files:**
- Modify: `internal/api/server.go:14-42`
- Modify: `internal/api/client.go`
- Modify: `internal/api/server_test.go:66-68`
- Modify: `cmd/bridge.go:31-43`

- [ ] **Step 1: Add features field to APIServer and update constructor**

In `internal/api/server.go`:

```go
import (
	"encoding/json"
	"fmt"
	"net/http"

	"wabridge/internal/action"
	"wabridge/internal/feature"
)

type APIServer struct {
	backend  action.Backend
	addr     string
	features feature.Config
}

func NewAPIServer(backend action.Backend, addr string, features feature.Config) *APIServer {
	return &APIServer{
		backend:  backend,
		addr:     addr,
		features: features,
	}
}
```

- [ ] **Step 2: Gate routes and add features endpoint in Handler()**

Replace the `Handler` method in `internal/api/server.go`:

```go
func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/features", s.handleFeatures)

	if s.features.Send {
		mux.HandleFunc("POST /api/send", s.handleSend)
		mux.HandleFunc("POST /api/send-file", s.handleSendFile)
		mux.HandleFunc("POST /api/send-audio", s.handleSendAudio)
	}
	if s.features.Download {
		mux.HandleFunc("POST /api/download", s.handleDownload)
	}
	if s.features.HistorySync {
		mux.HandleFunc("POST /api/sync-history", s.handleSyncHistory)
	}
	return mux
}
```

- [ ] **Step 3: Add handleFeatures handler**

Add to `internal/api/server.go`, after `handleHealth`:

```go
func (s *APIServer) handleFeatures(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    s.features,
	})
}
```

- [ ] **Step 4: Add GetFeatures to APIClient**

Add to `internal/api/client.go`:

```go
import (
	"wabridge/internal/feature"
)

// GetFeatures fetches the bridge's feature config from GET /api/features.
func (c *APIClient) GetFeatures() (feature.Config, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/features")
	if err != nil {
		return feature.Config{}, fmt.Errorf("get features: %w", err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Success bool           `json:"success"`
		Data    feature.Config `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return feature.Config{}, fmt.Errorf("decode features response: %w", err)
	}
	if !apiResp.Success {
		return feature.Config{}, fmt.Errorf("features endpoint returned success=false")
	}
	return apiResp.Data, nil
}
```

- [ ] **Step 5: Restore GetFeatures call in cmd/mcp.go**

Undo the temporary change from Task 3, Step 5. The `runMCP` function should use the full `GetFeatures` + `Intersect` logic:

```go
	remoteCfg, err := apiClient.GetFeatures()
	effectiveCfg := localCfg
	if err == nil {
		effectiveCfg = feature.Intersect(remoteCfg, localCfg)
	}

	mcpServer := mcp.NewServer(appStore, apiClient, effectiveCfg)
```

- [ ] **Step 6: Update cmd/bridge.go to build and pass feature.Config**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/feature"
)

var (
	bridgeAddr      string
	bridgeSessionDB string
	bridgeMediaDir  string
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Persistent WhatsApp bridge daemon with REST API",
	RunE:  runBridge,
}

func init() {
	bridgeCmd.Flags().StringVar(&bridgeAddr, "addr", ":8080", "address to listen on for REST API")
	bridgeCmd.Flags().StringVar(&bridgeSessionDB, "session-db", "whatsapp.db", "path to WhatsApp session database")
	bridgeCmd.Flags().StringVar(&bridgeMediaDir, "media-dir", "media", "directory for downloaded media files")
	rootCmd.AddCommand(bridgeCmd)
}

func runBridge(cmd *cobra.Command, args []string) error {
	featureCfg, err := feature.NewConfig(accessLevel, features)
	if err != nil {
		return err
	}

	rt, err := newRuntime(bridgeSessionDB, bridgeMediaDir)
	if err != nil {
		return err
	}
	defer rt.Store.Close()

	rt.handleShutdown()

	apiServer := api.NewAPIServer(rt.Backend, bridgeAddr, featureCfg)
	fmt.Fprintf(os.Stderr, "Bridge listening on %s\n", bridgeAddr)
	return apiServer.Start()
}
```

- [ ] **Step 7: Fix newTestServer in server_test.go**

In `internal/api/server_test.go`, update `newTestServer` to pass a full-access feature config:

```go
import (
	"wabridge/internal/feature"
)

func newTestServer(backend *mockBackend) *APIServer {
	return NewAPIServer(backend, ":0", feature.Config{Send: true, Download: true, HistorySync: true})
}
```

- [ ] **Step 8: Verify everything compiles and existing tests pass**

Run: `cd /home/untitled/personal/wabridge && go build ./... && go test ./...`
Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/api/server.go internal/api/client.go internal/api/server_test.go cmd/bridge.go cmd/mcp.go
git commit -m "feat: wire feature config into REST API, add GET /api/features endpoint"
```

---

### Task 5: REST API feature-gating tests

**Files:**
- Modify: `internal/api/server_test.go`

- [ ] **Step 1: Write tests for disabled routes returning 404**

Add to `internal/api/server_test.go`:

```go
func TestFeatureGating_Level0_SendReturns404(t *testing.T) {
	backend := &mockBackend{}
	srv := NewAPIServer(backend, ":0", feature.Config{})

	rr := doRequest(t, srv, "POST", "/api/send", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
		"message":   "hello",
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.False(t, backend.sendMessageCalled)
}

func TestFeatureGating_Level0_DownloadReturns404(t *testing.T) {
	backend := &mockBackend{}
	srv := NewAPIServer(backend, ":0", feature.Config{})

	rr := doRequest(t, srv, "POST", "/api/download", map[string]string{
		"message_id": "msg-123",
		"chat_jid":   "1234567890@s.whatsapp.net",
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.False(t, backend.downloadMediaCalled)
}

func TestFeatureGating_Level0_SyncHistoryReturns404(t *testing.T) {
	backend := &mockBackend{}
	srv := NewAPIServer(backend, ":0", feature.Config{})

	rr := doRequest(t, srv, "POST", "/api/sync-history", map[string]string{
		"chat_jid": "group@g.us",
	})

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.False(t, backend.historySyncCalled)
}

func TestFeatureGating_Level0_HealthStillWorks(t *testing.T) {
	backend := &mockBackend{}
	srv := NewAPIServer(backend, ":0", feature.Config{})

	rr := doRequest(t, srv, "GET", "/health", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)
}

func TestFeatureGating_Level0_FeaturesEndpointWorks(t *testing.T) {
	backend := &mockBackend{}
	cfg := feature.Config{}
	srv := NewAPIServer(backend, ":0", cfg)

	rr := doRequest(t, srv, "GET", "/api/features", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	resp := parseResponse(t, rr)
	assert.True(t, resp.Success)

	// Verify the returned config matches what was set
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, data["send"])
	assert.Equal(t, false, data["download"])
	assert.Equal(t, false, data["history_sync"])
}

func TestFeatureGating_DownloadOnlyEnabled(t *testing.T) {
	backend := &mockBackend{downloadMediaPath: "/media/photo.jpg"}
	srv := NewAPIServer(backend, ":0", feature.Config{Download: true})

	// download should work
	rr := doRequest(t, srv, "POST", "/api/download", map[string]string{
		"message_id": "msg-123",
		"chat_jid":   "1234567890@s.whatsapp.net",
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, backend.downloadMediaCalled)

	// send should 404
	rr = doRequest(t, srv, "POST", "/api/send", map[string]string{
		"recipient": "1234567890@s.whatsapp.net",
		"message":   "hello",
	})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
```

- [ ] **Step 2: Run tests**

Run: `cd /home/untitled/personal/wabridge && go test ./internal/api/ -v`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/server_test.go
git commit -m "test: add feature-gating tests for REST API routes"
```

---

### Task 6: Docker configuration

**Files:**
- Modify: `.env.example:10-11`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add env vars to .env.example**

Append to `.env.example` after the `WABRIDGE_GID` line:

```
# Access level controls which action tools are enabled.
# 0 = read-only (query tools only)
# 1 = read + media download
# 2 = read + download + history sync
# 3 = full access (default)
WABRIDGE_ACCESS_LEVEL=3

# Per-feature overrides applied on top of the access level.
# Comma-separated, prefix with + to enable or - to disable.
# Valid features: send, download, history-sync
# Example: +download,-send
WABRIDGE_FEATURES=
```

- [ ] **Step 2: Add flags to docker-compose.yml services**

In `docker-compose.yml`, add `--access-level` and `--features` args to the `bridge`, `mcp`, and `standalone` commands:

For the `bridge` service, after `--media-dir=...`:
```yaml
      - --access-level=${WABRIDGE_ACCESS_LEVEL:-3}
      - --features=${WABRIDGE_FEATURES:-}
```

For the `mcp` service, after `--bridge-url=...`:
```yaml
      - --access-level=${WABRIDGE_ACCESS_LEVEL:-3}
      - --features=${WABRIDGE_FEATURES:-}
```

For the `standalone` service, after `--media-dir=...`:
```yaml
      - --access-level=${WABRIDGE_ACCESS_LEVEL:-3}
      - --features=${WABRIDGE_FEATURES:-}
```

- [ ] **Step 3: Verify docker-compose config is valid**

Run: `cd /home/untitled/personal/wabridge && docker compose config --quiet 2>&1 || echo "config invalid"`
Expected: no output (valid config) or `config invalid` if Docker is not available (acceptable in dev).

- [ ] **Step 4: Commit**

```bash
git add .env.example docker-compose.yml
git commit -m "feat: expose access level and features env vars in Docker config"
```

---

### Task 7: Documentation updates

**Files:**
- Modify: `docs/ops/MCP_TOOLS.md:1-7`
- Modify: `docs/ops/REST_API.md:1-11`
- Modify: `docs/ops/AUTOMATION_SAFETY.md:24-26`
- Modify: `docs/ops/GETTING_STARTED.md:57-73,104-118,126-134`
- Modify: `docs/dev/ARCHITECTURE.md:64-81`
- Modify: `docs/ops/COOKBOOK.md:89-131`

- [ ] **Step 1: Update MCP_TOOLS.md — add access level section**

After line 7 (before `## Query Tools`), insert:

```markdown
## Access Levels

Action tools can be disabled at startup using the `--access-level` flag. Query tools are always available.

| Level | Available Action Tools |
|-------|-----------------------|
| 0 | None (read-only) |
| 1 | `download_media` |
| 2 | `download_media`, `request_history_sync` |
| 3 | All action tools (default) |

Per-feature overrides (`--features=+download,-send`) can grant or revoke individual tools on top of the preset. See [GETTING_STARTED.md](GETTING_STARTED.md) for configuration.

Disabled tools are not registered — they do not appear in the tool list.

---
```

- [ ] **Step 2: Update REST_API.md — add access level note and features endpoint**

After line 11 (before `## Endpoints`), insert:

```markdown
## Access Levels

Action endpoints can be disabled at startup using the `--access-level` flag. Disabled endpoints are not registered and return 404. The health and features endpoints are always available. See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels) for the level table.

---
```

After the `GET /health` section (after line 30), insert a new endpoint section:

```markdown
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
```

- [ ] **Step 3: Update AUTOMATION_SAFETY.md — replace read-only mode placeholder**

Replace line 26 in `docs/ops/AUTOMATION_SAFETY.md`:

```
- **Prefer reading over writing.** If your automation only needs to read messages (e.g., daily briefings, monitoring), consider running in read-only mode once available (see [backlog](../dev/backlogs/2026-04-06-read-only-mode.md)), or simply avoid calling send tools.
```

With:

```
- **Use read-only mode for read-only workloads.** If your automation only needs to read messages (e.g., daily briefings, monitoring), set `--access-level=0`. This prevents all action tools from being registered. For read + media download, use `--access-level=1`. See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels) for the full level table.
```

- [ ] **Step 4: Update GETTING_STARTED.md — add access level configuration**

After step 3 in "Bridge + MCP Mode" (after the MCP client config JSON block, around line 70), add:

```markdown
**Optional: restrict access level.** To run in read-only mode (no action tools):

```bash
./wabridge bridge --access-level=0
```

See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels) for access level details and per-feature overrides.
```

Add the same note after step 3 in "Standalone Mode" (around line 117) and after step 1 in "Docker Mode" (around line 134, referencing `WABRIDGE_ACCESS_LEVEL` in `.env`):

```markdown
**Optional: restrict access level.** Set `WABRIDGE_ACCESS_LEVEL=0` in `.env` for read-only mode. See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels).
```

- [ ] **Step 5: Update ARCHITECTURE.md — add feature config to package map**

In the package map section (around line 67), after the `root.go` line, add:

```
  root.go          Cobra root command, global flags (--db, --log-level, --access-level, --features)
```

Add a new entry to the `internal/` section:

```
  feature/         Access level presets and per-feature override parsing
```

- [ ] **Step 6: Update COOKBOOK.md — add access level notes to action recipes**

Add a note before the "Download Media" recipe (around line 89):

```markdown
> **Note:** The following recipes require specific access levels. `download_media` needs level 1+, `request_history_sync` needs level 2+, and send tools need level 3 (full access). See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels).
```

- [ ] **Step 7: Commit**

```bash
git add docs/ops/MCP_TOOLS.md docs/ops/REST_API.md docs/ops/AUTOMATION_SAFETY.md docs/ops/GETTING_STARTED.md docs/dev/ARCHITECTURE.md docs/ops/COOKBOOK.md
git commit -m "docs: add access level documentation across ops and dev guides"
```

---

### Task 8: Update backlog

**Files:**
- Modify: `docs/dev/backlogs/index.md:14`

- [ ] **Step 1: Mark the backlog item as done**

Replace line 14:
```
| [Read-only mode](2026-04-06-read-only-mode.md) | Feature switches to disable action tools, allowing wabridge to run in read-only mode |
```

With:
```
| ~~[Read-only mode](2026-04-06-read-only-mode.md)~~ | Done — tiered access levels (0-3) with `--access-level` and `--features` flags |
```

- [ ] **Step 2: Commit**

```bash
git add docs/dev/backlogs/index.md
git commit -m "docs: mark read-only mode backlog item as done"
```
