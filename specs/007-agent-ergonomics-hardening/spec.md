# Feature Specification: Agent Ergonomics Hardening

**Feature Branch**: `007-agent-ergonomics-hardening`

**Created**: 2026-06-24

**Status**: Draft

**Input**: User description: "Address architectural risks, strategic improvements and quick wins from the agent ergonomics review (Full Review, 2026-06-24): token-heavy defaults, capability/doc drift, schema session cost, kind-filter semantics, response naming collision, outcome-only eval, project snapshot drift, and long-term risks around description drift, gateway indirection, and OpenGrok heuristic overconfidence."

## User Scenarios & Testing *(mandatory)*

The primary consumer is an **AI agent operating the MCP server cold** — minimal prior
context, finite attention budget, and no access to server logs. Scenarios are ordered
so each story delivers independent value; quick wins are folded into stories where they
belong rather than treated as a separate product.

### User Story 1 - Economy profile makes token-frugal behavior the easy default (Priority: P1)

A cold agent connects and performs ordinary code search and symbol lookup. Without
reading long documentation, it receives responses sized for multi-step investigation
rather than one-shot dumps: citations and match snippets first, with file-context
expansion and redundant link fields only when the agent (or operator) opts into a
richer profile.

**Why this priority**: Tool output is the dominant context cost. Current defaults
(auto context expansion, full response detail, links on) burn budget before the
agent can reason or chain follow-up calls. This is the highest-ROI structural fix
from the ergonomics review.

**Independent Test**: Run the four existing token-benchmark scenarios under the
economy profile and compare warm totals to the current default profile. Delivers
value even if no other story ships: operators can set one env var and agents
immediately get leaner success-path payloads.

**Acceptance Scenarios**:

1. **Given** economy profile is active, **When** an agent calls search or symbol
   tools with only required fields, **Then** responses omit automatic file-context
   expansion and use a terse response detail level while preserving `citation.url`
   on every result.
2. **Given** rich profile is active (or no profile set, for backward compatibility
   during transition), **When** an agent omits economy knobs, **Then** behavior
   matches today's shipped defaults so existing deployments are not silently changed
   until the migration note documents any default flip.
3. **Given** an agent reads tool input schemas, **When** it inspects
   `expand_context` (or equivalent), **Then** the field description states whether
   expansion is on or off by default under the active profile — not "set true" when
   the default is already on.
4. **Given** economy profile, **When** an agent explicitly requests richer output
   (`expand_context=true`, terse/detail override, or equivalent per-call knobs),
   **Then** per-call overrides win over the profile without requiring a server restart.

---

### User Story 2 - Runtime capability manifest closes the doc/schema gap (Priority: P1)

A cold agent (or its harness) needs to know which compact operations exist on *this*
deployment before planning multi-step workflows. Partial OpenGrok instances gate
references, file listing, or compound reads at startup; static workflow docs may
describe operations that are absent from the live tool schema.

**Why this priority**: Agents treat documentation and prior patterns as ground truth.
Planning "define → read → references" against a gated server wastes turns and
produces avoidable errors. A machine-readable manifest makes runtime truth
discoverable without stderr or guesswork.

**Independent Test**: Start the server against a hermetic backend with a subset of
capabilities disabled; fetch the manifest; verify it lists exactly the enabled
operations per tool and names any gated families with remediation hints. Delivers
value without the economy profile or eval work.

**Acceptance Scenarios**:

1. **Given** a deployment where symbol references are gated off, **When** an agent
   reads the capability manifest, **Then** it sees that `references`, `find`, and
   related operations are unavailable and which environment or auth steps typically
   restore them — without calling a failing operation first.
2. **Given** a fully capable deployment, **When** an agent reads the manifest,
   **Then** every operation present in the live `ListTools` schemas appears with a
   one-line purpose consistent with tool descriptions.
3. **Given** startup probe failures (auth, TLS, unsupported mode), **When** the
   manifest is read, **Then** gated tool families are listed with human-actionable
   remediation text suitable for surfacing to the agent user.
4. **Given** `agent-usage-patterns.md`, **When** a cold agent follows the
   capability preamble added by this feature, **Then** it is directed to inspect the
   manifest or `operation` enum before assuming workflows from the doc apply verbatim.

