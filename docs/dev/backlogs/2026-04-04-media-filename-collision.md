# Media filename collisions on same-second messages

**Status:** Done (27c355f)

## Problem

When multiple media messages share the same timestamp (e.g., a batch of photos sent in the same second), `extractMediaInfo` generates identical filenames because the fallback name is based solely on `mediaType_timestamp.ext`. This causes `DownloadMedia` to overwrite earlier files with later ones since they resolve to the same path.

Discovered during E2E testing:
- 3 images from the same contact all timestamped within the same second got filename `image_20260403_023725.jpg` — only the last downloaded one survives on disk
- 2 images in a group chat both got `image_20260403_023722.jpg` (these were from the old `time.Now()` bug, now fixed, but same-second collisions remain)

## Solution

Use the message ID as the entire on-disk filename (with the original extension preserved when available). This separates concerns:

- **Database `filename` column** — still stores the display-friendly name from `extractMediaInfo` (e.g., `image_20260326_012742.jpg` for images, or the sender-supplied filename for documents)
- **On-disk path** — `DownloadMedia` writes to `<messageID>.<ext>`, guaranteeing uniqueness

```go
// DownloadMedia determines the on-disk filename:
ext = filepath.Ext(msg.Filename)       // preserve original extension
if ext == "" { ext = mediaTypeToExt() } // fallback by media type
localPath = filepath.Join(outputDir, msg.ID+ext)
```

A new `mediaTypeToExt` helper maps media types to default extensions (`.jpg`, `.mp4`, `.ogg`, `.webp`; other types fall through to `.bin`). Since documents almost always have a real filename from the sender, the `.bin` fallback is rarely reached in practice.

If the file already exists at the target path, `DownloadMedia` returns the existing path without re-downloading (idempotent).

## Scope

- `internal/whatsapp/media.go` — `DownloadMedia` uses `msg.ID+ext` as filename; added `mediaTypeToExt` helper
- `internal/whatsapp/handlers.go` — comment-only update to `extractMediaInfo` clarifying the filename split
- Existing database rows with duplicate filenames are not fixable without re-downloading (the URL and media key are still stored, so a migration script could re-derive unique names). Old collided files on disk are orphaned — they are not automatically cleaned up
