# opengrok-go-mcp Constitution

## Core Principles

### I. Agent-Focused MCP Contract

Every feature MUST preserve a clear, machine-usable MCP contract for coding
agents. Tool inputs, output fields, pagination cursors, citations, warnings,
and capability gates MUST be explicitly documented and tested when changed.
Full, compact, and gateway tool surfaces MUST remain coherent views over the
same underlying behavior unless a feature spec states a deliberate exception.
Tool names, descriptions, schemas, warnings, defaults, and examples MUST be
written for a cold agent seeing the server for the first time. Non-trivial
agent-facing changes SHOULD be evaluated by a fresh lightweight or mid-tier
subagent where available, or by a fresh-session simulation otherwise, before
merge. The evaluator SHOULD receive a realistic task and minimal upfront
project/tool context so its report preserves contrast with the initiated
implementer's expectations.

Rationale: this project exists to let agents search and read OpenGrok-backed
code reliably. Ambiguous schemas or uneven tool surfaces directly reduce agent
correctness. Mechanical tests prove behavior, but fresh-subagent evaluation
proves whether the tool surface is discoverable and usable by its actual
audience. That contrast is valuable evidence for improving descriptions,
schemas, warnings, defaults, and examples.

### II. Evidence-Backed OpenGrok Semantics

The server MUST represent OpenGrok results honestly. Features MUST distinguish
full-text, path, definition, reference, and best-effort structural searches, and
MUST surface uncertainty, truncation, page-local filtering, fallback behavior,
and attribution risk in responses or documentation. Answers and docs MUST NOT
claim AST-level, call-graph, or exhaustive implementation knowledge unless the
implementation provides that evidence.

Rationale: OpenGrok is a full-text and ctags-backed index, not a complete
semantic analysis engine. The MCP layer must make those limits visible instead
of hiding them behind convenient names.

### III. Test-Proven Go Changes

Behavioral changes MUST include focused tests that fail against the old behavior
or otherwise prove the new behavior. Tests MUST cover success behavior, edge
cases, and user-visible warnings or errors for any changed MCP contract. For
non-trivial behavior changes, tasks SHOULD be ordered test-first unless the
plan documents why another sequence is clearer. `go test ./...` MUST pass
before work is considered complete.

Rationale: the server is small enough that fast Go tests are the primary guard
against contract regressions, schema drift, and pagination or cursor mistakes.

### IV. Secure Local Operation

Secrets MUST be supplied through environment variables and MUST NOT be logged,
committed, or accepted as command-line flags in new workflows. HTTP transport
MUST remain loopback-first and documented as unsafe to expose directly without
external authentication and network controls. TLS verification bypasses and raw
file fallbacks MUST be explicit, opt-in, and documented with risk.

Rationale: the server often runs beside developer credentials and internal
OpenGrok instances. Local convenience cannot weaken token handling or network
exposure boundaries.

### V. Simplicity, Compatibility, and Documentation

Changes MUST prefer small, idiomatic Go packages and existing internal patterns
over new abstractions. Public tool behavior, response fields, environment
variables, and defaults MUST remain backward compatible unless a feature spec
and migration note justify the break. README and limitations documentation MUST
be updated in the same change when user-facing behavior, configuration, or
operational limits change.

Experimental features MUST be explicitly labeled in tool descriptions,
documentation, and configuration names. Experimental behavior may change between
minor versions, but MUST NOT silently alter stable tool behavior or defaults.
Features that can increase response size, tool-call count, or automatic file
fetching MUST define explicit limits, defaults, and warnings.

Rationale: agent integrations depend on stable contracts and concise docs.
Unnecessary architecture or undocumented defaults create avoidable integration
costs.

## Technical Constraints

The project is a Go MCP server with the module path
`github.com/rokasklive/opengrok-go-mcp`. New Go code MUST be formatted with
`gofmt`, follow existing package boundaries under `cmd/` and `internal/`, and
use table-driven tests where they keep behavior clear.

The MCP API is the compatibility boundary. Tool schemas, descriptions, response
fields, warnings, resources, and environment variables are user-facing even when
implemented in `internal/`. Pagination and cursor behavior MUST be deterministic
and documented for every paginated operation.

OpenGrok capability probing MUST gate unavailable tools or operations instead
of exposing calls that are known to fail. When behavior is best-effort,
truncated, heuristic, or page-local, responses MUST carry enough metadata or
warnings for an agent to decide whether to narrow, paginate, or verify.

## Development Workflow and Quality Gates

Specifications MUST define independently testable user stories, explicit edge
cases, and measurable outcomes for agent-facing behavior. Plans MUST record the
real repository paths being changed, the affected MCP surface, security impact,
documentation impact, agent UX validation plan, and the tests that will prove
the behavior.

Implementation tasks SHOULD be ordered test-first for each non-trivial
behavioral slice: write or update the proving test, run the targeted test,
implement the smallest coherent change, run the targeted test, then run the
relevant package or full-suite verification. If another sequence is clearer,
the plan MUST document why. Documentation-only changes MUST still be reviewed
against this constitution for accuracy and compatibility.

Completion requires no unexplained constitution gate violations, no remaining
placeholder template text in generated artifacts, passing verification commands
documented in the plan, fresh-subagent usability findings for non-trivial
agent-facing changes, and updated README or `docs/` content when user-facing
behavior changed.

## Governance

This file is the source of truth for project principles. Amend it in the same
change that needs the amendment.
