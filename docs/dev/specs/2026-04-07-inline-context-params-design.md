# Inline context params — Design spec

## Summary

Add `context_before` and `context_after` parameters to `list_messages` so
each query can return additional messages at the edges of a time window,
avoiding separate `get_message_context` round trips.

## Scope

Edge extension only — context messages are prepended/appended to the flat
result array when a time window (`after`/`before`) is active. Sparse-match
context (surrounding each individual search/sender hit) is out of scope.

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `context_before` | number | 0 | Messages to include before the `after` boundary |
| `context_after` | number | 0 | Messages to include after the `before` boundary |

Both are capped at 20. Values above 20 are clamped silently.

### Preconditions

- **Requires `chat_jid`** — context across chats is meaningless. Return an
  error if `context_before > 0` or `context_after > 0` without `chat_jid`.
- **Requires a time window** — `context_before` requires `after` to be set;
  `context_after` requires `before` to be set. Ignored (no-op) when the
  corresponding boundary is absent.

## Response shape

The response remains a flat `[]MessageResult` array in chronological order.
A new field marks edge messages:

```go
type MessageResult struct {
    Message
    ChatName   string `json:"chat_name"`
    SenderName string `json:"sender_name"`
    IsContext  bool   `json:"is_context,omitempty"`
}
```

- Context messages: `"is_context": true`
- Matched messages: field omitted (`omitempty` on `false`)

No change to the response shape when `context_before` and `context_after`
are both 0 (the default).

## Store layer changes

### `ListMessagesOpts`

```go
type ListMessagesOpts struct {
    // ...existing fields...
    ContextBefore int
    ContextAfter  int
}
```

### `ListMessages()` logic

After the existing query returns matched messages:

1. **Context before** — if `ContextBefore > 0` and `After != nil`:
   query messages in the same `chat_jid` with `timestamp < after`, ordered
   `DESC`, limit `ContextBefore`. Reverse. Mark `IsContext = true`. Prepend.

2. **Context after** — if `ContextAfter > 0` and `Before != nil`:
   query messages in the same `chat_jid` with `timestamp > before`, ordered
   `ASC`, limit `ContextAfter`. Mark `IsContext = true`. Append.

Both queries reuse `messageSelect` / `messageJoins` for consistent name
resolution.

`limit` and `page` apply to matched messages only.

## MCP tool registration

Add two parameters to the `list_messages` tool:

```go
mcplib.WithNumber("context_before", mcplib.Description("Number of messages to include before the time window (requires chat_jid and after)"))
mcplib.WithNumber("context_after", mcplib.Description("Number of messages to include after the time window (requires chat_jid and before)"))
```

Handler extracts them as ints (default 0), clamps to [0, 20], populates
`ListMessagesOpts.ContextBefore` / `.ContextAfter`.

Validation: if either value > 0 and `chat_jid` is empty, return an error.

## Documentation updates

- **`docs/ops/MCP_TOOLS.md`** — add `context_before` and `context_after`
  rows to the `list_messages` parameter table. Note the `is_context` field
  in the returns description.
- **`docs/dev/backlogs/index.md`** — strike through the inline context
  params entry.

## Test plan

Extend `TestListMessages` in `store_test.go`:

1. **Context before with time window** — 10 messages, query middle 5 with
   `context_before: 2`. Expect 7 messages, first 2 marked `IsContext`.
2. **Context after with time window** — same setup, `context_after: 2`.
   Expect 7 messages, last 2 marked `IsContext`.
3. **Both context before and after** — `context_before: 2, context_after: 2`.
   Expect 9 messages, edges marked.
4. **Context without time window** — `context_before: 2` with no `after`.
   Expect context params ignored, normal results.
5. **Fewer available than requested** — request `context_before: 5` but only
   2 messages exist before the window. Expect 2 context messages.
6. **Context with `latest=true`** — verify ordering is correct (context
   before still appears at the start of the array after reversal).
