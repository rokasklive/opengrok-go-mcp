# Contract: Capability Manifest

**Feature**: 007-agent-ergonomics-hardening | **Date**: 2026-06-24
**Audience**: AI agents seeing the server cold. **Status**: design.

Addresses **L2 Interface Ground Truth** and review finding F2 (doc/runtime drift).

## Resource

| Property | Value |
|----------|-------|
| URI | `opengrok://capabilities` |
| MIME type | `application/json` |
| Availability | Always registered when MCP server starts (including all-tools-gated) |
| Mutability | Process-lifetime snapshot; changes only on restart |

## JSON shape (normative fields)

```json
{
  "interface_version": "ergonomics-1",
  "tool_surface": "compact",
  "agent_profile": "economy",
  "tools": [
    {
      "name": "opengrok_symbols",
      "operations": ["definitions", "list"],
      "summary": "ctags symbols: definitions, structural listing."
    }
  ],
  "gated": [
    {
      "capability": "SearchSymbolReferences",
      "reason_code": "PROBE_UNAUTHORIZED",
      "remediation": "Set OPENGROK_MCP_API_TOKEN and restart the server."
    }
  ],
  "project_catalog": {
    "source": "api",
    "is_snapshot": true,
    "project_count": 3,
    "default_project": "platform",
    "project_required": false
  }
}
```

## Semantics

- `tools[]` MUST match live `ListTools` for the process — same names, same
  `operation` enum members (FR-006).
- `gated[]` lists capabilities probed at startup but disabled; MUST NOT list
  operations that appear in `tools[]`.
- `remediation` MUST be actionable and MUST NOT contain secret values.
- `project_catalog.is_snapshot` is always `true` in v1 (startup-resolved list).

## Agent workflow (documented in agent-usage-patterns.md)

1. Fetch `opengrok://capabilities` OR read `operation` enum from `ListTools`.
2. Plan workflows only from listed operations.
3. If `gated[]` non-empty, surface `remediation` to the user before retry loops.

## Gateway surface

When `tool_surface=gateway`, `tools[]` lists `opengrok_discover` and
`opengrok_call`; `gated[]` includes note that operations require discover-then-call
(indirection cost documented in description).

## Security

- No `Authorization` values, tokens, or probe URLs with embedded credentials.
- Reason codes are machine-readable; messages are safe to show agents.

## Acceptance tests

- Hermetic backend with references gated: manifest lists gated references family.
- Full-capability backend: `gated[]` empty; all compact operations in `tools[]`.
- Resource readable via MCP `resources/read` in eval harness.
- `agent-usage-patterns.md` capability preamble links to this URI.
