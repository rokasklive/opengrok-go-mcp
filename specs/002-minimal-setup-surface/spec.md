# Feature Specification: Minimal Setup Surface

**Feature Branch**: `002-minimal-setup-surface`

**Created**: 2026-06-10

**Status**: Draft

**Input**: User description: "Simplify MCP setup so only one environment variable is
absolutely mandatory. Auth is optional; 401 failures must tell the operator to supply a
token. When no default project or project list is configured, discover projects via the
REST API and automatically fall back to web-UI scraping. Scraping is on by default unless
the REST project list works; expose only an environment variable to disable scraping. North
star: the server works with only the base URL configured against a typical reverse-proxied
OpenGrok deployment."

---

## Background & Problem

Today, getting `opengrok-go-mcp` running requires assembling several environment variables
even for common deployments: base URL, often a default project, sometimes an explicit
project list, and frequently auth — plus an opt-in scrape toggle when the projects REST
endpoint is restricted. Operators (including the project author) must understand discovery
precedence, deferred validation, and capability probing to produce a working configuration.

That setup surface is too large for a tool whose primary job is to make OpenGrok usable by
agents quickly. The desired posture is **base URL only** for the happy path: point at an
OpenGrok instance, start the server, discover projects automatically, and only ask for more
configuration when the instance genuinely requires it.

This feature inverts several defaults and relaxes startup hard-failures while preserving
honest capability gating and agent-facing contracts.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start with only the base URL (Priority: P1)

An operator configures a single mandatory environment variable — the OpenGrok API base URL
— and starts the MCP server against a reverse-proxied OpenGrok instance reachable over
HTTPS. The server derives the web UI base URL when possible, discovers projects without
manual lists, selects a default project when unambiguous, probes capabilities, and exposes
working tools so an agent can list projects and search code without additional setup.

**Why this priority**: This is the north-star outcome. If base-URL-only startup does not
work on a typical proxied deployment, the feature has not delivered its core value.

**Independent Test**: Configure only the base URL against a test double or staging instance
that mirrors a reverse-proxied OpenGrok layout (API under `/api/v1`, web UI at the derived
host). Confirm the server starts, `list_projects` returns discovered projects, and at least
one search tool succeeds when the instance allows the underlying operation.

**Acceptance Scenarios**:

1. **Given** only the base URL is set and the projects REST endpoint returns a non-empty
   list, **When** the server starts, **Then** it starts successfully, records the project
   source as API-derived, auto-sets the default project when exactly one project is known,
   and does not fetch the web UI for project discovery.
2. **Given** only the base URL is set, the projects REST endpoint is unavailable or
   returns no projects, and the web UI landing page exposes a project picker, **When** the
   server starts, **Then** it starts successfully, discovers projects from the web UI once,
   records the source as scraped, and `list_projects` returns those projects.
3. **Given** only the base URL is set and exactly one project is discovered (via API or
   scrape), **When** an agent omits `project` on a search tool, **Then** the search uses
   that project without requiring `OPENGROK_MCP_DEFAULT_PROJECT`.
4. **Given** only the base URL is set and multiple projects are discovered, **When** the
   server starts, **Then** it starts successfully without `OPENGROK_MCP_DEFAULT_PROJECT`,
   `list_projects` returns all discovered projects, and scoped searches require an explicit
   `project` (or equivalent) per existing project-required semantics.

---

### User Story 2 - Optional auth with actionable 401 guidance (Priority: P1)

An operator does not configure auth because their instance allows anonymous access, or
because they want to validate connectivity before adding credentials. When OpenGrok returns
unauthorized responses, the server does not fail with opaque errors; it tells the operator
to supply `OPENGROK_MCP_API_TOKEN` or `OPENGROK_MCP_BASIC_AUTH_TOKEN` and continues in a
degraded but understandable state.

**Why this priority**: Auth is common but must not block the minimal setup path or force
up-front credential wiring when the operator is still validating the base URL.

