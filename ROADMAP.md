# ROADMAP

<!--
AI agent note: This roadmap uses a structured format with explicit metadata blocks per item.
Each entry has an ID, priority, status, and a clear decision-record section.
When implementing, check the `status` field first — only items marked `planned`
or `approved` are actionable for implementation.
-->

Roadmap items are directional notes, not implementation authority. Non-trivial
work must be converted into Spec Kit artifacts under `specs/FEATURE/` before
implementation (see `CONTRIBUTING.md` and `AGENTS.md`).

---

## Item: 001 — Mandatory Cursor Signing in HTTP Mode

- **Status:** planned
- **Priority:** medium
- **Effort:** small
- **Dependencies:** none
- **Labels:** security, http, cursor

### Problem

Cursor signing via `OPENGROK_MCP_CURSOR_SECRET` is optional and disabled by
default. The server logs `"WARNING: cursor signing disabled; set
OPENGROK_MCP_CURSOR_SECRET for integrity"` at startup when no secret is
configured.

For local stdio transport this is acceptable — the cursor value never leaves the
process boundary, and tampering requires access to the same machine/process.

In HTTP mode, cursors are serialized into JSON-RPC response payloads and
transmitted over the network. An unsigned cursor is a base64-encoded JSON blob
that any caller with access to the HTTP endpoint can decode, modify (e.g. bump
`offset` to skip results, alter `query` to mismatch state), and re-encode. The
server does call `Validate()` to check that the decoded query context matches
the expected context for the tool call, which catches many tampering attempts.
However, validation is not comprehensive — a determined caller could craft
cursor state that bypasses the checks, or simply reuse a cursor from a
different tool invocation in ways that confuse pagination.

### Proposed Solution

Two options, to be decided:

**Option A (recommended) — auto-generate a process-local secret.**

When `OPENGROK_MCP_CURSOR_SECRET` is not set **and** the transport is HTTP,
generate a random 32-byte HMAC key at process startup and assign it to
`cursor.Secret`. This makes cursors tamper-proof by default in HTTP mode
without any user configuration. The existing `signPayload` / `verifyPayload`
pair works unchanged — they just get a real key instead of the empty string.
The startup log line changes to not emit a warning in this case.

**Option B — strongly document the requirement.**

Keep the current behavior but add a prominent warning in the README and HTTP
mode docs that `OPENGROK_MCP_CURSOR_SECRET` should always be set when using
HTTP transport. Accept the risk that users may skip it.

### Decision Record

- **2026-05-23:** Added as planned item after review of cursor integrity
  posture. Option A preferred but needs implementation work.

### Implementation Notes

- Affected file: `cmd/opengrok-go-mcp/main.go` — conditionally generate secret
  when `cfg.CursorSecret == "" && transport == http`.
- Affected file: `internal/cursor/cursor.go` — no changes needed; already
  supports empty-secret graceful fallback via `signPayload`/`verifyPayload`.
- Affected test: `internal/cursor/cursor_test.go` — verify that a
  process-local secret produces signed cursors that `verifyPayload` accepts
  and that unsigned cursors are rejected by `verifyPayload` when a secret is
  set.
- Use `crypto/rand` for key generation, not `math/rand`.
- The secret must be set once before any cursor is encoded or decoded. A
  `sync.Once` guard in `cursor.go` is optional but defensive.

---

## Item: 002 — Split MCP Server Monolith

- **Status:** completed
- **Priority:** medium
- **Effort:** medium
- **Dependencies:** none
- **Labels:** refactor, maintainability, mcp-contract

### Problem

`internal/mcpserver/server.go` contains most of the MCP server implementation in
one large file: tool registration, gateway registration, resources, search
handling, pagination, result shaping, context expansion, and helper functions.
This makes future behavior changes harder to review because unrelated concerns
sit close together and large diffs are easy to misread.

The refactor should improve maintainability without changing the public MCP
contract, OpenGrok query semantics, warnings, pagination behavior, citations,
or compatibility expectations.

### Proposed Solution

Split the file by responsibility while keeping the existing package and public
types stable. A likely first pass:

- `server.go` — server construction and top-level registration orchestration.
- `tools.go` — full and compact MCP tool registration.
- `gateway.go` — gateway operation registry, discovery, and dispatch.
- `search.go` — search request handling, project resolution, warnings, sorting,
  and pagination.
- `results.go` — result shaping, citations, compact response shaping.
- `context_expansion.go` — automatic context expansion and diagnostics.
- `resources.go` — MCP resource and resource-template registration.

