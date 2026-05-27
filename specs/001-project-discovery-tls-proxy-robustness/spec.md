# Feature Specification: Project Discovery & TLS/Proxy Robustness

**Feature Branch**: `001-project-discovery-tls-proxy-robustness`

**Created**: 2026-05-27

**Status**: Draft

**Input**: User description: "Make project discovery work on OpenGrok instances where the
`/projects` REST API is disabled, by optionally scraping the web UI's project picker, with a
clear precedence ladder over the existing `OPENGROK_MCP_PROJECTS` config and the REST API. Also
make TLS/proxy failures diagnosable (cert-mismatch vs auth vs endpoint-disabled) and preserve
forward-proxy settings when TLS verification is skipped."

---

## Background & Problem

Some OpenGrok deployments sit behind a reverse proxy that restricts the REST API. Observed on two
real instances: `GET /api/v1/projects/indexed` returns `401` even with valid credentials (the
endpoint is disabled at the proxy/Tomcat layer), while full-text/definition search and the
server-rendered web UI remain reachable with the same Basic-auth credentials.

Consequences today:

- The server cannot enumerate projects when `/projects/indexed` is restricted. The only workaround
  is hand-listing every project in `OPENGROK_MCP_PROJECTS`, which is laborious and goes stale.
- Proxied instances may present a TLS certificate whose SAN does not match the proxy hostname
  (e.g. cert for `*.internal.example.com`, dialed as `proxy.example.com`), producing an opaque
  `tls: failed to verify certificate` error with no hint about the correct hostname.
- When `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY=true` is set, the HTTP transport is rebuilt from
  scratch and silently drops `http.ProxyFromEnvironment`, so operators behind a corporate forward
  proxy lose proxy routing exactly when they need the insecure flag.

This feature adds opt-in web-UI project discovery, a deterministic precedence model for where the
project list comes from, and TLS/proxy diagnostics — without changing behavior for instances where
the REST API already works.

Confirmed facts grounding this spec (collected via `curl` against two live instances):

- The web UI landing page (`<webBaseURL>/`) returns `200` with complete server-rendered HTML
  under the same Basic-auth credentials; no JavaScript rendering is required.
- That page contains a `<select id="project" name="project" multiple>` element whose
  `<option value="…">` entries are the canonical, top-level project list OpenGrok itself submits
  as the `projects=` search parameter. The `xref` anchor links on the page are navigation into
  project subdirectories (e.g. `project/submodule`) and are NOT a reliable project list.
- On the configured `/source/api/v1/` path the restricted endpoints return `401` directly (no
  cross-host redirect); the failure on the proxied instance is a TLS cert/hostname mismatch, not a
  redirect problem. Redirect-following / canonical-URL "sniffing" is therefore out of scope.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover projects when the REST API is disabled (Priority: P1)

An operator points the server at an OpenGrok instance where `/projects/indexed` is restricted
(`401`) and has not hand-listed projects. With web-UI project discovery explicitly enabled, the
server fetches the web UI landing page once at startup, parses the `<select id="project">` options,
and uses that list as the project allowlist and as the `list_projects` result — so an agent can
enumerate and search projects without any manual project configuration.

**Why this priority**: This is the core capability the feature exists to deliver. Without it,
restricted instances require manual project enumeration, which is the primary pain point.

**Independent Test**: Configure against an instance (or test double) whose `/projects/indexed`
returns `401` and whose web UI returns HTML containing a `<select id="project">` with N options,
enable the discovery toggle, and confirm the server starts, `list_projects` returns those N
projects, and searches scoped to those projects succeed.

**Acceptance Scenarios**:

1. **Given** `OPENGROK_MCP_PROJECTS` is unset, `/projects/indexed` returns `401`, the discovery
   toggle is enabled, and the web UI returns a `<select id="project">` with options
   `[a, b, c]`, **When** the server starts, **Then** the resolved project allowlist and the
   `list_projects` result are `[a, b, c]`, and a startup log records that the list was discovered
   by web-UI scraping.
