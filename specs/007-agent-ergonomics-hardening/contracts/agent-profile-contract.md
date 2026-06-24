# Contract: Agent Profile

**Feature**: 007-agent-ergonomics-hardening | **Date**: 2026-06-24
**Audience**: AI agents and operators. **Status**: design.

Shaped with **design-templates** §6 Progressive Disclosure and **agent-ergonomics**
L1/L5: the profile is the operator's one-knob default; per-call fields are the
agent's escape hatches.

## Environment

| Variable | Values | Default |
|----------|--------|---------|
| `OPENGROK_MCP_AGENT_PROFILE` | `economy`, `rich` | unset → `economy` |

Invalid value → server refuses to start with actionable error listing valid values.

## Bundled defaults (when per-call field omitted)

| Behavior | `rich` | `economy` |
|----------|--------|-----------|
| Context expansion around search hits | on | off |
| Response detail | `full` | `compact` |
| Include `display_url` / `raw_url` | on | off |

`citation.url` is **always** present on search/symbol results regardless of profile.

## Per-call overrides (always win)

- `expand_context` (bool pointer)
- `response_mode` (`full` | `compact`)
- `include_links` (bool pointer)
- `include_snippets`, `context_budget`, `page_size` — unchanged semantics

## Schema copy obligation

`expand_context` field description MUST state the default for the active profile
(see research D3). Applies to compact and full tool schemas.

## Tool description obligation

Compact search/symbols/read descriptions MUST include one economy-hint sentence
distinguishing:

- **Compact tool surface** — `OPENGROK_MCP_TOOL_SURFACE=compact` (four tools).
- **Lean response detail** — `response_mode=compact` (payload shape).

## Manifest exposure

`opengrok://capabilities` includes `agent_profile` string reflecting active profile.

## Non-goals (v1)

- Does not change OpenGrok query behavior.
- Does not flip shipped default to `economy` without separate migration note.

## Acceptance tests

- Config parse tests for valid/invalid profile.
- Helper tests: economy vs rich default resolution; per-call override.
- Token benchmark dimension: `economy` vs `rich` on four scenarios (SC-001).
- Compound tools still emit `warnings[]` under economy (FR-015).
