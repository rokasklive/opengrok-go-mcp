# Implementation Plan: Project Discovery & TLS/Proxy Robustness

**Branch**: `001-project-discovery-tls-proxy-robustness` | **Date**: 2026-05-27 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-project-discovery-tls-proxy-robustness/spec.md`

## Summary

Add a deterministic startup project-resolution ladder (`configured → api → scraped → none`) so the
server can enumerate projects on OpenGrok instances where `/projects/indexed` is restricted, by
optionally scraping the server-rendered web UI's `<select id="project">` element. Make the resolved
list a single startup snapshot that drives both search-project validation and `list_projects`.
Defer the "default project required" check until after discovery so a single discovered project can
satisfy it. Independently, make startup probe failures diagnosable (TLS hostname mismatch vs
unauthorized/restricted vs feature-unsupported vs transport error, with cert SAN reported on TLS
mismatch) and fix the `INSECURE_SKIP_TLS_VERIFY` transport so `http.ProxyFromEnvironment` survives.

Phase 0 research and the small data model are folded into this plan (sections **Research Decisions**
and **Data Model**). There are no new MCP tools or schema fields, so `contracts/` is not produced;
operator-facing usage is captured in **Quickstart** below and will land in `docs/`.

## Technical Context

**Language/Version**: Go 1.24.0

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp`; OpenGrok HTTP API and
server-rendered web UI. **New dependency (accepted)**: `golang.org/x/net/html` for streaming HTML
tokenizing — a Go-team-maintained `golang.org/x` module whose `html` package adds no third-party
transitive deps (only its sibling `html/atom`). See Research Decision R1.

**Storage**: Process-scoped configuration and a startup-resolved project snapshot; no persistence.

**Testing**: `go test ./...`; targeted `go test ./internal/<pkg>/` and `go test ./cmd/...` per slice.

**Target Platform**: Local stdio MCP server and loopback HTTP transport.

**Project Type**: Go CLI / MCP server.

**Performance Goals**: Discovery adds at most one extra startup HTTP GET, bounded by the existing
startup `checkCtx` (`ReadTimeout`, default 10s) and a dedicated response-size cap (8 MiB). The
tokenizer streams and early-exits at the project `<select>`, so actual reads are typically a small
fraction of the page and no full DOM is built; the 8 MiB cap is only a safety ceiling, not expected
consumption. Non-blocking/best-effort: any timeout or failure falls through to the `none` state. No
change to per-request tool latency.

**Constraints**: Preserve MCP schema compatibility; keep secrets in env vars and out of URLs/logs
(`URL.Redacted()`); keep TLS-skip and scraping opt-in and documented; no behavior change for
instances where the REST API already returns projects.

**Scale/Scope**: Tens of projects per instance (both observed instances had ~20 dropdown options).
The `<select id="project">` carries all options regardless of `size`, so no pagination of the page.

## Research Decisions (Phase 0)

Unknowns from the spec were resolved during the prior investigation (two live instances, captured
via `curl`). Decisions:

- **R1 — HTML parsing (DECIDED: `golang.org/x/net/html`, streaming `html.Tokenizer`)**.
  *Official & low-risk*: Go-team-maintained `golang.org/x` module; the `html` package's only
  non-stdlib import is its sibling `html/atom`, so no third-party transitive deps are added. It is
  the de-facto Go HTML parser. *Correctness*: the only real invariant is **a `<select>` start tag
  whose `id` attribute equals `project`, regardless of its other attributes (`class`, `name`,
  `multiple`, `size`, `tabindex`, …) or their order** — the tokenizer checks the `id` attribute
  positionally-independently; regex handles this poorly. *Streaming early-exit*: the tokenizer
  consumes the body incrementally; we scan to the matching `<select>`, collect descendant
  `<option>` values until its `</select>`, then stop and close the body — typically reading a
  fraction of the page, never building a full DOM. *Rejected*: `regexp`/`strings` — brittle against
  attribute reordering (exactly the failure mode called out in review).
