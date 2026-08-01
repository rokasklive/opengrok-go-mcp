# Implementation Plan: Honest Hit Accounting and Schema Fidelity

**Branch**: `009-hit-accounting-and-schema-fidelity` | **Date**: 2026-08-01 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/009-hit-accounting-and-schema-fidelity/spec.md`

## Summary

Make the search response's own numbers reconcilable. `total_hits` switches from
OpenGrok's document count to the line count it actually returns (D1); the document
count moves to a new `total_files` field, which keeps driving `total_pages`,
`has_more`, and the cursor, because paging stays document-based (D2). `page_size`
consequently bounds documents rather than results, and the schema says so.
Separately, each compact tool's top-level properties gain an annotation naming the
operations that accept them, so the schema stops advertising fields the selected
operation will reject (D3); the `oneOf` branches are untouched, preserving the 0.5.1
strict-client fix.

Both halves are one theme: **the surface reports one thing and delivers another.**
The unit swap is the breaking part and is done first, under test, with the migration
note written before the field flips.

## Technical Context

**Language/Version**: Go 1.25 (CI); 1.26.5 local

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp` v1.7.0,
`github.com/google/jsonschema-go/jsonschema` v0.4.3, OpenGrok HTTP API

**Storage**: In-memory process state; configuration via environment variables

**Testing**: `go test ./...`; `go test ./evals/ -count=1`; token benchmark
(`TestTokenBenchmark`); live checks gated on `OPENGROK_MCP_BASE_URL`

**Target Platform**: Local stdio MCP server and loopback HTTP transport

**Performance Goals**: `tools/list` byte delta from D3 annotations stays within a
budget agreed before implementation (SC-006); no per-response size regression

**Constraints**: `total_hits` is a breaking unit change and must land with migration
notes; pagination/cursor semantics must not change (D2); the 0.5.1 strict-client
behavior must survive (SC-005); full, compact, and gateway surfaces stay coherent

**Scale/Scope**: 4 compact tools + full surface; search, symbol, and compound
response paths

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Principle I — Agent-Focused MCP Contract**: Changes (a) the **meaning** of the
  existing `total_hits` output field on every search-shaped response — breaking, and
  the reason this feature needs a spec; (b) adds output field `total_files`; (c)
  adds per-property operation annotations to compact input schemas. No tool names,
  operation names, error codes, warning codes, cursor format, or citation semantics
  change. Full, compact, and gateway surfaces all move together.
- **Principle II — Evidence-Backed OpenGrok Semantics**: This feature *implements*
  it. The current surface implies a hit count that OpenGrok never gave it. After the
  change each figure is labeled with the unit OpenGrok actually reports, and the
  document-vs-line distinction is stated rather than hidden.
- **Principle III — Test-Proven Go Changes**: Test-first per slice. The reconcile
  invariant (`total_hits == len(results)` on an unpaged response) is asserted before
  the field changes, so the new tests fail against current `main`. The
  advertise-vs-accept matrix (SC-004) is generated from the published schema rather
  than hand-listed, so it cannot drift.
- **Principle IV — Secure Local Operation**: No transport, auth, or secret surface
  touched.
- **Principle V — Simplicity, Compatibility, Documentation**: The breaking change is
  deliberate, decision-recorded (D1), and carries migration notes in `CHANGELOG.md`
  and `docs/tool-contracts.md`. D3 chooses the additive option over the restructure,
  deferring the ≈1,800-token duplication saving rather than bundling it.

## Project Structure

### Documentation (this feature)

```
specs/009-hit-accounting-and-schema-fidelity/
├── spec.md      # Feature specification (decisions D1–D3 recorded)
├── plan.md      # This file
└── tasks.md     # Task breakdown
```

`research.md`, `data-model.md`, and `contracts/` are not created: there is no open
research question (the OpenGrok response shape is established in the spec), no new
entity beyond one output field, and no new error/warning contract.

### Source Code (repository root)

```
internal/opengrok/
└── client.go            # searchResponse.toResult — set both counts

internal/mcpserver/
├── types.go             # SearchOutput/Pagination: total_files; page_size docs
├── pagination.go        # newPagination takes the document count
├── search_core.go       # totalHits = line count; total_files = document count
├── symbols.go           # same split on the symbol path
├── compound.go          # propagate both counts
├── search_handlers.go   # propagate both counts
└── compact_schema.go    # D3 per-property operation annotations

evals/                   # token benchmark records the D3 delta
docs/                    # tool-contracts.md, limitations.md migration notes
```

## Build Order (breaking change proven before it flips)

1. **Reconcile invariant test, failing** — assert `total_hits == len(results)` on an
   unpaged multi-line-per-file fixture. Red against current `main`; this is the
   proof the defect exists.
2. **Plumb `total_files`** — carry OpenGrok's document count through
   `SearchResult` → `SearchOutput` without yet changing `total_hits`. Additive, all
   existing tests stay green.
3. **Repoint pagination at `total_files`** — `total_pages`, `has_more`, and the
   cursor read the document count explicitly instead of inheriting it from
   `total_hits`. Still no observable change; this is what makes step 4 safe.
4. **Flip `total_hits` to lines** — the breaking step. Step 1 goes green here.
   Migration notes land in the same change, not after (AP#3: never flip a public
   default before its guard and its note are in place).
5. **Schema wording for `page_size`** — state that it bounds documents, so a page
   returning more results than `page_size` reads as expected.
6. **D3 annotations + generated advertise-vs-accept matrix** — independent of 1–5;
   sequenced last so the breaking change is not held up behind it.
7. **Docs, token-benchmark delta, fresh-subagent UX run** per AGENTS.md.

Steps 1–4 are one reviewable sequence: the field's meaning changes exactly once, at
step 4, with its test and its note.

## Complexity Tracking

No constitution violations requiring justification. The breaking contract change is
explicitly sanctioned by maintainer decision D1 and carries the Principle V
migration obligation, which steps 4 and 7 discharge.
