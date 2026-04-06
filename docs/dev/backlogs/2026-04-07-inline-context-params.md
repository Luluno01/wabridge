# Inline context on `list_messages`

## Summary

Add optional `context_before` and `context_after` parameters to
`list_messages`, so each matched message is returned alongside surrounding
messages from the same chat.

## Motivation

Automated briefing agents that scan chats within a time window need
surrounding context (the message being replied to, the line that started a
discussion) to produce useful summaries. Without inline context the only
option is one `get_message_context` call per message — 30-80 extra round
trips for a typical briefing run.

## Proposed interface

```
list_messages(
  ...existing params...
  context_before: number   # messages to include before each match (default 0)
  context_after:  number   # messages to include after each match (default 0)
)
```

- Context messages outside the `after`/`before` window are expected.
- `limit` applies to **matched messages only**, not context messages.
- Context messages should be clearly distinguishable from matched messages
  (e.g. `is_context: true` or a nested `context` array).

## Typical call

```
list_messages(
  chat_jid: "...",
  after: "2026-04-06T12:00:00Z",
  before: "2026-04-07T08:30:00Z",
  context_before: 2,
  context_after: 0,
  limit: 50
)
```

Returns the 50 most recent in-window messages, each with the 2 preceding
messages for conversational context.
