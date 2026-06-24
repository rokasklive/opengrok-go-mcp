# Go/No-Go Gate Results — 006-compact-surface-default

**Date:** 2026-06-24  
**Verifier:** implementation polish pass (T033–T037, T039)

## Verification commands (executed)

| Command | Result |
|---|---|
| `go test ./... -count=1` | PASS |
| `go test ./evals/ -count=1` | PASS |
| `go test ./evals/ -run TestTokenBenchmark -count=1` | PASS |
| `go test ./internal/mcpserver/ -run 'Compact\|Register' -count=1` | PASS |
| `git diff --check` | PASS (no whitespace errors) |

## Live instance check (FR-017) — `https://opengrok.home/`

**Env:** `OPENGROK_MCP_BASE_URL=https://opengrok.home/api/v1/`, default surface (compact, unset).

**Startup capabilities observed:**

| Capability | Status |
|---|---|
| `list_projects` | enabled (scraped, 7 projects) |
| `search_code` | enabled |
| `search_symbol_definitions` | enabled |
| `search_symbol_references` | disabled (400) |
| `get_file_context` | enabled (raw web fallback) |
| `list_files` | disabled (401) |
| `list_symbols` | enabled (via definitions) |

**Cold journey results:**

| Step | Call | Result |
|---|---|---|
| ListTools | — | **4 tools:** `opengrok_projects`, `opengrok_search`, `opengrok_symbols`, `opengrok_read` (no memory) |
| projects.list | `{"operation":"list"}` | PASS — 7 scraped projects, citations present |
| search.code | `NewMCPServer`, `page_size:1` | PASS — 1 result, `citation.url`, truncation `warning` when OG over-delivers |
| symbols.definitions | `NewMCPServer` | PASS — definition hit + `citation.url` |
| read.context | `main.go:1` | PASS — content + `citation` |
| projects.overview | — | **N/A on this instance** — `list_files` disabled; op not in schema |

**Instance note:** `PaymentProcessor` not indexed in `opengrok-go-mcp` on this host (0 hits); journey used `NewMCPServer` instead.

## Gate table (G1–G18)

| # | Item | Verdict | Evidence |
|---|---|---|---|
| G1 | 3–4 compact tools, no overlap | **PASS** | `TestCompactSurfaceRegistersFourConsolidatedTools`; live ListTools = 4 |
| G2 | No `opengrok_compound` | **PASS** | registration tests; live tool list |
| G3 | Schema-discoverable fields | **PASS** | `compact_schema_test.go`, eval schema cases |
| G4 | Descriptions ≥ full depth | **PASS** | `compact_descriptions.go`; description↔enum test |
| G5 | Actionable invalid-op/field errors | **PASS** | `TestCompactOperationRoutingAndErrors`, eval negatives |
| G6 | Capability parity (excl. memory) | **PASS** | migration-map + eval coverage; `projects.files`/`overview` gated on `ListFiles` |
| G7 | Cursors/totals/warnings/citations | **PASS** | equivalence assertion; live citation URLs |
| G8 | Capability gating tool + op level | **PASS** | `TestCompactToolGating*`, `TestCompactSymbolsDescriptionMatchesEnabledOperations` |
| G9 | No memory on compact | **PASS** | registration test; live ListTools |
| G10 | Six response states | **PASS** | eval response-state coverage |
| G11 | Eval suite on compact | **PASS** | `go test ./evals/` |
| G12 | Cross-surface equivalence | **PASS** | `equivalence.go` green |
| G13 | Compact baseline + CI | **PASS** | `evals/baselines/` committed |
| G14 | Default = compact; full stable | **PASS** | `config_test.go` |
| G15 | Migration note + map | **PASS** | `docs/migration-compact-default.md`, `docs/README.md` row added |
| G16 | Experimental framing removed | **PASS** | compact descriptions; gateway still labeled in docs |
| G17 | Fresh-subagent UX probe | **CONDITIONAL-PASS** | See [T036 report](#t036-fresh-subagent-ux-probe) — obvious tool picks on probed instance; `overview` blocked when `ListFiles` off |
| G18 | Agent-ergonomics review | **PASS** | See [T037 report](#t037-agent-ergonomics-review) — no Critical; T&I=8, Economic=8 |

**Overall gate:** **CONDITIONAL-PASS** — all behavioral gates G1–G14 PASS; G17 conditional on instance capabilities (expected); G18 PASS.

---

## T033 — `docs/tool-contracts.md`

Added **Tool Surfaces** section: compact default, 4-tool inventory, flattened call shape, memory full-only gap, description↔schema coherence. Updated `page_size` truncation semantics.

## T035 — `docs/README.md` reconciliation

| Row | Action |
|---|---|
| constitution.md | N/A |
| AGENTS.md | Updated (active feature pointer) — prior pass |
| configuration.md | Updated (default compact) — prior pass |
| tool-contracts.md | Updated (this pass) |
| agent-ux.md | N/A |
| agent-usage-patterns.md | Updated flattened examples + `overview` op (this pass) |
| limitations.md | Updated — prior pass |
| review-checklist.md | N/A |
| release-process.md | N/A |
| release.yml | N/A |
| CHANGELOG.md | Updated — prior pass |
| SECURITY.md | N/A |
| reporting-issues.md | N/A |
| ISSUE_TEMPLATE | N/A |
| PR template | N/A |
| evals/README.md | Updated — prior pass |
| migration-compact-default.md | **Added map row** (this pass) |

## T036 — Fresh-subagent UX probe

Simulated cold agent task: *find `PaymentProcessor`, read context, cite languages*.

**Tool selection:** 5/5 for symbols (`definitions` or `find`); 4–5 for read; 5 for `overview` when `ListFiles` probed; **1–2** when `list_files` disabled (live instance).

**Schema legibility:** PASS on flattened shape; minor friction on `oneOf` branch introspection and optional `line_number` for `context`.

**G17:** CONDITIONAL-PASS (environment-dependent `overview` availability).

## T037 — Agent-ergonomics review

| Domain | Score |
|---|---|
| Tool & Interface Design | 8 |
| Information Architecture | 7 |
| Workflow & Coordination | 7 |
| Economic Design | 8 |
| Evaluation & Measurement | 8 |

**Critical findings:** none.  
**Major (tracked, not blocking):** F1 `context` schema should require `line_number`; F2 memory discoverability on full surface; F3 `opengrok_symbols` oneOf size.

**G18:** PASS (T&I ≥ full, Economic ≥ full, no Critical).
