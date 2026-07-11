---
name: doc-audit
description: >
  Audit project documentation for completeness and accuracy against the codebase.
  Scans all markdown docs, cross-references claims against source code, identifies
  stale/inaccurate/missing documentation, checks cross-doc consistency, and reports
  findings for approval before making changes. Use when the user asks to "audit
  docs", "check documentation", "are the docs up to date", "review docs for
  accuracy", or when evaluating whether docs are complete enough for onboarding
  or migration. Also trigger when the user wants to evaluate docs from a specific
  persona's perspective (e.g., "would a new developer understand this?"). Accepts
  an optional --fix flag to skip approval and fix immediately.
compatibility: Requires git. Designed for Claude Code or similar agent with subagent support.
---

# Doc Audit

Audit project documentation for completeness and accuracy. Find every place
where docs have drifted from reality, and every gap where a newcomer would
get stuck.

Good docs have three properties:
1. **Accurate** — every factual claim matches the current code
2. **Complete** — a newcomer can go from zero to productive without reading source
3. **Navigable** — indexes link to everything, cross-references connect related topics

This audit checks all three. It reports findings and **waits for approval**
before changing anything, unless `--fix` is passed.

## Arguments

- `--fix` — skip approval, fix issues immediately
- Any other text is a persona/scenario for the completeness evaluation
  - Example: `/doc-audit a new Claude instance migrating from whatsapp-web.js`
  - Example: `/doc-audit --fix`

## Phase 1: Discovery

### Find all docs

```
Glob: **/*.md at repo root
Pay special attention to: README.md, AGENTS.md, CLAUDE.md, docs/**/*.md
```

### Build doc inventory

For each doc, note:
- What it documents (tool reference, API, architecture, guide, backlog, etc.)
- What source files it references or should reference
- Whether it's indexed (linked from README, docs/README, or other index files)
- Whether it's archived (has a banner saying so) — archived docs get lighter audit

### Identify source-of-truth files

Scan the codebase for files that docs typically describe:
- Interface/type definitions
- API route registrations and handler implementations
- CLI flag definitions and subcommand setup
- Tool/command registrations (MCP tools, CLI commands, etc.)
- Event handlers and log statements
- Configuration files (docker-compose, env examples, Makefiles)

Read these files. You need their actual content to verify doc claims — skimming
is not enough.

## Phase 2: Audit

### Accuracy

For each doc, verify every factual claim against the code:

- **Interfaces and signatures** — method names, parameter names and types,
  return types. A renamed parameter is a bug in the doc.
- **API endpoints** — HTTP method, path, request body fields (names, types,
  required/optional), response schema. Check the actual handler code.
- **Tool parameters** — names, types, required/optional, descriptions. Check
  the actual registration code, not just other docs.
- **Behavioral descriptions** — does the code do what the doc says? "Substring
  search" vs "full-text search", "exact match" vs "partial match", "returns
  most recent first" — these matter.
- **Log messages and errors** — if the doc quotes a log message, check the
  exact string in code.
- **Code examples** — syntactically valid? Match actual API/usage?
- **Configuration** — flags, env vars, and defaults match the code?
- **Cross-doc consistency** — do different docs describe the same thing the
  same way? Inconsistency between an API doc and an architecture doc is a
  finding even if both are individually "close enough".

### Completeness

Think about what someone needs to go from zero to productive:

- **Onboarding** — is there a guide for first-run setup? Does it cover all
  deployment modes? Does it explain what to expect (output, timing, gotchas)?
- **Common workflows** — are typical tasks documented as recipes, not just
  individual tool references? (A tool catalog tells you what's available;
  a cookbook tells you how to accomplish a goal.)
- **Operational concerns** — safety, rate limits, maintenance, monitoring,
  disk usage, session management. Anything an operator needs to know.
- **Error recovery** — what to do when things go wrong. Troubleshooting
  guidance, not just happy-path docs.

### Navigation

- Does every doc appear in at least one index?
- Do all cross-reference links resolve (file paths AND anchors)?
- Are related docs cross-linked? (e.g., backlog items link to specs, specs
  link to architecture docs)

### Persona check (optional)

If a persona/scenario was provided, walk through the docs as that person:
- Can they accomplish their goal using only the docs?
- Where would they get stuck or make wrong assumptions?
- What knowledge do the docs assume that this person might not have?

If no persona was provided, skip this — the general audit above is sufficient.

## Phase 3: Self-review

The audit itself can be wrong. Before reporting, challenge your own findings
from multiple angles. This phase exists because a false finding wastes the
user's time, an inaccurate finding leads to a bad fix, and a missing finding
is a missed bug.

### 3a. Verify each finding

Do not verify your own work — it's too easy to confirm your own reasoning on
a second pass. Instead, launch two layers of independent review and consolidate
the results.

**Layer 1 — Holistic adversarial review:** Launch two agents with opposing
roles. Both receive the full findings list and the repo path.