2. **Given** the same conditions but the discovery toggle is **disabled** (default), **When** the
   server starts, **Then** no web-UI fetch occurs, no allowlist is derived, search validation
   stays permissive, and a startup log states the API was unavailable and scraping was disabled.
3. **Given** `OPENGROK_MCP_PROJECTS=[x, y]` is set, **When** the server starts, **Then** the
   allowlist is `[x, y]`, neither the REST API nor the web UI is consulted to build the allowlist,
   and a log records the source as operator-configured.
4. **Given** `OPENGROK_MCP_PROJECTS` is unset and `/projects/indexed` returns a non-empty list
   `[a, b]`, **When** the server starts, **Then** the allowlist is `[a, b]` from the REST API and
   no scraping occurs (even if the toggle is enabled).

---

### User Story 2 - Diagnose TLS / auth / disabled-endpoint failures (Priority: P2)

An operator misconfigures the base URL (e.g. dials a proxy hostname the TLS cert is not valid for),
or hits an instance with disabled endpoints. Instead of an opaque error, startup logs classify the
failure (TLS hostname mismatch vs unauthorized vs endpoint-disabled vs feature-unsupported), and for
a TLS hostname mismatch the log names the hostname(s) the presented certificate is actually valid
for, so the operator can fix configuration without trial-and-error.

**Why this priority**: Turns multi-hour opaque debugging into a single actionable log line. High
value, low risk, and independent of the discovery toggle, but secondary to actually enumerating
projects.

**Independent Test**: Point the server at a host whose cert SAN does not include the dialed
hostname and confirm the startup log states the cert is valid for the SAN hostname(s) and that the
failure is a TLS hostname mismatch (distinct from a `401`).

**Acceptance Scenarios**:

1. **Given** the configured host presents a certificate whose SAN does not match the dialed
   hostname and `INSECURE_SKIP_TLS_VERIFY` is unset, **When** a startup probe runs, **Then** the
   log classifies the failure as a TLS hostname mismatch and names the certificate's valid
   hostname(s).
2. **Given** an endpoint returns `401` while search endpoints succeed, **When** the probe runs,
   **Then** the log classifies that endpoint as unauthorized/disabled (not a TLS or transport
   error) and the dependent tool is capability-gated off as today.
3. **Given** reference search returns `400` while definition search succeeds, **When** probes run,
   **Then** `search_symbol_references` is gated off and the log distinguishes feature-unsupported
   (`400`) from unauthorized (`401`).

---

### User Story 3 - Forward-proxy compatibility with skipped TLS verification (Priority: P3)

An operator on a network that requires an outbound HTTP proxy (`HTTPS_PROXY`/`HTTP_PROXY`) needs
`OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY=true` for an internal instance with a self-signed/mismatched
cert. Requests must still route through the forward proxy when the insecure flag is set.

**Why this priority**: A latent correctness bug rather than a new capability, but it blocks exactly
the locked-down environments the insecure flag targets. Smallest, lowest-risk slice.

**Independent Test**: With `INSECURE_SKIP_TLS_VERIFY=true` and a proxy env var pointing at an
observable proxy, confirm outbound requests traverse that proxy (and that default transport
behaviors such as environment-proxy selection are preserved).

**Acceptance Scenarios**:

1. **Given** `INSECURE_SKIP_TLS_VERIFY=true` and `HTTPS_PROXY` set, **When** the client makes a
   request, **Then** the request is routed via the configured proxy (proxy-from-environment is
   honored) and TLS verification is skipped.
2. **Given** `INSECURE_SKIP_TLS_VERIFY` unset, **When** the client makes requests, **Then**
   behavior is unchanged from today.

---

### Edge Cases

- **REST API returns `200` with an empty array**: treated as "available but suspicious." If the
  discovery toggle is enabled, the server scrapes the web UI as a sanity check; if scraping finds
  projects, the scraped list becomes the allowlist and the API/scrape discrepancy is logged. If the
  toggle is disabled, or scraping also finds nothing, the server falls through to the terminal
  no-allowlist state.
