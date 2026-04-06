# Backlog

Feature ideas and improvements for future implementation.

| Item | Description |
|------|-------------|
| ~~[Quoted message references](2026-03-04-quoted-message.md)~~ | Done — `quoted_message_id`, `quoted_sender`, `quoted_content`, `quoted_media_type` extracted from ContextInfo |
| ~~[Fix history sync](2026-04-03-history-sync-fix.md)~~ | Done — requires `chat_jid`, queries oldest stored message as cursor for BuildHistorySyncRequest |
| ~~[Docker UID/GID & permissions](2026-04-03-docker-uid-permissions.md)~~ | Done — `user:` directive + `umask 077` in entrypoint |
| ~~[Docker path alignment](2026-04-03-docker-path-alignment.md)~~ | Done — `WABRIDGE_DATA_DIR` bind-mounted to same path in container |
| [Grouped message ID](2026-04-04-grouped-message-id.md) | Store `ContextInfo.GroupedMessageID` so consumers can associate album photos as a batch |
| [Unicode search normalization](2026-04-04-unicode-search-normalization.md) | Normalize smart quotes to ASCII so message search matches regardless of quote style |
| [FULL_HISTORY_SYNC_ON_DEMAND](2026-04-06-full-history-sync-on-demand.md) | Investigate enum 6 protocol message — may be how WhatsApp Web reliably fetches older history |
| [Read-only mode](2026-04-06-read-only-mode.md) | Feature switches to disable action tools, allowing wabridge to run in read-only mode |
| ~~[Media filename collision](2026-04-04-media-filename-collision.md)~~ | Done — on-disk filenames use message ID + extension, eliminating collisions |
| [Inline context params](2026-04-07-inline-context-params.md) | `context_before` / `context_after` on `list_messages` to return surrounding messages inline, avoiding per-message `get_message_context` round trips |
