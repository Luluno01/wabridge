# Fix request_history_sync to pass a valid message cursor

## Problem

`BuildHistorySyncRequest` requires a `*types.MessageInfo` pointing to the oldest known message as a cursor. We're passing `nil`, which causes a guaranteed nil pointer panic. The panic recovery catches it, but the tool never works.

## Root cause

Not a whatsmeow bug. The function signature is:

```go
func (cli *Client) BuildHistorySyncRequest(lastKnownMessageInfo *types.MessageInfo, count int) *waE2E.Message
```

It dereferences `lastKnownMessageInfo` immediately (accessing `.Chat`, `.ID`, `.IsFromMe`, `.Timestamp`) with no nil check. Our code passes `nil`.

## Proposed fix

Query the DB for the oldest message in a target chat, build a `types.MessageInfo` from it, and pass that as the cursor. This would fetch `count` messages before that cursor.

The tool's API would need to change — either accept a `chat_jid` parameter to specify which chat to fetch older history for, or iterate across all chats.

## Caveat

The knowledge doc notes this API "doesn't reliably fetch history beyond the initial automatic sync." Even with a valid cursor, WhatsApp may not return older messages. The initial sync on login is the primary source of history. This fix may have limited practical value.

## Version info

We're on whatsmeow `v0.0.0-20260327181659-02ec817e7cf4` (latest as of 2026-04-03). No upstream fix or nil guard exists.
