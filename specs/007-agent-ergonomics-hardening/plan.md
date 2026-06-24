# Implementation Plan: Agent Ergonomics Hardening

**Branch**: `007-agent-ergonomics-hardening` | **Date**: 2026-06-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/007-agent-ergonomics-hardening/spec.md`

**Ergonomic framing**: Implementation order and contracts follow the agent-
ergonomics inspector laws — **L1** (economy profile, ListTools gate), **L2**
(manifest, schema copy, kind metadata, description CUJ), **L3** (catalog snapshot
signals), **L5** (trajectory eval as the leverage layer). See [research.md](research.md).

## Summary

Harden the compact-default MCP surface for cold agents by: (1) an opt-in **economy
profile** that bundles token-frugal defaults while preserving per-call overrides and
`citation.url`; (2) a runtime **`opengrok://capabilities`** manifest aligned with
live tool registration; (3) **additive output fields** for kind-filter and project
catalog semantics; (4) **tool-header economy hints** and accurate `expand_context`
schema copy; (5) **trajectory eval graders** and a **compact ListTools CI ceiling**.
Ship in waves (additive → profile → manifest → eval gates). Shipped default is
`economy`; set `OPENGROK_MCP_AGENT_PROFILE=rich` for expanded defaults.

## Technical Context

**Language/Version**: Go 1.24

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp` v1.4.0;
`github.com/google/jsonschema-go` for schema inference/patching; OpenGrok HTTP API.

**Storage**: In-memory only; `CapabilityReport` snapshot at startup; no durable
agent state.

**Testing**: Test-first per wave — `go test ./internal/mcpserver/`,
`go test ./internal/config/`, `go test ./evals/ -count=1`; full `go test ./...`
before merge.

**Target Platform**: stdio MCP (primary) + loopback HTTP.

**Performance Goals**: Economy profile ≥15% warm token reduction on four benchmark
scenarios (SC-001); compact `ListTools` bytes must not exceed committed ceiling
(SC-005).

**Constraints**: Additive output fields only; preserve `citation.url`, `warnings[]`,
`error_code`, cursors, capability gating. Secrets in env only. Manifest never logs
or returns credentials.

**Scale/Scope**: All compact/full surfaces; gateway manifest entry only; memory
tools unchanged (full-only).

## Constitution Check

*GATE: PASS (pre-research and post-design).*

| Principle | Impact | Mitigation |
|-----------|--------|------------|
| **I MCP Contract** | New resource URI; additive JSON fields; env var; description/schema text; profile affects defaults not wire format | Migration note only if default profile flips later; contracts/ documents additive rules |
| **II OpenGrok Semantics** | No query changes; kind filter still page-local — made *more* explicit | `kind_filter_*` fields + existing warning |
| **III Test-Proven** | Every FR maps to unit + eval tests; trajectory + token gate new | Test-first waves in research D10 |
| **IV Secure** | Manifest remediation = env names only | Code review + manifest redaction test |
| **V Compatibility** | Unset profile = rich (today); additive fields | No required input changes |
| **Agent UX Validation** | Fresh-subagent probe in quickstart G7 | Before release tag |
| **Experimental** | Gateway indirection documented in manifest | No gateway behavior change |
| **Resource Bounds** | Economy profile *reduces* default fetch/expansion | Compound warnings preserved (FR-015) |

**Post-design re-check**: No unjustified violations. Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/007-agent-ergonomics-hardening/
├── plan.md              # This file
├── research.md          # Phase 0 — ergonomic decisions
├── data-model.md        # Phase 1 — entities
├── quickstart.md        # Go/no-go gate
├── contracts/           # Agent-facing contracts
│   ├── agent-profile-contract.md
│   ├── capability-manifest-contract.md
│   ├── kind-filter-catalog-contract.md
│   └── trajectory-eval-contract.md
├── gate-results.md      # Created at implementation end
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (touch map)

```text
cmd/opengrok-go-mcp/main.go          # CapabilityReport at probe time
internal/config/config.go            # AgentProfile env parse
internal/config/capability_report.go # NEW — structured probe outcomes
internal/mcpserver/helpers.go        # Profile-aware defaults; UNKNOWN_PROJECT copy
internal/mcpserver/resources.go      # opengrok://capabilities
internal/mcpserver/manifest.go       # NEW — build manifest from cfg
internal/mcpserver/symbols.go        # Kind-filter output fields
internal/mcpserver/projects.go       # Catalog metadata
internal/mcpserver/compact_descriptions.go  # Economy hints + disambiguation
internal/mcpserver/register_compact.go      # Schema description patching
internal/mcpserver/register_full.go           # Same patches where shared
internal/mcpserver/compound.go       # FR-015 warning audit
evals/trajectory_test.go             # NEW
evals/graders.go                     # NEW
evals/description_cuj.go             # NEW
evals/token_benchmark_test.go        # ListTools ceiling
evals/baselines/token_report.json    # ceiling field
docs/agent-usage-patterns.md
docs/configuration.md
docs/tool-contracts.md
docs/limitations.md
README.md
CHANGELOG.md
```

**Structure Decision**: Extend existing packages; no new top-level modules. Manifest
builder colocated with `resources.go` pattern in `internal/mcpserver`.

## Implementation Waves

Aligned with **implementation-playbooks** — each wave mergeable independently.

### Wave 1 — Additive outputs & copy (P3 quick wins)

**Goal**: L2 clarity without behavior change for rich-default deployments.

1. `ListProjectsOutput` + `UNKNOWN_PROJECT` message (`catalog_*`, snapshot hint).
2. `ListSymbolsOutput` kind metadata fields.
3. Tool-header economy hints + `response_mode` vs surface disambiguation in
   `compact_descriptions.go` (and full register descriptions).
4. `agent-usage-patterns.md` capability preamble → `opengrok://capabilities`.
5. Docs: `tool-contracts.md`, `limitations.md` field entries.

