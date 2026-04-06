# wabridge Implementation Plan

> **Archived.** This implementation plan was used to build wabridge and is now complete. Checkboxes are left in their original state as a historical record. Do not use this as a guide for current development — refer to the reference docs ([ARCHITECTURE.md](../ARCHITECTURE.md), [SCHEMA.md](../SCHEMA.md), [MCP_TOOLS.md](../../ops/MCP_TOOLS.md), [REST_API.md](../../ops/REST_API.md)) instead. Known post-implementation changes: `RequestHistorySync` now requires a `chatJID` parameter and `/api/sync-history` requires a `{"chat_jid": "..."}` body.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary that connects to WhatsApp via whatsmeow, stores messages in SQLite via GORM, and serves MCP tools over stdio — with three operating modes (standalone, bridge, mcp).

**Architecture:** Single binary with Cobra subcommands. `internal/store` handles all database operations via GORM. `internal/whatsapp` wraps whatsmeow for connection, event handling, and media. `internal/mcp` defines MCP tools. `internal/api` provides REST server/client for bridge+mcp mode. `internal/mention` resolves @JID references to display names.

**Tech Stack:** Go, GORM (SQLite), whatsmeow, Cobra, mcp-go (mark3labs), testify

**Reference:** Design spec at `docs/specs/2026-04-02-wabridge-design.md`. WhatsApp platform quirks at `whatsapp-bridge-knowledge.md`. Original implementation at ``.

---

## File Map

| File | Responsibility |
|------|---------------|
| `main.go` | Cobra entry point |
| `cmd/root.go` | Root command, global flags (db path, log level) |
| `cmd/standalone.go` | Standalone mode: WhatsApp + MCP in one process |
| `cmd/bridge.go` | Bridge mode: WhatsApp + REST API daemon |
| `cmd/mcp.go` | MCP mode: stdio server, reads SQLite, calls bridge REST |
| `internal/store/models.go` | GORM models: Chat, Contact, Message |
| `internal/store/store.go` | DB initialization, auto-migrate, Close |
| `internal/store/chats.go` | Chat CRUD + list queries |
| `internal/store/contacts.go` | Contact upsert (non-empty-only), search, dual-entry lookup |
| `internal/store/messages.go` | Message storage, list with JOINs, context queries |
| `internal/store/store_test.go` | Tests for all store operations |
| `internal/mention/resolve.go` | @JID to @DisplayName resolution |
| `internal/mention/resolve_test.go` | Mention resolution tests |
| `internal/whatsapp/client.go` | whatsmeow connection, QR pairing, session management |
| `internal/whatsapp/handlers.go` | Event handlers: Message, HistorySync, PushName, Contact, Connected |
| `internal/whatsapp/media.go` | Media download/upload, Ogg analysis, waveform generation |
| `internal/mcp/server.go` | MCP stdio server setup, tool registration |
| `internal/mcp/tools.go` | All 13 MCP tool handlers |
| `internal/mcp/backend.go` | ActionBackend interface + direct (whatsapp) implementation |
| `internal/api/server.go` | REST API server (bridge mode) |
| `internal/api/client.go` | REST API client (implements ActionBackend for mcp mode) |
| `Dockerfile` | Multi-stage build |
| `docker-compose.yml` | bridge, mcp, standalone services |
| `AGENTS.md` | Agent/human entry point with progressive disclosure |
| `docs/ARCHITECTURE.md` | System overview, modes, data flow |
| `docs/SCHEMA.md` | Database schema reference |
| `docs/MCP_TOOLS.md` | Tool catalog with parameters and examples |
| `docs/REST_API.md` | REST endpoint reference |
| `docs/WHATSAPP_QUIRKS.md` | Platform-specific gotchas |

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`

- [ ] **Step 1: Initialize Go module and install core dependencies**

```bash
cd /path/to/wabridge
go mod init wabridge
go get gorm.io/gorm gorm.io/driver/sqlite
go get github.com/spf13/cobra
go get github.com/mark3labs/mcp-go
go get github.com/stretchr/testify
go get go.mau.fi/whatsmeow
go get github.com/mdp/qrterminal/v3
go get github.com/mattn/go-sqlite3
```

- [ ] **Step 2: Create main.go**

```go
// main.go
package main

import "wabridge/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Create cmd/root.go**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	dbPath   string
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "wabridge",
	Short: "WhatsApp MCP bridge",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "messages.db", "path to SQLite database")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /path/to/wabridge && go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go
git commit -m "feat: project scaffolding with cobra CLI"
```

---

### Task 2: GORM Models + Store Initialization

**Files:**
- Create: `internal/store/models.go`
- Create: `internal/store/store.go`

- [ ] **Step 1: Create GORM models**

```go
// internal/store/models.go
package store

import "time"

type Chat struct {
	JID             string    `gorm:"primaryKey" json:"jid"`
	Name            *string   `json:"name,omitempty"`
	LastMessageTime time.Time `gorm:"index" json:"last_message_time"`
}

type Contact struct {
	JID       string    `gorm:"primaryKey" json:"jid"`
	PhoneJID  *string   `gorm:"index" json:"phone_jid,omitempty"`
	PushName  *string   `json:"push_name,omitempty"`
	FullName  *string   `json:"full_name,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	ChatJID       string    `gorm:"primaryKey;index" json:"chat_jid"`
	Sender        string    `gorm:"not null;index" json:"sender"`
	Content       string    `json:"content"`
	Timestamp     time.Time `gorm:"not null;index" json:"timestamp"`
	IsFromMe      bool      `gorm:"not null" json:"is_from_me"`
	MediaType     *string   `json:"media_type,omitempty"`
	MimeType      *string   `json:"mime_type,omitempty"`
	Filename      *string   `json:"filename,omitempty"`
	URL           *string   `json:"url,omitempty"`
	MediaKey      []byte    `json:"-"`
	FileSHA256    []byte    `json:"-"`
	FileEncSHA256 []byte    `json:"-"`
	FileLength    *int64    `json:"file_length,omitempty"`
	MentionedJIDs *string   `json:"mentioned_jids,omitempty"`
}
```

- [ ] **Step 2: Create store initialization**

```go
// internal/store/store.go
package store

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	db *gorm.DB
}

func New(dsn string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Chat{}, &Contact{}, &Message{}); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 4: Commit**

```bash
git add internal/store/models.go internal/store/store.go
git commit -m "feat: GORM models and store initialization"
```

---

### Task 3: Chat Operations + Tests

**Files:**
- Create: `internal/store/chats.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: Write chat operation tests**

```go
// internal/store/store_test.go
package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func strPtr(s string) *string { return &s }

func TestUpsertChat(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	// Insert new chat
	err := s.UpsertChat("group@g.us", strPtr("Family Group"), now)
	require.NoError(t, err)

	chat, err := s.GetChat("group@g.us")
	require.NoError(t, err)
	assert.Equal(t, "Family Group", *chat.Name)

	// Upsert updates name
	err = s.UpsertChat("group@g.us", strPtr("Renamed Group"), now.Add(time.Hour))
	require.NoError(t, err)

	chat, err = s.GetChat("group@g.us")
	require.NoError(t, err)
	assert.Equal(t, "Renamed Group", *chat.Name)
}

func TestUpsertChat_NilName(t *testing.T) {
	s := newTestStore(t)

	// 1:1 chats have nil name
	err := s.UpsertChat("123@s.whatsapp.net", nil, time.Now())
	require.NoError(t, err)

	chat, err := s.GetChat("123@s.whatsapp.net")
	require.NoError(t, err)
	assert.Nil(t, chat.Name)
}

func TestListChats(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	s.UpsertChat("a@g.us", strPtr("Alpha"), now)
	s.UpsertChat("b@g.us", strPtr("Beta"), now.Add(time.Hour))
	s.UpsertChat("c@s.whatsapp.net", nil, now.Add(2*time.Hour))

	// List by recency
	chats, err := s.ListChats("", 10)
	require.NoError(t, err)
	assert.Len(t, chats, 3)
	assert.Equal(t, "c@s.whatsapp.net", chats[0].JID) // most recent first

	// Filter by name
	chats, err = s.ListChats("alpha", 10)
	require.NoError(t, err)
	assert.Len(t, chats, 1)
	assert.Equal(t, "a@g.us", chats[0].JID)
}

func TestGetChat_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetChat("nonexistent@g.us")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -v -run TestUpsertChat`
Expected: FAIL — `UpsertChat`, `GetChat`, `ListChats` not defined.

- [ ] **Step 3: Implement chat operations**

```go
// internal/store/chats.go
package store

import (
	"time"

	"gorm.io/gorm/clause"
)

func (s *Store) UpsertChat(jid string, name *string, lastMessageTime time.Time) error {
	chat := Chat{
		JID:             jid,
		Name:            name,
		LastMessageTime: lastMessageTime,
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "jid"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "last_message_time"}),
	}).Create(&chat).Error
}

func (s *Store) GetChat(jid string) (*Chat, error) {
	var chat Chat
	if err := s.db.Where("jid = ?", jid).First(&chat).Error; err != nil {
		return nil, err
	}
	return &chat, nil
}

type ChatResult struct {
	Chat
	DisplayName string `json:"display_name"`
}

func (s *Store) ListChats(filter string, limit int) ([]ChatResult, error) {
	var results []ChatResult

	query := s.db.Table("chats").
		Select("chats.*, " +
			"COALESCE(chats.name, contacts.full_name, contacts.push_name, chats.jid) AS display_name").
		Joins("LEFT JOIN contacts ON chats.jid = contacts.jid").
		Order("chats.last_message_time DESC")

	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where(
			"chats.name LIKE ? OR contacts.full_name LIKE ? OR contacts.push_name LIKE ? OR chats.jid LIKE ?",
			like, like, like, like,
		)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	return results, query.Scan(&results).Error
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -v -run TestUpsertChat -run TestListChats -run TestGetChat`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/chats.go internal/store/store_test.go
git commit -m "feat: chat store operations with tests"
```

---

### Task 4: Contact Operations + Tests

**Files:**
- Modify: `internal/store/store_test.go`
- Create: `internal/store/contacts.go`

- [ ] **Step 1: Write contact operation tests**

Append to `internal/store/store_test.go`:

```go
func TestUpsertContact_Insert(t *testing.T) {
	s := newTestStore(t)

	err := s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Alice"),
		FullName: strPtr("Alice Smith"),
	})
	require.NoError(t, err)

	c, err := s.GetContact("123@s.whatsapp.net")
	require.NoError(t, err)
	assert.Equal(t, "Alice", *c.PushName)
	assert.Equal(t, "Alice Smith", *c.FullName)
}

