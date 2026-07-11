# Whatsmeow Runtime Warning Investigation

**Research date:** 2026-07-11
**Status:** Research complete; no fix or dependency upgrade implemented

This document investigates the warnings and errors retained in the Docker
`bridge` service logs. It covers operational severity, whether they indicate
deprecation, whether a newer whatsmeow version fixes them, likely ownership,
and options for a future implementation pass.

The investigation used read-only Docker log/status inspection, read-only
SQLite queries against `whatsapp.db`, the wabridge source tree, and upstream
GitHub source/issues. It did not alter the live session database.

## Executive Summary

| Signal | Immediate severity | Deprecation? | Fixed by current whatsmeow `main`? | Assessment |
|---|---|---|---|---|
| `mismatching LTHash` and missing previous value MAC | **Medium, persistent** | No | **No** | Two app-state collections are stuck, but they contain state wabridge does not expose or consume. Messaging continues. Recovery belongs in the application according to the maintainer. |
| Keepalive timeouts, websocket EOF/reset, close-lock timeout | **Low while recovery continues** | No | No direct fix | Transient transport failures. whatsmeow reconnects automatically; the close-lock warning is a secondary symptom during teardown, not the cause. |
| `503` stream error / stream end | **Low while recovery continues** | No | No | Explicitly treated by upstream as a server-requested restart/disconnect that auto-reconnect should handle. |
| Status notification content is `<nil>` | **Negligible for wabridge** | No | **No** | Upstream parser expects bytes and drops an unexpected/empty user “about” update. Wabridge does not consume this event. |
| Rare unavailable/decryption/plaintext warnings | **Low frequency; individual-message impact possible** | No | Mixed | Retry/recovery paths exist, and some post-pin message/LID fixes may help. These are not implicated in the current app-state problem. |

There are no retained log signals for `ClientOutdated`, `device_removed`,
`LoggedOut`, temporary bans, stream replacement, database corruption, OOM, or
a permanent disconnect. The bridge was running and receiving messages after
the latest app-state and transport errors.

## Runtime Evidence

The current container was created about three months ago. Its retained logs
span 2026-04-09 through 2026-07-11 (UTC). Privacy-safe aggregation found:

| Log class | Count | Interpretation |
|---|---:|---|
| Keepalive timeout | 595 | About 98 episodes; normally 5–6 failed pings over three minutes before reconnect |
| Graceful close could not acquire websocket lock | 98 | Nearly one per keepalive-forced reconnect; secondary teardown warning |
| Websocket read EOF | 374 | Transport closed; generally followed by authentication and connection within seconds |
| Connection reset by peer | 30 | Remote/network transport reset; auto-recovered |
| Stream end / 503 | 108 / 108 | Paired server stream shutdown signals; auto-recovered |
| DNS / TLS / Noise reconnect failure | 6 / 6 / 1 | Temporary reconnect failures; later attempts succeeded |
| Status notification with nil content | 54 | Parser discarded an unexpected/empty status payload |
| LTHash mismatch | 46 | Persistent app-state integrity failure |
| Missing previous SET value MAC | 11 | Strong evidence for why some LTHash calculations diverge |
| Unavailable message / missing sender key / plaintext without bytes | 6 / 3 / 2 | Rare message-level protocol/decryption cases |

The app-state problem is not new to the July restart:

- `regular` first failed at patch v36 on 2026-04-15.
- `regular_low` first failed at patch v201 on 2026-04-24.
- The latest failures are `regular` v42 and `regular_low` v227.
- The bridge continued to process normal messages after the latest failures.

Read-only inspection of `whatsapp.db` found:

| Collection | Stored version | Latest failing patch observed | State |
|---|---:|---:|---|
| `critical_block` | 16 | None | Healthy in retained logs |
| `critical_unblock_low` | 79 | None | Healthy in retained logs |
| `regular_high` | 56 | None | Healthy in retained logs |
| `regular` | 35 | 42 | Stuck |
| `regular_low` | 200 | 227 | Stuck |

All stored hashes have the required 128-byte length. `PRAGMA quick_check`
returned `ok`, `foreign_key_check` returned no violations, and 26 app-state
sync keys are present. The exact index MAC named in the latest `regular`
warning is absent from the mutation-MAC table. This proves the local state
cannot calculate that removal correctly, but it does not prove whether the
missing state originated in a bad server patch, an earlier local write loss,
or a historical session-store inconsistency.