**Independent Test**: Start the server with only the base URL against an instance that
returns `401`/`403` on some or all probed endpoints. Confirm startup completes, logs name
the missing auth configuration, and disabled capabilities are attributed to unauthorized
access rather than generic transport failure.

**Acceptance Scenarios**:

1. **Given** no auth token is configured and OpenGrok permits anonymous search and project
   listing, **When** the server starts, **Then** it starts successfully with no auth
   warnings and registers the corresponding tools.
2. **Given** no auth token is configured and probed endpoints return `401`/`403`, **When**
   the server starts, **Then** it still starts (does not abort solely for unauthorized
   probes), logs an actionable message naming the auth environment variables to set, and
   capability-gates affected tools with the same classification in startup diagnostics.
3. **Given** auth is later added and the server restarted, **When** probes succeed,
   **Then** previously gated tools become available without other configuration changes.
4. **Given** both auth token types would be configured, **When** the server validates
   configuration, **Then** it rejects the dual-token configuration with a clear error (unchanged
   security rule).

---

### User Story 3 - Scraping on by default with a single opt-out (Priority: P2)

An operator on an instance where `/projects/indexed` is restricted no longer needs to
discover and enable an experimental scrape toggle. Web-UI project discovery runs
automatically when the REST project list does not yield a usable allowlist. Operators who
do not want scraping (corporate policy, air-gapped API-only setups) can disable it with
one explicit environment variable.

**Why this priority**: Auto-scrape is the main mechanism that makes base-URL-only setup
work on restricted/proxied instances; the opt-out preserves control without expanding the
required surface.

**Independent Test**: Against a fixture where the projects API fails but the web UI exposes
project options, confirm scraping runs without any enable flag. Set the disable flag and
confirm no web-UI fetch occurs.

**Acceptance Scenarios**:

1. **Given** the projects REST call fails or is empty and scraping is not disabled,
   **When** the server starts, **Then** it attempts exactly one bounded web-UI fetch for
   project discovery and uses the parsed project list when successful.
2. **Given** the projects REST call returns a non-empty list, **When** the server starts,
   **Then** no web-UI scrape occurs regardless of the scrape disable flag.
3. **Given** scraping is disabled via the opt-out environment variable, **When** the
   projects REST call fails or is empty, **Then** no web-UI fetch occurs; startup proceeds
   with no discovered allowlist and relies on optional explicit project/default
   configuration if provided.
4. **Given** scraping succeeds, **When** startup logs are emitted, **Then** they note that
   project discovery used the web UI (best-effort/heuristic) without requiring the operator
   to have enabled an experimental toggle manually.

---

### User Story 4 - Reduced mandatory configuration in docs and examples (Priority: P2)

A new operator reading the README or client setup examples sees **one required** environment
variable (base URL) and a short optional section for auth, explicit project lists, and
scrape opt-out. Legacy variables remain documented as optional overrides, not prerequisites.

**Why this priority**: The configuration story must match the new behavior or operators will
continue to over-configure.

**Independent Test**: Review README quick-start and `docs/configuration.md` after
implementation; confirm required table lists only base URL and examples show a one-line
environment block that is sufficient for the north-star scenario.

**Acceptance Scenarios**:

1. **Given** the updated documentation, **When** an operator follows the minimal example,
   **Then** they are not instructed to set default project, project list, or scrape-enable
   flags for the standard path.
2. **Given** an operator still sets `OPENGROK_MCP_PROJECTS` or
   `OPENGROK_MCP_DEFAULT_PROJECT`, **When** the server starts, **Then** explicit values
   continue to take precedence over discovery (backward-compatible override path).

---

### Edge Cases

- Base URL does not end in `/api/v1` and web base URL cannot be derived → startup fails
  with a clear message to set the web base URL (unchanged constraint; not part of the
  one-variable happy path).
- Projects API, scrape, and explicit config all yield zero projects → server starts; search
  tools require explicit project configuration or fail with `PROJECT_REQUIRED` at call time;
  startup log explains that no projects were discovered.
