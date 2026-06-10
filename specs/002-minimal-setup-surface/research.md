# Research: Minimal Setup Surface

**Feature**: `002-minimal-setup-surface` | **Date**: 2026-06-10

## R1 — Scrape opt-out environment variable name

**Decision**: Introduce `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` (boolean, default `false`).

**Rationale**:
- Spec requires a single opt-out flag with scraping allowed by default.
- Positive opt-in name (`OPENGROK_MCP_PROJECT_SCRAPE=true`) misleads operators into thinking scraping is off unless enabled.
- `DISABLE_*` reads clearly in docs tables and startup logs.

**Backward compatibility**:
- Continue reading legacy `OPENGROK_MCP_PROJECT_SCRAPE` for one release cycle.
- Precedence: if `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` is explicitly set, it wins.
- Else if legacy `OPENGROK_MCP_PROJECT_SCRAPE` is explicitly set, map `true` → scraping enabled, `false` → disabled.
- Else default → scraping **enabled** when API discovery fails (behavior change from today).

**Alternatives considered**:
- Invert existing `OPENGROK_MCP_PROJECT_SCRAPE` default only — rejected; name still reads as opt-in and confuses docs.
- `OPENGROK_MCP_PROJECT_SCRAPE_DISABLED` — equivalent semantics; `DISABLE_PROJECT_SCRAPE` is shorter and matches common env naming.

---

## R2 — Startup when all search probes return 401/403

**Decision**: Do **not** abort startup when every search probe failed with unauthorized/forbidden **and** no auth token is configured. Emit a single consolidated auth remediation log line; return `(caps, nil)` with search capabilities false.

**Rationale**:
- FR-008 / SC-002 require eliminating `"check OpenGrok access: no search capabilities are available"` as a startup exit when auth is the only blocker.
- Operators validating base URL first should see a running MCP process and actionable guidance, not a crash loop.
- Capability gating already prevents agents from calling broken tools.

**Still abort startup when**:
- All search probes failed **and** at least one failure was not classified as `unauthorized`/`endpoint_disabled` attributable to missing credentials (e.g. TLS mismatch, transport error with no successful probe).
- Rationale: those indicate misconfiguration beyond "add a token".

**Auth remediation message** (canonical text for logs and docs):

```text
OpenGrok returned unauthorized responses and no auth token is configured.
Set OPENGROK_MCP_API_TOKEN (Bearer) or OPENGROK_MCP_BASIC_AUTH_TOKEN (Basic) and restart.
```

**Alternatives considered**:
- Always start regardless of failure class — rejected; TLS/URL misconfig should remain a hard fail.
- Register search tools that fail at call time with auth hints — rejected; violates honest capability gating (FR-009).

---

## R3 — Default project validation after discovery

**Decision**: Relax `validateDefaultProjectAfterDiscovery` so startup **never** requires `OPENGROK_MCP_DEFAULT_PROJECT`.

| `len(Projects)` | `DefaultProject` set | Behavior |
|---|---|---|
| 0 | any | Start; `source=none`; searches need explicit `project` or fail at call time |
| 1 | empty | Auto-set `DefaultProject = Projects[0]` (unchanged) |
| 1 | set | Validate membership if also in allowlist path with configured list; else accept |
| 2+ | empty | Start; no default; `ProjectRequired` forces explicit `project` on tools |
| 2+ | set | Must be in allowlist (unchanged validation) |

Also remove `Validate()` hard error for `DefaultProject == "" && len(Projects) > 1` when projects come from env — operator may intentionally omit default and rely on per-call `project`.

**Rationale**: FR-002, FR-003; multi-project instances are usable via `list_projects` without upfront default selection.

**Alternatives considered**:
- Auto-pick first scraped project as default with warning — rejected; silent wrong scope is worse than explicit `project` requirement.

---

## R4 — Documentation required-variable table

**Decision**: Document exactly one **required** variable: `OPENGROK_MCP_BASE_URL`.

Optional-but-common second line in examples: auth token when the target instance requires it (documented as optional, not required).

`OPENGROK_MCP_WEB_BASE_URL` stays optional override when derivation fails.

**Rationale**: FR-001, SC-004; matches north-star "one env var" story while staying honest that auth may be needed for function.

---

## R5 — Experimental labeling for scrape

**Decision**: Remove "experimental" label from the scrape **toggle** (now default fallback). Retain best-effort/heuristic labeling for **scraped project lists** in startup logs, `list_projects` behavior notes, and `docs/limitations.md`.

**Rationale**: Spec Experimental Impact — scraping is productized as default fallback; uncertainty is about list accuracy, not operator opt-in.

---

## R6 — Agent context file update

**Decision**: Add `<!-- SPECKIT START -->` / `<!-- SPECKIT END -->` markers to `AGENTS.md` (not present today) with active feature plan pointer.

**Rationale**: Spec Kit plan workflow expects downstream commands to find the active plan; no separate agent-context script exists in this repo.
