# Quoted Message References Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store quoted-message metadata (reply-to ID, sender, content snapshot, media type) so MCP consumers can see reply relationships.

**Architecture:** Add 4 nullable columns to the `messages` table. Replace `extractMentionedJIDs` with a unified `extractContextInfo` that returns all ContextInfo-derived fields (mentions + quoted info) from a single type-switch. `buildMessage` populates the new fields. MCP tools expose them automatically via the embedded `Message` struct.

**Tech Stack:** Go, GORM (AutoMigrate), whatsmeow proto (`waE2E.ContextInfo`), SQLite

---

### Task 1: Add quoted fields to Message model

**Files:**
- Modify: `internal/store/models.go:19-35`

- [ ] **Step 1: Add the four new fields to the Message struct**

Add after the `MentionedJIDs` field (line 34):

```go
QuotedMessageID *string `gorm:"column:quoted_message_id" json:"quoted_message_id,omitempty"`
QuotedSender    *string `gorm:"column:quoted_sender" json:"quoted_sender,omitempty"`
QuotedContent   *string `gorm:"column:quoted_content" json:"quoted_content,omitempty"`
QuotedMediaType *string `gorm:"column:quoted_media_type" json:"quoted_media_type,omitempty"`
```

- [ ] **Step 2: Run build to verify compilation**

Run: `go build ./...`
Expected: success (GORM AutoMigrate will add the columns at runtime)

- [ ] **Step 3: Commit**

```bash
git add internal/store/models.go
git commit -m "feat: add quoted message fields to Message model"
```

---

### Task 2: Add quoted fields to StoreMessage upsert

**Files:**
- Modify: `internal/store/messages.go:46-51`

- [ ] **Step 1: Write a test that stores a message with quoted fields and verifies round-trip**

Add to `internal/store/store_test.go`:

```go
func TestStoreMessage_QuotedFields(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:              "reply1",
		ChatJID:         "chat@g.us",
		Sender:          "alice@s.whatsapp.net",
		Content:         "I agree!",
		Timestamp:       time.Now().Truncate(time.Second),
		QuotedMessageID: strPtr("orig1"),
		QuotedSender:    strPtr("bob@s.whatsapp.net"),
		QuotedContent:   strPtr("What do you think?"),
	}

	err := s.StoreMessage(msg)
	require.NoError(t, err)

	got, err := s.GetMessage("reply1", "chat@g.us")
	require.NoError(t, err)
	require.NotNil(t, got.QuotedMessageID)
	assert.Equal(t, "orig1", *got.QuotedMessageID)
	require.NotNil(t, got.QuotedSender)
	assert.Equal(t, "bob@s.whatsapp.net", *got.QuotedSender)
	require.NotNil(t, got.QuotedContent)
	assert.Equal(t, "What do you think?", *got.QuotedContent)
	assert.Nil(t, got.QuotedMediaType)
}

func TestStoreMessage_QuotedFieldsUpsert(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:        "reply1",
		ChatJID:   "chat@g.us",
		Sender:    "alice@s.whatsapp.net",
		Content:   "I agree!",
		Timestamp: time.Now().Truncate(time.Second),
	}
	require.NoError(t, s.StoreMessage(msg))

	// Re-store with quoted fields populated (simulates re-sync)
	msg.QuotedMessageID = strPtr("orig1")
	msg.QuotedSender = strPtr("bob@s.whatsapp.net")
	msg.QuotedContent = strPtr("What do you think?")
	msg.QuotedMediaType = strPtr("image")
	require.NoError(t, s.StoreMessage(msg))

	got, err := s.GetMessage("reply1", "chat@g.us")
	require.NoError(t, err)
	require.NotNil(t, got.QuotedMessageID)
	assert.Equal(t, "orig1", *got.QuotedMessageID)
	require.NotNil(t, got.QuotedMediaType)
	assert.Equal(t, "image", *got.QuotedMediaType)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/store/ -run TestStoreMessage_Quoted -v`
Expected: FAIL — `QuotedMessageID` will be nil after round-trip because the upsert's `DoUpdates` list doesn't include the new columns yet.

- [ ] **Step 3: Add the four columns to the upsert's DoUpdates list**

In `internal/store/messages.go`, change the `DoUpdates` clause (line 46-51) from:

```go
DoUpdates: clause.AssignmentColumns([]string{
    "sender", "content", "timestamp", "is_from_me",
    "media_type", "mime_type", "filename", "url",
    "media_key", "file_sha256", "file_enc_sha256", "file_length",
    "mentioned_jids",
}),
```