Only one Compose service is running against the session database, so there is
no evidence of concurrent wabridge processes racing the same store.

## Dependency Assessment

Wabridge pins:

```text
go.mau.fi/whatsmeow v0.0.0-20260327181659-02ec817e7cf4
```

As of this research, upstream `main` is commit
[`73fe7355f59f`](https://github.com/tulir/whatsmeow/commit/73fe7355f59fba573f554d4c6ac9e71ea1001c1f)
from 2026-07-09, equivalent to pseudo-version
`v0.0.0-20260709092057-73fe7355f59f`. Whatsmeow publishes no GitHub releases;
consumers track pseudo-versions. Current `main` is 71 commits ahead of the
pinned version.

### What an upgrade would and would not change

- **LTHash:** not fixed. Upstream issue
  [#858](https://github.com/tulir/whatsmeow/issues/858) remains open. The
  current decoder still treats a mismatching LTHash as a hard failure, and
  current `main` has no automatic recovery state machine.
- **App-state error events:** current `main` emits `AppStateSyncError` more
  consistently after commit
  [`d4ffc1d`](https://github.com/tulir/whatsmeow/commit/d4ffc1df2442), which
  would improve application-level observability, but does not recover the
  state.
- **Keepalive:** the deadlines, three-minute failure threshold, warning, and
  reconnect logic are unchanged between the pinned version and current
  `main`.
- **Close-lock warning:** current whatsmeow upgrades
  `github.com/coder/websocket` from v1.8.14 to v1.8.15 and makes FrameSocket
  fields atomic in
  [`b10b707`](https://github.com/tulir/whatsmeow/commit/b10b70708a4e418ce8bca4b92e55e3099ca5b124).
  The atomic change fixes reported data races, but neither change directly
  changes the graceful-close lock acquisition that produced the warning.
- **Status `<nil>`:** the exact byte-content check and warning remain in
  [current `notification.go`](https://github.com/tulir/whatsmeow/blob/73fe7355f59fba573f554d4c6ac9e71ea1001c1f/notification.go#L440-L450).
- **Protocol currency:** current `main` has several protobuf and WhatsApp Web
  client-version bumps plus LID/message fixes. An upgrade is reasonable
  maintenance, but evidence does not support presenting it as a fix for the
  observed warning classes.

Conclusion: **do not upgrade solely to fix these logs**. Evaluate an upgrade
as a separate compatibility task with build, migration, and live-session
verification.

## Root Cause and Ownership

### 1. App-state LTHash failures

Whatsmeow maintains a 128-byte LT hash and per-index value MACs for each app
state collection. A REMOVE mutation must subtract the prior SET value MAC.
When that MAC is absent, whatsmeow logs a warning, calculates a hash without
the missing subtraction, and rejects the patch when the calculated snapshot
MAC does not match the server-provided MAC. The implementation is visible in
[`appstate/hash.go`](https://github.com/tulir/whatsmeow/blob/73fe7355f59fba573f554d4c6ac9e71ea1001c1f/appstate/hash.go)
and
[`appstate/decode.go`](https://github.com/tulir/whatsmeow/blob/73fe7355f59fba573f554d4c6ac9e71ea1001c1f/appstate/decode.go).

Upstream has multiple reports of the same persistent failure. The maintainer's
recommended escalation is documented in
[#858](https://github.com/tulir/whatsmeow/issues/858#issuecomment-4679328028):
try a full sync, then request recovery from the primary phone, and only as a
last resort send a fatal app-state notification that unlinks every device.

The proposed “skip broken patches” PR
[#1171](https://github.com/tulir/whatsmeow/pull/1171) is not a safe general
solution. The maintainer explains that the hash covers the whole collection,
so skipping a patch usually makes the hash invalid forever. An earlier PR that
automatically requested recovery on each error was also rejected because
unthrottled requests are unsafe; see
[#1120](https://github.com/tulir/whatsmeow/pull/1120#issuecomment-4294497229).

Ownership is shared:

- The initial divergence is a known upstream/protocol failure class, not
  evidence that wabridge called whatsmeow incorrectly.
- Wabridge uses the standard `sqlstore.New → GetFirstDevice → NewClient →
  Connect` path and does not customize app-state processing.
- Wabridge **does** omit handling for `events.AppStateSyncError`. Upstream
  intentionally leaves safe escalation policy to the application, so this
  omission explains why the failure persists indefinitely once encountered.

Impact is currently contained. Upstream defines `regular_low` as pin/archive/
read-state settings and `regular` primarily as protocol/business/label state;
wabridge exposes none of those operations and registers no handlers for those
events. Contact and push-name collections (`critical_unblock_low` and
`critical_block`) are not failing. Core send/receive and stored-message paths
are therefore not blocked.

**Confidence:**

- 100% that wabridge lacks the recovery handler and current whatsmeow does not
  auto-recover this condition.
- 100% that a plain upgrade to current `main` does not fix the LTHash failure.
- 90% that the current issue is session/protocol state divergence rather than
  wabridge API misuse. Database integrity and single-process checks support
  this, but retained logs cannot reconstruct the original missing mutation.
- 85% that a throttled full-sync/recovery flow can repair the two collections;
  the primary phone may refuse the recovery request, as upstream documents.

### 2. Keepalive, websocket, and 503 signals

Current and pinned whatsmeow both wait ten seconds for a keepalive reply and
force a reconnect after failures continue for three minutes. In the retained
logs, the close-lock warning follows that threshold and is followed by a
successful reconnect. It is therefore teardown noise caused by an already
unresponsive connection, not evidence of a second outage.

Upstream explicitly treats a 503 stream error as a server restart-like event
and relies on automatic reconnect in
[`connectionevents.go`](https://github.com/tulir/whatsmeow/blob/73fe7355f59fba573f554d4c6ac9e71ea1001c1f/connectionevents.go#L52-L55).
EOF, peer reset, DNS, and TLS errors are transport/environmental failures. No
wabridge call pattern in the inspected code would cause them.

**Confidence:** 95% that these are non-deprecation transport events and are
non-critical while reconnect succeeds. Escalate if a disconnect is not
followed by `Successfully authenticated` and `Connected to WhatsApp` within a
reasonable window, or if their frequency materially increases.

### 3. Nil status notifications

Whatsmeow's status-notification handler expects `<set>` content to be `[]byte`.
Nil content is logged and discarded. This handler was introduced by
[#670](https://github.com/tulir/whatsmeow/pull/670) to emit contact “about”
status changes. Wabridge does not handle `events.UserAbout`, so the warning has
no application-visible impact today.

Most likely, WhatsApp is sending an empty/variant status update that the parser
does not model. Treating nil as an empty status may be appropriate upstream,
but raw DEBUG protocol evidence should be captured before proposing that
change.

**Confidence:** 95% that this is a harmless upstream parser gap for wabridge;
60% that nil specifically means the user cleared their “about” value.

## Recommended Future Implementation (Not Yet Performed)

The app-state recovery should be a small, explicit state machine per
collection, not an unconditional retry:

1. Handle `*events.AppStateSyncError` and record collection, error class,
   `FullSync`, attempt stage, and last-attempt time.
2. On the first LTHash/PatchMAC failure outside a cooldown window, run
   `FetchAppState(ctx, name, true, false)` once.
3. If the full sync fails with the same integrity class, send one
   `BuildAppStateRecoveryRequest(name)` using `SendPeerMessage` and wait for an
   `AppStateSyncComplete{Recovery: true}` event or a bounded timeout.
4. Singleflight and rate-limit the process per collection so repeated server
   notifications cannot spam the primary phone.
5. Never automate `BuildFatalAppStateExceptionNotification`. It logs out all
   linked devices and should require explicit operator confirmation, a session
   database backup, and readiness to pair again.
6. Add redacted operational status so a persistent app-state failure is
   visible without logging message/contact identifiers.

These recovery primitives already exist in the pinned whatsmeow version, so
the recovery fix does not require combining an upgrade with the behavior
change. Keeping those as separate changes will make causality and rollback
clearer.

Before implementation, capture DEBUG-level raw nodes for one nil-status event
and one app-state failure if that can be done without retaining unrelated
message content. During implementation, verify that `regular` and
`regular_low` stored versions advance beyond 35/200, that
`AppStateSyncComplete` is observed, and that ordinary message flow remains
healthy.

## Operational Decision

- **Do not stop the bridge immediately.** Current messaging is healthy and
  reconnects are succeeding.
- **Treat app-state recovery as the next targeted reliability fix**, especially
  before adding archive/mute/pin/label/read-state tools.
- **Do not use validation bypasses or patch skipping.** They weaken integrity
  checks and are explicitly rejected as a general solution upstream.
- **Do not interpret these logs as a deprecation warning.** Watch instead for
  `ClientOutdated`, permanent disconnect, logout/session deletion, or sustained
  failure to reconnect.