- **R2 — Source of project names**: `<option value>` of `<select id="project">`, taken verbatim.
  `xref` anchor links are NOT used (they are subdirectory navigation, e.g. `project/submodule`).
  No version/suffix assumptions. Confirmed identical structure on versioned and flat instances.
- **R3 — TLS failure is a cert/hostname mismatch, not a redirect**: the configured
  `/source/api/v1/` path returns `401` directly; no cross-host redirect. Canonical-URL sniffing and
  redirect-following are out of scope. Cert SANs are recovered from
  `*tls.CertificateVerificationError.UnverifiedCertificates` via `errors.As` (Go 1.20+; available on
  1.24) — no second dial needed.
- **R4 — No JavaScript rendering**: the landing page is complete server-rendered HTML under Basic
  auth; static parsing suffices. Embedded headless browsers (Playwright) are out of scope.
- **R5 — `401` cannot alone distinguish "disabled" from "bad credentials"**: both return `401`.
  The orchestrator infers "endpoint restricted" vs "credentials rejected" *contextually* — if any
  authenticated probe (e.g. search) succeeded, a `401` on another endpoint is treated as
  endpoint-restricted; if every probe is `401`, it is treated as an auth failure. This is how
  FR-016's `unauthorized` vs `endpoint_disabled` categories are separated in practice.

## Data Model (Phase 1)

State introduced (all process-scoped, resolved once at startup):

- **Resolved project inventory**: `projects []string` + `source` enum
  `configured | api | scraped | none`. Stored by assigning the resolved list into the existing
  `cfg.Projects` (the established allowlist field) and recording `source` for logging.
- **Probe failure classification**: `category` enum
  `tls_mismatch | unauthorized | endpoint_disabled | feature_unsupported | transport_error`, plus
  optional `certSANs []string` for `tls_mismatch`. Derived from a typed client error
  (`*opengrok.StatusError{Code int}`) and `errors.As` on `*tls.CertificateVerificationError`.

No new persisted entities; no new MCP response fields.

## Constitution Check

*GATE: passed at planning. Re-check after implementation.*

- **MCP Contract**: No tool, schema field, cursor, citation, or resource changes. `list_projects`
  changes *source* only — it serves the startup-resolved snapshot (`cfg.Projects`, or
  `[DefaultProject]` when `source = none`) instead of a live per-call `/projects` fetch (FR-014).
  Search validation (`validateConfiguredProjects`) is unchanged in mechanism but its allowlist may
  now be populated from `api`/`scraped` sources, so `codeUnknownProject` can occur where it
  previously could not (FR-015). Pagination of `list_projects` stays deterministic over the
  snapshot. Affected tool surfaces: `full`, `compact`, `gateway` all read the same snapshot, so they
  stay coherent.
- **OpenGrok Semantics**: Scraping is best-effort/heuristic and a point-in-time startup snapshot;
  surfaced via startup logs and `docs/limitations.md`. The `<select id="project">` is the
  authoritative enumeration; results outside the snapshot remain possible only via explicit
  all-projects search (which bypasses the allowlist). Probe classification makes failures honest.
- **Test Evidence**: See **Test Plan**. Behavioral slices are ordered **test-first** (write failing
  test → implement → targeted test → package/full verification). Pure resolution-ladder and HTML
  parser logic are table-driven and network-free; client/TLS behavior uses `httptest`.
- **Agent UX Validation**: A fresh mid-tier subagent is given a restricted-instance scenario (mock
  or recorded) and the realistic task: *"List the available OpenGrok projects and search one for a
  symbol."* With minimal upfront context, observe whether `list_projects` returns a usable list, the
  scoped search succeeds, and startup/error messaging is comprehensible. Findings feed wording of
  logs, the new env var description, and `docs/`. (Captured in `tasks.md`.)
