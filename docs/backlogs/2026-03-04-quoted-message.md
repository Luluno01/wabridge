# Store quoted message references

## Problem

WhatsApp messages can be replies to other messages. The protobuf includes this in `ContextInfo.QuotedMessage` and `ContextInfo.StanzaID` (the ID of the quoted message). Currently we don't store this metadata, so we can only infer reply relationships from message ordering — which is unreliable.

## Proposed solution

Add two columns to the `messages` table:

- `quoted_message_id TEXT` — the ID of the message being replied to
- `quoted_content TEXT` — snapshot of the quoted message content (WhatsApp includes this in the protobuf)

Extract from `ContextInfo` during message handling (both real-time and history sync). The `StanzaID` field gives the referenced message ID, and `QuotedMessage` gives the content snapshot.

## MCP tool changes

- `list_messages` and `get_message_context` results should include `quoted_message_id` when present
- Consider a `quoted_sender_name` resolved field for convenience

## Why snapshot the content

The quoted content in the protobuf is a point-in-time copy. Storing it means we can show what was quoted even if the original message was deleted or not in our database (e.g., older than our sync window).
