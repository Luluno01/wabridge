# Docker path alignment for media files

## Problem

When running in Docker with named volumes, file paths diverge between host and container:

- `download_media` returns container paths (e.g., `/app/store/media/...`) that the host can't access
- `send_file` receives host paths that the container can't access

## Proposed solution

Bind-mount a host directory to the **same absolute path** inside the container, so paths are identical on both sides. Use `WABRIDGE_DATA_DIR` env var to configure the shared path.

```yaml
volumes:
  - ${WABRIDGE_DATA_DIR}:${WABRIDGE_DATA_DIR}
```

## Setup script

A `scripts/setup.sh` handles:
1. Creating the data directory
2. Writing `.env` for docker compose
3. Building the image
4. Interactive QR pairing if no session exists

## Additional .gitignore entries needed

- `.env`
- `data/`

## Status

Draft implementation exists in `docker-compose.yml` and `scripts/setup.sh` but hasn't been tested end-to-end. Needs proper spec before finalizing.