**Tests first**: `projects_test.go`, `symbols_test.go` extensions; doc whitespace
`git diff --check`.

### Wave 2 — Agent profile (P1)

**Goal**: L1/L5 — structural token savings on opt-in.

1. `OPENGROK_MCP_AGENT_PROFILE` in config with validation.
2. `resolveResponseModeDefault`, profile-aware `shouldExpandContext` /
   `includeLinks`.
3. Patch `expand_context` jsonschema descriptions at registration.
4. Token benchmark: record rich vs economy in `token_report.json`; validate SC-001.
5. `docs/configuration.md`, README env table.

**Tests first**: `config_test.go`, helper tests, `results_test.go` economy paths,
compound warning retention test.

### Wave 3 — Capability manifest (P1)

**Goal**: L2 runtime ground truth.

1. `config.CapabilityReport` populated in `main.go` from probe functions.
2. `BuildCapabilityManifest(cfg)` → JSON.
3. Register `opengrok://capabilities` always (handler uses report).
4. Eval: read resource on gated + full fixtures.

**Tests first**: `manifest_test.go`, `resources_test.go`; hermetic gated-capability
eval case.

### Wave 4 — Trajectory eval & ListTools gate (P2)

**Goal**: L5 measurement layer.

1. `evals/graders.go`, `trajectory_test.go`, `testdata/trajectory/*.json`.
2. `description_cuj.go` + seeded regression test.
3. `compact_list_tools_ceiling_bytes` in baseline; fail `TestTokenBenchmark` on
   exceed.
4. CI already runs `go test ./evals/` — no workflow change required.

**Tests first**: trajectory cases red before implementation green.

### Wave 5 — Validation & gate

1. Run quickstart commands; write `gate-results.md` with SC-001 numbers.
2. Fresh-subagent probe notes (G7).
3. Optional real-instance check on opengrok.home.

## Test Evidence Map

| FR | Primary tests |
|----|----------------|
| FR-001–003 | `config_test`, helper default tests, token benchmark rich baseline |
| FR-004 | `register_test` / `compact_schema_test` description assertions |
| FR-005–006 | `manifest_test`, resource read eval |
| FR-007 | Doc review + manifest eval |
| FR-008 | `symbols_test`, trajectory `kind-filter-metadata` |
| FR-009–010 | `projects_test`, `search_test` UNKNOWN_PROJECT |
| FR-011 | Description string tests in `compact_test` |
| FR-012–014 | `TestTrajectorySuite`, `description_cuj` |
| FR-013 | `TestTokenBenchmark` ceiling |
| FR-015 | `compound_test` / search expansion warning tests |

**Agent UX validation task** (Constitution I): see [quickstart.md](quickstart.md) G7.

## Complexity Tracking

> No constitution violations requiring justification.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |

## Artifacts Generated

| Artifact | Path |
|----------|------|
| Research | [research.md](research.md) |
| Data model | [data-model.md](data-model.md) |
| Contracts | [contracts/](contracts/) |
| Quickstart / gate | [quickstart.md](quickstart.md) |
| Tasks | *pending* `/speckit-tasks` |

**Next command**: `/speckit-tasks`