func TestUpsertContact_OnlyUpdatesNonEmpty(t *testing.T) {
	s := newTestStore(t)

	// Initial insert with full name
	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		FullName: strPtr("Alice Smith"),
	})

	// Upsert with only push name — should NOT clear full name
	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Ali"),
	})

	c, err := s.GetContact("123@s.whatsapp.net")
	require.NoError(t, err)
	assert.Equal(t, "Alice Smith", *c.FullName) // preserved
	assert.Equal(t, "Ali", *c.PushName)         // updated
}

func TestUpsertContact_DualEntry(t *testing.T) {
	s := newTestStore(t)

	// Phone JID entry
	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Alice"),
		FullName: strPtr("Alice Smith"),
	})

	// LID entry pointing to phone JID
	s.UpsertContact(&Contact{
		JID:      "456@lid",
		PhoneJID: strPtr("123@s.whatsapp.net"),
		PushName: strPtr("Alice"),
	})

	// Lookup by LID should work
	c, err := s.GetContact("456@lid")
	require.NoError(t, err)
	assert.Equal(t, "Alice", *c.PushName)
	assert.Equal(t, "123@s.whatsapp.net", *c.PhoneJID)
}

func TestSearchContacts(t *testing.T) {
	s := newTestStore(t)

	s.UpsertContact(&Contact{JID: "1@s.whatsapp.net", FullName: strPtr("Alice Smith")})
	s.UpsertContact(&Contact{JID: "2@s.whatsapp.net", FullName: strPtr("Bob Jones")})
	s.UpsertContact(&Contact{JID: "3@s.whatsapp.net", PushName: strPtr("Alice W")})

	results, err := s.SearchContacts("alice", 50)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGetContactName(t *testing.T) {
	s := newTestStore(t)

	// full_name takes priority over push_name
	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Ali"),
		FullName: strPtr("Alice Smith"),
	})

	name := s.GetContactName("123@s.whatsapp.net")
	assert.Equal(t, "Alice Smith", name)

	// Falls back to push_name if no full_name
	s.UpsertContact(&Contact{
		JID:      "456@s.whatsapp.net",
		PushName: strPtr("Bob"),
	})

	name = s.GetContactName("456@s.whatsapp.net")
	assert.Equal(t, "Bob", name)

	// Returns empty string if not found
	name = s.GetContactName("unknown@s.whatsapp.net")
	assert.Equal(t, "", name)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -v -run TestUpsertContact`
Expected: FAIL — `UpsertContact`, `GetContact`, `SearchContacts`, `GetContactName` not defined.

- [ ] **Step 3: Implement contact operations**

```go
// internal/store/contacts.go
package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

func (s *Store) UpsertContact(contact *Contact) error {
	contact.UpdatedAt = time.Now()

	return s.db.Transaction(func(tx *gorm.DB) error {
		var existing Contact
		err := tx.Where("jid = ?", contact.JID).First(&existing).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(contact).Error
		}
		if err != nil {
			return err
		}

		updates := map[string]interface{}{
			"updated_at": contact.UpdatedAt,
		}
		if contact.PhoneJID != nil && *contact.PhoneJID != "" {
			updates["phone_jid"] = *contact.PhoneJID
		}
		if contact.PushName != nil && *contact.PushName != "" {
			updates["push_name"] = *contact.PushName
		}
		if contact.FullName != nil && *contact.FullName != "" {
			updates["full_name"] = *contact.FullName
		}

		return tx.Model(&existing).Updates(updates).Error
	})
}

func (s *Store) ClearContactField(jid, field string) error {
	return s.db.Model(&Contact{}).Where("jid = ?", jid).Update(field, nil).Error
}

func (s *Store) GetContact(jid string) (*Contact, error) {
	var contact Contact
	if err := s.db.Where("jid = ?", jid).First(&contact).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}

func (s *Store) GetContactName(jid string) string {
	var contact Contact
	err := s.db.Where("jid = ? OR phone_jid = ?", jid, jid).First(&contact).Error
	if err != nil {
		return ""
	}
	if contact.FullName != nil && *contact.FullName != "" {
		return *contact.FullName
	}
	if contact.PushName != nil && *contact.PushName != "" {
		return *contact.PushName
	}
	return ""
}

