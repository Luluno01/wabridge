# Backlog

Feature ideas and improvements for future implementation.

| Item | Description |
|------|-------------|
| ~~[Quoted message references](2026-03-04-quoted-message.md)~~ | Done — `quoted_message_id`, `quoted_sender`, `quoted_content`, `quoted_media_type` extracted from ContextInfo |
| [Fix history sync](2026-04-03-history-sync-fix.md) | Pass a valid message cursor to BuildHistorySyncRequest instead of nil (current call always panics) |
| ~~[Docker UID/GID & permissions](2026-04-03-docker-uid-permissions.md)~~ | Done — `user:` directive + `umask 077` in entrypoint |
| ~~[Docker path alignment](2026-04-03-docker-path-alignment.md)~~ | Done — `WABRIDGE_DATA_DIR` bind-mounted to same path in container |
| [Grouped message ID](2026-04-04-grouped-message-id.md) | Store `ContextInfo.GroupedMessageID` so consumers can associate album photos as a batch |
| [Unicode search normalization](2026-04-04-unicode-search-normalization.md) | Normalize smart quotes to ASCII so message search matches regardless of quote style |
| ~~[Media filename collision](2026-04-04-media-filename-collision.md)~~ | Done — on-disk filenames use message ID + extension, eliminating collisions |
