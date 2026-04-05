# Quoted message references — design spec

## Problem

WhatsApp messages can reply to other messages. The protobuf includes this in `ContextInfo.stanzaID` (quoted message ID), `ContextInfo.participant` (quoted sender), and `ContextInfo.quotedMessage` (content snapshot). We don't store any of this, so reply relationships are invisible to MCP consumers.

## Design

### Schema

Four nullable columns on `messages`, mirroring the shape of a regular message:

| Column | Type | Source |
|--------|------|--------|
| `quoted_message_id` | TEXT | `ContextInfo.stanzaID` |
| `quoted_sender` | TEXT | `ContextInfo.participant` |
| `quoted_content` | TEXT | `extractTextContent(ContextInfo.quotedMessage)` |
| `quoted_media_type` | TEXT | media type from `ContextInfo.quotedMessage` (image/video/audio/document/sticker) |

No index needed — these fields are read with the message, not queried independently. GORM AutoMigrate adds the columns.

### Extraction

Replace `extractMentionedJIDs` with `extractContextInfo` returning a struct:

```go
type contextInfoResult struct {
    MentionedJIDs   string
    QuotedMessageID string
    QuotedSender    string
    QuotedContent   string
    QuotedMediaType string
}
```

Single type-switch walks ExtendedText, Image, Video, Document, Audio, Sticker to find `ContextInfo`. From that one `ContextInfo`:
- `GetMentionedJID()` — same as today
- `GetStanzaID()` — quoted message ID
- `GetParticipant()` — quoted sender JID
- `GetQuotedMessage()` — run `extractTextContent` for content, check media presence for type

This eliminates the duplicated type-switch that would occur with a separate function.

### Store changes

- `Message` model gets four new `*string` fields with `json:"...,omitempty"` tags
- `StoreMessage` upsert's `DoUpdates` list includes the four new columns
- `buildMessage` calls `extractContextInfo` instead of `extractMentionedJIDs`

### MCP tool exposure

`MessageResult` embeds `Message`. The new fields appear automatically in JSON output from `list_messages`, `get_message_context`, and `get_last_interaction`. No tool registration changes needed.

Quoted sender names are resolved at query time via the existing `ct_sender` JOIN pattern — no additional JOIN needed since the quoted sender JID is just a data field, not resolved to a display name in the query. Consumers can cross-reference with `search_contacts` if needed.

### What we skip

- Download-related fields (URL, media key, SHA256) — quoted media snapshots aren't independently downloadable
- Quoted thumbnails — binary data, not useful for text-based MCP consumers
- `remoteJID` — the chat where the quoted message lives; redundant since cross-chat quoting is rare and consumers have the chat context already

## Applies to

Both real-time messages and history sync — `buildMessage` is the shared path for both.