func (s *Store) SearchContacts(query string, limit int) ([]Contact, error) {
	var contacts []Contact
	like := "%" + query + "%"

	err := s.db.Where(
		"full_name LIKE ? OR push_name LIKE ? OR jid LIKE ?",
		like, like, like,
	).Where("jid NOT LIKE ?", "%@g.us").
		Limit(limit).
		Find(&contacts).Error

	return contacts, err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -v -run "TestUpsertContact|TestSearchContacts|TestGetContactName"`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/contacts.go internal/store/store_test.go
git commit -m "feat: contact store operations with dual-entry and smart upsert"
```

---

### Task 5: Message Operations + Tests

**Files:**
- Modify: `internal/store/store_test.go`
- Create: `internal/store/messages.go`

- [ ] **Step 1: Write message operation tests**

Append to `internal/store/store_test.go`:

```go
func TestStoreMessage(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:        "msg1",
		ChatJID:   "chat@g.us",
		Sender:    "123@s.whatsapp.net",
		Content:   "Hello world",
		Timestamp: time.Now().Truncate(time.Second),
		IsFromMe:  false,
	}

	err := s.StoreMessage(msg)
	require.NoError(t, err)

	// Storing same message again should upsert without error
	msg.Content = "Updated content"
	err = s.StoreMessage(msg)
	require.NoError(t, err)

	got, err := s.GetMessage("msg1", "chat@g.us")
	require.NoError(t, err)
	assert.Equal(t, "Updated content", got.Content)
}

func TestStoreMessage_CreatesChat(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:        "msg1",
		ChatJID:   "new-chat@g.us",
		Sender:    "123@s.whatsapp.net",
		Content:   "First message",
		Timestamp: time.Now(),
		IsFromMe:  false,
	}

	err := s.StoreMessage(msg)
	require.NoError(t, err)

	// Chat should be auto-created
	chat, err := s.GetChat("new-chat@g.us")
	require.NoError(t, err)
	assert.Equal(t, "new-chat@g.us", chat.JID)
}

func TestListMessages(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	s.UpsertContact(&Contact{JID: "alice@s.whatsapp.net", FullName: strPtr("Alice Smith")})
	s.UpsertChat("chat@g.us", strPtr("Test Group"), base.Add(2*time.Minute))

	for i := 0; i < 5; i++ {
		s.StoreMessage(&Message{
			ID:        fmt.Sprintf("msg%d", i),
			ChatJID:   "chat@g.us",
			Sender:    "alice@s.whatsapp.net",
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}

	// Basic list
	results, err := s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 5)
	assert.Equal(t, "Alice Smith", results[0].SenderName)
	assert.Equal(t, "Test Group", results[0].ChatName)

	// Date range
	after := base.Add(2 * time.Minute)
	results, err = s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", After: &after, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 3) // messages 2, 3, 4

	// Text search
	results, err = s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Search: "Message 3", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Pagination
	results, err = s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 2, Page: 2})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGetMessageContext(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.StoreMessage(&Message{
			ID:        fmt.Sprintf("msg%d", i),
			ChatJID:   "chat@g.us",
			Sender:    "alice@s.whatsapp.net",
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}

	// Get 2 messages before and after msg5
	results, err := s.GetMessageContext("msg5", "chat@g.us", 2, 2)
	require.NoError(t, err)
	assert.Len(t, results, 5) // msg3, msg4, msg5, msg6, msg7
	assert.Equal(t, "Message 3", results[0].Content)
	assert.Equal(t, "Message 7", results[4].Content)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -v -run "TestStoreMessage|TestListMessages|TestGetMessageContext"`
Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement message operations**

```go
// internal/store/messages.go
package store

import (
	"fmt"
	"time"

	"gorm.io/gorm/clause"
)

type ListMessagesOpts struct {
	ChatJID string
	Sender  string
	After   *time.Time
	Before  *time.Time
	Search  string
	Limit   int
	Page    int
}

type MessageResult struct {
	Message
	ChatName   string `json:"chat_name"`
	SenderName string `json:"sender_name"`
}

func (s *Store) StoreMessage(msg *Message) error {
	// Auto-create chat entry
	s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Chat{
		JID:             msg.ChatJID,
		LastMessageTime: msg.Timestamp,
	})

	// Update chat's last message time if this message is newer
	s.db.Model(&Chat{}).
		Where("jid = ? AND last_message_time < ?", msg.ChatJID, msg.Timestamp).
		Update("last_message_time", msg.Timestamp)

	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}, {Name: "chat_jid"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sender", "content", "timestamp", "is_from_me",
			"media_type", "mime_type", "filename", "url",
			"media_key", "file_sha256", "file_enc_sha256", "file_length",
			"mentioned_jids",
		}),
	}).Create(msg).Error
}

func (s *Store) GetMessage(id, chatJID string) (*Message, error) {
	var msg Message
	if err := s.db.Where("id = ? AND chat_jid = ?", id, chatJID).First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Store) ListMessages(opts ListMessagesOpts) ([]MessageResult, error) {
	var results []MessageResult

	query := s.db.Table("messages").
		Select("messages.*, " +
			"COALESCE(chats.name, ct_chat.full_name, ct_chat.push_name, messages.chat_jid) AS chat_name, " +
			"COALESCE(ct_sender.full_name, ct_sender.push_name, messages.sender) AS sender_name").
		Joins("LEFT JOIN chats ON messages.chat_jid = chats.jid").
		Joins("LEFT JOIN contacts ct_chat ON messages.chat_jid = ct_chat.jid").
		Joins("LEFT JOIN contacts ct_sender ON messages.sender = ct_sender.jid")

	if opts.ChatJID != "" {
		query = query.Where("messages.chat_jid = ?", opts.ChatJID)
	}
	if opts.Sender != "" {
		like := "%" + opts.Sender + "%"
		query = query.Where("messages.sender LIKE ?", like)
	}
	if opts.After != nil {
		query = query.Where("messages.timestamp >= ?", *opts.After)
	}
	if opts.Before != nil {
		query = query.Where("messages.timestamp <= ?", *opts.Before)
	}
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where("messages.content LIKE ?", like)
	}

	query = query.Order("messages.timestamp ASC")

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	query = query.Limit(limit)

	if opts.Page > 1 {
		query = query.Offset((opts.Page - 1) * limit)
	}

	return results, query.Scan(&results).Error
}

func (s *Store) GetMessageContext(id, chatJID string, beforeCount, afterCount int) ([]MessageResult, error) {
	// Get the target message's timestamp
	var target Message
	if err := s.db.Where("id = ? AND chat_jid = ?", id, chatJID).First(&target).Error; err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}

	var results []MessageResult

	query := s.db.Table("messages").
		Select("messages.*, " +
			"COALESCE(chats.name, ct_chat.full_name, ct_chat.push_name, messages.chat_jid) AS chat_name, " +
			"COALESCE(ct_sender.full_name, ct_sender.push_name, messages.sender) AS sender_name").
		Joins("LEFT JOIN chats ON messages.chat_jid = chats.jid").
		Joins("LEFT JOIN contacts ct_chat ON messages.chat_jid = ct_chat.jid").
		Joins("LEFT JOIN contacts ct_sender ON messages.sender = ct_sender.jid").
		Where("messages.chat_jid = ?", chatJID).
		Where("messages.timestamp >= ? AND messages.timestamp <= ?",
			target.Timestamp.Add(-time.Duration(beforeCount)*24*time.Hour),
			target.Timestamp.Add(time.Duration(afterCount)*24*time.Hour)).
		Order("messages.timestamp ASC")

	// Use a subquery approach: get messages by position relative to target
	// Before: messages with timestamp <= target, ordered DESC, limit beforeCount
	// After: messages with timestamp >= target, ordered ASC, limit afterCount
	// Union both

	var before, after []MessageResult

	baseSelect := "messages.*, " +
		"COALESCE(chats.name, ct_chat.full_name, ct_chat.push_name, messages.chat_jid) AS chat_name, " +
		"COALESCE(ct_sender.full_name, ct_sender.push_name, messages.sender) AS sender_name"

	baseJoins := func(q *gorm.DB) *gorm.DB {
		return q.Joins("LEFT JOIN chats ON messages.chat_jid = chats.jid").
			Joins("LEFT JOIN contacts ct_chat ON messages.chat_jid = ct_chat.jid").
			Joins("LEFT JOIN contacts ct_sender ON messages.sender = ct_sender.jid")
	}

	// Before (including target)
	q := s.db.Table("messages").Select(baseSelect).
		Where("messages.chat_jid = ? AND messages.timestamp <= ?", chatJID, target.Timestamp).
		Order("messages.timestamp DESC").
		Limit(beforeCount + 1)
	baseJoins(q).Scan(&before)

	// Reverse the before slice
	for i, j := 0, len(before)-1; i < j; i, j = i+1, j-1 {
		before[i], before[j] = before[j], before[i]
	}

	// After (excluding target to avoid duplicate)
	q = s.db.Table("messages").Select(baseSelect).
		Where("messages.chat_jid = ? AND messages.timestamp > ?", chatJID, target.Timestamp).
		Order("messages.timestamp ASC").
		Limit(afterCount)
	baseJoins(q).Scan(&after)

	// Ignore the broad query variable
	_ = query

	results = append(before, after...)
	return results, nil
}
```

Note: `GetMessageContext` needs `"gorm.io/gorm"` imported for the `*gorm.DB` type in `baseJoins`. Add it to the imports.

- [ ] **Step 4: Add missing import and run tests**

Run: `go test ./internal/store/ -v -run "TestStoreMessage|TestListMessages|TestGetMessageContext"`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/messages.go internal/store/store_test.go
git commit -m "feat: message store operations with joins and context queries"
```

---

### Task 6: Mention Resolution + Tests

**Files:**
- Create: `internal/mention/resolve.go`
- Create: `internal/mention/resolve_test.go`

- [ ] **Step 1: Write mention resolution tests**

```go
// internal/mention/resolve_test.go
package mention

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func mockLookup(jid string) string {
	names := map[string]string{
		"1234567890@s.whatsapp.net": "Alice Smith",
		"9876543210@s.whatsapp.net": "Bob Jones",
		"555@lid":                   "Charlie",
	}
	return names[jid]
}

func TestResolve_SingleMention(t *testing.T) {
	content := "Hey @1234567890 check this out"
	jids := `["1234567890@s.whatsapp.net"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "Hey @Alice Smith check this out", result)
}

func TestResolve_MultipleMentions(t *testing.T) {
	content := "@1234567890 and @9876543210 should see this"
	jids := `["1234567890@s.whatsapp.net", "9876543210@s.whatsapp.net"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "@Alice Smith and @Bob Jones should see this", result)
}

func TestResolve_NoMentions(t *testing.T) {
	content := "Just a regular message"
	result := Resolve(content, "", mockLookup)
	assert.Equal(t, "Just a regular message", result)
}

func TestResolve_NilJIDs(t *testing.T) {
	content := "Message with @1234567890"
	result := Resolve(content, "", mockLookup)
	assert.Equal(t, "Message with @1234567890", result) // no resolution without JID list
}

func TestResolve_UnknownContact(t *testing.T) {
	content := "Hey @9999999999"
	jids := `["9999999999@s.whatsapp.net"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "Hey @9999999999", result) // unchanged if lookup returns empty
}

func TestResolve_LIDMention(t *testing.T) {
	content := "Hey @555"
	jids := `["555@lid"]`

	result := Resolve(content, jids, mockLookup)
	assert.Equal(t, "Hey @Charlie", result)
}

func TestResolve_MalformedJSON(t *testing.T) {
	content := "Hey @1234567890"
	result := Resolve(content, "not-json", mockLookup)
	assert.Equal(t, "Hey @1234567890", result) // graceful fallback
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mention/ -v`
Expected: FAIL — `Resolve` not defined.

- [ ] **Step 3: Implement mention resolution**

```go
// internal/mention/resolve.go
package mention

import (
	"encoding/json"
	"strings"
)

// Resolve replaces @<number> patterns in content with display names
// using the mentioned JIDs list and a lookup function.
// The lookupFn takes a full JID and returns a display name (or empty string).
func Resolve(content, mentionedJIDsJSON string, lookupFn func(jid string) string) string {
	if mentionedJIDsJSON == "" {
		return content
	}

	var jids []string
	if err := json.Unmarshal([]byte(mentionedJIDsJSON), &jids); err != nil {
		return content
	}

	for _, jid := range jids {
		name := lookupFn(jid)
		if name == "" {
			continue
		}

		// Extract user part: "1234567890@s.whatsapp.net" -> "1234567890"
		// or "555@lid" -> "555"
		user := jid
		if idx := strings.Index(jid, "@"); idx > 0 {
			user = jid[:idx]
		}

		content = strings.ReplaceAll(content, "@"+user, "@"+name)
	}

	return content
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mention/ -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mention/resolve.go internal/mention/resolve_test.go
git commit -m "feat: @mention JID to display name resolution"
```

---

### Task 7: WhatsApp Client

**Files:**
- Create: `internal/whatsapp/client.go`

This task has no automated tests — whatsmeow requires a live WhatsApp connection. Manual testing happens when wiring up the CLI commands (Task 14).

- [ ] **Step 1: Create the WhatsApp client wrapper**

```go
// internal/whatsapp/client.go
package whatsapp

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/mattn/go-sqlite3"

	appstore "wabridge/internal/store"
)

type Client struct {
	WAClient *whatsmeow.Client
	Store    *appstore.Store
	Log      waLog.Logger

	mu            sync.Mutex
	syncSettled   chan struct{}
	lastSyncEvent int64
}

func NewClient(sessionDBPath string, appStore *appstore.Store, logLevel string) (*Client, error) {
	dbLog := waLog.Stdout("Database", logLevel, true)
	container, err := sqlstore.New("sqlite3", "file:"+sessionDBPath+"?_foreign_keys=on", dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to open session store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	clientLog := waLog.Stdout("Client", logLevel, true)
	waClient := whatsmeow.NewClient(deviceStore, clientLog)

	return &Client{
		WAClient:    waClient,
		Store:       appStore,
		Log:         clientLog,
		syncSettled: make(chan struct{}),
	}, nil
}

func (c *Client) Connect(ctx context.Context) error {
	if c.WAClient.Store.ID == nil {
		// No session — need QR code pairing
		qrChan, _ := c.WAClient.GetQRChannel(ctx)
		if err := c.WAClient.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}

		for evt := range qrChan {
			switch evt.Event {
			case "code":
				fmt.Println("Scan this QR code with WhatsApp:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stderr)
			case "login":
				c.Log.Infof("Login successful")
				return nil
			case "timeout":
				return fmt.Errorf("QR code timed out")
			}
		}
		return nil
	}

	// Existing session — just connect
	return c.WAClient.Connect()
}

func (c *Client) Disconnect() {
	c.WAClient.Disconnect()
}

func (c *Client) IsConnected() bool {
	return c.WAClient.IsConnected()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build. Some unused imports may need cleanup.

- [ ] **Step 3: Commit**

```bash
git add internal/whatsapp/client.go
git commit -m "feat: WhatsApp client with QR pairing and session management"
```

---

### Task 8: WhatsApp Event Handlers

**Files:**
- Create: `internal/whatsapp/handlers.go`

- [ ] **Step 1: Create event handlers**

```go
// internal/whatsapp/handlers.go
package whatsapp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	appstore "wabridge/internal/store"
)

func (c *Client) RegisterHandlers() {
	c.WAClient.AddEventHandler(c.handleEvent)
}

func (c *Client) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.HistorySync:
		c.handleHistorySync(v)
	case *events.PushName:
		c.handlePushName(v)
	case *events.Contact:
		c.handleContact(v)
	case *events.Connected:
		c.handleConnected()
	case *events.LoggedOut:
		c.Log.Warnf("Logged out — session expired, re-pairing required")
	}
}