- Scrape returns HTML without a project picker → treat as empty discovery; log scrape failure
  reason without aborting startup solely for that reason.
- Instance requires auth for scrape but not documented separately → unauthorized scrape is
  logged with the same auth guidance as API probes.
- Multiple discovered projects and no default → agent must pass `project`; `list_projects`
  is the discovery path (no startup failure).
- Operator sets scrape opt-out on an instance where both API and scrape would be needed →
  startup succeeds but project allowlist may be empty unless explicit project config is
  provided; logs must state that scraping was disabled and API discovery failed.
- TLS or proxy misconfiguration → retain existing classified diagnostics from prior work;
  do not regress TLS hostname mismatch vs unauthorized distinction.

---

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: Tool schemas unchanged in shape; startup and error messaging
  MUST improve discoverability for cold agents. Capability-gated tools remain gated when
  probes fail, but startup MUST NOT hide the auth remediation path. Warnings for scraped
  project lists remain best-effort/heuristic.
- **OpenGrok Semantics**: Discovery precedence preserved (explicit config > API > scrape >
  none) with scrape now default-on when API does not produce a list. No change to search
  semantics, pagination, or citation behavior.
- **Security Impact**: Auth tokens remain env-only, never CLI flags or logs. Scraping
  default-on increases one startup GET to the web UI when the API path fails — same bounded
  fetch as today's opt-in scrape, now default. Dual-token rejection unchanged. HTTP inbound
  auth posture unchanged.
- **Documentation Impact**: README, `docs/configuration.md`, `docs/limitations.md`, and
  client setup examples MUST reflect one required variable, opt-out scrape flag, optional
  auth, and relaxed default-project requirement. Migration note for inverted scrape default
  and removed startup hard-fail on unauthorized probes.
- **Experimental Impact**: Web-UI project discovery moves from opt-in experimental to
  default fallback behavior; documentation MUST still label scrape-derived lists as
  best-effort/heuristic. The opt-out env var name MUST NOT use "experimental" wording unless
  the scrape path itself remains partially heuristic.
- **Resource Bounds**: Unchanged: one bounded startup scrape (existing cap), capability
  probes remain bounded; no new automatic fetch behavior at tool-call time.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The server MUST require exactly one environment variable at startup for a
  valid minimal configuration: `OPENGROK_MCP_BASE_URL` (OpenGrok API base URL). No other
  variable MAY be mandatory for startup validation.
- **FR-002**: `OPENGROK_MCP_DEFAULT_PROJECT` MUST NOT be required at startup. When exactly
  one project is known after discovery, the server MUST set it as the default project
  automatically.
- **FR-003**: When multiple projects are known after discovery and no default is configured,
  the server MUST start successfully and require explicit project selection on tool calls per
  existing project-required semantics.
- **FR-004**: Project discovery precedence MUST remain: operator-configured project list
  (`OPENGROK_MCP_PROJECTS`) overrides API listing; API listing overrides scraping; scraping
  is attempted only when the API does not yield a usable non-empty project list.
- **FR-005**: Web-UI project discovery MUST be enabled by default when FR-004 reaches the
  scrape step. The server MUST expose a single opt-out environment variable to disable
  scraping (replacing the current opt-in `OPENGROK_MCP_PROJECT_SCRAPE=true` model).
- **FR-006**: When scraping is not disabled and is attempted, the server MUST perform at
  most one bounded startup fetch of the web UI landing page and parse the project picker,
  consistent with existing scrape semantics.
- **FR-007**: Auth tokens (`OPENGROK_MCP_API_TOKEN`, `OPENGROK_MCP_BASIC_AUTH_TOKEN`) MUST
  remain optional. When neither is set, the server MUST omit authorization headers on
  outbound requests.
- **FR-008**: When startup or discovery probes fail with `401`/`403`, the server MUST emit
  an actionable startup message instructing the operator to configure one of the auth token
  environment variables. The server MUST NOT abort startup solely because unauthorized probes
  left some capabilities disabled.