to:

```go
DoUpdates: clause.AssignmentColumns([]string{
    "sender", "content", "timestamp", "is_from_me",
    "media_type", "mime_type", "filename", "url",
    "media_key", "file_sha256", "file_enc_sha256", "file_length",
    "mentioned_jids",
    "quoted_message_id", "quoted_sender", "quoted_content", "quoted_media_type",
}),
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/store/ -run TestStoreMessage_Quoted -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/messages.go internal/store/store_test.go
git commit -m "feat: include quoted fields in message upsert"
```

---

### Task 3: Replace extractMentionedJIDs with extractContextInfo

**Files:**
- Modify: `internal/whatsapp/handlers.go:491-523` (replace `extractMentionedJIDs`)
- Modify: `internal/whatsapp/handlers.go:362-409` (update `buildMessage`)

- [ ] **Step 1: Write tests for extractContextInfo**

Create `internal/whatsapp/handlers_test.go`:

```go
package whatsapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestExtractContextInfo_NoContextInfo(t *testing.T) {
	msg := &waE2E.Message{
		Conversation: proto.String("plain text"),
	}
	result := extractContextInfo(msg)
	assert.Empty(t, result.MentionedJIDs)
	assert.Empty(t, result.QuotedMessageID)
	assert.Empty(t, result.QuotedSender)
	assert.Empty(t, result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_NilMessage(t *testing.T) {
	result := extractContextInfo(nil)
	assert.Empty(t, result.MentionedJIDs)
	assert.Empty(t, result.QuotedMessageID)
}

func TestExtractContextInfo_QuotedTextMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("I agree"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String("original-msg-id"),
				Participant: proto.String("bob@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("What do you think?"),
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "original-msg-id", result.QuotedMessageID)
	assert.Equal(t, "bob@s.whatsapp.net", result.QuotedSender)
	assert.Equal(t, "What do you think?", result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_QuotedImageMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("Nice photo!"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String("img-msg-id"),
				Participant: proto.String("alice@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					ImageMessage: &waE2E.ImageMessage{
						Caption: proto.String("sunset"),
					},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "img-msg-id", result.QuotedMessageID)
	assert.Equal(t, "sunset", result.QuotedContent)
	assert.Equal(t, "image", result.QuotedMediaType)
}

func TestExtractContextInfo_QuotedVideoMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("LOL"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("vid-msg-id"),
				QuotedMessage: &waE2E.Message{
					VideoMessage: &waE2E.VideoMessage{
						Caption: proto.String("funny clip"),
					},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "funny clip", result.QuotedContent)
	assert.Equal(t, "video", result.QuotedMediaType)
}

func TestExtractContextInfo_QuotedAudioMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("heard it"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("aud-msg-id"),
				QuotedMessage: &waE2E.Message{
					AudioMessage: &waE2E.AudioMessage{},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "audio", result.QuotedMediaType)
	assert.Empty(t, result.QuotedContent)
}

func TestExtractContextInfo_QuotedDocumentMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("thanks for the doc"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("doc-msg-id"),
				QuotedMessage: &waE2E.Message{
					DocumentMessage: &waE2E.DocumentMessage{
						Caption: proto.String("quarterly report"),
					},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "document", result.QuotedMediaType)
	assert.Equal(t, "quarterly report", result.QuotedContent)
}

func TestExtractContextInfo_QuotedStickerMessage(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("haha"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID: proto.String("stk-msg-id"),
				QuotedMessage: &waE2E.Message{
					StickerMessage: &waE2E.StickerMessage{},
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "sticker", result.QuotedMediaType)
	assert.Empty(t, result.QuotedContent)
}

func TestExtractContextInfo_MentionsPreserved(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("Hey @alice"),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: []string{"alice@s.whatsapp.net"},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Contains(t, result.MentionedJIDs, "alice@s.whatsapp.net")
	assert.Empty(t, result.QuotedMessageID)
}

func TestExtractContextInfo_MentionsAndQuotedTogether(t *testing.T) {
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("@alice I agree with Bob"),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: []string{"alice@s.whatsapp.net"},
				StanzaID:     proto.String("bob-msg"),
				Participant:  proto.String("bob@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("Let's do it"),
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Contains(t, result.MentionedJIDs, "alice@s.whatsapp.net")
	assert.Equal(t, "bob-msg", result.QuotedMessageID)
	assert.Equal(t, "bob@s.whatsapp.net", result.QuotedSender)
	assert.Equal(t, "Let's do it", result.QuotedContent)
	assert.Empty(t, result.QuotedMediaType)
}

func TestExtractContextInfo_ImageMessageWithContextInfo(t *testing.T) {
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String("replying with a photo"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:    proto.String("orig-id"),
				Participant: proto.String("sender@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("send me a pic"),
				},
			},
		},
	}
	result := extractContextInfo(msg)
	assert.Equal(t, "orig-id", result.QuotedMessageID)
	assert.Equal(t, "send me a pic", result.QuotedContent)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/whatsapp/ -run TestExtractContextInfo -v`
