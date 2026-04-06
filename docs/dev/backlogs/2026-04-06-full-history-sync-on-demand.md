# Investigate FULL_HISTORY_SYNC_ON_DEMAND for reliable older history

## Problem

The current `request_history_sync` tool uses `HISTORY_SYNC_ON_DEMAND` (enum 3), which sends a peer message to the phone. The phone can ignore or decline the request, and in practice it almost never responds. This is a known issue across WhatsApp libraries (whatsmeow#654, Baileys#1934). The mautrix bridge doesn't even bother implementing it.

WhatsApp Web successfully fetches older messages via its "click to get older messages" button.

## Investigation lead

The protobuf definitions reveal a separate, newer message type: `FULL_HISTORY_SYNC_ON_DEMAND` (enum 6). It uses `FullHistorySyncOnDemandRequest` with different parameters:

- `requestMetadata` (ID, business product, opaque data)
- `historySyncConfig` (device history sync configuration)
- `fullHistorySyncOnDemandConfig` (with `historyFromTimestamp` and `historyDurationDays`)

This requests a full re-sync within a time range rather than paginating by message cursor. It may be the mechanism WhatsApp Web actually uses. whatsmeow has no helper function for it.

## Additional leads

- whatsmeow's `BuildHistorySyncRequest` doesn't populate the `AccountLid` field (proto field 6), which may matter for LID-migrated accounts.
- The response handler in whatsmeow (`message.go:817-825`) has no case for `HISTORY_SYNC_ON_DEMAND` in the `PeerDataOperationRequestResponseMessage` switch — responses may arrive via the `HistorySyncNotification` path instead, but this is undocumented.

## References

- whatsmeow issue: tulir/whatsmeow#654 (closed NOT_PLANNED)
- Baileys issue: WhiskeySockets/Baileys#1934
- mautrix backfill docs: docs.mau.fi/bridges/general/backfill.html
- Proto definitions: `WAWebProtobufsE2E.proto` enums 3 vs 6, field definitions around line 812