Keep the split mechanical at first. Avoid renaming JSON fields, changing tool
descriptions, changing defaults, or moving behavior across package boundaries
unless a later Spec Kit feature explicitly calls for it.

### Decision Record

- **2026-05-26:** Added as planned item to track near-term cleanup before more
  MCP tool and configuration work.
- **2026-06-10:** Completed in `specs/003-split-mcp-server` — monolithic
  `server.go` removed; per-concern files under `internal/mcpserver/` (see
  `internal/mcpserver/README.md`).

### Implementation Notes

- Completed layout: `register*.go`, `projects.go`, `search_*.go`, `results.go`,
  `symbols.go`, `filecontext.go`, `compact.go`, `memory_handlers.go`,
  `resources.go`, `helpers.go`, `service.go` — no `server.go`.
- Preserve existing exported and unexported behavior first; this should be a
  reviewable mechanical refactor.
- Run the full Go test suite before and after the split and compare failures, if
  any.
- Use focused follow-up commits for behavioral improvements after the split.
- A full Spec Kit workflow is not required for a purely mechanical split, but
  any MCP contract, schema, warning, pagination, citation, or tool-description
  change discovered during the refactor should become its own spec-backed item.

---

## Item: 003 — Security, OAuth, and Transport Posture Review

- **Status:** planned
- **Priority:** high
- **Effort:** large
- **Dependencies:** none
- **Labels:** security, oauth, transport, http, stdio, mcp-contract

### Problem

The project needs a deliberate security and transport model before it grows more
remote-execution and multi-client use cases. HTTP mode is currently the clearest
deployment path, but it has no built-in inbound client authentication and relies
on loopback binding, trusted networks, or external controls. Stdio mode is useful
for local MCP clients, but its lifecycle, shutdown behavior, and security
assumptions need a clearer support contract.

OAuth, remote MCP execution, stdio behavior, cursor integrity, local credential
handling, and deployment boundaries should be reviewed together instead of
patched piecemeal. This area is security-sensitive and should be spec-backed
before implementation.

### Proposed Solution

Start with a Spec Kit investigation that defines the supported deployment
profiles and threat model:

- local stdio client launching the server process
- local HTTP client using the loopback default
- remote or shared HTTP deployment behind external auth/network controls
- future OAuth-aware deployment, if compatible with the MCP SDK and expected
  clients

The investigation should produce concrete recommendations for:

- whether OAuth belongs in this server or should remain an external proxy concern
- whether HTTP mode should require stronger defaults for shared deployments
- what stdio support guarantees the project should make
- how credentials, cursor secrets, debug logs, and raw fallback URLs should be
  documented for local and remote use
- which security behaviors need tests, warnings, or startup diagnostics

Do not implement OAuth or transport policy changes directly from this roadmap
item. Convert the selected direction into one or more focused specs first.

### Decision Record

- **2026-05-26:** Added as planned security investigation item. Scope is
  intentionally broad because OAuth, HTTP, stdio, and deployment assumptions
  affect the same MCP contract and security posture.

### Implementation Notes

- Affected docs likely include `README.md`, `docs/limitations.md`,
  `docs/reporting-issues.md`, and any future deployment/security guide.
- Affected code may include `cmd/opengrok-go-mcp/main.go`,
  `internal/config/config.go`, cursor setup, HTTP server setup, and startup
  diagnostics.
- Use current MCP SDK documentation and client behavior as primary sources when
  evaluating OAuth or transport support.
- Keep stable local workflows working while improving remote/shared deployment
  safety.
- Any change to defaults, environment variables, transport behavior, auth
  behavior, warnings, or exposed tool/resource behavior requires Spec Kit
  coverage and fresh-agent UX validation.

---

## Template (for future items)

Copy-paste this block for new roadmap entries:

```markdown
---

## Item: XXX — Short Title

- **Status:** planned | approved | in-progress | completed | deferred | rejected
- **Priority:** low | medium | high | critical
- **Effort:** tiny | small | medium | large | xlarge
- **Dependencies:** none | item-XXX | item-XXX
- **Labels:** comma-separated tags

### Problem

Describe the issue or opportunity in 2–4 sentences. What doesn't work well or
what new capability is needed?

### Proposed Solution

1–3 approaches with trade-offs. If there is a clear recommendation, mark it.

### Decision Record

- **YYYY-MM-DD:** Added / updated / marked completed. One-liner per entry.

### Implementation Notes

- Bullet list of affected files, modules, test changes, or architectural
  concerns that an implementing agent should know.
```
