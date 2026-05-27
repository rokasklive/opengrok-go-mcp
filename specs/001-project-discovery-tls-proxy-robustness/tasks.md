---
description: "Task list for Project Discovery & TLS/Proxy Robustness"
---

# Tasks: Project Discovery & TLS/Proxy Robustness

**Input**: Design documents from `/specs/001-project-discovery-tls-proxy-robustness/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (user stories). `research.md`,
`data-model.md`, and quickstart are folded into plan.md; `contracts/` is N/A (no new MCP tool/schema).

**Tests**: Behavioral changes use focused tests that fail against old behavior. Non-trivial slices
are ordered test-first per the plan.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency)
- **[Story]**: `US1` (P1, discovery ladder), `US2` (P2, failure diagnostics), `US3` (P3, transport fix)

## Story ↔ plan slice ↔ priority

| Story | Spec priority | Plan slice | Value |
|-------|---------------|------------|-------|
| US1 — Project discovery ladder | P1 | Slice C | **MVP value** |
| US2 — TLS/auth/disabled diagnostics | P2 | Slice B | Robustness |
| US3 — Forward-proxy compat w/ TLS-skip | P3 | Slice A | Robustness |

**Build order note**: stories are independent and individually testable. They are presented below in
the plan's recommended *build* order (US3 → US2 → US1, smallest-risk first to de-risk shared edits to
`cmd/opengrok-go-mcp/main.go`). The primary feature value lives in **US1**; an MVP could implement
US1 alone. Either order is valid because each story has its own proving tests.

---

## Phase 1: Setup

- [ ] T001 Add the `golang.org/x/net` dependency: `go get golang.org/x/net/html`, `go mod tidy`;
  confirm `go.mod`/`go.sum` updated and `go build ./...` + baseline `go test ./...` are green before
  changes. (Only US1 consumes it; added up front so the module graph is stable.)

---

## Phase 2: Foundational (confirmation gates — no shared code)

The three stories are file-independent except for separate edits to `cmd/opengrok-go-mcp/main.go`
(transport construction, discovery orchestration, probe logging). No shared production code blocks
the stories.

- [ ] T002 Sequence the `cmd/opengrok-go-mcp/main.go` edits across stories (US3 transport
  construction → US2 classification/logging → US1 discovery orchestration) so they do not conflict;
  these `main.go` tasks are NOT `[P]` with each other.
- [ ] T003 [P] Confirm the docs set to update (README.md, docs/configuration.md, docs/limitations.md,
  docs/tool-contracts.md, CHANGELOG.md) and the experimental-label + resource-bound requirements from
  the plan's Constitution Check.

**Checkpoint**: foundation ready — stories can proceed.

---

## Phase 3: User Story 3 — Forward-proxy compat with skipped TLS verify (Priority: P3) — plan Slice A

**Goal**: When `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY=true`, outbound requests still honor
`HTTPS_PROXY`/`HTTP_PROXY` (proxy-from-environment preserved). (FR-018)

**Independent Test**: With skip-verify on and a proxy configured via environment, requests traverse
that proxy and TLS verification is skipped.

### Tests (write first)

- [ ] T004 [P] [US3] In `cmd/opengrok-go-mcp/main_test.go`, add a test asserting the HTTP client
  built with skip-verify preserves proxy-from-environment (e.g. extract `newHTTPClient(cfg)` and
  assert its `*http.Transport.Proxy != nil` / routes via a configured proxy, and
  `TLSClientConfig.InsecureSkipVerify == true`). Confirm it fails against the current bare-transport
  construction.

### Implementation

- [ ] T005 [US3] In `cmd/opengrok-go-mcp/main.go`, replace the bare `&http.Transport{TLSClientConfig:…}`
  with `http.DefaultTransport.(*http.Transport).Clone()` then set
  `TLSClientConfig = &tls.Config{InsecureSkipVerify: true}`; extract a `newHTTPClient(cfg)` helper so
  the behavior is unit-testable. Keep the existing skip-verify warning log.
- [ ] T006 [US3] Run `go test ./cmd/...`; `gofmt -w` changed files.

**Checkpoint**: US3 complete and independently testable.

---

## Phase 4: User Story 2 — Diagnose TLS / auth / disabled / unsupported failures (Priority: P2) — plan Slice B

**Goal**: Startup probe failures are classified (`tls_mismatch` / `unauthorized` /
`endpoint_disabled` / `feature_unsupported` / `transport_error`) and, on a TLS mismatch, the log
names the certificate's valid hostname(s). (FR-016, FR-017)

**Independent Test**: A cert/hostname mismatch logs the cert SAN and a `tls_mismatch` classification
distinct from a `401`; a `400` unsupported search mode is distinguished from a `401`.

### Tests (write first)

- [ ] T007 [P] [US2] In `internal/opengrok/client_test.go`, assert a non-2xx response yields a
  `*opengrok.StatusError` with the correct `Code` (via `errors.As`). Fails against the current
  generic `unexpected status` error.
- [ ] T008 [P] [US2] In `cmd/opengrok-go-mcp/main_test.go`, table-test `classifyProbeError`:
  `401` + a prior authed-probe success → `endpoint_disabled`; `401` + no prior success →
  `unauthorized`; `400` → `feature_unsupported`; a `*tls.CertificateVerificationError` →
  `tls_mismatch` with expected SANs (build a self-signed cert with known `DNSNames` and dial a
  mismatched host via `httptest` TLS); other transport error → `transport_error`.

### Implementation

- [ ] T009 [US2] Add `internal/opengrok/errors.go`: `StatusError{Code int; Status, Path string}`
  implementing `error`.
- [ ] T010 [US2] In `internal/opengrok/client.go` `doGET`, return `*StatusError` for non-2xx
  (preserve message text where feasible); ensure transport/TLS errors remain wrapped so
  `*tls.CertificateVerificationError` survives `errors.As`.
- [ ] T011 [US2] In `cmd/opengrok-go-mcp/main.go`, add
  `classifyProbeError(err error, anyAuthedProbeSucceeded bool) (category string, certSANs []string)`;
  track whether any authenticated probe has succeeded (R5) and extend `logCapability` / startup
  logging to print the category and, for `tls_mismatch`, the cert SANs. Log secrets-free.
- [ ] T012 [US2] Run `go test ./internal/opengrok/ ./cmd/...`; `gofmt -w` changed files.

**Checkpoint**: US2 + US3 complete and independently testable.

---

## Phase 5: User Story 1 — Project discovery resolution ladder (Priority: P1) 🎯 MVP — plan Slice C

**Goal**: Resolve the project allowlist at startup via `configured → api → scraped → none`, with
opt-in web-UI scraping of `<select id="project">`, a single snapshot driving both `list_projects`
and search validation, and the default-project requirement deferred so a single discovered project
satisfies it. (FR-001–015, FR-019, FR-020)

**Independent Test**: Against an instance whose `/projects/indexed` returns `401` with scraping
enabled and no `OPENGROK_MCP_PROJECTS`, the server starts and `list_projects` returns exactly the
web UI's `<select id="project">` option values.

### Tests (write first)

- [ ] T013 [P] [US1] In `internal/config/config_test.go`: `Validate()` permits empty `Projects` +
  empty `DefaultProject` (deferred) and still errors for `>1` projects without a default;
  `OPENGROK_MCP_PROJECT_SCRAPE` parses via the shared bool convention (`true`/`1`/`TRUE`/`0`/… all
  honored uniformly).
- [ ] T014 [P] [US1] In `internal/opengrok/scrape_test.go` (parser): fixtures for versioned
  (`app-1.2-full`…) and flat (`alpha`, `beta`) option sets; a `<select>` with extra/reordered
  attributes still matched by `id=="project"`; xref anchors present but ignored; missing/malformed
  select → empty; `<option>` without `value` falls back to text; tokenizer early-exits at
  `</select>`.
- [ ] T015 [P] [US1] In `internal/opengrok/scrape_test.go` (fetch): `httptest` server — auth header
  is sent, the 8 MiB cap is enforced on an oversized body, `401` → error.
- [ ] T016 [P] [US1] In `cmd/opengrok-go-mcp/main_test.go`: resolution-ladder table over a fake
  `projectResolver` — `configured` wins (no API/scrape); `api` non-empty wins (no scrape even if toggle on);
  `api` error + scrape on → `scraped`; `api` error + scrape off → `none`; `api` empty-200 + scrape on
  → sanity-scrape → `scraped` (+ discrepancy logged); `api` empty-200 + scrape off → `none`; exactly
  one resolved project → default relaxed; `none` + no default → startup error.
- [ ] T017 [P] [US1] In `internal/mcpserver/server_test.go`: `list_projects` serves the resolved
  snapshot and agrees with `validateConfiguredProjects` (a listed project is searchable; a non-listed
  named project is rejected with `codeUnknownProject`). (FR-014, FR-015)

### Implementation

- [ ] T018 [US1] `internal/config/config.go`: parse `OPENGROK_MCP_PROJECT_SCRAPE` via the shared
  `strconv.ParseBool` convention (optionally extract `parseBoolEnv(name string, def bool) bool` and
  route booleans through it); add `ProjectScrapeEnabled` and `ProjectSource`; relax `Validate()` to
  defer the default-project check when `len(Projects)==0` while keeping the `>1`-projects-need-default
  error.
- [ ] T019 [P] [US1] `internal/opengrok/scrape.go`: `ScrapeProjects(ctx) ([]string, error)` —
  authenticated streaming GET to `webBaseURL+"/"` (reuse `addAuth`/`httpClient`/redacted logging),
  `io.LimitReader` 8 MiB, `html.NewTokenizer`; match the first `<select>` with `id=="project"`
  (attribute-order-independent), collect each descendant `<option>`'s `value` (text fallback) until
  `</select>`, then stop and close the body. `ScrapeProjects` is a method on the concrete
  `*opengrok.Client`; define a cmd-local `projectResolver` interface (`ListProjects` +
  `ScrapeProjects`) satisfied by the client for the resolution step — do **not** add it to
  `mcpserver.Backend`/`CachingBackend`.
- [ ] T020 [US1] `cmd/opengrok-go-mcp/main.go`: implement the ladder orchestration
  (`configured → api → scraped → none`, incl. the empty-200 sanity-scrape with discrepancy log);
  assign the resolved list to `cfg.Projects`; set `cfg.ProjectSource`; perform the post-discovery
  default-project resolution (single → set; `none`/`>1` → clear startup error); log resolved source +
  count and emit the experimental web-UI-fetch warning when scraping ran. Run this as a distinct step
  **before** the search/file capability probes, against the raw `*opengrok.Client` via the
  `projectResolver` interface, and remove the now-redundant `ListProjects` call from
  `detectCapabilities`. (Depends on T018, T019.)
- [ ] T021 [US1] `internal/mcpserver/server.go`: change the `ListProjects` handler to serve the
  resolved `cfg.Projects` snapshot (fallback `[DefaultProject]` when empty) and remove the live
  `backend.ListProjects` call so display and validation cannot diverge. (FR-014)
- [ ] T022 [US1] Run `go test ./internal/config/ ./internal/opengrok/ ./internal/mcpserver/ ./cmd/...`;
  `gofmt -w` changed files.

**Checkpoint**: all stories independently functional; MVP (US1) delivered.

---

## Phase 6: Polish & Cross-Cutting

- [ ] T023 [P] **Documentation reconciliation gate** — walk every row of `docs/README.md` (the
  documentation source-of-truth map) and update the single home of each concern this change affects,
  or mark it explicitly N/A. Do not restate canon across docs. For this feature:
  - `docs/configuration.md` — add `OPENGROK_MCP_PROJECT_SCRAPE` (default OFF, experimental) and the
    `configured → api → scraped → none` precedence ladder.
  - `docs/tool-contracts.md` — `list_projects` result source (startup snapshot) and the
    search-validation / `codeUnknownProject` behavior.
  - `docs/limitations.md` — best-effort scrape, startup-snapshot staleness, allowlist-from-discovery,
    and the TLS/auth/disabled failure classification + cert-SAN diagnostic.
  - `docs/agent-usage-patterns.md` — discovery/enablement workflow, if agent guidance changes.
  - `README.md` — enabling scraping + behavior summary.
  - `CHANGELOG.md` — entry (and `docs/release-process.md` only if the release flow itself changes).
  - `.specify/memory/constitution.md`, `AGENTS.md`, `docs/agent-ux.md`, `docs/review-checklist.md`,
    `SECURITY.md`, `docs/reporting-issues.md`, `.github/` issue/PR templates — review each; expected
    **N/A** unless a rule, rubric, or security posture actually changes. Confirm every row is
    updated-or-N/A before push.
- [ ] T024 Dispatch a fresh mid-tier subagent with minimal context and the realistic task —
  *"list the available OpenGrok projects and search one for a symbol"* — against a restricted-instance
  mock/recording; capture first-use findings on `list_projects` usability, startup/error-message
  comprehensibility, and the new env var description; fold wording fixes back in.
- [ ] T025 Verify the experimental label and resource-bound warning appear consistently across docs,
  the startup log, and the config description.
- [ ] T026 [P] Run `gofmt -w` on all changed Go files; `git diff --check` on docs.
- [ ] T027 Full `go test ./...` green; re-check the plan's Constitution Check gate.

---

## Dependencies & Execution Order

- **Setup (T001)** → **Foundational (T002–T003)** → stories.
- **Within US3**: T004 → T005 → T006.
- **Within US2**: T007, T008 (parallel) → T009 → T010 → T011 → T012. (T009/T010 are in
  `internal/opengrok`; T011 is in `cmd/main.go`.)
- **Within US1**: tests T013–T017 (all `[P]`, different files) → T018, T019 (parallel: config vs
  scrape) → T020 (depends on T018+T019) → T021 → T022.
- **Cross-story**: US3, US2, US1 are independent; their `cmd/main.go` edits (T005, T011, T020) are
  serialized per T002. Polish (Phase 6) after the desired stories complete.

### Parallel opportunities

- US1 tests T013–T017 run in parallel (distinct `_test.go` files).
- T018 (config) and T019 (scrape) run in parallel.
- T007 and T008 run in parallel.
- Docs (T023) and gofmt/whitespace (T026) are `[P]` with most code-complete work.

## Notes

- Verify each test fails against current behavior before implementing its slice.
- Commit after each task or logical group; keep secrets out of logs (`URL.Redacted()`).
- Stop at any story checkpoint to validate independently.
- T024 (fresh-subagent UX check) is a constitution gate for non-trivial agent-facing changes.
