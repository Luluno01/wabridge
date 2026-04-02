# Docker UID/GID alignment and strict file permissions

## Problem

The Docker container runs as root by default. Files created inside the container (databases, media) are owned by root on the host, making them inaccessible to the host user. Conversely, host-owned files may not be readable by the container process.

WhatsApp session data and message history are sensitive — they should not be world-readable.

## Proposed solution

### UID/GID alignment

Run the container process with the same UID/GID as the host user. Options:

1. **`--user` flag:** `docker compose run --user $(id -u):$(id -g)` — simple but must be passed every time
2. **Build-time ARG:** Create a non-root user in the Dockerfile with configurable UID/GID via build args
3. **Entrypoint script:** Detect mounted volume ownership and switch to matching UID at runtime

### File permissions

Create all files with strict permissions:

- Databases (`*.db`): `0600` (owner read/write only)
- Media directory: `0700` (owner only)
- Media files: `0600`
- Session database: `0600` (contains auth tokens)

Permissions could be configurable via env var (e.g., `WABRIDGE_FILE_MODE=0600`, `WABRIDGE_DIR_MODE=0700`) for users who need group access.

### Implementation notes

- SQLite WAL files (`*.db-wal`, `*.db-shm`) inherit permissions from the main db file
- GORM's `sqlite.Open` doesn't set file permissions — need to `os.Chmod` after creation or use `umask`
- whatsmeow's sqlstore also creates its own db — may need a wrapper or post-creation chmod