func (c *Client) handleMessage(msg *events.Message) {
	chatJID := msg.Info.Chat.String()
	sender := msg.Info.Sender.ToNonAD().String()

	content := extractTextContent(msg.Message)
	mediaType, mimeType, filename := extractMediaInfo(msg.Message)
	mentionedJIDs := extractMentionedJIDs(msg.Message)

	storeMsg := &appstore.Message{
		ID:        msg.Info.ID,
		ChatJID:   chatJID,
		Sender:    sender,
		Content:   content,
		Timestamp: msg.Info.Timestamp,
		IsFromMe:  msg.Info.IsFromMe,
	}

	if mediaType != "" {
		storeMsg.MediaType = &mediaType
		storeMsg.MimeType = &mimeType
		storeMsg.Filename = &filename
	}

	if msg.Message.GetImageMessage() != nil {
		im := msg.Message.GetImageMessage()
		setMediaFields(storeMsg, im.GetURL(), im.GetMediaKey(), im.GetFileSHA256(), im.GetFileEncSHA256(), int64(im.GetFileLength()))
	} else if msg.Message.GetVideoMessage() != nil {
		vm := msg.Message.GetVideoMessage()
		setMediaFields(storeMsg, vm.GetURL(), vm.GetMediaKey(), vm.GetFileSHA256(), vm.GetFileEncSHA256(), int64(vm.GetFileLength()))
	} else if msg.Message.GetAudioMessage() != nil {
		am := msg.Message.GetAudioMessage()
		setMediaFields(storeMsg, am.GetURL(), am.GetMediaKey(), am.GetFileSHA256(), am.GetFileEncSHA256(), int64(am.GetFileLength()))
	} else if msg.Message.GetDocumentMessage() != nil {
		dm := msg.Message.GetDocumentMessage()
		setMediaFields(storeMsg, dm.GetURL(), dm.GetMediaKey(), dm.GetFileSHA256(), dm.GetFileEncSHA256(), int64(dm.GetFileLength()))
	}

	if mentionedJIDs != "" {
		storeMsg.MentionedJIDs = &mentionedJIDs
	}

	if err := c.Store.StoreMessage(storeMsg); err != nil {
		c.Log.Errorf("Failed to store message: %v", err)
		return
	}

	// Update chat name for groups
	if msg.Info.Chat.Server == types.GroupServer {
		c.updateGroupName(msg.Info.Chat, chatJID)
	}

	// Store sender's push name
	if msg.Info.PushName != "" && !msg.Info.IsFromMe {
		c.Store.UpsertContact(&appstore.Contact{
			JID:      sender,
			PushName: strPtr(msg.Info.PushName),
		})
	}
}

func (c *Client) handleHistorySync(evt *events.HistorySync) {
	c.mu.Lock()
	c.lastSyncEvent = time.Now().Unix()
	c.mu.Unlock()

	conversations := evt.Data.GetConversations()
	c.Log.Infof("History sync: %d conversations", len(conversations))

	for _, conv := range conversations {
		chatJID := conv.GetID()
		jid, err := types.ParseJID(chatJID)
		if err != nil {
			c.Log.Warnf("Failed to parse chat JID %s: %v", chatJID, err)
			continue
		}

		// Store chat with name from history sync metadata
		var chatName *string
		if name := conv.GetDisplayName(); name != "" {
			chatName = &name
		} else if name := conv.GetName(); name != "" {
			chatName = &name
		}

		for _, msg := range conv.GetMessages() {
			webMsg := msg.GetMessage()
			if webMsg == nil || webMsg.Message == nil {
				continue
			}

			// ParseWebMessage correctly resolves sender JIDs
			parsed, err := c.WAClient.ParseWebMessage(jid, webMsg)
			if err != nil {
				c.Log.Warnf("Failed to parse history message: %v", err)
				continue
			}

			c.handleMessage(parsed)
		}

		// Update chat name after processing messages (so chat exists)
		if chatName != nil {
			c.Store.UpsertChat(chatJID, chatName, time.Now())
		}
	}

	// Start settle detection (runs once per sync batch)
	go c.detectSyncSettled()
}

func (c *Client) detectSyncSettled() {
	const settleDelay = 15 * time.Second
	time.Sleep(settleDelay)

	c.mu.Lock()
	elapsed := time.Since(time.Unix(c.lastSyncEvent, 0))
	c.mu.Unlock()

	if elapsed >= settleDelay {
		c.Log.Infof("History sync settled, dumping contacts")
		c.dumpContacts()

		select {
		case c.syncSettled <- struct{}{}:
		default:
		}
	}
}

func (c *Client) handlePushName(evt *events.PushName) {
	jid := evt.JID.ToNonAD().String()
	c.Store.UpsertContact(&appstore.Contact{
		JID:      jid,
		PushName: strPtr(evt.Message.GetPushName()),
	})
}

func (c *Client) handleContact(evt *events.Contact) {
	jid := evt.JID.ToNonAD().String()
	c.Store.UpsertContact(&appstore.Contact{
		JID:      jid,
		FullName: strPtr(evt.Contact.GetFullName()),
	})
}

func (c *Client) handleConnected() {
	c.Log.Infof("Connected to WhatsApp")
	go c.dumpContacts()
}

func (c *Client) dumpContacts() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	contacts, err := c.WAClient.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		c.Log.Errorf("Failed to get contacts: %v", err)
		return
	}

	for jid, info := range contacts {
		phoneJID := jid.ToNonAD().String()
		var fullName, pushName *string
		if info.FullName != "" {
			fullName = &info.FullName
		}
		if info.PushName != "" {
			pushName = &info.PushName
		}

		c.Store.UpsertContact(&appstore.Contact{
			JID:      phoneJID,
			FullName: fullName,
			PushName: pushName,
		})

		// Create LID dual-entry
		lidJID, err := c.WAClient.Store.LIDs.GetLIDForPN(ctx, jid)
		if err == nil && !lidJID.IsEmpty() {
			c.Store.UpsertContact(&appstore.Contact{
				JID:      lidJID.String(),
				PhoneJID: &phoneJID,
				FullName: fullName,
				PushName: pushName,
			})
		}
	}

	c.Log.Infof("Dumped %d contacts", len(contacts))
}

func (c *Client) updateGroupName(jid types.JID, chatJID string) {
	// Check if we already have a non-placeholder name
	chat, err := c.Store.GetChat(chatJID)
	if err == nil && chat.Name != nil && *chat.Name != "" && !strings.HasPrefix(*chat.Name, "Group ") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	groupInfo, err := c.WAClient.GetGroupInfo(ctx, jid)
	if err != nil {
		c.Log.Warnf("Failed to get group info for %s: %v", chatJID, err)
		return
	}

	if groupInfo.Name != "" {
		c.Store.UpsertChat(chatJID, &groupInfo.Name, time.Now())
	}
}

// Helper functions

func extractTextContent(msg *events.Message_Message) string {
	if msg == nil {
		return ""
	}
	if msg.GetConversation() != "" {
		return msg.GetConversation()
	}
	if msg.GetExtendedTextMessage() != nil {
		return msg.GetExtendedTextMessage().GetText()
	}
	// Check captions on media messages
	if msg.GetImageMessage() != nil {
		return msg.GetImageMessage().GetCaption()
	}
	if msg.GetVideoMessage() != nil {
		return msg.GetVideoMessage().GetCaption()
	}
	if msg.GetDocumentMessage() != nil {
		return msg.GetDocumentMessage().GetCaption()
	}
	return ""
}

func extractMediaInfo(msg *events.Message_Message) (mediaType, mimeType, filename string) {
	if msg == nil {
		return "", "", ""
	}
	if im := msg.GetImageMessage(); im != nil {
		return "image", im.GetMimetype(), ""
	}
	if vm := msg.GetVideoMessage(); vm != nil {
		return "video", vm.GetMimetype(), ""
	}
	if am := msg.GetAudioMessage(); am != nil {
		return "audio", am.GetMimetype(), ""
	}
	if dm := msg.GetDocumentMessage(); dm != nil {
		return "document", dm.GetMimetype(), dm.GetFileName()
	}
	return "", "", ""
}

