# Cookbook

Common workflows and recipes for using wabridge MCP tools. See [MCP_TOOLS.md](MCP_TOOLS.md) for the full parameter reference.

## Daily Briefing

Get a summary of messages from the last 24 hours.

**Step 1:** List recent messages across all chats.

```
list_messages(after: "2026-04-05T00:00:00Z", latest: true, limit: 100)
```

**Step 2:** Or scope to a specific chat by name:

```
list_messages(chat_name: "Project Team", after: "2026-04-05T00:00:00Z")
```

`chat_name` resolves the display name to a JID automatically. If multiple chats match, it prefers an exact name match; otherwise it returns the candidates so you can use `chat_jid` instead.

**Tip:** Use `latest: true` to get the most recent messages first, which is usually what you want for a briefing.

## Find a Contact and Their Messages

**Step 1:** Search for the contact.

```
search_contacts(query: "Alice")
```

This returns JIDs, phone JIDs, and names.

**Step 2:** Get their most recent message.

```
get_last_interaction(jid: "1234567890@s.whatsapp.net")
```

**Step 3:** Or list all their messages in a time range.

```
list_messages(sender: "1234567890@s.whatsapp.net", after: "2026-04-01T00:00:00Z")
```

## Find Which Chats a Contact Participates In

```
get_contact_chats(jid: "1234567890@s.whatsapp.net")
```

Returns all chats where this contact has sent at least one message — useful for finding group chats.

## Read a Conversation Thread

**Step 1:** Find a specific message (e.g., via search).

```
list_messages(chat_jid: "120363012345678901@g.us", search: "deployment", limit: 5)
```

**Step 2:** Get surrounding context for a message of interest.

```
get_message_context(message_id: "3EB0ABC123", chat_jid: "120363012345678901@g.us", before: 10, after: 10)
```

This returns messages before and after the target message in chronological order.

## Search Messages

Substring search across all chats (case-insensitive for ASCII characters):

```
list_messages(search: "quarterly report", limit: 20)
```

Narrow by chat and time:

```
list_messages(chat_jid: "120363012345678901@g.us", search: "deadline", after: "2026-04-01T00:00:00Z")
```

> **Note:** The following recipes require specific access levels. `download_media` needs level 1+, `request_history_sync` needs level 2+, and send tools need level 3 (full access). See [MCP_TOOLS.md](MCP_TOOLS.md#access-levels).

## Download Media

When a message contains media (image, video, audio, document), the message object includes `media_type` and `filename` fields but the actual file is not stored locally until you download it.

**Step 1:** Find a message with media.

```
list_messages(chat_jid: "1234567890@s.whatsapp.net", limit: 10)
```

Look for messages where `media_type` is non-null.

**Step 2:** Download it.

```
download_media(message_id: "3EB0ABC123", chat_jid: "1234567890@s.whatsapp.net")
```

Returns the local file path. The file is saved as `<media_dir>/<chat_jid>/<message_id>.<ext>`. Repeated calls for the same message return the cached file without re-downloading.

## Send a Message

```
send_message(recipient: "1234567890@s.whatsapp.net", message: "Hello!")
```

The recipient can be a full JID (`xxx@s.whatsapp.net`) or a phone number. For groups, use the group JID (`xxx@g.us`).

## Send a File

```
send_file(recipient: "1234567890@s.whatsapp.net", file_path: "/path/to/report.pdf")
```

The media type (image, video, audio, document) is auto-detected from the file extension.

## Send a Voice Message

```
send_audio_message(recipient: "1234567890@s.whatsapp.net", file_path: "/path/to/voice.ogg")
```

The file must be Ogg Opus encoded. WhatsApp displays it as a push-to-talk voice message.

## Pagination

`list_messages` supports pagination for large result sets:

```
list_messages(chat_jid: "120363012345678901@g.us", limit: 50, page: 1)
list_messages(chat_jid: "120363012345678901@g.us", limit: 50, page: 2)
```

Pages are 1-indexed. Combine with `after`/`before` filters to bound the query.

## Working with JIDs

WhatsApp uses JIDs (Jabber IDs) to identify chats and contacts:

| Format | Example | Meaning |
|--------|---------|---------|
| `xxx@s.whatsapp.net` | `1234567890@s.whatsapp.net` | Individual (phone-based) |
| `xxx@lid` | `18273648@lid` | Individual (server-assigned, newer) |
| `xxx@g.us` | `120363012345678901@g.us` | Group chat |

You typically don't need to construct JIDs manually — use `search_contacts` or `list_chats` to find them, then pass them to other tools.
