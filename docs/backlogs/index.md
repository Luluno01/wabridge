# Backlog

Feature ideas and improvements for future implementation.

| Item | Description |
|------|-------------|
| [Quoted message references](2026-03-04-quoted-message.md) | Store reply-to metadata from ContextInfo so we can definitively link replies to their parent messages |
| [Fix history sync](2026-04-03-history-sync-fix.md) | Pass a valid message cursor to BuildHistorySyncRequest instead of nil (current call always panics) |