func extractMentionedJIDs(msg *events.Message_Message) string {
	if msg == nil {
		return ""
	}
	var contextInfo interface{ GetMentionedJID() []string }
	switch {
	case msg.GetExtendedTextMessage() != nil:
		contextInfo = msg.GetExtendedTextMessage().GetContextInfo()
	case msg.GetImageMessage() != nil:
		contextInfo = msg.GetImageMessage().GetContextInfo()
	case msg.GetVideoMessage() != nil:
		contextInfo = msg.GetVideoMessage().GetContextInfo()
	}

	if contextInfo == nil {
		return ""
	}

	jids := contextInfo.GetMentionedJID()
	if len(jids) == 0 {
		return ""
	}

	data, _ := json.Marshal(jids)
	return string(data)
}

func setMediaFields(msg *appstore.Message, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength int64) {
	if url != "" {
		msg.URL = &url
	}
	msg.MediaKey = mediaKey
	msg.FileSHA256 = fileSHA256
	msg.FileEncSHA256 = fileEncSHA256
	if fileLength > 0 {
		msg.FileLength = &fileLength
	}
}

func strPtr(s string) *string {
	return &s
}
```

**Important:** The `extractTextContent` and `extractMediaInfo` functions reference `events.Message_Message` which is actually the protobuf `Message` type. The exact type path depends on the whatsmeow version. During implementation, check the actual type with `go doc go.mau.fi/whatsmeow/types/events Message` and adjust imports accordingly. The protobuf message type is typically `waProto.Message` from `go.mau.fi/whatsmeow/proto/waE2E`.

- [ ] **Step 2: Verify it compiles (fix type references as needed)**

Run: `go build ./...`
Expected: Clean build. May need to adjust protobuf type imports — replace `events.Message_Message` with the correct type from `go.mau.fi/whatsmeow/proto/waE2E`.

- [ ] **Step 3: Commit**

```bash
git add internal/whatsapp/handlers.go
git commit -m "feat: WhatsApp event handlers for messages, history sync, contacts"
```

---

### Task 9: WhatsApp Media Handling

**Files:**
- Create: `internal/whatsapp/media.go`

- [ ] **Step 1: Implement media download, upload, and Ogg analysis**

```go
// internal/whatsapp/media.go
package whatsapp

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	appstore "wabridge/internal/store"
)

func (c *Client) DownloadMedia(ctx context.Context, msg *appstore.Message, outputDir string) (string, error) {
	if msg.URL == nil || msg.MediaKey == nil {
		return "", fmt.Errorf("message has no downloadable media")
	}

	// Build a downloadable proto message based on media type
	downloadable := buildDownloadable(msg)
	if downloadable == nil {
		return "", fmt.Errorf("unsupported media type: %v", msg.MediaType)
	}

	data, err := c.WAClient.Download(ctx, downloadable)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	filename := fmt.Sprintf("%s_%s", msg.ID, derefOr(msg.Filename, "media"))
	outPath := filepath.Join(outputDir, filename)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return "", err
	}

	return outPath, nil
}

func (c *Client) UploadMedia(ctx context.Context, filePath string, mediaType whatsmeow.MediaType) (*whatsmeow.UploadResponse, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	resp, err := c.WAClient.Upload(ctx, data, mediaType)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	return &resp, nil
}

// AnalyzeOggOpus reads an Ogg Opus file and returns its duration in seconds.
func AnalyzeOggOpus(data []byte) (uint32, error) {
	if len(data) < 4 || string(data[:4]) != "OggS" {
		return 0, fmt.Errorf("not an Ogg file")
	}

	var lastGranule uint64
	pos := 0

	for pos+27 < len(data) {
		if string(data[pos:pos+4]) != "OggS" {
			break
		}

		granule := binary.LittleEndian.Uint64(data[pos+6 : pos+14])
		if granule != 0 && granule != math.MaxUint64 {
			lastGranule = granule
		}

		numSegments := int(data[pos+26])
		if pos+27+numSegments > len(data) {
			break
		}

		pageSize := 27 + numSegments
		for i := 0; i < numSegments; i++ {
			pageSize += int(data[pos+27+i])
		}
		pos += pageSize
	}

	// Opus uses 48kHz sample rate
	duration := uint32(lastGranule / 48000)
	return duration, nil
}

// PlaceholderWaveform generates a synthetic 64-byte waveform for voice messages.
func PlaceholderWaveform(duration uint32) []byte {
	rng := rand.New(rand.NewSource(int64(duration)))
	waveform := make([]byte, 64)
	for i := range waveform {
		// Generate values that look like speech (mostly 20-80 range with occasional peaks)
		base := 20 + rng.Intn(60)
		if rng.Float64() < 0.1 {
			base = 60 + rng.Intn(40)
		}
		waveform[i] = byte(base)
	}
	return waveform
}

func buildDownloadable(msg *appstore.Message) proto.Message {
	mediaType := derefOr(msg.MediaType, "")
	url := derefOr(msg.URL, "")
	fileLen := uint64(0)
	if msg.FileLength != nil {
		fileLen = uint64(*msg.FileLength)
	}

	switch mediaType {
	case "image":
		return &waProto.ImageMessage{
			URL:           &url,
			MediaKey:      msg.MediaKey,
			FileSHA256:    msg.FileSHA256,
			FileEncSHA256: msg.FileEncSHA256,
			FileLength:    &fileLen,
		}
	case "video":
		return &waProto.VideoMessage{
			URL:           &url,
			MediaKey:      msg.MediaKey,
			FileSHA256:    msg.FileSHA256,
			FileEncSHA256: msg.FileEncSHA256,
			FileLength:    &fileLen,
		}
	case "audio":
		return &waProto.AudioMessage{
			URL:           &url,
			MediaKey:      msg.MediaKey,
			FileSHA256:    msg.FileSHA256,
			FileEncSHA256: msg.FileEncSHA256,
			FileLength:    &fileLen,
		}
	case "document":
		return &waProto.DocumentMessage{
			URL:           &url,
			MediaKey:      msg.MediaKey,
			FileSHA256:    msg.FileSHA256,
			FileEncSHA256: msg.FileEncSHA256,
			FileLength:    &fileLen,
		}
	default:
		return nil
	}
}

func derefOr(p *string, fallback string) string {
	if p != nil {
		return *p
	}
	return fallback
}
```

**Important:** The exact protobuf types (`waProto.ImageMessage`, etc.) depend on the whatsmeow version. Check with `go doc go.mau.fi/whatsmeow/proto/waE2E ImageMessage` during implementation and adjust the import path. The whatsmeow `Download` function accepts anything implementing the `DownloadableMessage` interface — verify which protobuf types satisfy it.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build. Adjust protobuf imports as needed.

- [ ] **Step 3: Commit**

```bash
git add internal/whatsapp/media.go
git commit -m "feat: media download/upload and Ogg Opus analysis"
```

---

### Task 10: ActionBackend Interface

**Files:**
- Create: `internal/mcp/backend.go`

- [ ] **Step 1: Define ActionBackend and direct implementation**

```go
// internal/mcp/backend.go
package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"

	appstore "wabridge/internal/store"
	appwa "wabridge/internal/whatsapp"
)

// ActionBackend abstracts actions that require a live WhatsApp connection.
// Implemented by DirectBackend (standalone mode) and api.Client (mcp mode).
type ActionBackend interface {
	SendMessage(ctx context.Context, recipient, text string) error
	SendFile(ctx context.Context, recipient, filePath string) error
	SendAudioMessage(ctx context.Context, recipient, filePath string) error
	DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error)
	RequestHistorySync(ctx context.Context) error
}

// DirectBackend calls whatsmeow directly. Used in standalone mode.
type DirectBackend struct {
	Client   *appwa.Client
	Store    *appstore.Store
	MediaDir string
}

func (b *DirectBackend) SendMessage(ctx context.Context, recipient, text string) error {
	jid, err := types.ParseJID(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient JID: %w", err)
	}

	_, err = b.Client.WAClient.SendMessage(ctx, jid, &waProto.Message{
		Conversation: &text,
	})
	return err
}

func (b *DirectBackend) SendFile(ctx context.Context, recipient, filePath string) error {
	jid, err := types.ParseJID(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient JID: %w", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	mediaType := detectMediaType(filePath)
	resp, err := b.Client.WAClient.Upload(ctx, data, mediaType)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	msg := buildMediaMessage(filePath, data, resp, mediaType)
	_, err = b.Client.WAClient.SendMessage(ctx, jid, msg)
	return err
}

func (b *DirectBackend) SendAudioMessage(ctx context.Context, recipient, filePath string) error {
	jid, err := types.ParseJID(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient JID: %w", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	resp, err := b.Client.WAClient.Upload(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	duration, _ := appwa.AnalyzeOggOpus(data)
	waveform := appwa.PlaceholderWaveform(duration)
	ptt := true
	seconds := duration
	mime := "audio/ogg; codecs=opus"
	fileLen := uint64(len(data))

	_, err = b.Client.WAClient.SendMessage(ctx, jid, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &fileLen,
			Seconds:       &seconds,
			PTT:           &ptt,
			Mimetype:      &mime,
			Waveform:      waveform,
		},
	})
	return err
}

func (b *DirectBackend) DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error) {
	msg, err := b.Store.GetMessage(messageID, chatJID)
	if err != nil {
		return "", fmt.Errorf("message not found: %w", err)
	}

	return b.Client.DownloadMedia(ctx, msg, b.MediaDir)
}

