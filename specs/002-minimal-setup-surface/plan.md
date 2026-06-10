# Implementation Plan: Minimal Setup Surface

**Branch**: `002-minimal-setup-surface` | **Date**: 2026-06-10 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-minimal-setup-surface/spec.md`

## Summary

Reduce operator setup to **one mandatory environment variable** (`OPENGROK_MCP_BASE_URL`) by
(1) enabling web-UI project discovery by default when the REST project list fails, exposed via a
single opt-out flag `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE`; (2) never requiring
`OPENGROK_MCP_DEFAULT_PROJECT` at startup; (3) allowing startup to complete when search probes
fail with unauthorized responses and no auth token is configured, emitting an actionable auth
remediation log instead of exiting with `no search capabilities are available`; and (4) updating
README/docs/examples to show base-URL-only setup as the primary path.

Implementation reuses the existing discovery ladder (`configured → api → scraped → none`) and
capability probe infrastructure from `001-project-discovery-tls-proxy-robustness`; changes are
concentrated in `internal/config`, `cmd/opengrok-go-mcp/main.go`, tests, and documentation.

## Technical Context

**Language/Version**: Go 1.25.0

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp`; OpenGrok HTTP API and
server-rendered web UI; existing `golang.org/x/net/html` scrape parser (no new deps).

**Storage**: Process-scoped configuration and startup snapshots only.

**Testing**: `go test ./internal/config/`; `go test ./cmd/opengrok-go-mcp/`; `go test ./...`

**Target Platform**: Local stdio MCP server and loopback HTTP transport.

**Project Type**: Go CLI / MCP server.