---

### User Story 3 - Kind-filter responses cannot be misread as global counts (Priority: P2)

An agent runs a structural sweep (`list` symbols with `kind=class` under a path).
The response must not invite the conclusion that `total_hits` equals the number of
matching symbols project-wide when filtering is applied page-locally.

**Why this priority**: Misread counts cause wrong architectural conclusions and
premature stop/continue pagination decisions. Warnings exist today but are easy to
miss inside large JSON.

**Independent Test**: Call symbol list with an active kind filter on a hermetic
fixture with multiple pages; verify additive output fields make the page-local
semantics obvious without reading `limitations.md`.

**Acceptance Scenarios**:

1. **Given** `kind` filter is set, **When** results return, **Then** the response
   includes an explicit indicator that kind filtering is page-local and that
   `total_hits` is the pre-filter OpenGrok count (or an equivalently unambiguous
   pair of fields).
2. **Given** `kind` filter is set, **When** results return, **Then** the response
   reports how many kind matches appear on the current page separately from
   `total_hits`.
3. **Given** no `kind` filter, **When** results return, **Then** no kind-filter
   caveat fields imply a false special case (additive fields absent or clearly N/A).

---

### User Story 4 - Trajectory eval catches agent-workflow regressions (Priority: P2)

Maintainers change tool descriptions, defaults, or warning text. Contract evals
prove field presence; they do not detect wrong tool choice, ignored warnings, or
recovered detours. This story extends measurement to multi-step replay scenarios
with graders for agent-observable behavior.

**Why this priority**: Description and default changes are silent behavior changes
for agents. Outcome-only checks miss a large class of ergonomic regressions noted in
the review.

**Independent Test**: Add at least three trajectory cases to the hermetic harness
derived from existing benchmark scenarios; run in CI; a seeded regression (e.g.
removed economy hint in description) fails a grader.

**Acceptance Scenarios**:

1. **Given** a "symbol investigation" replay scenario, **When** the harness runs,
   **Then** graders verify the agent-expected tool sequence, presence of
   `citation.url` in outputs, and that `HIGH_HIT_COUNT` (or equivalent) warnings
   are emitted when fixtures exceed the threshold.
2. **Given** a "search and read" scenario under compact surface, **When** the
   harness runs, **Then** graders verify warnings are machine-matchable via
   `warnings[].code`, not prose parsing alone.
3. **Given** a change that increases compact `ListTools` estimated tokens beyond a
   committed ceiling, **When** CI runs the token benchmark, **Then** the job fails
   with a clear regression message (soft ceiling with documented baseline refresh
   process).
4. **Given** description text for a compact tool changes, **When** a critical-user-
   journey trajectory case runs, **Then** a wrong-tool or wrong-operation selection
   induced by the wording fails CI (guards architectural risk: description drift).

---

### User Story 5 - Project catalog freshness is visible to agents (Priority: P3)

An agent calls project list during a long session. It must understand whether the
catalog is a startup snapshot or live data, and how to react when a later call
rejects an unknown project name.

**Why this priority**: Session-boundary drift causes mysterious `UNKNOWN_PROJECT`
loops; lower severity than economy and capability gaps but cheap to fix (quick win).

**Independent Test**: List projects; verify additive metadata fields; attempt
unknown project after fixture change without restart; verify error message mentions
snapshot staleness and restart.

**Acceptance Scenarios**:

1. **Given** project list returns, **When** an agent reads the response, **Then**
   additive fields disclose catalog `source` (e.g. configured, api, scraped, none)
   and that the list reflects startup discovery, not a live per-request fetch.
2. **Given** a project added to OpenGrok after server start, **When** an agent
   names it explicitly, **Then** `UNKNOWN_PROJECT` (or equivalent) message advises
   restart or allowlist update — not only "unknown project".

---

### User Story 6 - Tool headers teach economy without reading external docs (Priority: P3)

Cold agents read tool descriptions first. Economy knobs (`include_snippets`,
response detail level, context expansion) must be discoverable in the tool header
prose, not only in `agent-usage-patterns.md` or `tool-contracts.md`.