func (b *DirectBackend) RequestHistorySync(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			// BuildHistorySyncRequest can panic — known whatsmeow issue
			fmt.Fprintf(os.Stderr, "panic in BuildHistorySyncRequest: %v\n", r)
		}
	}()

	req, err := b.Client.WAClient.BuildHistorySyncRequest(nil, 50)
	if err != nil {
		return fmt.Errorf("failed to build history sync request: %w", err)
	}

	statusJID := types.JID{User: "status", Server: types.DefaultUserServer}
	_, err = b.Client.WAClient.SendNode(*req)
	_ = statusJID
	return err
}

func detectMediaType(path string) whatsmeow.MediaType {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return whatsmeow.MediaImage
	case ".mp4", ".avi", ".mov", ".mkv":
		return whatsmeow.MediaVideo
	case ".ogg", ".mp3", ".wav", ".m4a":
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

func buildMediaMessage(filePath string, data []byte, resp whatsmeow.UploadResponse, mediaType whatsmeow.MediaType) *waProto.Message {
	mime := "application/octet-stream" // Detect properly in implementation
	filename := filepath.Base(filePath)
	fileLen := uint64(len(data))

	switch mediaType {
	case whatsmeow.MediaImage:
		return &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &fileLen,
				Mimetype:      &mime,
			},
		}
	case whatsmeow.MediaVideo:
		return &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &fileLen,
				Mimetype:      &mime,
			},
		}
	case whatsmeow.MediaAudio:
		return &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &fileLen,
				Mimetype:      &mime,
			},
		}
	default:
		return &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &fileLen,
				Mimetype:      &mime,
				FileName:      &filename,
			},
		}
	}
}
```

**Note:** The `SendNode` and `BuildHistorySyncRequest` APIs may differ across whatsmeow versions. Check the actual API during implementation. The `RequestHistorySync` function includes panic recovery as documented in the knowledge file.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build. Adjust whatsmeow API calls as needed.

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/backend.go
git commit -m "feat: ActionBackend interface with direct WhatsApp implementation"
```

---

### Task 11: MCP Server + Query Tools

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools.go`

- [ ] **Step 1: Create MCP server setup**

```go
// internal/mcp/server.go
package mcp

import (
	appstore "wabridge/internal/store"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcp     *mcpserver.MCPServer
	store   *appstore.Store
	backend ActionBackend
}

func NewServer(store *appstore.Store, backend ActionBackend) *Server {
	s := &Server{
		mcp:     mcpserver.NewMCPServer("wabridge", "1.0.0"),
		store:   store,
		backend: backend,
	}
	s.registerTools()
	return s
}

func (s *Server) ServeStdio() error {
	return mcpserver.ServeStdio(s.mcp)
}
```

- [ ] **Step 2: Create all MCP tool definitions and handlers**

```go
// internal/mcp/tools.go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appstore "wabridge/internal/store"
	"wabridge/internal/mention"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) registerTools() {
	s.registerQueryTools()
	s.registerActionTools()
}

func (s *Server) registerQueryTools() {
	// search_contacts
	s.mcp.AddTool(
		mcplib.NewTool("search_contacts",
			mcplib.WithDescription("Search contacts by name or phone number"),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search term (name or phone number)")),
			mcplib.WithNumber("limit", mcplib.Description("Maximum results to return (default 50)")),
		),
		s.handleSearchContacts,
	)

	// list_chats
	s.mcp.AddTool(
		mcplib.NewTool("list_chats",
			mcplib.WithDescription("List chats sorted by most recent message. Filter by name or JID."),
			mcplib.WithString("filter", mcplib.Description("Filter chats by name or JID (optional)")),
			mcplib.WithNumber("limit", mcplib.Description("Maximum chats to return (default 50)")),
		),
		s.handleListChats,
	)

	// get_chat
	s.mcp.AddTool(
		mcplib.NewTool("get_chat",
			mcplib.WithDescription("Get metadata for a single chat by JID"),
			mcplib.WithString("jid", mcplib.Required(), mcplib.Description("Chat JID")),
		),
		s.handleGetChat,
	)

	// get_direct_chat_by_contact
	s.mcp.AddTool(
		mcplib.NewTool("get_direct_chat_by_contact",
			mcplib.WithDescription("Find a 1:1 chat by phone number"),
			mcplib.WithString("phone", mcplib.Required(), mcplib.Description("Phone number (or partial match)")),
		),
		s.handleGetDirectChatByContact,
	)

	// get_contact_chats
	s.mcp.AddTool(
		mcplib.NewTool("get_contact_chats",
			mcplib.WithDescription("Get all chats involving a specific contact"),
			mcplib.WithString("jid", mcplib.Required(), mcplib.Description("Contact JID")),
			mcplib.WithNumber("limit", mcplib.Description("Maximum chats to return (default 20)")),
		),
		s.handleGetContactChats,
	)

	// list_messages
	s.mcp.AddTool(
		mcplib.NewTool("list_messages",
			mcplib.WithDescription("Query messages with optional filters. Returns messages with resolved sender names."),
			mcplib.WithString("chat_jid", mcplib.Description("Filter by chat JID")),
			mcplib.WithString("sender", mcplib.Description("Filter by sender phone number")),
			mcplib.WithString("after", mcplib.Description("Only messages after this ISO-8601 datetime")),
			mcplib.WithString("before", mcplib.Description("Only messages before this ISO-8601 datetime")),
			mcplib.WithString("search", mcplib.Description("Full-text search on message content")),
			mcplib.WithNumber("limit", mcplib.Description("Maximum messages to return (default 50)")),
			mcplib.WithNumber("page", mcplib.Description("Page number for pagination (default 1)")),
			mcplib.WithBoolean("raw", mcplib.Description("If true, return raw message content without @mention resolution (default false)")),
		),
		s.handleListMessages,
	)

	// get_last_interaction
	s.mcp.AddTool(
		mcplib.NewTool("get_last_interaction",
			mcplib.WithDescription("Get the most recent message with a specific contact"),
			mcplib.WithString("jid", mcplib.Required(), mcplib.Description("Contact JID")),
			mcplib.WithBoolean("raw", mcplib.Description("If true, return raw message content without @mention resolution")),
		),
		s.handleGetLastInteraction,
	)

	// get_message_context
	s.mcp.AddTool(
		mcplib.NewTool("get_message_context",
			mcplib.WithDescription("Get messages surrounding a specific message (before and after)"),
			mcplib.WithString("message_id", mcplib.Required(), mcplib.Description("Target message ID")),
			mcplib.WithString("chat_jid", mcplib.Required(), mcplib.Description("Chat JID the message belongs to")),
			mcplib.WithNumber("before", mcplib.Description("Number of messages before (default 5)")),
			mcplib.WithNumber("after", mcplib.Description("Number of messages after (default 5)")),
			mcplib.WithBoolean("raw", mcplib.Description("If true, return raw message content without @mention resolution")),
		),
		s.handleGetMessageContext,
	)
}

func (s *Server) registerActionTools() {
	// send_message
	s.mcp.AddTool(
		mcplib.NewTool("send_message",
			mcplib.WithDescription("Send a text message to a WhatsApp chat"),
			mcplib.WithString("recipient", mcplib.Required(), mcplib.Description("Recipient JID (e.g. 1234567890@s.whatsapp.net or group@g.us)")),
			mcplib.WithString("message", mcplib.Required(), mcplib.Description("Message text to send")),
		),
		s.handleSendMessage,
	)

	// send_file
	s.mcp.AddTool(
		mcplib.NewTool("send_file",
			mcplib.WithDescription("Send a file (image, video, document) to a WhatsApp chat"),
			mcplib.WithString("recipient", mcplib.Required(), mcplib.Description("Recipient JID")),
			mcplib.WithString("file_path", mcplib.Required(), mcplib.Description("Path to the file to send")),
		),
		s.handleSendFile,
	)

	// send_audio_message
	s.mcp.AddTool(
		mcplib.NewTool("send_audio_message",
			mcplib.WithDescription("Send an audio file as a WhatsApp voice message (PTT). File should be Ogg Opus format."),
			mcplib.WithString("recipient", mcplib.Required(), mcplib.Description("Recipient JID")),
			mcplib.WithString("file_path", mcplib.Required(), mcplib.Description("Path to the audio file (Ogg Opus format)")),
		),
		s.handleSendAudioMessage,
	)

	// download_media
	s.mcp.AddTool(
		mcplib.NewTool("download_media",
			mcplib.WithDescription("Download a media attachment from a message"),
			mcplib.WithString("message_id", mcplib.Required(), mcplib.Description("Message ID")),
			mcplib.WithString("chat_jid", mcplib.Required(), mcplib.Description("Chat JID")),
		),
		s.handleDownloadMedia,
	)

	// request_history_sync
	s.mcp.AddTool(
		mcplib.NewTool("request_history_sync",
			mcplib.WithDescription("Request older message history from WhatsApp. Results arrive asynchronously."),
		),
		s.handleRequestHistorySync,
	)
}

// --- Query tool handlers ---