- **Security**: New env var is opt-in and default-OFF; scraping reuses the existing
  `doGET`/`addAuth` path (auth in `Authorization` header, never URL/CLI; logs use `URL.Redacted()`).
  Cert-SAN diagnostic logs only certificate hostnames. Transport fix preserves
  `http.ProxyFromEnvironment`. No new inbound exposure; HTTP stays loopback-first; TLS-skip stays
  explicit.
- **Compatibility and Docs**: Backward compatible for `configured`/`api` instances (only added
  logs). Behavior changes — `list_projects` snapshot vs live, allowlist from discovery, deferred
  default-project check — are documented. Updates: `README.md`, `docs/configuration.md`,
  `docs/limitations.md`, `docs/tool-contracts.md`, `CHANGELOG.md`. New dependency (if R1 picks
  `x/net`) noted in `go.mod`.
- **Experimental Surface**: Web-UI scraping is experimental — labeled in `docs/`, in the startup log
  emitted when scraping is active, and described as experimental where the env var is documented.
  Default (scraping off) and REST-API behavior are unchanged.
- **Resource Bounds**: Scraping = one startup GET, dedicated 8 MiB response cap, bounded by
  `ReadTimeout`, best-effort with fall-through; a startup warning notes the web-UI fetch.

No constitution violations → **Complexity Tracking is empty**.

## Project Structure

### Documentation (this feature)

```text
specs/001-project-discovery-tls-proxy-robustness/
├── spec.md      # Feature specification (done)
├── plan.md      # This file
└── tasks.md     # Phase 2 (/speckit-tasks — NOT created here)
```

`research.md`, `data-model.md`, and `quickstart.md` are folded into this plan; `contracts/` is N/A
(no new MCP tool/schema).

### Source Code (repository root)

```text
cmd/opengrok-go-mcp/
  main.go            # transport clone; capture API list; resolution ladder orchestration;
                     # post-discovery default-project check; probe-failure classification + logging
  main_test.go       # ladder orchestration (fake projectResolver) + classification tests
internal/config/
  config.go          # OPENGROK_MCP_PROJECT_SCRAPE parsing; relax Validate() default-project for
                     # the empty-PROJECTS (deferred) case; ProjectSource for logging
  config_test.go     # validation-ordering + toggle-parsing tests
internal/opengrok/
  client.go          # surface HTTP status via typed *StatusError from doGET
  scrape.go          # NEW: ScrapeProjects(ctx) — fetch webBaseURL + "/" via existing GET path,
                     # parse <select id="project"> option values; 8 MiB cap
  errors.go          # NEW: StatusError type (carries HTTP status code/path)
  scrape_test.go     # NEW: parser (versioned + flat samples, xref noise ignored, malformed→empty)
                     # + fetch tests (httptest: auth header sent, cap enforced, 401→error)
  client_test.go     # StatusError propagation tests
internal/mcpserver/
  server.go          # ListProjects tool handler serves the resolved snapshot (FR-014)
  server_test.go     # snapshot consistency: list_projects vs search validation
docs/
  configuration.md, limitations.md, tool-contracts.md   # behavior + env var + caveats
README.md            # setup/behavior note
CHANGELOG.md         # release entry
go.mod, go.sum       # add golang.org/x/net if R1 chooses it
```

**Structure Decision**: Keep existing `cmd/` + `internal/` boundaries. Scraping lives in
`internal/opengrok` (alongside the client, reusing `doGET`/`addAuth`/`webBaseURL`); `ScrapeProjects`
is a method on the concrete `*opengrok.Client`. Project resolution is orchestrated in
`cmd/opengrok-go-mcp/main.go` as a **distinct step that runs before the capability probes**, against
the **raw `*opengrok.Client`** (constructed before any cache wrap) — NOT through `mcpserver.Backend`
or `CachingBackend`, which stay untouched (a one-shot startup fetch gains nothing from caching and
must not force `ScrapeProjects` onto those interfaces). For testability the orchestration takes a
small cmd-local interface `projectResolver { ListProjects(ctx); ScrapeProjects(ctx) }` that the
client satisfies, so the ladder is exercised with a fake and no network. No new package is introduced
(constitution: prefer existing patterns).

