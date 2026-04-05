# Normalize Unicode characters in message search

## Problem

SQLite `LIKE` does byte-level comparison. When users search for messages containing apostrophes (e.g. "don't"), the search fails if the stored message uses a Unicode curly/smart apostrophe (`'` U+2019) instead of the ASCII apostrophe (`'` U+0027). Phones commonly auto-replace ASCII quotes with smart quotes when typing.

Discovered during E2E testing: searching for `don't know why` returned no results, but `know why we` (avoiding the apostrophe) found the message. The stored content contained U+2019.

## Proposed solution

Normalize the search term before the `LIKE` query in `ListMessages`. At minimum, map common smart-quote variants to their ASCII equivalents:

- `'` (U+2018) and `'` (U+2019) to `'` (U+0027)
- `"` (U+201C) and `"` (U+201D) to `"` (U+0022)

Apply the same normalization to message content at storage time (`StoreMessage`) so both sides match consistently. This avoids needing SQLite extensions or custom collations.

## Scope

- `internal/store/messages.go` — normalize `opts.Search` in `ListMessages`
- `internal/whatsapp/handlers.go` — normalize `content` in `buildMessage` before storing
- Add a shared `normalizeQuotes(s string) string` helper