**Performance Goals**: No change to per-request latency. At most one extra startup web GET when
API project discovery fails (same bounded fetch as today's opt-in scrape). API-success path must
not regress (zero scrape).

**Constraints**: MCP tool schemas unchanged; capability gating remains honest; secrets env-only;
backward compat for explicit project list/default and legacy scrape env var (compat shim).

**Scale/Scope**: Startup/config/docs only; no pagination, citation, or search semantic changes.

## Constitution Check

*GATE: passed at planning. Re-check after implementation.*

- **MCP Contract**: No tool input/output schema changes. Tool **availability** may differ when
  probes fail (unchanged gating model). Startup logs gain canonical auth remediation text (operator
  contract, not agent schema). `list_projects` continues to serve startup snapshot. All surfaces
  (`full`, `compact`, `gateway`) share the same capability gates.
- **OpenGrok Semantics**: Scraped lists remain best-effort/heuristic; documented in limitations.
  Discovery precedence unchanged. No new claims about search semantics.
- **Test Evidence**: See **Test Plan**. Ordered **test-first** per behavioral slice. Primary
  packages: `internal/config`, `cmd/opengrok-go-mcp`.
- **Agent UX Validation**: Fresh mid-tier subagent task with **only** `OPENGROK_MCP_BASE_URL` in
  env (mock or staging): *"List OpenGrok projects and search for a symbol in one of them."*
  Observe: server starts, `list_projects` usable, auth message clear if probes gated, no need to
  read configuration docs first.
- **Security**: Auth tokens remain env-only; dual-token rejection unchanged. Default-on scrape
  adds one startup GET to web UI when API fails — same auth path as existing scrape (`addAuth`).
  No inbound HTTP auth changes.
- **Compatibility and Docs**: **Breaking default change** for scrape (off→on when API fails);
  documented in CHANGELOG + migration note. Legacy `OPENGROK_MCP_PROJECT_SCRAPE` shim for one
  release. Updates: `README.md`, `docs/configuration.md`, `docs/limitations.md`, `CHANGELOG.md`,
  client setup examples in README.
- **Experimental Surface**: Scrape toggle no longer labeled experimental; scraped **lists** remain
  best-effort in docs/logs. New env var is stable opt-out, not experimental.
- **Resource Bounds**: Unchanged — one bounded startup scrape (8 MiB cap), probe timeouts via
  `ReadTimeout`.

No constitution violations → **Complexity Tracking is empty**.

## Project Structure

### Documentation (this feature)

```text
specs/002-minimal-setup-surface/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── configuration-contract.md
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/opengrok-go-mcp/
  main.go            # scrape default gate; relaxed validateDefaultProjectAfterDiscovery;
                     # detectCapabilities auth-only soft-start; auth remediation log;
                     # updated startup diagnostics
  main_test.go       # ladder + capability + auth-remediation table tests
internal/config/
  config.go          # OPENGROK_MCP_DISABLE_PROJECT_SCRAPE; legacy shim; Default() scrape=true;
                     # remove multi-project default required in Validate()
  config_test.go     # disable flag parsing; legacy compat; validation relaxation tests
docs/
  configuration.md   # one required var; disable flag; migration
  limitations.md     # scrape default-on note; zero-project startup
README.md            # minimal one-line examples; optional auth section
CHANGELOG.md         # breaking default + migration
AGENTS.md            # SPECKIT active plan pointer
```

**Structure Decision**: Single-package behavioral change in existing `cmd` + `config` boundaries;
no new packages; scrape implementation in `internal/opengrok` unchanged.

## Implementation Design

### 1. Config layer (`internal/config/config.go`)

- Add `ProjectScrapeDisabled bool` (or keep single `ProjectScrapeEnabled` with inverted default).
- Parse `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` via existing `parseBoolEnv`.
- Legacy shim: if disable unset, honor explicit `OPENGROK_MCP_PROJECT_SCRAPE`; else default
  `ProjectScrapeEnabled = true`.
- `Default()`: set `ProjectScrapeEnabled: true`.
- `Validate()`: remove block requiring `OPENGROK_MCP_DEFAULT_PROJECT` when `len(Projects) > 1`.
  Keep validation that explicit default ∈ explicit project list when both set at parse time.
- Update `RegisterFlags` only if a disable flag is exposed on CLI (optional; env-only is fine per
  security rules — prefer env-only for scrape disable).

### 2. Project resolution (`cmd/.../main.go` — `resolveProjectAllowlist`)

- Invert gate: scrape when `cfg.ProjectScrapeEnabled` (default true) instead of checking disabled
  experimental toggle.
- Update log strings:
  - Remove "experimental" from default scrape path; use "web-UI discovery; best-effort".
  - When disabled: `web-UI project discovery disabled (OPENGROK_MCP_DISABLE_PROJECT_SCRAPE)`.

### 3. Post-discovery validation (`validateDefaultProjectAfterDiscovery`)

- **case 0**: return `nil` always (explicit `DefaultProject` preserved).
- **case 1**: auto-set default (unchanged).
- **case default (N>1)**: return `nil` when `DefaultProject == ""`; keep allowlist membership check
  when default is set.

### 4. Capability detection (`detectCapabilities`)

- Track per search probe: success + classification.
- Replace hard error at end:
  ```go
  if !caps.SearchCode && !caps.SearchSymbolDefinitions && !caps.SearchSymbolReferences {
      return caps, errors.New("check OpenGrok access: ...")
  }
  ```
  With:
  - If **any** search probe succeeded → return `(caps, nil)`.
  - If all failed and `authRemediationNeeded()` → log remediation line; return `(caps, nil)`.
  - If all failed and not auth-only → return `(caps, err)` with existing message (TLS/transport).

- Helper `authRemediationNeeded(noToken, probes []probeResult) bool`:
  all search probes failed AND every failure classified `unauthorized` (or `endpoint_disabled` with
  `anyAuthedProbeSucceeded == false`) AND no token configured.

- Do **not** emit auth warning when at least one probe succeeded (FR-010).

### 5. Startup diagnostics (`logStartupDiagnostics`)

- Include `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` in override list; deprecate listing legacy scrape
  enable in primary docs.
- Log scrape state: `project scrape=default-on` or `project scrape=disabled`.

### 6. Documentation

- `docs/configuration.md`: Required table = base URL only; add disable flag; demote default project.
- `README.md`: Replace multi-var examples with one-line base URL block; "Add auth if needed"
  subsection; migration blurb.
- `docs/limitations.md`: Note default scrape fallback and zero-project startup behavior.
- `CHANGELOG.md`: Breaking change entry with migration table (from `contracts/configuration-contract.md`).

## Test Plan

| ID | Package | Test | Proves |
|---|---|---|---|
| T1 | config | `Default()` has `ProjectScrapeEnabled == true` | FR-005 default-on |
| T2 | config | `DISABLE_PROJECT_SCRAPE=true` → scrape off | opt-out |
| T3 | config | Legacy `PROJECT_SCRAPE=false` → scrape off; disable wins over legacy | compat |
| T4 | config | `Validate()` allows multi-project env without default | FR-002/FR-003 |
| T5 | cmd | `resolveProjectAllowlist`: API fail + scrape default → scraped source | US1/US3 |
| T6 | cmd | `resolveProjectAllowlist`: API success → no scrape | SC-003 |
| T7 | cmd | `resolveProjectAllowlist`: disable + API fail → source none, no scrape call | US3 |
| T8 | cmd | `validateDefaultProjectAfterDiscovery`: 0 projects, no default → nil | edge case |
| T9 | cmd | `validateDefaultProjectAfterDiscovery`: N>1, no default → nil | FR-003 |
| T10 | cmd | `detectCapabilities`: all search 401, no token → nil error + caps false | FR-008/SC-002 |
| T11 | cmd | `detectCapabilities`: all search 401, no token → was hard error (regression guard) | SC-002 |
| T12 | cmd | `detectCapabilities`: TLS failure all probes → still error | no over-relaxation |
| T13 | cmd | `detectCapabilities`: anonymous success → no auth log | FR-010 |
| T14 | integration | Update/remove tests expecting startup fail on missing default | FR-002 |

Run order per slice: write failing test → implement → `go test ./internal/config/` or
`go test ./cmd/opengrok-go-mcp/` → `go test ./...`.

## Agent UX Validation Plan

Before merge, run fresh-subagent simulation:

- **Env**: `OPENGROK_MCP_BASE_URL` only (+ auth if staging requires it).
- **Task**: "List projects and run a full-text search for a known string."
- **Pass**: Completes without operator reading configuration docs; if tools missing, subagent
  report cites auth log env var names correctly.
- **Capture**: Findings in PR description or `docs/agent-usage-patterns.md` if workflow guidance
  changes.

## Complexity Tracking

> Empty — no constitution violations.

## Quickstart Reference

See [quickstart.md](./quickstart.md) for operator-facing minimal setup and migration.

## Contracts Reference

See [contracts/configuration-contract.md](./contracts/configuration-contract.md) for env var and
startup behavior contract.

## Next Step

Run **`/speckit-tasks`** to generate dependency-ordered `tasks.md`.
