# Docker path alignment for media files

**Status:** Done (27c355f)

## Problem

When running in Docker with named volumes, file paths diverge between host and container:

- `download_media` returns container paths (e.g., `/app/store/media/...`) that the host can't access
- `send_file` receives host paths that the container can't access

## Solution

Bind-mount the host data directory to the **same absolute path** inside the container using `WABRIDGE_DATA_DIR`. Since both sides see the same path, media paths returned by `download_media` work on the host and paths passed to `send_file` work in the container.

```yaml
volumes:
  - ${WABRIDGE_DATA_DIR}:${WABRIDGE_DATA_DIR}
```

All service commands reference `WABRIDGE_DATA_DIR` for `--db`, `--session-db`, and `--media-dir` flags. The named `store:` volume was removed in favor of this bind mount.

## Scope

- `docker-compose.yml` — replaced named volume with identity bind mount; all `command:` flags use `${WABRIDGE_DATA_DIR}`
- `.env.example` — documents `WABRIDGE_DATA_DIR` with explanation of the identity mount pattern
- `.gitignore` — added `.env` and `data/`
