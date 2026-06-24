# Feature Specification: Compact Surface as Default

**Feature Branch**: `006-compact-surface-default`

**Created**: 2026-06-24

**Status**: Draft

**Input**: User description: "Let's focus on non-default interface - compact, let's polish it to a usable state, to the point where it would be actually eligible to become the new default in the system. We have a real opengrok instance to test against too (https://opengrok.home/). Use agent ergonomics skills to dial in the agent ergonomics aspects because first and foremost these interfaces are to be used by an agent, not a human. Compared to full mode, compact mode should not sacrifice descriptions (L2 focus) but rather reduce the overall tool surface; compact mode shouldn't have overlapping tools and should consolidate tools that can be consolidated."

## Clarifications

### Session 2026-06-24

- Q: How should eval coverage change to measure compact as a first-class interface — parameterize existing contract scenarios onto compact, add compact-specific cases, or both? → A: Both — run the existing contract scenarios on compact via the canonical-op→surface resolver AND add compact-specific cases (project overview, file listing on compact, invalid-operation errors, constructing a call from the typed schema alone).
- Q: Should the harness assert compact↔full result equivalence on shared scenarios and fail on divergence? → A: Yes — assert cross-surface equivalence (hits, `citation.url`, pagination, warnings); a compact/full divergence fails the eval.
- Q: What CI gating should compact get? → A: Compact gets its own committed baseline and CI fails on compact contract regressions; token-cost deltas remain reported but non-gating (token benchmark v1 policy).
- Q: Will compact use the same capability-gated registration as full — registering tools/operations only when their backing capability is available, so unavailable ones cost no agent attention/context? → A: Yes (confirmed). Gating applies at both levels: a compact tool with no available operations is not registered at all (no `ListTools` entry); within a registered tool, unavailable operations are absent from the operation schema/enum and rejected at dispatch — identical to the full surface.
- Q: Should the compact surface include the memory tool? → A: No — omit `opengrok_memory` from compact for now (memory is likely to be sunset in a future change). Compact is a stable 3–4 tool surface (projects, search, symbols, read). Memory remains available only on the full surface (stdio-only) as a deliberate, documented surface exception.

## User Scenarios & Testing *(mandatory)*

The consumer of this surface is an **AI agent seeing the server cold**, not a human.
Every scenario is judged from the agent's point of view: can it pick the right tool
on the first try, construct a valid call from what the interface tells it, and
recover from mistakes — while spending as little of its context budget as possible.

### User Story 1 - Consolidated, non-overlapping compact surface a cold agent can navigate (Priority: P1)

A fresh agent connects to the server with no prior knowledge of it. It is asked to
do ordinary code-intelligence work — find where a class is defined, read the code
around a match, search for a phrase, list the symbols in a package. The compact
surface presents a **small set of tools with no two tools (or operations) competing
for the same job**, so the agent can map each task to exactly one obvious tool and
operation without guessing.

**Why this priority**: This is the core of the feature. The current compact surface
has overlapping responsibilities (reference-finding appears under search, symbols,
and compound tools) and a vague `opengrok_compound` tool whose name does not tell an
agent what it does. Removing overlap and making each tool's job unambiguous is the
single biggest lever on whether an agent can use the surface at all. Without it,
nothing else matters.

