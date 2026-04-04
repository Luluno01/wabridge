# Store grouped message ID for album association

## Problem

When a user sends multiple photos/videos at once, WhatsApp creates separate messages for each but links them via `ContextInfo.GroupedMessageID`. Our store treats them as unrelated messages, so MCP consumers (e.g., an LLM) see 3 separate image messages with no indication they were sent as a batch.

## Proposed solution

Add a `grouped_message_id` column to the `messages` table. Extract from `ContextInfo.GroupedMessageID` during message handling (both real-time and history sync).

### Schema change

```sql
ALTER TABLE messages ADD COLUMN grouped_message_id TEXT;
CREATE INDEX idx_messages_grouped ON messages(grouped_message_id) WHERE grouped_message_id IS NOT NULL;
```

### MCP tool changes

- `list_messages` and `get_message_context` results should include `grouped_message_id` when present
- Consumers can use this to group consecutive media messages into a single logical album

## Why this matters

An LLM reading message history currently sees 3 separate image messages with no context that they form an album. With the grouped ID, it can reason about them as "user sent 3 photos together" rather than "user sent 3 separate messages".