- **FR-009**: Capability gating MUST remain honest: tools whose backing probes failed MUST
  stay unavailable or clearly degraded; unauthorized failures MUST be classified distinctly
  from TLS, transport, and feature-unsupported failures in startup logs.
- **FR-010**: When no auth is configured and at least one probe succeeds, the server MUST
  start and register tools for successful probes without auth warnings.
- **FR-011**: Explicit operator overrides (`OPENGROK_MCP_PROJECTS`, `OPENGROK_MCP_DEFAULT_PROJECT`,
  auth tokens) MUST continue to work and take precedence over discovered values where
  applicable.
- **FR-012**: Documentation and README examples MUST show base-URL-only setup as the primary
  path and MUST document the scrape opt-out variable, optional auth, and optional explicit
  project overrides.
- **FR-013**: A migration note MUST document the behavior change from opt-in scraping to
  default-on scraping with opt-out, and the removal of mandatory default-project validation
  at startup.

### Key Entities

- **Minimal configuration**: The smallest valid operator input — base URL only — from which
  the server attempts full discovery and capability probing.
- **Project discovery source**: One of `configured`, `api`, `scraped`, or `none`, recording
  how the startup allowlist was resolved.
- **Scrape opt-out flag**: Operator-controlled switch that skips web-UI project discovery
  when the REST project list is unusable.
- **Auth remediation hint**: Standard startup message text pointing operators to the auth
  token environment variables when unauthorized responses are detected.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new operator can copy the documented minimal environment block (base URL
  only) and reach a running MCP server in one attempt on a representative reverse-proxied
  OpenGrok staging instance that requires scrape fallback, without reading discovery
  precedence documentation.
- **SC-002**: 100% of startup failures caused solely by missing auth on otherwise reachable
  instances are eliminated — the server starts and logs auth guidance instead of exiting with
  "no search capabilities available" when unauthorized probes are the only blocker.
- **SC-003**: On instances where the projects REST API works, zero additional startup web-UI
  fetches occur compared to today's API-success path (no regression in API-first behavior).
- **SC-004**: Documentation "Required" configuration table lists exactly one variable (`OPENGROK_MCP_BASE_URL`)
  for the minimal path; quick-start examples require at most one line in the environment block
  for the north-star scenario.
- **SC-005**: Operators who previously needed four or more environment variables for a
  working proxied deployment can reduce to one mandatory variable plus auth only when the
  instance requires it (validated by author dogfooding on their private instance before merge).

---

## Assumptions

- The north-star target instance follows the common OpenGrok layout: API under `/api/v1`,
  web UI at the host with `/api/v1` stripped from the base URL, optionally behind a reverse
  proxy terminating HTTPS.
- Web-UI project picker HTML structure (`<select id="project">`) remains consistent with
  the scrape logic validated in prior project-discovery work.
- Instances that require auth for all useful operations will still need auth tokens for
  tools to function; this feature improves discoverability of that requirement, not bypass
  of OpenGrok access control.
- `OPENGROK_MCP_WEB_BASE_URL` remains an optional override when base URL derivation is
  insufficient; it is not counted toward the mandatory surface.
- Backward compatibility for explicit `OPENGROK_MCP_PROJECTS` and `OPENGROK_MCP_DEFAULT_PROJECT`
  is required; operators with existing configs should see no behavior change when the API
  path already succeeds.
- The opt-out scrape variable will supersede the current opt-in `OPENGROK_MCP_PROJECT_SCRAPE`;
  exact naming is deferred to planning but MUST be a single disable flag (default: scraping
  allowed).

---

## Out of Scope

- Inbound authentication for HTTP MCP transport.
- Inferring project from the agent's local repository or workspace.
- Runtime refresh of the project allowlist without restart.
- Changing search, pagination, citation, or memory tool contracts beyond startup/messaging.
- Canonical URL sniffing or redirect following for discovery.