**Independent Test**: Give a fresh agent (minimal context) a fixed list of realistic
tasks against the real instance (https://opengrok.home/) using only the compact
surface. Measure whether it selects the correct tool+operation on the first attempt
and completes each task. Delivers value on its own even if the default is never
flipped: a coherent, learnable compact surface.

**Acceptance Scenarios**:

1. **Given** the compact surface, **When** an agent inspects the tool list, **Then**
   each task in the standard suite maps to exactly one tool+operation, and no two
   operations are near-duplicates of each other without a documented, agent-visible
   distinction.
2. **Given** a task like "find the definition of `PaymentProcessor` and read its
   surrounding code", **When** the agent works only from tool names and
   descriptions, **Then** it chooses the intended tool+operation on the first attempt
   without trial-and-error across overlapping tools.
3. **Given** a tool that bundles several operations, **When** the agent reads its
   description, **Then** the description names every operation, when to use each, and
   the relevant gotchas — at no less depth than the equivalent full-surface tools.

---

### User Story 2 - Typed per-operation schemas restore field-level ground truth (Priority: P1)

When the agent picks a tool and operation, the tool exposes a **typed schema for
that operation** — the exact fields, which are required, their types, and any
enumerated values — instead of an opaque payload the agent can only learn about by
parsing prose. The agent can therefore construct a valid call from the schema alone.

**Why this priority**: The current compact surface wraps every tool in an untyped
`{operation, payload: object}` envelope. The payload's fields exist only inside the
description text, so a schema-aware client sees an empty object and the agent must
reverse-engineer required fields from prose. This is a direct interface-ground-truth
(L2) failure and a primary reason compact is not yet default-quality. It is P1
alongside Story 1 because consolidation without real schemas would just move the
ambiguity from tool selection to call construction.

**Independent Test**: For every operation, introspect the tool's input schema and
confirm the operation's required fields, types, and enum values are present and
accurate. Then have an agent construct calls using only schema introspection (no
description prose) and confirm the calls are valid.

**Acceptance Scenarios**:

1. **Given** any compact tool, **When** a client introspects its input schema for a
   selected operation, **Then** the operation's required fields, optional fields,
   types, and enumerated values are all discoverable from the schema — none exist
   only in prose.
2. **Given** an agent constructs a call from the schema alone, **When** it submits
   the call, **Then** the call is accepted without the agent having needed to read
   the free-text description to learn a required field.
3. **Given** an operation that does not apply a field, **When** the agent reads the
   schema, **Then** that field is not presented as applicable to that operation.

---

### User Story 3 - Capability parity: compact does everything an agent needs from full (Priority: P2)

Any code-intelligence task an agent can accomplish on the full surface can also be
accomplished on the compact surface (memory is the one deliberate exception — it
stays full-only, FR-014). In particular, the gaps that exist today — listing files in
a project directory and getting a project overview (languages, counts, top-level
structure) — are present in compact.

**Why this priority**: Compact cannot become the default while it can do strictly
less than full. Today `opengrok_projects` only lists projects; there is no compact
equivalent of `list_files` or `get_project_overview`, so an agent on compact loses
project-structure and overview capability. Parity is required before the default can
move, but it is P2 because it depends on the consolidated structure from Story 1.

**Independent Test**: Enumerate every agent-relevant capability reachable on the full
surface and confirm each maps to a compact tool+operation that returns equivalent
information against the real instance.

**Acceptance Scenarios**:

1. **Given** the full surface's capability list, **When** mapped onto compact,
   **Then** every full capability has a compact equivalent (including file listing
   and project overview), with no capability reachable only on full **except the
   memory operations, which are intentionally full-only (FR-014)**.
2. **Given** an agent asks "what languages does project X use?" on the compact
   surface, **When** it calls the appropriate tool+operation, **Then** it receives a
   per-language breakdown equivalent to the full surface's project overview.

---

### User Story 4 - Compact becomes the shipped default without breaking existing users (Priority: P2)

With no surface configured, the server now starts on the **compact** surface. Users
who explicitly select the full surface continue to get the existing full behavior,
byte-for-byte. A migration note explains the change and how to restore full.

**Why this priority**: This is the explicit goal — making compact the default — but
it is only safe once Stories 1–3 make compact non-overlapping, schema-complete, and
capability-complete. It is P2 because the polish must land first; flipping the
default on an inadequate surface would regress agent reliability.

**Independent Test**: Start the server with no surface environment variable and
confirm the compact tools are registered; start it with the full surface explicitly
selected and confirm the full tools are registered with unchanged behavior.

**Acceptance Scenarios**:

1. **Given** no tool-surface configuration, **When** the server starts, **Then** the
   compact tools are registered and the full fine-grained tools are not.
2. **Given** the full surface is explicitly selected, **When** the server starts,
   **Then** the full tools are registered and behave exactly as before this feature.
3. **Given** a user upgrading from a prior version, **When** they read the migration
   note, **Then** they can identify which tool names/shapes changed and how to pin
   the full surface.

---

### User Story 5 - Compact measured as a first-class surface, with demonstrated default-readiness (Priority: P3)

Before compact is declared default-ready, the eval harness treats compact as a
**first-class measured surface**, not a secondary comparison: the contract suite runs
its scenarios on compact (and adds compact-specific cases), cross-surface equivalence
is asserted, and compact carries its own committed baseline that gates CI. On top of
that, an evaluation run shows compact matches or beats full on task success without
costing more tokens, and an agent-ergonomics review of the compact surface surfaces no
Critical findings.

**Why this priority**: The decision to flip the default must rest on enforced evidence,
not assertions. Today the contract suite is full-surface-only and the token benchmark
skips compact's missing operations, so compact's correctness is largely unmeasured. This
is P3 because it validates rather than builds the surface, but it is the gate that
authorizes the default change.

**Independent Test**: Run the eval harness (contract checks + token economy benchmark)
and confirm every contract scenario executes on compact, compact-specific cases cover
the new/consolidated operations, cross-surface equivalence holds, and a compact baseline
is committed and gates CI; run an agent-ergonomics review of the compact surface and
inspect the scorecard and findings.

**Acceptance Scenarios**:

1. **Given** the scenario suite, **When** evaluated on compact versus full, **Then**
   compact's task-success rate is greater than or equal to full's and its token cost
   per successful task is less than or equal to full's.
2. **Given** an agent-ergonomics review of the compact surface, **When** the report
   is produced, **Then** it contains no Critical findings and scores Tool & Interface
   Design and Economic Design no lower than the full surface.
3. **Given** the contract eval suite, **When** it runs, **Then** every existing scenario
   is exercised on the compact surface (none remain full-surface-only) and compact-specific
   cases cover the new/consolidated operations (project overview, file listing on compact,
   invalid-operation errors, and constructing a call from the typed schema alone).
4. **Given** a scenario that runs on both surfaces, **When** compact's result diverges
   from full's in hits, citations, pagination, or warnings, **Then** the eval fails.
5. **Given** a committed compact baseline, **When** a change regresses compact's contract
   behavior, **Then** CI fails; token-cost deltas are reported but do not fail CI (v1).

---

### Edge Cases

- **Unknown operation**: When an agent passes an operation value a tool does not
  support, the response is an actionable error that names the valid operations — not
  a silent empty result or an opaque payload error.
- **Missing required field**: When a required field for the selected operation is
  absent, the error names the missing field and the operation it belongs to.
- **Capability-gated tool/operation**: When the backing OpenGrok capability for an
  operation was not verified at startup, the operation is absent from the tool's schema
  enum and rejected at dispatch with an explanation. When none of a tool's operations are
  available, the tool is **not registered at all** — it never appears in `ListTools`, so
  it costs the agent no attention or context. This matches how the full surface gates its
  tools.
- **Response-state distinguishability**: For every consolidated tool, the agent can
  tell apart success, empty/zero-result, partial/truncated, warning-carrying, error,
  and unauthorized responses — in particular an empty result is never indistinguishable
  from an error.
- **Pagination across operations**: A cursor issued by a consolidated tool round-trips
  correctly and continues the same operation's result set; truncation flags and
  `total_*` counts are preserved.
- **Memory excluded from compact**: The compact surface registers no memory tool;
  memory remains a full-surface, stdio-only capability, pending a separate decision to
  sunset it. An agent on compact never sees a memory tool.
- **Removed/renamed tool names**: An agent or config referencing a prior compact tool
  name (or a full-only tool name) can find the new tool+operation via the migration
  note.

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: Significant and intentional. The compact tools change from
  an untyped `{operation, payload}` envelope to typed per-operation input schemas;
  overlapping operations are consolidated or removed; compact gains file-listing and
  project-overview operations and **omits the memory tool** (memory stays full-only,
  pending sunset); one or more compact tool names may be clarified (e.g.
  the vague `opengrok_compound`); and the **default tool surface changes from full to
  compact**. Output fields, warnings, cursor/pagination semantics, and `citation.url`
  are preserved across the consolidation. All changes are documented as public
  contract with a migration note. The full surface contract is unchanged.
- **OpenGrok Semantics**: Underlying semantics are unchanged — full-text + ctags,
  with best-effort/heuristic/page-local behaviors (implementation search,
  cross-project attribution, kind filtering, sorting, overview data) preserved and
  still surfaced through warnings. The redesign is validated against the real
  instance at https://opengrok.home/.
- **Security Impact**: None new. Secrets remain in environment variables; HTTP mode
  retains its existing trust assumptions; memory stays process-scoped and disabled over
  HTTP and is exposed only on the full surface (compact omits it). Changing the default
  surface does not change the security posture.
- **Documentation Impact**: README (default surface and setup), `docs/configuration.md`
  (default value of the surface selector), `docs/tool-contracts.md` (compact tool
  inputs/outputs/operations), `docs/limitations.md` (any best-effort/heuristic
  operations on compact), `docs/agent-usage-patterns.md` (compact-first workflow),
  `evals/README.md` and the committed `evals/baselines/` (compact added as a first-class
  measured surface), `CHANGELOG.md`, and a migration note must be updated.
- **Experimental Impact**: Compact moves from non-default to default, so any
  "experimental" or "non-default" framing of compact in tool descriptions, docs, and
  config naming must be removed. The gateway surface remains experimental and
  unaffected.
- **Resource Bounds**: Existing response-size, page-size, truncation, and
  automatic-fetch limits/defaults/warnings carry over unchanged to the consolidated
  tools; no new auto-fetch or response-size behavior is introduced. Consolidation
  must not raise per-call output size relative to the equivalent full tools.

## Requirements *(mandatory)*

### Functional Requirements

**Consolidation and non-overlap (Story 1)**

- **FR-001**: The compact surface MUST present a consolidated set of tools in which
  no two tools or operations serve overlapping purposes; each task in the standard
  task suite MUST map to exactly one obvious tool+operation.
- **FR-002**: Reference-finding, implementation-finding, and cross-project-reference
  behaviors MUST be reachable through a single, clearly delineated place in the
  surface rather than duplicated across multiple tools, with any retained variants
  given an agent-visible, documented distinction.
- **FR-003**: Every compact tool name MUST communicate the tool's job to a cold agent;
  vague names (e.g. a generic "compound" tool) MUST be replaced with names that
  describe what the tool does.

**Descriptions and schemas (Stories 1–2)**

- **FR-004**: Each compact tool MUST expose typed, per-operation input schemas
  (selected by the operation field) so that required fields, optional fields, types,
  and enumerated values for the chosen operation are discoverable via schema
  introspection, not only via prose.
- **FR-005**: Each operation MUST carry a description, written for a cold agent, that
  states its purpose, when to use it, and its gotchas, at no less depth than the
  equivalent full-surface tool description. Compact MUST NOT reduce description
  quality relative to full.
- **FR-006**: Invalid operation values and missing required fields MUST produce
  actionable errors that name the valid operations or the missing field and its
  operation; the surface MUST NOT fail silently or return opaque payload errors.

**Capability parity (Story 3)**

- **FR-007**: The compact surface MUST cover every agent-relevant capability available
  on the full surface, including listing files in a project directory and retrieving a
  project overview (language breakdown, file/directory counts, top-level structure). The
  memory operations are the sole intentional exception (see FR-014).
- **FR-008**: Switching from full to compact MUST NOT reduce the set of tasks an agent
  can accomplish; there MUST be no capability reachable only on the full surface, with the
  single deliberate, documented exception of the memory operations (FR-014).

**Default change and compatibility (Story 4)**

- **FR-009**: When no tool surface is configured, the server MUST start on the compact
  surface (the shipped default changes from full to compact).
- **FR-010**: The full surface MUST remain selectable via the existing surface
  configuration and MUST behave byte-for-byte as before this feature (no silent change
  to stable full behavior).
- **FR-011**: A migration note MUST document the default change, the mapping from prior
  compact tool names/shapes and from full tool names to the new compact tools+operations,
  and how to pin the full surface.

**Contract preservation across consolidation (all stories)**

- **FR-012**: Consolidated tools MUST preserve output fields, pagination cursors,
  truncation flags, `total_*` counts, warnings, and `citation.url` semantics identical
  to their full-surface counterparts.
- **FR-013**: Capability gating MUST apply to compact at both levels, identically to the
  full surface: (a) a compact tool MUST be registered only when at least one of its
  operations has a verified backing capability — a tool with no available operations MUST
  NOT be registered, so it never appears in `ListTools` and consumes no agent attention or
  context; and (b) within a registered tool, an operation whose backing capability was not
  verified MUST be absent from the operation schema/enum and rejected at dispatch with an
  explanation.
- **FR-014**: The compact surface MUST NOT expose any memory tool or memory operations.
  Memory remains available only on the full surface (process-scoped, stdio-only), pending
  a separate decision to sunset the memory feature. This is a deliberate, documented
  divergence between surfaces (Constitution I).
- **FR-015**: Every distinct response state (success, empty/zero-result,
  partial/truncated, warning, error, unauthorized) MUST be distinguishable by the agent
  for each consolidated tool.

**Labeling, validation, and forward guidance (Stories 4–5)**

- **FR-016**: Any "experimental" or "non-default" labeling of the compact surface in
  tool descriptions, docs, and config naming MUST be removed once compact is the default;
  the gateway surface MUST remain labeled experimental.
- **FR-017**: The redesigned compact surface MUST be validated against the real OpenGrok
  instance (https://opengrok.home/) and through the eval harness (contract checks and
  token economy benchmark) before compact is declared default-ready.
- **FR-018**: A non-binding recommendation MUST be recorded for full-surface tools that
  could later be consolidated (e.g. `read_file`/`get_file_context`,
  `search_symbol_definitions`/`search_symbol_references`, `search_implementations`),
  without changing the full surface in this feature.

**Evaluation and measurement (Story 5)**

- **FR-019**: The contract eval suite MUST exercise its scenarios on the compact surface
  (resolved through the canonical-op→surface mapping), not on the full surface alone; no
  contract scenario may remain full-surface-only.
- **FR-020**: Compact-specific eval cases MUST be added for the consolidated and new
  operations, including project overview, file listing on compact, invalid-operation
  errors, and constructing a valid call from an operation's typed schema alone.
- **FR-021**: Where a scenario runs on more than one surface, the eval harness MUST assert
  that the compact result is equivalent to the full result (matching hits, `citation.url`,
  pagination cursors and `total_*` counts, and warnings) and MUST fail on divergence.
- **FR-022**: Compact MUST have its own committed eval baseline, and CI MUST fail on a
  compact contract regression. Token-cost deltas for compact MUST be reported but remain
  non-gating, consistent with the token benchmark v1 policy.
- **FR-023**: The eval harness MUST NOT skip a canonical operation on compact for lack of
  an equivalent; once parity lands, every canonical operation (including file listing) MUST
  resolve to a real compact tool+operation, and the compact resolver MUST target the typed
  per-operation schemas rather than the untyped envelope.

### Key Entities *(include if feature involves data)*

- **Tool surface**: The named set of MCP tools registered for an agent session
  (full, compact, gateway). The selectable, shipped-default value is what this feature
  changes.
- **Compact tool**: A consolidated MCP tool that exposes several related operations,
  each with its own typed input schema and description.
- **Operation**: A named sub-action within a compact tool (e.g. code search, symbol
  listing, file read). Carries the typed schema and description the agent relies on.
- **Capability gate**: A startup-verified backing OpenGrok capability that controls
  whether a tool or operation is available; when all of a tool's operations are
  unavailable, the tool is not registered at all (so it costs the agent no attention).
- **Migration note**: The documentation artifact mapping prior compact names/shapes and
  full tool names onto the new compact tools+operations, and recording the default
  change.
- **Eval scenario (canonical)**: A surface-agnostic scenario (canonical `op` + `args`)
  resolved onto each surface by the harness, so one scenario measures full and compact
  alike.
- **Eval baseline**: The committed per-surface reference results (`evals/baselines/`) that
  CI compares against to detect regressions; compact gains its own.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A cold agent selects the correct compact tool+operation on the first
  attempt for at least 90% of the standard task suite, with no task requiring
  trial-and-error across overlapping tools.
- **SC-002**: With all capabilities available, the compact surface exposes a stable 3–4
  tools (typically four: `opengrok_projects`, `opengrok_search`, `opengrok_symbols`,
  `opengrok_read` — no memory tool), with zero overlapping tools (each task maps to exactly
  one obvious tool); when a backing capability is unavailable, its tool/operations are not
  registered at all.
- **SC-003**: 100% of agent-relevant full-surface capabilities — except the memory
  operations, which are intentionally full-only (FR-014) — are reachable on the compact
  surface (capability parity, including file listing and project overview).
- **SC-004**: 100% of compact operations have their required fields, types, and enum
  values discoverable from schema introspection alone (zero operations whose required
  inputs exist only in prose).
- **SC-005**: On the eval scenario suite, compact's task-success rate is greater than
  or equal to full's, and its token cost per successful task is less than or equal to
  full's (the token economy benchmark shows a non-negative success delta and a
  non-positive token delta).
