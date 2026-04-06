# Automation Safety

Guidelines for running wabridge in unattended or high-frequency scenarios (scheduled briefings, monitoring, chatbots).

## Reading is Safe

All query tools (`list_chats`, `list_messages`, `search_contacts`, `get_message_context`, etc.) read from the local SQLite database. They do not contact WhatsApp servers and have no rate limit concerns. You can query as frequently as you like.

## Action Tools Require Caution

Action tools (`send_message`, `send_file`, `send_audio_message`, `request_history_sync`) communicate through WhatsApp's servers. The send tools are subject to WhatsApp's anti-spam and anti-automation enforcement. `request_history_sync` sends a peer message to your primary device — calling it in a tight loop against many chats could look abnormal to WhatsApp.

### What We Know

WhatsApp does not publish official rate limits for linked devices. What is known from community experience:

- **Bulk messaging** (same content to many recipients in a short window) is the most common trigger for temporary or permanent bans
- **High-frequency sending** (dozens of messages per minute to the same chat) can trigger throttling or temporary blocks
- **Sending to contacts who haven't messaged you first** (cold outreach) increases ban risk significantly
- **Normal conversational patterns** (a few messages per minute, to people you regularly chat with) are generally safe

### Recommendations

- **Do not use wabridge for bulk messaging or spam.** This violates WhatsApp's Terms of Service and will get your account banned.
- **Add delays between sends** in automation scripts. A few seconds between messages is a reasonable baseline.
- **Prefer reading over writing.** If your automation only needs to read messages (e.g., daily briefings, monitoring), consider running in read-only mode once available (see [backlog](../dev/backlogs/2026-04-06-read-only-mode.md)), or simply avoid calling send tools.
- **Test with your own account first.** Don't deploy automation against important accounts without testing the pattern.
- **Monitor for disconnections.** If WhatsApp blocks your linked device, the bridge will log a `Device logged out` warning. Set up log monitoring if running unattended.

### What Happens When You Get Blocked

WhatsApp enforcement is opaque, but typical outcomes include:
- **Temporary block** — sending fails for minutes to hours, then resumes
- **Device logout** — the linked device session is revoked; you need to re-pair
- **Account ban** — the phone number is permanently banned from WhatsApp (rare for linked devices, more common for the primary app)

wabridge cannot detect or work around these blocks. If sends start failing, reduce frequency and check your phone's WhatsApp for any warnings.

## Session Stability

- WhatsApp allows only **one active session per linked device**. If you start a second instance sharing the same session database, the first one gets disconnected.
- Use **bridge+mcp mode** for automation. The bridge holds a single persistent session; MCP servers can start and stop freely without affecting the connection.
- The bridge automatically reconnects on transient network issues. Permanent disconnections (device unlinked, account action) require manual re-pairing.

## Download Media

`download_media` contacts WhatsApp servers to fetch the encrypted media file. This is read-oriented but does make network requests. In practice, downloading media at conversational rates is fine. Avoid downloading thousands of files in rapid succession.

Repeated calls for the same message ID are no-ops — the file is cached locally and the cached path is returned without re-downloading.

## Disk Usage

wabridge has no automatic data retention or cleanup:

- **Message database** — every message is stored permanently. The SQLite database grows with each incoming message and is never pruned. For high-traffic accounts running over months, plan for this.
- **Media directory** — each `download_media` call saves the file to disk under `<media_dir>/<chat_jid>/`. Files are deduplicated by message ID but never evicted. If your automation downloads media routinely, monitor disk usage.

Neither of these is a problem for typical use, but unattended long-running deployments should monitor disk space or implement external cleanup.