## Implementation Approach (by spec slice)

### Slice A — Transport fix (FR-018) — P3, smallest/independent

`cmd/.../main.go` ~L61-66: replace the bare `&http.Transport{TLSClientConfig: …}` with
`http.DefaultTransport.(*http.Transport).Clone()` then set `TLSClientConfig.InsecureSkipVerify`.
Preserves `Proxy: http.ProxyFromEnvironment` and pooling defaults.

### Slice B — Typed status errors + probe classification (FR-016, FR-017) — P2

- `internal/opengrok/errors.go`: `StatusError{Code int; Status, Path string}` implementing `error`.
- `client.go doGET`: return `*StatusError` for non-2xx instead of the generic
  `unexpected status` error (keep message text stable where feasible). Transport/TLS errors stay
  wrapped (preserving `*tls.CertificateVerificationError`).
- `main.go`: `classifyProbeError(err, anyAuthedProbeSucceeded bool) (category, certSANs)` using
  `errors.As`. `tls_mismatch` extracts `UnverifiedCertificates[0].DNSNames`. `401/403` →
  `endpoint_disabled` when another authed probe succeeded (R5), else `unauthorized`. `400` (and
  other 4xx) → `feature_unsupported`. Else → `transport_error`. `logCapability` extended to print
  category and SANs.

### Slice C — Resolution ladder + snapshot + deferred default (FR-001–015) — P1, core

- `config.go`: parse `OPENGROK_MCP_PROJECT_SCRAPE` as a bool via the **same `strconv.ParseBool`
  convention every existing boolean env var uses** (so `true`/`1`/`TRUE`/`t`/`0`/`false`/… are all
  accepted uniformly — no per-var format quirks) → `cfg.ProjectScrapeEnabled`; optionally extract a
  shared `parseBoolEnv(name string, def bool) bool` helper for DRY and to keep all booleans uniform.
  Add `cfg.ProjectSource string` (for logging). Relax `Validate()`: drop the hard error on empty
  `DefaultProject` **only** when `len(Projects)==0` (defer); keep the multi-project-needs-default
  error for `len(Projects)>1`.
- `internal/opengrok/scrape.go`: `ScrapeProjects(ctx) ([]string, error)` — issue an authenticated
  GET to `webBaseURL + "/"` reusing the client's `addAuth` + `httpClient` + redacted logging, wrap
  the body in `io.LimitReader` (8 MiB safety ceiling), and feed it to `html.NewTokenizer`. Match the
  first `<select>` whose `id` attribute == `project` (attribute-order-independent), collect each
  descendant `<option>`'s `value` (fallback to option text when `value` is absent) until the closing
  `</select>`, then stop reading and close the body. This **streams** rather than reusing `doGET`
  (which buffers the whole body via `io.ReadAll`), to enable early-exit.
- Add `ScrapeProjects` as a method on the concrete `*opengrok.Client`; define a cmd-local
  `projectResolver` interface (`ListProjects` + `ScrapeProjects`) that the client satisfies, used only
  by the resolution orchestration. Do **not** add `ScrapeProjects` to `mcpserver.Backend` or
  `CachingBackend`.
