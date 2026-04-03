# Backlog

Feature ideas and improvements for future implementation.

| Item | Description |
|------|-------------|
| [Quoted message references](2026-03-04-quoted-message.md) | Store reply-to metadata from ContextInfo so we can definitively link replies to their parent messages |
| [Fix history sync](2026-04-03-history-sync-fix.md) | Pass a valid message cursor to BuildHistorySyncRequest instead of nil (current call always panics) |
| [Docker UID/GID & permissions](2026-04-03-docker-uid-permissions.md) | Run container as host user, create files with strict permissions (0600/0700) |
| [Docker path alignment](2026-04-03-docker-path-alignment.md) | Bind-mount strategy so media paths match between host and container (depends on UID/GID fix) |
| [Unicode search normalization](2026-04-04-unicode-search-normalization.md) | Normalize smart quotes to ASCII so message search matches regardless of quote style |
| [Media filename collision](2026-04-04-media-filename-collision.md) | Include message ID in fallback filenames to prevent overwrites when multiple media share the same timestamp |
