# Dedicated search for `list_chats`

## Summary

Add a `search` parameter to `list_chats` that matches only on display name (not JID), complementing the existing `filter` parameter which matches across all fields including JID.

## Motivation

The current `filter` parameter does `LIKE %term%` across `chats.name`, `contacts.full_name`, `contacts.push_name`, and `chats.jid`. This is useful for general lookup but can return false positives when a search term happens to match part of a JID. A dedicated `search` parameter that only matches on resolved display names would give consumers a cleaner way to find chats by name.

## Notes

- `filter` should remain as-is for backward compatibility — it is useful for JID-based lookups.
- Consider whether `search` should use the same `COALESCE(chats.name, contacts.full_name, contacts.push_name)` expression as `display_name` (excluding JID fallback), or match each name field independently.
- Fuzzy matching (e.g. trigram similarity) may be overkill for typical chat list sizes — substring match on display name is likely sufficient.
