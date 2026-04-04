# Docker UID/GID alignment and strict file permissions

**Status:** Done (27c355f)

## Problem

The Docker container runs as root by default. Files created inside the container (databases, media) are owned by root on the host, making them inaccessible to the host user. Conversely, host-owned files may not be readable by the container process.

WhatsApp session data and message history are sensitive — they should not be world-readable.

## Solution

### UID/GID alignment

The `user:` directive in `docker-compose.yml` runs each service as `${WABRIDGE_UID}:${WABRIDGE_GID}`, defaulting to `1000:1000`. Users set their values in `.env` (see `.env.example`).

### File permissions

An entrypoint script (`entrypoint.sh`) sets `umask 077` before exec-ing the main process. All files created by the container — databases, WAL files, media — are owner-only (`0600` files, `0700` directories) with no additional configuration needed.

## Scope

- `docker-compose.yml` — added `user: "${WABRIDGE_UID:-1000}:${WABRIDGE_GID:-1000}"` to all three services
- `entrypoint.sh` — new file: `umask 077` then `exec wabridge "$@"`
- `Dockerfile` — copies `entrypoint.sh` and uses it as `ENTRYPOINT` (replaces direct `wabridge` invocation)
- `.env.example` — documents `WABRIDGE_UID` and `WABRIDGE_GID` variables
