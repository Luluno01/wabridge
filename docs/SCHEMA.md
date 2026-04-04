# Database Schema

wabridge uses a single SQLite database (`messages.db`) managed by GORM with auto-migration. Three tables store all application data.

## Tables

### chats

Tracks known conversations. Auto-created when the first message in a chat is stored.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `jid` | TEXT | PRIMARY KEY | Chat JID (`xxx@s.whatsapp.net`, `xxx@lid`, `xxx@g.us`) |
| `name` | TEXT | nullable | Display name; set for groups, NULL for 1:1 chats |
| `last_message_time` | TIMESTAMP | indexed | Most recent message timestamp in this chat |

### contacts

Stores contact name information. Each person may have two rows (see Dual-Entry Strategy below).

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `jid` | TEXT | PRIMARY KEY | Phone JID or LID JID |
| `phone_jid` | TEXT | nullable, indexed | Cross-reference: LID rows point to the phone JID |
| `push_name` | TEXT | nullable | Name set by the contact (transient, can change) |
| `full_name` | TEXT | nullable | Name from address book sync (stable) |
| `updated_at` | TIMESTAMP | | Last upsert time |

### messages

Stores message content and media metadata. Media bytes are not stored -- only enough metadata to download on demand.

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `id` | TEXT | PK (composite) | WhatsApp message ID |
| `chat_jid` | TEXT | PK (composite), indexed | Chat this message belongs to |
| `sender` | TEXT | NOT NULL, indexed | Sender JID in `ToNonAD()` form |
| `content` | TEXT | | Text content (empty for media-only) |
| `timestamp` | TIMESTAMP | NOT NULL, indexed | Message timestamp |
| `is_from_me` | BOOLEAN | NOT NULL | Whether the message was sent by the bridge user |
| `media_type` | TEXT | nullable | `image`, `video`, `audio`, `document`, `sticker`, or NULL |
| `mime_type` | TEXT | nullable | MIME type string |
| `filename` | TEXT | nullable | Original filename |
| `url` | TEXT | nullable | WhatsApp CDN URL |
| `media_key` | BLOB | nullable | Decryption key for media download |
| `file_sha256` | BLOB | nullable | SHA-256 of decrypted file |
| `file_enc_sha256` | BLOB | nullable | SHA-256 of encrypted file |
| `file_length` | INTEGER | nullable | File size in bytes |
| `mentioned_jids` | TEXT | nullable | JSON array of JIDs mentioned in the message (e.g., `["123@s.whatsapp.net"]`) |

## JID Format Reference

| Format | Example | Meaning |
|--------|---------|---------|
| Phone JID | `1234567890@s.whatsapp.net` | Individual chat, phone-number-based |
| LID JID | `18273648@lid` | Individual chat, opaque server-assigned ID |
| Group JID | `120363012345678901@g.us` | Group chat |

JIDs are always stored in "non-AD" form (`ToNonAD().String()`), which strips the `:device` suffix to produce a canonical identifier that matches across contexts. See [WHATSAPP_QUIRKS.md](WHATSAPP_QUIRKS.md) for behavioral context on JID migration and `ParseWebMessage`.

## Name Resolution Strategy

Names are resolved at query time, never stored alongside messages. This keeps names current when contacts update their profiles.

### COALESCE Chain

For chat names:
```sql
COALESCE(chats.name, ct_chat.full_name, ct_chat.push_name, messages.chat_jid)
```

For sender names:
```sql
COALESCE(ct_sender.full_name, ct_sender.push_name, messages.sender)
```

### Dual JOINs

Group messages need two contact lookups -- one for the chat name and one for the sender name:

```sql
LEFT JOIN contacts ct_chat   ON messages.chat_jid = ct_chat.jid
LEFT JOIN contacts ct_sender ON messages.sender   = ct_sender.jid
```

## Contact Dual-Entry Strategy

WhatsApp is migrating from phone-based JIDs to LID-based JIDs. The same person can appear as either format. To handle this, each contact gets two rows:

1. **Phone row**: `{jid: "1234567890@s.whatsapp.net", phone_jid: NULL, push_name: "Alice", full_name: "Alice Smith"}`
2. **LID row**: `{jid: "18273648@lid", phone_jid: "1234567890@s.whatsapp.net", push_name: "Alice", full_name: "Alice Smith"}`

The LID-to-phone mapping comes from whatsmeow's `client.Store.LIDs.GetLIDForPN()`. See [WHATSAPP_QUIRKS.md — Phone-to-LID Migration](WHATSAPP_QUIRKS.md#phone-to-lid-migration) for why this is necessary.

## Contact Upsert Behavior

`UpsertContact` only overwrites non-empty fields. If a contact already exists, only fields with non-empty new values are updated. This prevents a push-name update from wiping a previously stored full name (or vice versa).

```go
if contact.PushName != nil && *contact.PushName != "" {
    updates["push_name"] = *contact.PushName
}
```