Expected: FAIL — `extractContextInfo` does not exist yet.

- [ ] **Step 3: Implement extractContextInfo and the quotedMediaType helper**

Replace `extractMentionedJIDs` (lines 491-523) in `internal/whatsapp/handlers.go` with:

```go
type contextInfoResult struct {
	MentionedJIDs   string
	QuotedMessageID string
	QuotedSender    string
	QuotedContent   string
	QuotedMediaType string
}

// extractContextInfo extracts all ContextInfo-derived fields from a message:
// mentioned JIDs, quoted message ID, quoted sender, quoted content snapshot,
// and quoted media type. Single type-switch avoids duplicating the ContextInfo lookup.
func extractContextInfo(msg *waE2E.Message) contextInfoResult {
	if msg == nil {
		return contextInfoResult{}
	}

	var ctx *waE2E.ContextInfo

	if ext := msg.GetExtendedTextMessage(); ext != nil {
		ctx = ext.GetContextInfo()
	} else if img := msg.GetImageMessage(); img != nil {
		ctx = img.GetContextInfo()
	} else if vid := msg.GetVideoMessage(); vid != nil {
		ctx = vid.GetContextInfo()
	} else if doc := msg.GetDocumentMessage(); doc != nil {
		ctx = doc.GetContextInfo()
	} else if aud := msg.GetAudioMessage(); aud != nil {
		ctx = aud.GetContextInfo()
	} else if stk := msg.GetStickerMessage(); stk != nil {
		ctx = stk.GetContextInfo()
	}

	if ctx == nil {
		return contextInfoResult{}
	}

	var result contextInfoResult

	// Mentions
	if jids := ctx.GetMentionedJID(); len(jids) > 0 {
		data, _ := json.Marshal(jids)
		result.MentionedJIDs = string(data)
	}

	// Quoted message
	if stanzaID := ctx.GetStanzaID(); stanzaID != "" {
		result.QuotedMessageID = stanzaID
		result.QuotedSender = ctx.GetParticipant()

		if qm := ctx.GetQuotedMessage(); qm != nil {
			result.QuotedContent = extractTextContent(qm)
			result.QuotedMediaType = quotedMediaType(qm)
		}
	}

	return result
}

// quotedMediaType returns the media type string for a quoted message,
// or "" if it's a text-only message.
func quotedMediaType(msg *waE2E.Message) string {
	switch {
	case msg.GetImageMessage() != nil:
		return "image"
	case msg.GetVideoMessage() != nil:
		return "video"
	case msg.GetAudioMessage() != nil:
		return "audio"
	case msg.GetDocumentMessage() != nil:
		return "document"
	case msg.GetStickerMessage() != nil:
		return "sticker"
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/whatsapp/ -run TestExtractContextInfo -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/whatsapp/handlers.go internal/whatsapp/handlers_test.go
git commit -m "feat: add extractContextInfo replacing extractMentionedJIDs"
```

---

### Task 4: Wire extractContextInfo into buildMessage

**Files:**
- Modify: `internal/whatsapp/handlers.go:362-409` (update `buildMessage`)

- [ ] **Step 1: Update buildMessage to call extractContextInfo and populate quoted fields**

In `internal/whatsapp/handlers.go`, replace the `buildMessage` function (lines 362-410):

