# Implementation Plan: Compact Surface as Default

**Branch**: `006-compact-surface-default` | **Date**: 2026-06-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/006-compact-surface-default/spec.md`

## Summary

Polish the `compact` MCP tool surface to default-eligible quality and flip the
shipped default from `full` to `compact`. The redesign (a) consolidates compact to
**four non-overlapping tools** (projects, search, symbols, read; the memory tool is
omitted — clarified 2026-06-24) with all symbol/reference work in one place and the
vague `opengrok_compound` removed, (b) replaces the untyped `{operation, payload}`
envelope with **typed schemas discriminated by `operation`, composed from the
existing input structs** so they cannot drift from full, (c) closes the parity gaps
(`projects.files`, `projects.overview`), and (d) makes compact a **first-class
measured surface** in the eval harness (parameterized contract scenarios +
compact-specific cases + cross-surface equivalence assertion + a committed compact
baseline that gates CI). The default flip ships **last**, gated on parity and
equivalence being proven. The full surface stays byte-for-byte stable; full-surface
consolidation is a non-binding recommendation only.

Design grounded in the agent-ergonomics skills: see `research.md` (decisions,
laws/anti-patterns), `data-model.md` (operation inventory), `contracts/`
(tool-interface spec + migration map), and `quickstart.md` (go/no-go gate).

## Technical Context

**Language/Version**: Go 1.24

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp` v1.4.0 (uses
`github.com/google/jsonschema-go` for schema inference/validation, `jsonschema.For[T]`
+ `Schema{OneOf,AllOf,If,Then,Const,Enum}` — confirmed via `go doc`); OpenGrok HTTP API.

**Storage**: In-memory only (process memory bank, stdio); configuration via env vars.

**Testing**: `go test ./...`; targeted `./internal/mcpserver/` and `./evals/`;
token benchmark `./evals/ -run TestTokenBenchmark`. Test-first per Constitution III.

**Target Platform**: Local stdio MCP server + loopback HTTP transport.

**Project Type**: Go CLI/MCP server.

**Performance Goals**: Compact `ListTools` + per-call bytes ≤ full per successful
task (SC-005); typed schemas add `ListTools` bytes vs the terse envelope — bounded
and measured by the token benchmark (reported, non-gating in v1).

**Constraints**: Preserve MCP schema compatibility, capability gating, `citation.url`,
warnings, cursors/pagination, and env-based auth. Full surface unchanged. Default flip
is a deliberate, migration-noted public-default break.

**Scale/Scope**: 5 compact tools / ~16 operations over the existing service methods;
no new OpenGrok behavior. Resolved unknowns: discriminated-schema composition (research
D2, one spike), scalar coercion under flattened schemas (D3).

## Constitution Check

*GATE: must pass before Phase 0 and re-checked after Phase 1 design. Result: **PASS**
(one justified public-default break — see Complexity Tracking).*

- **MCP Contract**: Changed — compact tool set (4 tools; memory tool omitted), operation sets, **typed
  discriminated input schemas**, removal of `opengrok_compound`, `cross_project`
  rename, new `projects.files`/`.overview`, and **default surface → compact**. Output
  fields, warnings, cursors, citations preserved (FR-012). Full + gateway unchanged.
  Coherence (Principle I) held by generating compact schemas from the same input
  structs as full.
- **OpenGrok Semantics**: No semantic change. Best-effort/heuristic/page-local
  behaviors (implementations, cross-project, overview) keep their warnings (Principle
  II). Validated against the real instance (FR-017).
- **Test Evidence**: Test-first. New/extended tests that fail against old behavior:
  registration/schema tests (tool set, discriminated schemas, capability gating);
  compact handler tests (dispatch, flattened decode, errors); `evals/surface.go`
  resolver tests (new mapping, no `files.list` skip); `TestEvalSuite` parameterized
  on compact; cross-surface equivalence assertion; config default test. Targeted
  commands in `quickstart.md`.
- **Agent UX Validation**: Fresh-subagent first-use probe defined in `quickstart.md`
  (realistic task, minimal context) — required gate G17 (Principle I).
- **Security**: No change. Secrets stay in env; HTTP loopback-first; memory is full-only
  (compact omits it, FR-014), stdio-only and disabled over HTTP. No new flags/logs.
- **Compatibility and Docs**: Default-default break (full→compact) with migration note
  + restore path (FR-009/010/011). Docs to update: README, `docs/configuration.md`,
  `docs/tool-contracts.md`, `docs/limitations.md`, `docs/agent-usage-patterns.md`,
  `evals/README.md`, `CHANGELOG.md`, migration note.