func (s *Server) handleSearchContacts(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	query := req.GetString("query", "")
	limit := int(req.GetFloat64("limit", 50))

	contacts, err := s.store.SearchContacts(query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return jsonResult(contacts)
}

func (s *Server) handleListChats(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	filter := req.GetString("filter", "")
	limit := int(req.GetFloat64("limit", 50))

	chats, err := s.store.ListChats(filter, limit)
	if err != nil {
		return nil, fmt.Errorf("list chats failed: %w", err)
	}

	return jsonResult(chats)
}

func (s *Server) handleGetChat(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	jid := req.GetString("jid", "")

	chat, err := s.store.GetChat(jid)
	if err != nil {
		return nil, fmt.Errorf("chat not found: %w", err)
	}

	return jsonResult(chat)
}

func (s *Server) handleGetDirectChatByContact(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	phone := req.GetString("phone", "")

	chats, err := s.store.ListChats(phone, 1)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Filter to non-group chats containing the phone number
	for _, chat := range chats {
		if strings.HasSuffix(chat.JID, "@s.whatsapp.net") && strings.Contains(chat.JID, phone) {
			return jsonResult(chat)
		}
	}

	return mcplib.NewToolResultText("No direct chat found for this contact"), nil
}

func (s *Server) handleGetContactChats(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	jid := req.GetString("jid", "")
	limit := int(req.GetFloat64("limit", 20))

	chats, err := s.store.GetContactChats(jid, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get contact chats: %w", err)
	}

	return jsonResult(chats)
}

func (s *Server) handleListMessages(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	opts := appstore.ListMessagesOpts{
		ChatJID: req.GetString("chat_jid", ""),
		Sender:  req.GetString("sender", ""),
		Search:  req.GetString("search", ""),
		Limit:   int(req.GetFloat64("limit", 50)),
		Page:    int(req.GetFloat64("page", 1)),
	}

	if after := req.GetString("after", ""); after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err != nil {
			return nil, fmt.Errorf("invalid 'after' datetime: %w", err)
		}
		opts.After = &t
	}
	if before := req.GetString("before", ""); before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return nil, fmt.Errorf("invalid 'before' datetime: %w", err)
		}
		opts.Before = &t
	}

	results, err := s.store.ListMessages(opts)
	if err != nil {
		return nil, fmt.Errorf("list messages failed: %w", err)
	}

	raw := req.GetBool("raw", false)
	if !raw {
		s.resolveMentions(results)
	}

	return jsonResult(results)
}

func (s *Server) handleGetLastInteraction(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	jid := req.GetString("jid", "")

	results, err := s.store.ListMessages(appstore.ListMessagesOpts{
		Sender: jid,
		Limit:  1,
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if len(results) == 0 {
		return mcplib.NewToolResultText("No messages found"), nil
	}

	raw := req.GetBool("raw", false)
	if !raw {
		s.resolveMentions(results)
	}

	return jsonResult(results[0])
}

func (s *Server) handleGetMessageContext(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	messageID := req.GetString("message_id", "")
	chatJID := req.GetString("chat_jid", "")
	before := int(req.GetFloat64("before", 5))
	after := int(req.GetFloat64("after", 5))

	results, err := s.store.GetMessageContext(messageID, chatJID, before, after)
	if err != nil {
		return nil, fmt.Errorf("get context failed: %w", err)
	}

	raw := req.GetBool("raw", false)
	if !raw {
		s.resolveMentions(results)
	}

	return jsonResult(results)
}

// --- Action tool handlers ---

func (s *Server) handleSendMessage(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	recipient := req.GetString("recipient", "")
	message := req.GetString("message", "")

	if err := s.backend.SendMessage(ctx, recipient, message); err != nil {
		return nil, fmt.Errorf("send failed: %w", err)
	}

	return mcplib.NewToolResultText("Message sent successfully"), nil
}

func (s *Server) handleSendFile(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	recipient := req.GetString("recipient", "")
	filePath := req.GetString("file_path", "")

	if err := s.backend.SendFile(ctx, recipient, filePath); err != nil {
		return nil, fmt.Errorf("send file failed: %w", err)
	}

	return mcplib.NewToolResultText("File sent successfully"), nil
}

func (s *Server) handleSendAudioMessage(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	recipient := req.GetString("recipient", "")
	filePath := req.GetString("file_path", "")

	if err := s.backend.SendAudioMessage(ctx, recipient, filePath); err != nil {
		return nil, fmt.Errorf("send audio failed: %w", err)
	}

	return mcplib.NewToolResultText("Audio message sent successfully"), nil
}

func (s *Server) handleDownloadMedia(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	messageID := req.GetString("message_id", "")
	chatJID := req.GetString("chat_jid", "")

	path, err := s.backend.DownloadMedia(ctx, messageID, chatJID)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	return jsonResult(map[string]string{
		"path":    path,
		"message": "Media downloaded successfully",
	})
}

func (s *Server) handleRequestHistorySync(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if err := s.backend.RequestHistorySync(ctx); err != nil {
		return nil, fmt.Errorf("history sync request failed: %w", err)
	}

	return mcplib.NewToolResultText("History sync requested. Messages will arrive asynchronously."), nil
}

// --- Helpers ---

func (s *Server) resolveMentions(results []appstore.MessageResult) {
	for i := range results {
		if results[i].MentionedJIDs != nil {
			results[i].Content = mention.Resolve(
				results[i].Content,
				*results[i].MentionedJIDs,
				s.store.GetContactName,
			)
		}
	}
}

func jsonResult(v interface{}) (*mcplib.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return mcplib.NewToolResultText(string(data)), nil
}
```

- [ ] **Step 3: Add missing store method `GetContactChats`**

This method is referenced by `handleGetContactChats` but not yet implemented. Add to `internal/store/messages.go`:

```go
func (s *Store) GetContactChats(contactJID string, limit int) ([]ChatResult, error) {
	var results []ChatResult

	err := s.db.Table("chats").
		Select("DISTINCT chats.*, "+
			"COALESCE(chats.name, contacts.full_name, contacts.push_name, chats.jid) AS display_name").
		Joins("LEFT JOIN contacts ON chats.jid = contacts.jid").
		Joins("INNER JOIN messages ON messages.chat_jid = chats.jid").
		Where("messages.sender = ? OR messages.chat_jid LIKE ?", contactJID, "%"+contactJID+"%").
		Order("chats.last_message_time DESC").
		Limit(limit).
		Scan(&results).Error

	return results, err
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/tools.go internal/store/messages.go
git commit -m "feat: MCP server with all 13 tool definitions and handlers"
```

---

### Task 12: REST API Server

**Files:**
- Create: `internal/api/server.go`

- [ ] **Step 1: Implement REST API server**

```go
// internal/api/server.go
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	appmcp "wabridge/internal/mcp"
)

type APIServer struct {
	backend appmcp.ActionBackend
	addr    string
}

func NewAPIServer(backend appmcp.ActionBackend, addr string) *APIServer {
	return &APIServer{backend: backend, addr: addr}
}

func (s *APIServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/send", s.handleSend)
	mux.HandleFunc("POST /api/send-file", s.handleSendFile)
	mux.HandleFunc("POST /api/send-audio", s.handleSendAudio)
	mux.HandleFunc("POST /api/download", s.handleDownload)
	mux.HandleFunc("POST /api/sync-history", s.handleSyncHistory)

	fmt.Printf("REST API listening on %s\n", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

type apiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Message: "ok"})
}

func (s *APIServer) handleSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"`
		Message   string `json:"message"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Message: err.Error()})
		return
	}

	if err := s.backend.SendMessage(r.Context(), req.Recipient, req.Message); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Message: "sent"})
}

func (s *APIServer) handleSendFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"`
		FilePath  string `json:"file_path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Message: err.Error()})
		return
	}

	if err := s.backend.SendFile(r.Context(), req.Recipient, req.FilePath); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Message: "file sent"})
}

func (s *APIServer) handleSendAudio(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"`
		FilePath  string `json:"file_path"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Message: err.Error()})
		return
	}

	if err := s.backend.SendAudioMessage(r.Context(), req.Recipient, req.FilePath); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Message: "audio sent"})
}

func (s *APIServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MessageID string `json:"message_id"`
		ChatJID   string `json:"chat_jid"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Message: err.Error()})
		return
	}

	path, err := s.backend.DownloadMedia(r.Context(), req.MessageID, req.ChatJID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Message: "downloaded",
		Data:    map[string]string{"path": path},
	})
}

func (s *APIServer) handleSyncHistory(w http.ResponseWriter, r *http.Request) {
	if err := s.backend.RequestHistorySync(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Message: "history sync requested"})
}

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/api/server.go
git commit -m "feat: REST API server with health check and action endpoints"
```

---

### Task 13: REST API Client

**Files:**
- Create: `internal/api/client.go`

- [ ] **Step 1: Implement REST API client that satisfies ActionBackend**

```go
// internal/api/client.go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIClient implements mcp.ActionBackend by calling the bridge REST API.
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

func (c *APIClient) SendMessage(ctx context.Context, recipient, text string) error {
	return c.post(ctx, "/api/send", map[string]string{
		"recipient": recipient,
		"message":   text,
	})
}

func (c *APIClient) SendFile(ctx context.Context, recipient, filePath string) error {
	return c.post(ctx, "/api/send-file", map[string]string{
		"recipient": recipient,
		"file_path": filePath,
	})
}

func (c *APIClient) SendAudioMessage(ctx context.Context, recipient, filePath string) error {
	return c.post(ctx, "/api/send-audio", map[string]string{
		"recipient": recipient,
		"file_path": filePath,
	})
}

func (c *APIClient) DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error) {
	body, err := c.postWithResponse(ctx, "/api/download", map[string]string{
		"message_id": messageID,
		"chat_jid":   chatJID,
	})
	if err != nil {
		return "", err
	}

	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}

	if data, ok := resp.Data.(map[string]interface{}); ok {
		if path, ok := data["path"].(string); ok {
			return path, nil
		}
	}

	return "", fmt.Errorf("unexpected response format")
}

func (c *APIClient) RequestHistorySync(ctx context.Context) error {
	return c.post(ctx, "/api/sync-history", nil)
}