```go
func (c *Client) buildMessage(id types.MessageID, chatJID, sender string, msg *waE2E.Message, ts time.Time, isFromMe bool) *appstore.Message {
	content := extractTextContent(msg)
	mediaType, mimeType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg, ts)
	ctxInfo := extractContextInfo(msg)

	if content == "" && mediaType == "" {
		return nil
	}

	storeMsg := &appstore.Message{
		ID:        string(id),
		ChatJID:   chatJID,
		Sender:    sender,
		Content:   content,
		Timestamp: ts,
		IsFromMe:  isFromMe,
	}

	if mediaType != "" {
		storeMsg.MediaType = strPtr(mediaType)
	}
	if mimeType != "" {
		storeMsg.MimeType = strPtr(mimeType)
	}
	if filename != "" {
		storeMsg.Filename = strPtr(filename)
	}
	if url != "" {
		storeMsg.URL = strPtr(url)
	}
	if mediaKey != nil {
		storeMsg.MediaKey = mediaKey
	}
	if fileSHA256 != nil {
		storeMsg.FileSHA256 = fileSHA256
	}
	if fileEncSHA256 != nil {
		storeMsg.FileEncSHA256 = fileEncSHA256
	}
	if fileLength > 0 {
		fl := int64(fileLength)
		storeMsg.FileLength = &fl
	}
	if ctxInfo.MentionedJIDs != "" {
		storeMsg.MentionedJIDs = strPtr(ctxInfo.MentionedJIDs)
	}
	if ctxInfo.QuotedMessageID != "" {
		storeMsg.QuotedMessageID = strPtr(ctxInfo.QuotedMessageID)
	}
	if ctxInfo.QuotedSender != "" {
		storeMsg.QuotedSender = strPtr(ctxInfo.QuotedSender)
	}
	if ctxInfo.QuotedContent != "" {
		storeMsg.QuotedContent = strPtr(ctxInfo.QuotedContent)
	}
	if ctxInfo.QuotedMediaType != "" {
		storeMsg.QuotedMediaType = strPtr(ctxInfo.QuotedMediaType)
	}

	return storeMsg
}
```

- [ ] **Step 2: Delete the old extractMentionedJIDs function**

If it still exists after Task 3, ensure it's fully removed. The only caller was `buildMessage`, which now uses `extractContextInfo`.

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: PASS (all existing tests + new tests)

- [ ] **Step 4: Commit**

```bash
git add internal/whatsapp/handlers.go
git commit -m "feat: wire extractContextInfo into buildMessage"
```

---

### Task 5: Update backlog and verify MCP exposure

**Files:**
- Modify: `docs/backlogs/index.md:7`

- [ ] **Step 1: Verify quoted fields appear in ListMessages JSON output**

Add to `internal/store/store_test.go`:

```go
func TestListMessages_IncludesQuotedFields(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	s.StoreMessage(&Message{
		ID:        "orig1",
		ChatJID:   "chat@g.us",
		Sender:    "bob@s.whatsapp.net",
		Content:   "What do you think?",
		Timestamp: base,
	})
	s.StoreMessage(&Message{
		ID:              "reply1",
		ChatJID:         "chat@g.us",
		Sender:          "alice@s.whatsapp.net",
		Content:         "I agree!",
		Timestamp:       base.Add(time.Minute),
		QuotedMessageID: strPtr("orig1"),
		QuotedSender:    strPtr("bob@s.whatsapp.net"),
		QuotedContent:   strPtr("What do you think?"),
	})

	results, err := s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// First message (orig) has no quoted fields
	assert.Nil(t, results[0].QuotedMessageID)

	// Second message (reply) has quoted fields
	require.NotNil(t, results[1].QuotedMessageID)
	assert.Equal(t, "orig1", *results[1].QuotedMessageID)
	require.NotNil(t, results[1].QuotedSender)
	assert.Equal(t, "bob@s.whatsapp.net", *results[1].QuotedSender)
	require.NotNil(t, results[1].QuotedContent)
	assert.Equal(t, "What do you think?", *results[1].QuotedContent)
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/store/ -run TestListMessages_IncludesQuotedFields -v`
Expected: PASS (fields flow through via embedded `Message` struct in `MessageResult`)

- [ ] **Step 3: Mark backlog item as done**

In `docs/backlogs/index.md`, change line 7 from:

```
| [Quoted message references](../backlogs/2026-03-04-quoted-message.md) | Store reply-to metadata from ContextInfo so we can definitively link replies to their parent messages |
```

to:

```
| ~~[Quoted message references](../backlogs/2026-03-04-quoted-message.md)~~ | Done — `quoted_message_id`, `quoted_sender`, `quoted_content`, `quoted_media_type` extracted from ContextInfo |
```

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/store_test.go docs/backlogs/index.md
git commit -m "feat: verify quoted fields in query results, mark backlog done"
```