- `main.go` orchestration — run as a distinct step **before** the search/file capability probes,
  against the raw `*opengrok.Client` via the `projectResolver` interface, and **drop the now-redundant
  `ListProjects` call from `detectCapabilities`** (resolution owns it):
  1. `Projects` non-empty → `source=configured`.
  2. else call `ListProjects`: non-empty 200 → `source=api`; on `401/error` or empty-200 →
     if `ProjectScrapeEnabled` call `ScrapeProjects` (sanity-scrape on empty-200, logging the
     discrepancy) → non-empty → `source=scraped`; else `source=none`.
  3. Assign resolved list to `cfg.Projects`; set `cfg.ProjectSource`.
  4. Post-discovery default check: if `DefaultProject==""` then exactly-one resolved → set it;
     else (`none` or >1) → fail startup with the existing clear message.
  5. Log resolved source + count; emit experimental web-UI-fetch warning when scraping ran.
- `internal/mcpserver/server.go ListProjects` handler: serve `cfg.Projects` snapshot (fallback
  `[DefaultProject]` when empty); remove the live `backend.ListProjects` call so display and
  validation cannot diverge (FR-014).

## Test Plan (test-first per slice)

- **A**: unit test asserting the constructed transport (with skip-verify) has a non-nil `Proxy`
  equal to `http.ProxyFromEnvironment` behavior (or a behavioral `httptest` proxy round-trip).
- **B**: `client_test.go` — non-2xx yields `*StatusError` with the right code (`errors.As`).
  `main_test.go` — `classifyProbeError` table: `401` with/without a prior authed success →
  `endpoint_disabled`/`unauthorized`; `400` → `feature_unsupported`; a `tls.CertificateVerificationError`
  → `tls_mismatch` + expected SANs (construct via a self-signed cert with known `DNSNames` and a
  hostname mismatch over `httptest` TLS).
- **C**: `config_test.go` — `Validate()` allows empty `Projects`+empty `DefaultProject` (deferred),
  still errors for `>1` projects without default; toggle parsing. `scrape_test.go` — parse fixtures
  (versioned `app-1.2-full…`, flat `alpha`/`beta`), ignore xref anchors, malformed/missing select
  → empty; fetch via `httptest` sends auth header, enforces 8 MiB cap, maps `401`→error.
  `main_test.go` — ladder table over a fake `projectResolver` covering all rungs incl. empty-200 sanity-scrape,
  single-project default relaxation, and `none`→default-required-failure. `server_test.go` —
  `list_projects` serves the snapshot and agrees with `validateConfiguredProjects`.
- **Verification**: targeted `go test ./internal/config/ ./internal/opengrok/ ./internal/mcpserver/ ./cmd/...`
  per slice, then full `go test ./...`; `gofmt -w` on changed files; `git diff --check` on docs.

## Quickstart (operator)

To enable discovery on an instance where `/projects/indexed` is restricted:

1. Configure as usual (`OPENGROK_MCP_BASE_URL`, auth, `OPENGROK_MCP_WEB_BASE_URL` if not derivable).
2. Leave `OPENGROK_MCP_PROJECTS` unset.
3. Set `OPENGROK_MCP_PROJECT_SCRAPE=true` (experimental).
4. Start the server; the startup log reports the resolved project source (`scraped`) and count.
   If the only project is unique it becomes the default; otherwise set `OPENGROK_MCP_DEFAULT_PROJECT`.

If the REST API works, or `OPENGROK_MCP_PROJECTS` is set, nothing changes and no web-UI fetch occurs.

## Resolved Decisions (plan review)

1. **HTML dependency — ACCEPTED `golang.org/x/net/html`**: official Go-team module, no third-party
   transitive deps, streaming tokenizer improves maintainability and enables early-exit (R1).
2. **Env var name — ACCEPTED `OPENGROK_MCP_PROJECT_SCRAPE`**: boolean, parsed via the shared
   `strconv.ParseBool` convention used by all existing boolean vars (uniform `true`/`1`/… handling).
   Experimental status conveyed via docs + startup log + description (value-level, matching how the
   `gateway` surface is marked), not by embedding "EXPERIMENTAL" in the name.
3. **Scrape size cap — ACCEPTED 8 MiB**: a safety ceiling only; streaming early-exit means typical
   reads are far smaller.