func (c *APIClient) post(ctx context.Context, path string, payload any) error {
	_, err := c.postWithResponse(ctx, path, payload)
	return err
}

func (c *APIClient) postWithResponse(ctx context.Context, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to bridge failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var apiResp apiResponse
		json.Unmarshal(respBody, &apiResp)
		return nil, fmt.Errorf("bridge returned %d: %s", resp.StatusCode, apiResp.Message)
	}

	return respBody, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/api/client.go
git commit -m "feat: REST API client implementing ActionBackend for mcp mode"
```

---

### Task 14: Standalone Command

**Files:**
- Create: `cmd/standalone.go`

- [ ] **Step 1: Implement standalone subcommand**

```go
// cmd/standalone.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"wabridge/internal/mcp"
	"wabridge/internal/store"
	"wabridge/internal/whatsapp"
)

var (
	sessionDB string
	mediaDir  string
)

var standaloneCmd = &cobra.Command{
	Use:   "standalone",
	Short: "All-in-one mode: WhatsApp connection + MCP server in one process",
	RunE:  runStandalone,
}

func init() {
	standaloneCmd.Flags().StringVar(&sessionDB, "session-db", "whatsapp.db", "path to whatsmeow session database")
	standaloneCmd.Flags().StringVar(&mediaDir, "media-dir", "media", "directory for downloaded media files")
	rootCmd.AddCommand(standaloneCmd)
}

func runStandalone(cmd *cobra.Command, args []string) error {
	// Initialize app store
	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer appStore.Close()

	// Initialize WhatsApp client
	waClient, err := whatsapp.NewClient(sessionDB, appStore, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp client: %w", err)
	}
	waClient.RegisterHandlers()

	// Connect to WhatsApp
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := waClient.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to WhatsApp: %w", err)
	}
	defer waClient.Disconnect()

	// Create direct backend (calls whatsmeow directly)
	absMediaDir, _ := filepath.Abs(mediaDir)
	backend := &mcp.DirectBackend{
		Client:   waClient,
		Store:    appStore,
		MediaDir: absMediaDir,
	}

	// Start MCP server on stdio
	mcpServer := mcp.NewServer(appStore, backend)

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "Shutting down...")
		cancel()
		waClient.Disconnect()
		os.Exit(0)
	}()

	return mcpServer.ServeStdio()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add cmd/standalone.go
git commit -m "feat: standalone command — all-in-one WhatsApp + MCP server"
```

---

### Task 15: Bridge Command

**Files:**
- Create: `cmd/bridge.go`

- [ ] **Step 1: Implement bridge subcommand**

```go
// cmd/bridge.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/mcp"
	"wabridge/internal/store"
	"wabridge/internal/whatsapp"
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
	bridgeCmd.Flags().StringVar(&bridgeAddr, "addr", ":8080", "REST API listen address")
	bridgeCmd.Flags().StringVar(&bridgeSessionDB, "session-db", "whatsapp.db", "path to whatsmeow session database")
	bridgeCmd.Flags().StringVar(&bridgeMediaDir, "media-dir", "media", "directory for downloaded media files")
	rootCmd.AddCommand(bridgeCmd)
}

func runBridge(cmd *cobra.Command, args []string) error {
	// Initialize app store
	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer appStore.Close()

	// Initialize WhatsApp client
	waClient, err := whatsapp.NewClient(bridgeSessionDB, appStore, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp client: %w", err)
	}
	waClient.RegisterHandlers()

	// Connect to WhatsApp
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := waClient.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to WhatsApp: %w", err)
	}
	defer waClient.Disconnect()

	// Create direct backend for REST API to use
	absMediaDir, _ := filepath.Abs(bridgeMediaDir)
	backend := &mcp.DirectBackend{
		Client:   waClient,
		Store:    appStore,
		MediaDir: absMediaDir,
	}

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "Shutting down bridge...")
		cancel()
		waClient.Disconnect()
		os.Exit(0)
	}()

	// Start REST API server (blocks)
	apiServer := api.NewAPIServer(backend, bridgeAddr)
	return apiServer.Start()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add cmd/bridge.go
git commit -m "feat: bridge command — persistent daemon with REST API"
```

---

### Task 16: MCP Command

**Files:**
- Create: `cmd/mcp.go`

- [ ] **Step 1: Implement mcp subcommand**

```go
// cmd/mcp.go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"wabridge/internal/api"
	"wabridge/internal/mcp"
	"wabridge/internal/store"
)

var (
	bridgeURL string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Ephemeral MCP stdio server (reads SQLite, calls bridge REST API for actions)",
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&bridgeURL, "bridge-url", "http://localhost:8080", "bridge REST API base URL")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	// Initialize app store (read-only queries)
	appStore, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer appStore.Close()

	// Create API client backend (calls bridge REST API for actions)
	backend := api.NewAPIClient(bridgeURL)

	// Start MCP server on stdio
	mcpServer := mcp.NewServer(appStore, backend)
	return mcpServer.ServeStdio()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 3: Verify the full binary runs**

Run: `go run . --help`
Expected: Shows `standalone`, `bridge`, and `mcp` subcommands.

Run: `go run . standalone --help`
Expected: Shows `--db`, `--session-db`, `--media-dir` flags.

- [ ] **Step 4: Commit**

```bash
git add cmd/mcp.go
git commit -m "feat: mcp command — ephemeral stdio server with bridge REST backend"
```

---

### Task 17: Dockerfile + Docker Compose

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`

- [ ] **Step 1: Create multi-stage Dockerfile**

```dockerfile
# Dockerfile
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /wabridge .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates ffmpeg

COPY --from=builder /wabridge /usr/local/bin/wabridge

WORKDIR /app/store

ENTRYPOINT ["wabridge"]
```

- [ ] **Step 2: Create docker-compose.yml**

```yaml
# docker-compose.yml
services:
  bridge:
    build: .
    command: ["bridge", "--db", "/app/store/messages.db", "--session-db", "/app/store/whatsapp.db", "--media-dir", "/app/store/media"]
    restart: unless-stopped
    volumes:
      - store:/app/store
    networks:
      - wabridge-net

  mcp:
    build: .
    command: ["mcp", "--db", "/app/store/messages.db", "--bridge-url", "http://bridge:8080"]
    stdin_open: true
    volumes:
      - store:/app/store
    networks:
      - wabridge-net
    depends_on:
      - bridge
    profiles:
      - mcp

  standalone:
    build: .
    command: ["standalone", "--db", "/app/store/messages.db", "--session-db", "/app/store/whatsapp.db", "--media-dir", "/app/store/media"]
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

- [ ] **Step 3: Verify Docker build works**

Run: `docker build -t wabridge .`
Expected: Successful image build.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: Dockerfile and docker-compose with bridge, mcp, standalone services"
```

---

### Task 18: Documentation

**Files:**
- Create: `AGENTS.md`
- Create: `docs/ARCHITECTURE.md`
- Create: `docs/SCHEMA.md`
- Create: `docs/MCP_TOOLS.md`
- Create: `docs/REST_API.md`
- Create: `docs/WHATSAPP_QUIRKS.md`

- [ ] **Step 1: Create AGENTS.md**

```markdown
# wabridge

WhatsApp MCP bridge — single Go binary that connects to WhatsApp and serves MCP tools.

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
```

- [ ] **Step 2: Create docs/ARCHITECTURE.md**

Content: expand the Architecture section from the design spec. Cover the three modes, data flow diagrams, and how packages connect. Follow progressive disclosure — summary at top, details below.

- [ ] **Step 3: Create docs/SCHEMA.md**

Content: expand the Database Schema section from the design spec. Include all three tables with column details, JID format reference, name resolution query, and contact dual-entry strategy.

- [ ] **Step 4: Create docs/MCP_TOOLS.md**

Content: list all 13 tools with their parameters (name, type, required/optional, description) and example responses. Group into query tools and action tools.

- [ ] **Step 5: Create docs/REST_API.md**

Content: document all 6 REST endpoints with method, path, request body, and response format. Include curl examples.

- [ ] **Step 6: Create docs/WHATSAPP_QUIRKS.md**

Content: distill `whatsapp-bridge-knowledge.md` into a maintained reference. Cover JID formats, ParseWebMessage, history sync, session expiry, media handling, and known whatsmeow issues.

- [ ] **Step 7: Commit**

```bash
git add AGENTS.md docs/
git commit -m "docs: AGENTS.md and reference documentation"
```

---

## Dependency Summary

```
Task  1: scaffolding
Task  2: models + store init          (depends on 1)
Task  3: chat ops + tests             (depends on 2)
Task  4: contact ops + tests          (depends on 2)
Task  5: message ops + tests          (depends on 2)
Task  6: mention resolution + tests   (depends on nothing)
Task  7: WhatsApp client              (depends on 2)
Task  8: WhatsApp event handlers      (depends on 3, 4, 5, 7)
Task  9: WhatsApp media               (depends on 7)
Task 10: ActionBackend                 (depends on 7, 9)
Task 11: MCP server + tools           (depends on 3, 4, 5, 6, 10)
Task 12: REST API server              (depends on 10)
Task 13: REST API client              (depends on 12)
Task 14: standalone command            (depends on 8, 11)
Task 15: bridge command                (depends on 8, 12)
Task 16: mcp command                   (depends on 11, 13)
Task 17: Docker                        (depends on 14, 15, 16)
Task 18: docs                          (depends on nothing, can run in parallel)
```

Tasks 3, 4, 5, 6 can run in parallel.
Tasks 7, 8, 9 are sequential.
Tasks 12, 13 are sequential.
Tasks 14, 15, 16 can run in parallel after their deps.
Task 18 can run anytime.
