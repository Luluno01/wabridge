# Media filename collisions on same-second messages

## Problem

When multiple media messages share the same timestamp (e.g., a batch of photos sent in the same second), `extractMediaInfo` generates identical filenames because the fallback name is based solely on `mediaType_timestamp.ext`. This causes `DownloadMedia` to overwrite earlier files with later ones since they resolve to the same path.

Discovered during E2E testing:
- 3 images from the same contact all timestamped within the same second got filename `image_20260403_023725.jpg` — only the last downloaded one survives on disk
- 2 images in a group chat both got `image_20260403_023722.jpg` (these were from the old `time.Now()` bug, now fixed, but same-second collisions remain)

## Proposed solution

Include the message ID in the fallback filename to guarantee uniqueness:

```go
// Before:
"image_" + tsStr + ".jpg"

// After:
"image_" + tsStr + "_" + id[:8] + ".jpg"
```

Pass the message ID into `extractMediaInfo` alongside the timestamp. The first 8 characters of the message ID provide sufficient uniqueness without making filenames unwieldy.

## Scope

- `internal/whatsapp/handlers.go` — add `id` parameter to `extractMediaInfo`, use in fallback filenames
- Existing database rows with duplicate filenames are not fixable without re-downloading (the URL and media key are still stored, so a migration script could re-derive unique names)