**Why this priority**: Quick win with high clarity impact; complements Story 1.

**Independent Test**: Inspect compact tool descriptions; confirm each search/read
tool includes one sentence listing the three most impactful economy knobs for sweeps.

**Acceptance Scenarios**:

1. **Given** compact search and symbols tools, **When** an agent reads descriptions,
   **Then** each includes a single prominent sentence recommending economy settings
   for large sweeps vs. deep reads.
2. **Given** compact and full surfaces, **When** descriptions mention "compact",
   **Then** tool-surface compact is distinguished from response detail compact/terse
   naming to prevent the collision flagged in the review.

---

### Edge Cases

- What happens when economy profile and per-call knobs conflict? Per-call wins;
  profile is the default only.
- What happens when every capability is gated and no search tools register? Manifest
  still publishes, listing empty tool families and remediation; server starts per
  existing policy.
- What happens when kind filter matches zero rows on a page but more pages exist?
  Response reports zero page-local kind matches, retains pagination cursor, and
  keeps kind-filter caveat fields.
- What happens when token benchmark ceiling is exceeded by a justified schema
  change? Maintainer refreshes committed baseline via documented script after
  review; CI documents delta.
- What happens on gateway surface? Manifest includes gateway operations; economy
  profile applies to underlying behavior; gateway discover indirection cost remains
  documented, not hidden.

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: Additive output fields for kind-filter semantics and
  project catalog metadata; new MCP resource for capability manifest; possible new
  env var `OPENGROK_MCP_AGENT_PROFILE` (or equivalent) with documented values;
  tool description and input-schema text updates; optional rename of response detail
  level vocabulary (migration note if `response_mode` value renamed). Preserve
  `citation.url`, `warnings[]`, `error_code`, pagination cursors, and capability
  gating semantics.
- **OpenGrok Semantics**: No change to OpenGrok query semantics. Kind filter remains
  page-local; manifest and output fields make that explicit. Best-effort/heuristic
  behaviors (implementations, cross-project, expansion limits) remain labeled.
- **Security Impact**: Manifest must not echo secrets; remediation hints reference
  env var names only. Economy profile does not weaken auth or TLS posture.
- **Documentation Impact**: `README.md`, `docs/configuration.md`,
  `docs/agent-usage-patterns.md` (capability preamble), `docs/tool-contracts.md`,
  `docs/limitations.md`, `CHANGELOG.md`, and migration note if default profile or
  response detail vocabulary changes.
- **Experimental Impact**: Gateway remains experimental; manifest documents gateway
  indirection tradeoff explicitly. Trajectory eval graders are harness-internal until
  stabilized.
- **Resource Bounds**: Economy profile tightens default expansion and response size;
  rich profile preserves current bounds. ListTools byte ceiling is a CI policy on
  compact surface, not a runtime truncation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support an operator-selectable **agent profile** with at
  least `economy` and `rich` values that bundle default context expansion, response
  detail level, and link/snippet defaults.
- **FR-002**: Per-call economy knobs MUST override the active agent profile without
  restart.
- **FR-003**: Deployments that omit `OPENGROK_MCP_AGENT_PROFILE` receive **economy**
  bundled defaults. Set `OPENGROK_MCP_AGENT_PROFILE=rich` to restore prior
  expanded-default behavior (documented in CHANGELOG).
- **FR-004**: Input schema text for context expansion MUST accurately state the
  default under the active profile (opt-in vs opt-out wording).
- **FR-005**: System MUST expose a **capability manifest** readable by agents (MCP
  resource or equivalent stable URI) listing enabled tools, operations per tool, and
  gated families with remediation hints.
- **FR-006**: Manifest content MUST match the live `ListTools` registration for the
  running process (no static superset).
- **FR-007**: `docs/agent-usage-patterns.md` MUST begin workflows with a capability
  discovery step referencing the manifest or live `operation` enum.
- **FR-008**: Symbol list responses with an active `kind` filter MUST include
  additive fields that distinguish pre-filter `total_hits` from page-local kind match
  count and flag page-local filtering — without breaking existing consumers.
- **FR-009**: Project list responses MUST include additive catalog `source` and
  snapshot semantics fields.
