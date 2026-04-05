# wabridge docs

WhatsApp-to-SQLite bridge with MCP tool interface.

## Using wabridge

- [MCP Tools](ops/MCP_TOOLS.md) -- tool catalog with parameters
- [REST API](ops/REST_API.md) -- HTTP endpoints for the bridge process

## Documentation principles

- **Specs are source of truth.** Code is the "compilation" output of specs. If they diverge, update the spec (or fix the code).
- **Progressive disclosure.** Index pages first, details on drill-down. Don't front-load everything.
- **Cross-reference.** Link between related docs (backlog → spec → plan) so readers can navigate without searching.
- **Separate ops from dev.** Operational docs (how to use) and development docs (how we build) serve different audiences and should not be mixed.

## Developing wabridge

- [Architecture](dev/ARCHITECTURE.md) -- modes, data flow, package map
- [Database Schema](dev/SCHEMA.md) -- tables, JID formats, name resolution
- [WhatsApp Quirks](dev/WHATSAPP_QUIRKS.md) -- platform-specific gotchas
- [Design Specs](dev/specs/) -- feature design documents
- [Implementation Plans](dev/plans/) -- step-by-step build plans
- [Backlog](dev/backlogs/index.md) -- future work
