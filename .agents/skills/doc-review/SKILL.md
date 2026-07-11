---
name: doc-review
description: >
  Adversarial review of documentation against source code. Cross-checks factual
  claims in markdown docs against the actual codebase — interfaces, function
  signatures, parameter names, log messages, file paths, behavioral descriptions,
  and cross-doc consistency. Use when docs have been written or updated, after a
  doc-audit fix pass, before committing doc changes, or when the user says
  "review the docs", "check if docs are accurate", "verify documentation",
  or wants to cross-check docs against code. Also supports per-file parallel
  sweeps for mechanical checks like privacy scanning.
compatibility: Requires git. Designed for Claude Code or similar agent with subagent support.
---

# Doc Review

Adversarial review of documentation against source code. The goal is to catch
every factual inaccuracy, cross-doc inconsistency, and coverage gap before it
reaches users.

Documentation that contradicts the code is worse than no documentation. A
developer who reads "the endpoint takes an empty body" and builds against that
will waste hours debugging a 400 error. Every claim in a doc is a promise to
the reader — verify it.

## Identify target docs

Pick the right strategy based on context:

1. **Git diff** (default after changes): find changed/new `.md` files via
   `git diff --name-only` against the appropriate base
2. **Explicit list**: if the user names specific files, use those
3. **All docs**: if the user says "review all docs", glob for `**/*.md`

## Review modes

### Holistic review (default)

This is the primary review mode. Launch **one agent** (or a small number grouped
by domain, e.g., ops docs vs dev docs) that reviews all target docs **together**.

The holistic view is essential because the most valuable findings are cross-cutting:
- ARCHITECTURE.md shows one interface signature, but the design spec shows another
- REST_API.md documents a parameter that MCP_TOOLS.md omits (or vice versa)
- A getting-started guide references a flag that was renamed in the CLI code
- Two docs describe the same behavior differently

A per-file reviewer cannot see these. Do not default to per-file review.

**Agent brief template:**

```
Review these documentation files for factual accuracy and cross-doc consistency
against the source code in [repo path]:

[list of doc files]

For EACH doc, read it fully, then find and read the source code it describes.
Verify:

- Function/method signatures (names, parameters, types, return types)
- Interface definitions (all methods listed, signatures match code)
- Tool/parameter names (exact string match against registration code)
- API endpoints (method, path, request/response body, required fields)
- Log messages (exact string match against log statements)
- Behavioral descriptions (what code actually does vs what doc says)
- File paths and directory structures
- Cross-reference links (do target files and anchors exist?)
- Code examples (valid syntax, match actual usage)

ALSO check cross-doc consistency:
- Do different docs describe the same thing consistently?
- Are parameter names/types consistent across MCP tool docs, REST API docs,
  and architecture docs?
- Do index files (README.md, docs/README.md) list all docs that exist?
- Are archived docs properly marked and do their divergence notes cover
  all known changes?

For each issue found, report:
- Doc file and line
- The claim made
- What the code actually shows (source file and line)
- Severity: incorrect | stale | misleading | inconsistent | broken-link | gap

Do NOT report claims that check out. Only report problems.
```

### Per-file parallel sweep

Use this mode **only** for mechanical, exhaustive checks where thoroughness on
individual file content matters more than cross-file consistency. Examples:

- Privacy/PII scanning (names, phone numbers, addresses, internal paths)
- Style/formatting compliance
- Link validation

Launch one agent per file in parallel. Each agent focuses on its single file
and the specific check being performed.

**When the user asks for this**: they might say "scan all docs for PII",
"check each file for privacy leaks", "audit every doc for [specific thing]".

## Collecting results

Merge findings into a single report table:

```
| # | Doc file | Line | Claim | Reality | Source | Severity |
|---|----------|------|-------|---------|--------|----------|
```

Present the table. If no issues found, say so clearly — a clean report is
a meaningful result.

## Iteration

If issues are found and fixed:

1. Re-run review on files that had issues (not necessarily all docs again,
   but include related docs if cross-doc issues were found)
2. Present new findings or clean bill of health
3. Repeat until clean or user says stop

Each round should find fewer issues. If a round finds new issues in areas
that were previously clean, flag it as a possible regression.

## Review calibration

- **Don't trust the doc's framing.** If a doc says "partial match", don't just
  check the tool exists — check whether the query uses `LIKE` or `=`.
- **Check both directions.** A doc might be correct about what exists but miss
  something important (undocumented required parameter, unmentioned side effect).
- **Archived docs get lighter review.** If a doc has an "Archived" banner,
  verify the banner's divergence notes are complete. Don't flag every stale
  detail in the body — the banner is the fix.
- **Be specific.** "The interface is wrong" wastes time. "ARCHITECTURE.md:93
  shows `Foo(ctx)` but action.go:13 defines `Foo(ctx, id string)`" is actionable.