- **FR-010**: `UNKNOWN_PROJECT` errors MUST mention snapshot staleness and restart
  when the named project may have been added after startup.
- **FR-011**: Compact tool descriptions MUST include a one-sentence economy hint for
  sweeps vs deep reads and MUST distinguish tool-surface "compact" from response
  detail naming.
- **FR-012**: Hermetic eval harness MUST add trajectory graders on at least three
  multi-step scenarios covering tool choice, warning codes, and citations.
- **FR-013**: CI MUST fail when compact-surface `ListTools` estimated tokens exceed
  a committed baseline ceiling (documented refresh procedure).
- **FR-014**: CI MUST include at least one description-sensitive trajectory case
  that fails when compact tool wording regresses cold tool selection.
- **FR-015**: Compound and search responses that merge heuristic steps SHOULD
  continue to surface `warnings[]` and diagnostics so agents do not treat merged
  results as semantic certainty (addresses OpenGrok ceiling architectural risk).

### Key Entities

- **Agent profile**: Named bundle of default response economy settings (`economy`,
  `rich`); operator-configured; overridable per call.
- **Capability manifest**: Machine-readable snapshot of registered tools,
  operations, probe outcomes, and remediation hints for the running process.
- **Kind-filter metadata**: Additive response fields clarifying page-local kind
  filtering and per-page match counts.
- **Catalog metadata**: Additive project-list fields describing discovery source
  and snapshot semantics.
- **Trajectory case**: Multi-step hermetic scenario plus graders for agent-observable
  outcomes (tools used, warnings heeded, citations present).
- **ListTools baseline**: Committed token estimate ceiling for compact surface used
  in CI regression detection.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Under economy profile, warm token totals for the four standard
  benchmark scenarios decrease by at least 15% versus current default profile on the
  same fixtures (directional target; exact figure validated in plan phase).
- **SC-002**: In a fresh-agent simulation on a partially gated deployment, zero
  wasted calls on known-absent operations when the agent reads the manifest first
  (measured on a fixed task suite of at least five workflows).
- **SC-003**: 100% of symbol-list eval cases with `kind` filter pass graders that
  assert page-local semantics fields are present and internally consistent.
- **SC-004**: Trajectory eval suite contains at least three scenarios with at least
  eight total graders; CI fails on an intentional seeded regression within one
  maintenance iteration of shipping.
- **SC-005**: Compact `ListTools` regression gate prevents silent growth beyond
  committed baseline without an explicit baseline refresh in the same change.
- **SC-006**: Agent ergonomics review quick wins (schema text, capability preamble,
  tool-header economy hints, catalog metadata) are each traceable to at least one FR
  and acceptance scenario with no `[NEEDS CLARIFICATION]` markers remaining.

## Assumptions

- Primary surface in scope is **compact** (shipped default); full surface receives
  parallel description and schema text fixes where shared structs apply.
- Shipped default agent profile is **economy** (`OPENGROK_MCP_AGENT_PROFILE` unset).
  Set `rich` for expanded search context and per-result links by default.
- Response detail level may keep the value `compact` internally but prose MUST
  disambiguate from tool surface; optional rename to `terse` is a plan-phase decision
  with migration note if chosen.
- Capability manifest is an MCP resource (pattern: existing `opengrok://projects`);
  URI name to be chosen in plan (e.g. `opengrok://capabilities`).
- Trajectory graders are deterministic (field/warning/sequence checks) in v1; LLM-as-
  judge is out of scope for this feature.
- Gateway surface receives manifest entries and documentation updates but is not the
  primary token-ceiling gate target.
- Real-instance validation (https://opengrok.home/) is recommended in plan/quickstart
  but hermetic evals are the CI source of truth.

## Out of Scope

- Changing OpenGrok upstream semantics (AST search, global kind filter, live project
  list per request).
- Memory tool changes or cross-session durable storage.
- LLM-as-judge eval infrastructure with human gold-set (noted as future work).
- Slimming `oneOf` JSON schema structure via shared `$defs` (separate feature if
  ListTools ceiling cannot be met by description/profile work alone).
- Inbound HTTP client authentication.
