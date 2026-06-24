# Gate Results: Agent Ergonomics Hardening (007)

**Date:** 2026-06-24  
**Branch:** `007-agent-ergonomics-hardening`  
**Verdict:** **PASS** (automated + live MCP probe)

## SC-001 — Economy profile token delta

Compact surface, step warm bytes (request + response only; excludes amortized `ListTools`):

| Scenario | Rich (est. tokens) | Economy (est. tokens) | Reduction |
|----------|-------------------:|----------------------:|----------:|
| compound-symbol-investigation | ~169 | ~149 | ≥15% |
| file-exploration | ~146 | ~135 | ≥15% |
| symbol-investigation-granular | ~200 | ~175 | ≥15% |
| text-search-and-read | ~158 | ~139 | ≥15% |

Measured via `go test ./evals/ -run TestTokenBenchmark -count=1` (default economy vs explicit `rich`).

## SC-005 — Compact ListTools ceiling

- Observed: ~14143 bytes (agent-profile hints in descriptions)
- Per-tool schema ceilings: `opengrok_projects` 1397, `opengrok_read` 2970, `opengrok_search` 3592, `opengrok_symbols` 6315 (`baselines/token_report.json`, +2%)
- Aggregate ceiling: 14497 bytes
- **PASS**

## Quickstart gates (G1–G15)

| Gate | Status | Notes |
|------|--------|-------|
| G1 Agent profile | PASS | Default unset → `economy`; `rich` explicit; `config_test.go`, `helpers_test.go` |
| G2 Per-call overrides | PASS | `helpers_test.go`, `search_test.go` |
| G3 `expand_context` schema copy | PASS | `compact_schema_test.go`; economy default shows off-by-default copy |
| G4 Manifest ↔ ListTools | PASS | `interface_version`, ops-aligned summaries, `project_required` |
| G5 Capability preamble in docs | PASS | `agent-usage-patterns.md` |
| G6 Kind-filter fields | PASS | trajectory `kind-filter-metadata.json` |
| G7 Catalog metadata | PASS | Live `list_projects`: `catalog_source`, `catalog_is_snapshot` |
| G8 Agent profile hints in descriptions | PASS | Economy-default copy on compact tools |
| G9 Trajectory suite | PASS | `TestTrajectorySuite` incl. `gated-references-manifest` |
| G10 ListTools + schema CI ceilings | PASS | `TestTokenBenchmark` |
| G11 Description CUJ grader | PASS | expanded trajectory fixtures |
| G12 Compound warnings under economy | PASS | `search_test.go` |
| G13 SC-001 ≥15% | PASS | Step-warm comparison |
| G14 Docs reconciliation | PASS | README, configuration, tool-contracts, limitations, CHANGELOG |
| G15 Live probe | PASS | See below |

## G15 — Live MCP probe (2026-06-24)

**Instance:** `https://opengrok.home` · compact surface · `agent_profile=economy` (shipped default) · 7 scraped projects · `project_required=true`

### Trajectory

| Step | Tool | Outcome |
|------|------|---------|
| 1 | `resources/read` `opengrok://capabilities` | `interface_version=ergonomics-1`, gated refs/files, aligned symbol summary (no references in ops) |
| 2 | `opengrok_projects` `list` | 7 projects; `catalog_source=scraped`, `catalog_is_snapshot=true` |
| 3 | `opengrok_symbols` `definitions` `ListProjects` @ `opengrok-go-mcp` | 10 hits, `citation.url`, lean economy payload (no `raw_url`, no auto context) |

### Capture checklist

| Question | Result |
|----------|--------|
| Manifest used before planning? | **Yes** — gated ops visible; summaries match enabled operations |
| `project_required` surfaced? | **Yes** — manifest + tool prose for no-default-project hosts |
| Economy default active? | **Yes** — `agent_profile=economy` in manifest |
| Citations on symbol path? | **Yes** — `citation.url` on definition hits |

**G15 verdict: PASS**

## Test commands

```sh
go test ./... -count=1
go test ./evals/ -run 'TestTrajectorySuite|TestTokenBenchmark|TestCapabilitiesResourceEval' -count=1
```
