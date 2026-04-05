# Store quoted message references

**Status:** Done — see [design spec](../superpowers/specs/2026-04-05-quoted-message-references-design.md)

## Problem

WhatsApp messages can be replies to other messages. The protobuf includes this in `ContextInfo.StanzaID` (the ID of the quoted message), `ContextInfo.participant` (the quoted sender), and `ContextInfo.QuotedMessage` (content snapshot). Without storing this metadata, reply relationships are invisible to MCP consumers.

## Solution

Four nullable columns on `messages`, mirroring the shape of a regular message:

- `quoted_message_id TEXT` — the ID of the message being replied to
- `quoted_sender TEXT` — JID of the person who sent the quoted message
- `quoted_content TEXT` — text snapshot via `extractTextContent(QuotedMessage)`
- `quoted_media_type TEXT` — media type if the quoted message was media (image/video/audio/document/sticker)

Extracted via `extractContextInfo`, which replaced `extractMentionedJIDs` to avoid duplicating the ContextInfo type-switch. Both mentions and quoted fields are now extracted in a single pass.

## Why snapshot the content

The quoted content in the protobuf is a point-in-time copy. Storing it means we can show what was quoted even if the original message was deleted or not in our database (e.g., older than our sync window).

## What we skip

- Download-related fields (URL, media key, SHA256) — quoted media snapshots aren't independently downloadable
- `quoted_sender_name` resolved field — consumers can cross-reference via `search_contacts`
- `remoteJID` — the chat where the quoted message lives; redundant for same-chat replies
