# Contract: Trajectory Eval & ListTools Gate

**Feature**: 007-agent-ergonomics-hardening | **Date**: 2026-06-24
**Audience**: maintainers / CI. **Status**: design.

Shaped with **evaluation-harness-designer**: measure agent-observable trajectories,
not only final JSON fields (anti-pattern #4).

## Trajectory suite

**Location**: `evals/testdata/trajectory/*.json`  
**Runner**: `TestTrajectorySuite` in `evals/trajectory_test.go`

### Minimum coverage (FR-012, SC-004)

| Scenario ID | Origin | Graders (min) |
|-------------|--------|---------------|
| `symbol-investigation-compact` | extend token scenario | `tool_sequence`, `citation_present` ×2 |
| `search-narrow-warnings` | new fixture | `warning_code` (`HIGH_HIT_COUNT`) |
| `kind-filter-metadata` | new fixture | `field_present` ×3 on kind fields |
| `description-cuj-symbol` | new | `description_cuj` (task → `opengrok_symbols`) |

**Total**: ≥3 scenarios, ≥8 graders.

### Grader types

| Type | Args | Pass condition |
|------|------|----------------|
| `tool_sequence` | `tools: []` | Actual call order matches prefix |
| `warning_code` | `value: string`, `step_index` | `warnings[].code` contains value |
| `citation_present` | `step_index`, `field` | Dotted path non-empty on all items |
| `field_present` | `step_index`, `field` | Path exists |
| `field_eq` | `step_index`, `field`, `value` | Equality |
| `description_cuj` | `task`, `expect_tool` | Resolver picks tool (static map v1) |

### Profile dimension

At least one trajectory case runs with `OPENGROK_MCP_AGENT_PROFILE=economy` and
asserts expansion diagnostics absent/skipped while citations remain.

## ListTools ceiling (FR-013, SC-005)

**Policy file**: `evals/baselines/token_report.json`

```json
"compact_list_tools_ceiling_bytes": <int>
```

**CI rule** (`TestTokenBenchmark`):

```
compact list_tools_bytes <= compact_list_tools_ceiling_bytes
```

Default ceiling = current baseline + 2% at feature merge; intentional increases
require baseline refresh in same PR with justification in CHANGELOG.

**Escape hatch**: `OPENGROK_MCP_EVAL_LISTTOOLS_CEILING` env for local override only
(not used in CI).

## Description CUJ (FR-014)

Static map in `evals/description_cuj.go` (v1):

| Task label | Expected first compact tool |
|------------|----------------------------|
| `find_symbol_definition` | `opengrok_symbols` |
| `search_code_text` | `opengrok_search` |
| `read_known_file` | `opengrok_read` |
| `list_projects` | `opengrok_projects` |

Test fails if compact tool descriptions are edited to misroute the resolver's
keyword rules (guards anti-pattern #3). Map updated only with deliberate review.

## CI integration

- `go test ./evals/ -count=1` runs contract + trajectory + token gate.
- Trajectory failures print step index and grader type.
- Token gate failure prints bytes, ceiling, and delta.

## Out of scope v1

- LLM-as-judge trajectories
- Production trace ingestion
- Gateway as primary ceiling target

## Fresh-subagent gate (manual)

See `quickstart.md` G7 — not automated in CI; required before release tag.