- **Prosecutor:** assumes the findings are correct. For each finding, builds
  the strongest case that the issue is real — reads the cited source code,
  gathers additional evidence, and argues for the finding's severity. Also
  looks for patterns across findings and argues for any additional issues the
  audit may have missed.
- **Defense:** assumes the findings are wrong. For each finding, tries to
  disprove it — reads the same source code, looks for context that exonerates
  the doc (maybe the doc is correct and the auditor misread the code, maybe
  the severity is overstated, maybe the claim is technically accurate in a
  way the auditor didn't consider). Also argues that the findings list is
  complete enough and no major issues are missing.

Neither agent sees the other's arguments. The value is in the *tension* —
a finding that the defense can't poke holes in is solid; a finding the
prosecutor can barely defend is weak.

**Layer 2 — Per-finding adversarial review:** For each finding, launch a
prosecutor/defense pair in parallel. Each pair receives only their single
finding and the repo path. This catches details the holistic reviewers skim
over, because each pair focuses on one claim.

- **Prosecutor:** reads the cited source and doc, builds the case that the
  finding is accurate and the severity is warranted.
- **Defense:** reads the same sources, argues the finding is wrong, overstated,
  or misdescribed.

**Consolidation (you do this):** For each finding, read both sides from both
layers and render a verdict:

- **Finding stands:** the prosecution's evidence is compelling across both
  layers and defense couldn't rebut it. Keep the finding (use the most
  precise description from either side).
- **Finding corrected:** defense showed the finding is partially wrong (e.g.,
  overstated severity, wrong root cause) but prosecution proved a real issue
  exists. Rewrite the finding to reflect the actual issue.
- **Finding dropped:** defense successfully rebutted the finding in both
  layers. Drop it — a false finding erodes trust in the report.
- **Escalated:** the two sides raised genuinely conflicting evidence. Read the
  code yourself and break the tie.

### 3b. Pattern extrapolation

Look at the findings you confirmed and ask: does this class of issue appear
elsewhere?

- If you found a renamed parameter in one doc, check *all* docs that reference
  that parameter.
- If an interface was stale in one place, check every place that shows that
  interface.
- If a behavioral description was wrong in one doc, check if other docs make
  the same claim.

The initial audit found specific instances. This step finds the rest of the
class. Add any new findings to the list (and verify them too — don't skip 3a
for newly discovered issues).

### 3c. Reverse pass

The main audit works forward: doc → code ("does what the doc says match the
code?"). Now work backward: code → doc ("did the audit miss any discrepancies
between code and docs?").

Take the source-of-truth files from Phase 1. For each significant public
surface (exported interface, API endpoint, registered tool, CLI flag), check
whether the audit already covers any discrepancy between that code and the
docs. If you spot something the forward pass missed, add it as a finding.

This is not about ensuring every code detail is documented — that would spam
the report with trivial gaps. It's about catching real issues that the forward
pass had a blind spot for, because reading code-first surfaces different things
than reading docs-first.

## Phase 4: Report

Present findings as a structured table:

```
| # | File | Issue | Severity | Source |
|---|------|-------|----------|--------|
| 1 | REST_API.md:160 | Endpoint documented as empty body, code requires chat_jid | incorrect | api/server.go:202 |
| 2 | README.md:10 | Says "full-text search", impl is LIKE (substring) | misleading | store/messages.go:93 |
| 3 | (missing) | No onboarding guide for first-run pairing | gap | — |
| 4 | ARCHITECTURE.md:93 vs action.go:13 | Interface signature differs between doc and code | stale | — |
```

Severity levels:
- **incorrect** — factually wrong, will cause failures
- **stale** — was correct but code changed since
- **misleading** — technically defensible but gives wrong mental model
- **inconsistent** — two docs disagree about the same thing
- **gap** — something important is not documented at all
- **broken-link** — cross-reference target doesn't exist

**After presenting the table, STOP and WAIT for the user.** Do not make changes
unless:
- The user says "fix", "go ahead", or similar
- `--fix` was passed as an argument

## Phase 5: Fix (when approved)

For each finding:

1. **incorrect / stale / misleading / inconsistent** — edit the doc to match
   the code. Read the source first to understand the correct state. When two
   docs are inconsistent, determine which one matches the code and fix the other.
2. **gap** — write new documentation. Match existing doc style and conventions.
   Keep it factual — do not invent behaviors.
3. **broken-link** — fix the link, or remove it if the target no longer exists.

After fixing, update index files to reference any new docs.

## Phase 6: Verify

After fixes are applied, invoke `/doc-review` to run adversarial verification
on the files you just fixed or created.

The purpose is to catch issues that the fixes themselves introduced — a wrong
correction, a new inconsistency with a doc you didn't touch, a broken link
from a renamed section.

If doc-review finds additional issues:
1. Fix them
2. Re-run `/doc-review` on the affected files
3. Repeat until clean or the user says stop
