# WhatsApp Quirks

Practical notes on working with WhatsApp via whatsmeow. Distilled from building wabridge.

## JID Formats

WhatsApp uses three identifier formats:

| Format | Example | Meaning |
|--------|---------|---------|
| Phone JID | `1234567890@s.whatsapp.net` | Individual chat, phone-number-based |
| LID JID | `18273648@lid` | Individual chat, opaque server-assigned ID |
| Group JID | `120363012345678901@g.us` | Group chat |

### Phone-to-LID Migration

WhatsApp is migrating from phone-based JIDs to LID-based JIDs. The same person can appear as either format depending on context -- history sync messages may use phone JIDs while real-time messages use LID JIDs, or vice versa.

You must handle both and link them. wabridge does this with a dual-entry contact strategy: each contact gets two rows in the contacts table, one keyed by phone JID and one by LID JID, cross-referenced via the `phone_jid` column. The LID-to-phone mapping comes from `client.Store.LIDs.GetLIDForPN()`.

Always store JIDs in "non-AD" form (`Sender.ToNonAD().String()`), which strips the `:device` suffix to produce a canonical identifier.

## ParseWebMessage

When processing history sync data, you must use:

```go
evt, err := client.ParseWebMessage(chatJID, webMsg.Message)
```

This converts raw `WebMessageInfo` protobuf into a proper `events.Message` with correctly resolved sender JID. Without it, all group messages show the group JID as sender instead of the actual member. Manual protobuf field extraction does not handle JID resolution, device mapping, or the various message wrapper types correctly.

## History Sync

### How It Works

1. On first login, WhatsApp automatically pushes recent history as `events.HistorySync` events
2. Each event contains conversation batches, each with messages as `WebMessageInfo` protobufs
3. Process each message through `ParseWebMessage` (see above)

### Completion Detection

History sync arrives in multiple batches with no explicit "done" signal. Detect completion by waiting for a settling period (e.g., 15 seconds with no new events).

### BuildHistorySyncRequest Panics

`client.BuildHistorySyncRequest()` can panic with certain account states. Always wrap in panic recovery:

```go
defer func() {
    if r := recover(); r != nil {
        log.Errorf("panic in BuildHistorySyncRequest: %v", r)
    }
}()
```

In practice, this method does not reliably fetch history beyond the initial automatic sync. Older messages require the user to scroll in the WhatsApp mobile app.

## Session Expiry

WhatsApp sessions expire approximately every 20 days. Re-pairing requires:

1. Remove the whatsmeow device from WhatsApp mobile settings (Linked Devices)
2. Wipe the session store (`whatsapp.db`)
3. Scan a new QR code

The session database (`whatsapp.db`) must be kept separate from the application database (`messages.db`) so that re-pairing does not lose stored messages.

## Media Handling

### Metadata-Only Storage

Only media metadata is stored in SQLite (URL, media key, SHA-256 hashes, file length). Actual media bytes are downloaded on demand via whatsmeow's `client.Download()`. This keeps the database small and avoids downloading media the user never requests.

### Media Type Detection

Detected from file extension at send time:
- **Image**: jpg, jpeg, png, gif, webp
- **Video**: mp4, avi, mov, mkv
- **Audio**: ogg, mp3, wav, m4a
- **Document**: everything else

### Voice Messages (Ogg Opus + Waveform)

Sending audio as a WhatsApp voice message (PTT = push-to-talk) requires:
- Ogg container with Opus codec
- Duration calculated from Ogg page structure (granule positions)
- Waveform array (64 bytes) -- can be synthetic/placeholder
- `PTT = true` on the `AudioMessage` protobuf

### Container Path Pitfall

Downloaded media paths are inside the process's filesystem. In Docker, the media directory must be on a shared volume for the MCP process to access downloaded files.