- **SC-006**: An agent-ergonomics review of the compact surface returns no Critical
  findings and scores Tool & Interface Design and Economic Design no lower than the
  full surface.
- **SC-007**: Users who explicitly select the full surface observe zero behavioral
  change versus the prior version (full remains byte-for-byte stable).
- **SC-008**: After the default flip, zero capability regressions are observed: every
  task that succeeded before the flip still succeeds on the new default.
- **SC-009**: For every consolidated tool, all six response states (success,
  empty/zero-result, partial/truncated, warning, error, unauthorized) are individually
  distinguishable by the agent.
- **SC-010**: 100% of contract eval scenarios execute on the compact surface (zero
  scenarios remain full-surface-only), and every new/consolidated compact operation has at
  least one dedicated eval case.
- **SC-011**: Cross-surface equivalence holds on all shared scenarios — zero divergences
  between compact and full in hits, citations, pagination, or warnings.
- **SC-012**: A compact eval baseline is committed and CI fails on any compact contract
  regression; the number of canonical operations skipped on compact for "no equivalent" is
  zero.
- **SC-013**: When a backing capability is unavailable, neither its compact tool nor its
  operations appear in `ListTools` (zero empty tools; no agent attention spent on
  unavailable capabilities), identical to the full surface's registration gating.

