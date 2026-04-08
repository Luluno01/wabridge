# Chat name resolution for `list_messages`

## Summary

Allow `list_messages` to accept human-readable chat display names (not just JIDs), so consumers can skip the manual `list_chats` lookup step. Also improve `list_chats` with search/filter and pagination support.

## Motivation

A typical consumer workflow for pulling messages from human-designated chats is:

1. Call `list_chats` (returns the full, unpaginated list)
2. Manually scan through the results to find the matching chat by display name
3. Extract the JID
4. Call `list_messages` with that JID

This is friction-heavy — `list_chats` returns everything with no search or pagination, forcing the consumer to do its own filtering over a potentially long list. For an agent pulling messages from chats specified by a human (who knows names, not JIDs), this round-trip and manual lookup is wasteful.

## Possible approaches

### A. Add `chat_name` parameter to `list_messages`

Accept a display name string, resolve it to a JID internally, and query messages. Ambiguous matches (multiple chats with similar names) would need a defined resolution strategy (exact match first, then error or return candidates).

### B. Improve `list_chats` filtering

Add a `search` parameter (substring/fuzzy match on display name) and `page`/`limit` pagination to `list_chats`, so the consumer can efficiently narrow down chats without reading the full list.

### C. Both

Combine A and B — better `list_chats` for exploration, plus direct name resolution on `list_messages` for the common case.

## Notes

- `list_chats` already has a `filter` parameter but it is a simple exact/substring match on name or JID, with no pagination.
- Display names come from contact lookup and can change over time; JIDs are stable identifiers.
- Group chats can have identical display names — need to handle ambiguity.