- **Discovery toggle enabled but web UI also returns `401`** (different/again-restricted auth):
  no allowlist is derived; the server falls through to the terminal state and logs the scrape
  failure.
- **Scrape succeeds (`200`) but no `<select id="project">` options parse** (markup change,
  truncated body, size-cap hit): treated as a scrape failure → terminal state, logged.
- **Resolved list has exactly one project** (from config, API, or scrape): that project becomes the
  default project and `OPENGROK_MCP_DEFAULT_PROJECT` is not required.
- **Terminal no-allowlist state** (config unset, API unavailable/empty, scrape disabled or failed):
  search validation remains permissive (today's behavior) and `OPENGROK_MCP_DEFAULT_PROJECT` is
  mandatory — startup fails if it is unset, with a clear message.
- **Discovered allowlist staleness**: discovery is a one-time startup snapshot; a project added to
  the instance after startup is not searchable (rejected as unknown) and not listed until restart.
  This must be documented.
- **Explicitly named project not in a discovered/API allowlist**: rejected with the existing
  unknown-project error; the error message names the resolved allowlist and its source.
- **All-projects search**: an explicit all-projects search request continues to bypass the
  allowlist (the allowlist only constrains explicitly named projects).
- **`list_projects` vs search validation consistency**: both read the same startup-resolved
  snapshot, so a project shown by `list_projects` is always searchable and vice versa.
- **Scrape response exceeds the configured size cap or times out**: treated as a scrape failure →
  terminal state, logged; startup is never blocked beyond the scrape timeout.

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: No tool/schema field additions or removals. Behavioral changes to the
  *source* of the `list_projects` result (now a startup-resolved snapshot that may come from REST
  API or web-UI scrape) and to search validation (the allowlist may now be populated from API or
  scrape, so `codeUnknownProject` can occur where it previously could not). New startup log lines
  and warnings. Pagination/cursor behavior of `list_projects` is unchanged and remains
  deterministic over the resolved snapshot.
- **OpenGrok Semantics**: Web-UI project discovery is best-effort and heuristic (HTML parsing of
  the `<select id="project">` element) and is a point-in-time startup snapshot; this uncertainty
  and staleness MUST be surfaced in logs and docs. The dropdown is the authoritative project
  enumeration; `xref` links are explicitly NOT used. Failure classification (TLS/auth/disabled/
  unsupported) makes probe outcomes honest rather than collapsing all failures to "disabled."
- **Security Impact**: One new experimental, default-OFF environment variable to enable scraping;
  scraping reuses the existing authenticated GET path (auth in the `Authorization` header, never in
  the URL or a command line; all logged URLs use `URL.Redacted()`). The cert-SAN diagnostic MUST
  log only certificate hostnames, never secrets. The transport fix preserves
  `http.ProxyFromEnvironment` and other default transport behavior. No new inbound exposure; HTTP
  transport remains loopback-first. TLS bypass remains explicit and opt-in.
- **Documentation Impact**: `README.md` (new env var + discovery behavior), `docs/configuration.md`
  (new env var, defaults, precedence), `docs/limitations.md` (best-effort scrape, snapshot
  staleness, allowlist-from-discovery, TLS diagnostics), `docs/tool-contracts.md` (`list_projects`
  result source + search validation behavior), and `docs/agent-usage-patterns.md` if agent guidance
  changes. `CHANGELOG.md` updated on release.
- **Experimental Impact**: The web-UI scraping discovery path is experimental and MUST be labeled
  as such in the config variable name, documentation, and the startup log when active. It MUST NOT
  alter behavior for instances where the REST API already returns projects, and MUST NOT change the
  default (scraping off).
- **Resource Bounds**: Scraping adds at most one additional HTTP fetch at startup. It MUST have an
  explicit response-size cap and a timeout, MUST be non-blocking/best-effort (startup proceeds to
  the terminal state on timeout/failure), and MUST emit a warning noting it is fetching the web UI.

## Requirements *(mandatory)*

### Functional Requirements

**Project source precedence (resolution ladder)**

- **FR-001**: At startup the server MUST resolve a single project allowlist and record its source
  as one of: `configured`, `api`, `scraped`, or `none`.
- **FR-002**: If `OPENGROK_MCP_PROJECTS` is non-empty, the server MUST use it as the allowlist with
  source `configured` and MUST NOT consult the REST API or web UI to build the allowlist.
- **FR-003**: If `OPENGROK_MCP_PROJECTS` is empty and `GET /projects/indexed` returns a non-empty
  list, the server MUST use that list as the allowlist with source `api` and MUST NOT scrape.
- **FR-004**: If `OPENGROK_MCP_PROJECTS` is empty and `/projects/indexed` is unavailable (transport
  error, TLS failure, `401`/`403`, `5xx`, or any non-`200`), the server MUST attempt web-UI
  scraping only when the discovery toggle is enabled; a successful non-empty scrape yields source
  `scraped`.
- **FR-005**: If `/projects/indexed` returns `200` with an empty array, the server MUST treat this
  as unavailable for allowlist purposes; with the discovery toggle enabled it MUST scrape as a
  sanity check, and if scraping yields a non-empty list it MUST use it (source `scraped`) and log
  the API-empty/scrape-found discrepancy.
- **FR-006**: If no source yields projects (config empty; API unavailable/empty; scraping disabled,
  failed, or empty), the server MUST set source `none`, leave the allowlist empty, and keep search
  validation permissive (unchanged from current behavior).

**Web-UI scraping**

- **FR-007**: Web-UI scraping MUST be gated by a new experimental environment variable that
  defaults to OFF; when OFF, the server MUST NOT fetch the web UI for discovery under any rung.
- **FR-008**: Scraping MUST fetch the web UI landing page once via the existing authenticated GET
  path (reusing configured auth and the existing HTTP client, including any TLS settings) and MUST
  extract project names from the `<select id="project">` element's `<option>` values verbatim.
- **FR-009**: Scraping MUST NOT derive projects from `xref` anchor links and MUST NOT assume any
  project-name suffix or versioned naming pattern.
- **FR-010**: Scraping MUST enforce a response-size cap and a timeout, MUST be best-effort, and MUST
  NOT block startup beyond the timeout; on any failure it MUST fall through per FR-006 with a log.

**Default-project requirement & validation ordering**

- **FR-011**: When the resolved allowlist (from any source) contains exactly one project, that
  project MUST become the default project and `OPENGROK_MCP_DEFAULT_PROJECT` MUST NOT be required.
- **FR-012**: When the resolved source is `none` (no allowlist), `OPENGROK_MCP_DEFAULT_PROJECT`
  MUST be required and the server MUST fail startup with a clear message if it is unset.
- **FR-013**: The "default project required unless exactly one project is known" check MUST be
  evaluated after discovery for the `api`/`scraped`/`none` cases; pure-config validation that does
  not depend on discovery MAY remain at config-parse time. Startup MUST NOT fail for a missing
  default project before discovery has had a chance to supply a single-project list.

**Snapshot consistency**

- **FR-014**: The resolved allowlist MUST be a single startup snapshot used consistently for both
  search-project validation and the `list_projects` tool result, so the two never disagree within a
  process lifetime.
- **FR-015**: Explicitly named projects outside a non-empty allowlist MUST be rejected with the
  existing unknown-project error, whose message names the resolved allowlist and its source; an
  explicit all-projects search MUST continue to bypass the allowlist.

**TLS / proxy diagnostics & transport**

- **FR-016**: Startup probe failures MUST be classified and logged as one of: TLS hostname/cert
  mismatch, transport error, unauthorized (`401`/`403`), endpoint-disabled, or feature-unsupported
  (`4xx` such as `400` for an unsupported search mode) — rather than a single generic failure.
- **FR-017**: On a TLS hostname/cert mismatch, the server MUST log the hostname(s) the presented
  certificate is valid for (its SAN entries) to guide reconfiguration, and MUST NOT log any secret.
- **FR-018**: When `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY=true`, the HTTP transport MUST preserve
  default transport behavior — in particular `http.ProxyFromEnvironment` — while skipping TLS
  verification, so forward-proxy settings remain in effect.

**Compatibility & logging**

- **FR-019**: For instances where the REST API returns projects, or where `OPENGROK_MCP_PROJECTS`
  is set, observable behavior (allowlist, `list_projects`, default-project requirement) MUST be
  unchanged from the current release, except for added log lines.
- **FR-020**: The server MUST log, at startup, the resolved project source, the project count, and
  (when scraping is active) an experimental-feature notice that it fetched the web UI.

### Key Entities

- **Project allowlist**: the resolved set of project names plus a `source` of
  `configured` | `api` | `scraped` | `none`; drives search validation, `list_projects`, and the
  default-project requirement.
- **Discovery precedence**: the ordered decision producing the allowlist
  (`configured` → `api` → `scraped` → `none`), with the empty-API sanity-scrape branch.
- **Probe failure classification**: the category assigned to a failed startup probe
  (`tls_mismatch` | `transport_error` | `unauthorized` | `endpoint_disabled` |
  `feature_unsupported`), used for logging and capability gating.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On an instance whose `/projects/indexed` returns `401`, with the discovery toggle
  enabled and no `OPENGROK_MCP_PROJECTS`, the server starts and `list_projects` returns exactly the
  set of `<option>` values in the web UI's `<select id="project">`.
- **SC-002**: With the discovery toggle at its default (OFF), no web-UI fetch occurs under any rung;
  configurations that work today produce byte-identical project resolution and tool behavior (only
  additional log lines may appear).
- **SC-003**: When the resolved project list has exactly one project (from any source), the server
  starts without `OPENGROK_MCP_DEFAULT_PROJECT`; when the source is `none`, the server fails startup
  with a message naming the missing `OPENGROK_MCP_DEFAULT_PROJECT` requirement.
- **SC-004**: A TLS hostname mismatch yields a startup log that names the certificate's valid
  hostname(s); an operator can correct the base URL from that single log line without external cert
  inspection.
- **SC-005**: With `INSECURE_SKIP_TLS_VERIFY=true` and a forward proxy configured via environment,
  outbound requests traverse that proxy (verified by the proxy receiving the request).
- **SC-006**: Probe failures are distinguishable in logs: a `401` endpoint, a `400` unsupported
  search mode, and a TLS mismatch each produce a different, correctly-labeled classification.
- **SC-007**: `list_projects` and search-project validation never disagree within a process: every
  project returned by `list_projects` is accepted by a scoped search, and any rejected named project
  is absent from `list_projects`.

## Assumptions

- The OpenGrok web UI landing page is server-rendered HTML reachable with the same credentials as
  the API, and exposes a `<select id="project">` element; this held on both instances examined and
  is treated as the discovery contract. If an instance lacks this element, discovery falls through
  to the terminal state (FR-006).
- The `<option>` values of `<select id="project">` are the authoritative top-level project names
  used as the `projects=` search parameter.
- The configured base/web URLs already point at a reachable proxy or backend; canonical-URL
  detection and redirect-following are out of scope (the configured `/source/api/v1/` path returns
  `401` directly, not a cross-host redirect).
- A startup-time snapshot of the project list is acceptable; live per-call project refresh is out of
  scope for this feature, and staleness until restart is an accepted, documented limitation.
- Reading file content via the web UI (a separate `/raw` or browser fallback) is out of scope here;
  this feature concerns project enumeration and TLS/proxy robustness only.
- Embedded headless-browser approaches (e.g. Playwright) are explicitly out of scope; static HTML
  parsing is sufficient and avoids large runtime dependencies.