## Assumptions

- The real OpenGrok instance at https://opengrok.home/ is reachable from the
  development/test environment without authentication for validation, consistent with
  prior verification recorded in `specs/002-minimal-setup-surface/quickstart.md`.
- "Eligible to become the new default" is satisfied in this feature by both (a) the
  polish work and (b) actually flipping the shipped default to compact, per the
  clarified scope; flipping is in scope, not deferred.
- The full surface stays byte-for-byte stable; consolidation of full-surface tools is
  out of scope and recorded only as a non-binding recommendation for a future feature.
- Compact tools keep the `opengrok_*` naming convention, with operation-oriented names
  where they improve agent clarity.
- Memory is a candidate for a future sunset; this feature omits it from the compact
  surface only (a stable 3–4 tool surface) and does not change memory on the full surface.
- The standard task suite and token/score thresholds are measured using the existing
  eval harness scenario suite (`evals/`, including the token economy benchmark) as the
  measurement basis. Per the clarifications, compact is measured as a first-class surface:
  existing contract scenarios are parameterized onto compact, compact-specific cases are
  added for new/consolidated operations, cross-surface equivalence is asserted, and a
  committed compact baseline gates CI (token-cost deltas remain reported but non-gating per
  the token benchmark v1 policy).
- The agent-ergonomics review, the tool-interface design, the build, and the go/no-go
  gate are produced with the project's agent-ergonomics skills (design templates,
  agent-ergonomics inspector, evaluation-harness designer, implementation playbooks,
  review checklists), since the primary consumer of this surface is an agent.
- Surface selection continues to be controlled by the existing environment variable;
  no new configuration mechanism is introduced beyond changing its default value.
