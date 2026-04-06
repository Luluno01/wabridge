# MCP Tools

wabridge exposes 13 MCP tools over stdio. They are split into query tools (read from SQLite) and action tools (require a live WhatsApp connection).

All query tools that return messages resolve @mentions and contact names automatically unless `raw: true` is passed.

---

## Query Tools

### search_contacts

Search contacts by name, phone number, or JID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Search query string |
| `limit` | number | no | Maximum results (default 20) |

Returns: array of contact objects (`jid`, `phone_jid`, `push_name`, `full_name`).

### list_chats

List chats, optionally filtered by name or JID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter` | string | no | Filter chats by name or JID |
| `limit` | number | no | Maximum results (0 = unlimited) |

Returns: array of chat objects with `display_name` resolved via contact lookup.

### get_chat

Get a specific chat by JID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `jid` | string | yes | Chat JID |

Returns: single chat object.

### get_direct_chat_by_contact

Find a direct (non-group) chat by phone number.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `phone` | string | yes | Phone number to search for |

Returns: matching chat object, or text error if not found.

### get_contact_chats

List chats that a contact has participated in.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `jid` | string | yes | Contact JID |
| `limit` | number | no | Maximum results (default 20) |

Returns: array of chat objects where the contact appears as a sender.

### list_messages

List messages with filtering options. The primary query tool.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat_jid` | string | no | Filter by chat JID |
| `sender` | string | no | Filter by sender JID (exact match) |
| `after` | string | no | Only messages after this time (RFC 3339) |
| `before` | string | no | Only messages before this time (RFC 3339) |
| `search` | string | no | Search message content (substring match) |
| `limit` | number | no | Maximum results (default 50) |
| `page` | number | no | Page number for pagination |
| `raw` | boolean | no | If true, skip mention resolution |
| `latest` | boolean | no | If true, return most recent messages first (default false) |

Returns: array of message objects with `chat_name` and `sender_name` resolved. Reply messages include `quoted_message_id`, `quoted_sender`, `quoted_content`, and optionally `quoted_media_type` — see [SCHEMA.md](../dev/SCHEMA.md) for details.

### get_last_interaction

Get the most recent message sent by a contact.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `jid` | string | yes | Contact JID |
| `raw` | boolean | no | If true, skip mention resolution |

Returns: single message object (the most recent one from that sender).

### get_message_context

Get messages surrounding a specific message for context.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `message_id` | string | yes | Message ID |
| `chat_jid` | string | yes | Chat JID the message belongs to |
| `before` | number | no | Number of messages before (default 10) |
| `after` | number | no | Number of messages after (default 10) |
| `raw` | boolean | no | If true, skip mention resolution |

Returns: array of messages in chronological order, centered around the target message.

---

## Action Tools

Action tools require a live WhatsApp connection. In bridge+mcp mode, the MCP server delegates these to the bridge process over REST — see [ARCHITECTURE.md](../dev/ARCHITECTURE.md) and [REST_API.md](REST_API.md) for details.

### send_message

Send a text message to a recipient.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `recipient` | string | yes | Recipient JID or phone number |
| `message` | string | yes | Message text to send |

Returns: confirmation text.

### send_file

Send a file as a media message (image, video, audio, or document).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `recipient` | string | yes | Recipient JID or phone number |
| `file_path` | string | yes | Path to the file to send |

Returns: confirmation text. Media type is auto-detected from file extension.

### send_audio_message

Send an audio file as a WhatsApp voice message (push-to-talk).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `recipient` | string | yes | Recipient JID or phone number |
| `file_path` | string | yes | Path to the Ogg Opus audio file |

Returns: confirmation text. The file must be Ogg Opus encoded.

### download_media

Download media from a stored message.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `message_id` | string | yes | Message ID |
| `chat_jid` | string | yes | Chat JID the message belongs to |

Returns: path to the downloaded file on disk.

### request_history_sync

Request additional message history from the primary WhatsApp device for a specific chat. Uses the oldest stored message in the chat as the cursor.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chat_jid` | string | yes | JID of the chat to request older history for |

Returns: confirmation text. History arrives asynchronously via WhatsApp events. Requires at least one stored message in the target chat to build a valid cursor.

**Limitation:** This uses WhatsApp's `HISTORY_SYNC_ON_DEMAND` peer message, which asks the primary phone to send older messages. The phone can ignore or decline the request, and in practice it rarely responds. This is a known issue across WhatsApp libraries (not specific to wabridge). The most reliable way to get message history is the initial sync that happens automatically when pairing the device. To re-sync, unlink and re-pair.