- **Experimental Surface**: Remove compact's experimental/non-default framing
  (FR-016); gateway stays experimental. Net experimental surface unchanged.
- **Resource Bounds**: No new auto-fetch or response-size behavior; existing
  page-size/truncation/warning limits carry over unchanged (FR-012). Token cost
  bounded by SC-005 and measured by the benchmark.

## Project Structure

### Documentation (this feature)

```text
specs/006-compact-surface-default/
├── plan.md              # This file
├── research.md          # Phase 0 — decisions D1–D7, laws/anti-patterns
├── data-model.md        # Phase 1 — entities + operation inventory
├── quickstart.md        # Phase 1 — go/no-go gate + verification commands
├── contracts/
│   ├── tool-interface-spec.md   # consolidated tools, typed schemas, descriptions
│   └── migration-map.md          # prior-compact/full → new compact mapping
├── checklists/
│   └── requirements.md  # spec quality checklist (from /speckit-specify)
└── tasks.md             # Phase 2 — created by /speckit-tasks (NOT here)
```

### Source Code (repository root)

```text
cmd/opengrok-go-mcp/            # startup capability probing (unchanged behavior)
internal/config/
  config.go                    # Default(): ToolSurfaceFull -> ToolSurfaceCompact (flip, last)
internal/mcpserver/
  register_compact.go          # 5 consolidated tools; set discriminated InputSchema
  compact.go                   # dispatch: flattened decode per operation; projects/overview ops
  compact_schema.go (new)      # compose discriminated *jsonschema.Schema from input structs
  coerce.go                    # register per-tool scalar-field union (D3)
  types.go                     # compact input types (flattened or raw + per-op decode)
  register.go                  # surface switch (unchanged)
docs/                          # configuration, tool-contracts, limitations, agent-usage
evals/
  surface.go                   # resolveCompact -> new tools/ops; drop files.list skip
  scenarios.go / *_test.go     # parameterize TestEvalSuite over surfaces; equivalence assert
  baselines/                   # commit compact baseline
README.md                      # default surface + setup
```

**Structure Decision**: Single Go MCP server; all changes live in
`internal/mcpserver/` (surface), `internal/config/` (default), `evals/` (measurement),
and docs. No new packages except an internal `compact_schema.go` helper (Constitution
V: prefer existing patterns; one small, idiomatic addition for schema composition).

## Build sequence (safety-first ordering — implementation-playbooks)

Order **is** the safety property. The default flip is the optimization; it ships only
after compact is proven equivalent and parity-complete (research D4; AP#3 fix). Detailed
tasks come from `/speckit-tasks`; the load-bearing ordering:

1. **Schema composition + typed compact tools** (full untouched): `compact_schema.go`,
   flattened dispatch, descriptions in-step. Gate: registration/schema tests green (G1–G4).
2. **Errors + capability gating + response states** under the new shape. Gate: G5, G8, G10.
3. **Parity + overlap removal**: `projects.files`/`.overview`; move references into
   `opengrok_symbols`; remove `opengrok_compound`. Gate: G6, migration-map coverage.
4. **Eval first-class**: parameterize `TestEvalSuite` over surfaces, drop `files.list`
   skip, add compact-specific cases, add cross-surface equivalence assertion, commit
   compact baseline. Gate: G7, G11, G12, G13.
5. **Docs + migration note**; remove experimental framing. Gate: G15, G16.
6. **Flip the default** (`config.Default()`), confirm `full` still selectable/stable.
   Gate: G14 — only after 1–4 are PASS.
7. **Validation**: real-instance check, fresh-subagent probe, agent-ergonomics review.
   Gate: G17, G18; Definition of Done.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Public-default break (`full` → `compact`) | The feature's explicit, clarified goal: make compact the agent-first default | Keeping `full` default leaves the smaller, less ambiguous surface non-default; the spec + migration note justify the break per Constitution V. Mitigated by: `full` still selectable + byte-for-byte stable, migration note, restore path, and flip sequenced last behind equivalence/parity gates. |
| New internal file `compact_schema.go` | Compose discriminated `*jsonschema.Schema` from input structs | Hand-written JSON Schema literals would duplicate field defs and drift from full (anti-pattern #3); generating from the existing structs is the smaller-risk, single-source-of-truth path. |
