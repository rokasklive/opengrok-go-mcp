# Review Checklist

This checklist is usable both as a human review guide and as a prompt for a
review agent. Work through each section before approving or merging a change.

## Spec Compliance

- Change matches the accepted spec or design document; no scope has been
  exceeded.
- If no spec exists, the change is small enough that it does not require one
  (see Spec Kit Workflow in `AGENTS.md`).

## Constitution Compliance

- **I. Agent-Focused MCP Contract** — tool inputs, outputs, warnings, cursors,
  citations, and capability gates are documented and tested; fresh-subagent
  evaluation completed for non-trivial agent-facing changes.
- **II. Evidence-Backed OpenGrok Semantics** — search types (full-text, path,
  definition, reference) are distinguished; uncertainty, truncation, page-local
  filtering, and attribution risk are surfaced.
- **III. Test-Proven Go Changes** — behavioral changes include focused tests;
  `go test ./...` passes.
- **IV. Secure Local Operation** — secrets in env only; HTTP loopback assumption
  intact; TLS bypass and raw fallback are explicit, opt-in, and documented.
- **V. Simplicity, Compatibility, and Documentation** — idiomatic Go, existing
  patterns preferred; public defaults and fields backward compatible; README and
  `docs/` updated when user-facing behavior changed.

Any accepted violation of a principle is documented in the plan's Complexity
Tracking table with the reason and the simpler alternative that was rejected.

See [`.specify/memory/constitution.md`](../.specify/memory/constitution.md) for the full rationale of each principle.

## Grounded Tool Transparency Gate

Scope-limitation statement: this is a contract-compliance verification, not a
system-performance evaluation, and it does not certify that the tests it relies
on are themselves correct. A pass means "meets the verified contracts," not
"ready to ship."

- **G1 Claim registry and bijection:** every non-`none` claim has a resolvable
  conformance check, every check binds an existing `claim_id`, evidence fields
  are non-empty, and every `none` exemption is justified.
- **G2 Descriptions:** compact descriptions are registry-grounded, include the
  OpenGrok text+ctags/non-AST nature, supported and unsupported syntax, at least
  one example, and the configured default project when present.
- **G2 Schema transparency:** compact schema field descriptions equal full
  schema descriptions; optional field prose is not stripped for token savings.
- **G3 Errors and empty states:** compact validation returns specific
  `ToolErrorBody` codes with `suggestion`, query parser `400` maps to
  `QUERY_PARSER_FAILED`, and zero-hit search remains `IsError=false` with
  `total_hits=0`.
- **G4 Diagnostics:** response `diagnostics` is absent by default and present
  only when `OPENGROK_MCP_DIAGNOSTICS=true`.
- **G5 Evaluation validity:** trajectory grading is deterministic over the
  tool-call stream, reliability is reported as Pass^k, and token economy gates
  on cost per successful task while schema/payload bytes are secondary anomaly
  checks.
- **G6 Compatibility and coherence:** migration notes exist for public contract
  changes, all surfaces avoid claims the registry marks unsupported, and a
  fresh first-use/audit trajectory exists for agent-facing changes.

## MCP Contract Stability

- Tool names unchanged, or rename documented with migration note.
- Input fields: no new required fields on existing tools; removed/renamed fields
  have a spec and migration note.
- Output fields: additive-only additions are fine; repurposed or renamed fields
  require a spec, migration note, and version bump.
- Warnings, pagination metadata (`next_cursor`, `has_more`, `total_hits`,
  `page`, `total_pages`), and cursor round-trip semantics preserved.
- `citation.url` present and unmodified on all result items.
- Truncation indicators (`truncated=true` + `warning`) present on every new cap.
- Cross-surface consistency: behavior is coherent across `full`, `compact`, and
  `gateway` surfaces, or a spec explicitly records the divergence.

See [`tool-contracts.md`](tool-contracts.md) for the field-by-field contract reference.

## OpenGrok Semantic Honesty

- No claim of AST-level, call-graph, or exhaustive implementation knowledge
  unless the implementation provides that evidence.
- Best-effort, heuristic, page-local, and truncated behavior is surfaced in
  responses (warnings) or documentation.
- Attribution uncertainty (`attribution_uncertain=true` + warning) emitted where
  cross-project result paths cannot be matched.
- `search_implementations` and kind-filtered results described as candidate
  matches, not guaranteed semantic results.
- Fresh-subagent usability findings captured for agent-facing changes, with a
  realistic task and minimal upfront context.

See [`limitations.md`](limitations.md) for the complete list of known limitations.

## Security Posture

- Secrets supplied through environment variables; not logged, committed, or
  accepted as CLI flags in new workflows.
- HTTP transport remains loopback-first; no change widens network exposure
  without documentation and an explicit opt-in.
- `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY` and raw file fallback remain opt-in,
  documented with risk, and not silently activated.
- Memory tools remain disabled in HTTP mode.

See [`SECURITY.md`](../SECURITY.md) for the full security policy.

## Token / Response Budget

- No unbounded growth in response size, tool-call count, or automatic file
  fetching introduced without explicit limits, defaults, and warnings.
- New caps follow the truncation contract: `truncated=true` + `warning` with a
  narrowing suggestion.
- Experimental features that increase response size or auto-fetch behavior define
  limits before shipping.

## Tests

- Focused tests added or updated to prove new behavior; tests fail against old
  behavior or otherwise demonstrate the change.
- Success behavior, edge cases, and user-visible warnings/errors covered for any
  changed MCP contract.
- `go test ./...` passes.

## Documentation

- `README.md` updated for human-facing setup, configuration, or behavior
  changes.
- `docs/limitations.md` updated when behavior is best-effort, heuristic,
  truncated, page-local, or security-sensitive.
- `docs/agent-usage-patterns.md` updated when agent workflow guidance changes.
- Tool descriptions, schemas, warnings, defaults, and examples written for a
  cold agent seeing the server for the first time.
- Experimental features labeled in tool descriptions, docs, and config names.

## Compatibility And Migration Notes

- Public defaults unchanged, or change justified with a feature spec and
  migration note.
- Breaking changes (removed/renamed fields, altered pagination/cursor semantics)
  accompanied by a spec, migration note, and version bump.
- Additive changes (new optional inputs, new output fields, new warnings) include
  tests and docs in the same change.
