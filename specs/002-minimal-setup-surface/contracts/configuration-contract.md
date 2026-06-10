# Configuration Contract: Minimal Setup Surface

**Feature**: `002-minimal-setup-surface` | **Date**: 2026-06-10

This document defines operator-facing configuration contract changes. MCP tool input/output
schemas are unchanged.

## Required variables

| Variable | Required | Description |
|---|---|---|
| `OPENGROK_MCP_BASE_URL` | **Yes** | OpenGrok REST API base URL (typically ending in `/api/v1`) |

No other environment variable is required for startup validation.

## New variable

| Variable | Default | Description |
|---|---|---|
| `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` | `false` | When `true`, skip web-UI project discovery when the REST project list is unavailable or empty |

## Deprecated variable (compat shim)

| Variable | Status | Migration |
|---|---|---|
| `OPENGROK_MCP_PROJECT_SCRAPE` | Deprecated | Ignored when `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` is set. Otherwise `false` disables scraping; `true` enables (redundant with new default). Remove in a future release. |

## Previously required → now optional

| Variable | New status |
|---|---|
| `OPENGROK_MCP_DEFAULT_PROJECT` | Optional always. Auto-set when exactly one project is discovered. |
| `OPENGROK_MCP_API_TOKEN` | Optional. Full `Authorization` value: `Bearer <token>` or `Basic <credentials>`. Never logged. |
| `OPENGROK_MCP_PROJECTS` | Optional override; takes precedence over discovery. |

## Startup log contract (new)

When unauthorized responses occur and no auth token is configured, startup MUST emit:

```text
OpenGrok returned unauthorized responses and no auth token is configured.
Set OPENGROK_MCP_API_TOKEN to "Bearer <token>" or "Basic <credentials>" and restart.
```

When project discovery uses web UI:

```text
startup config: project source=scraped count=N (web-UI discovery; best-effort project list)
```

When scraping is disabled and API discovery failed:

```text
startup config: web-UI project discovery disabled (OPENGROK_MCP_DISABLE_PROJECT_SCRAPE); project source=none
```

## Startup failure contract (revised)

| Condition | Before | After |
|---|---|---|
| All search probes 401/403, no auth token | Exit: `no search capabilities are available` | **Start**; auth remediation log; search tools gated off |
| All search probes TLS/transport failure | Exit | Exit (unchanged) |
| Zero projects discovered, no default | Exit: default project required | **Start**; `source=none`; call-time `PROJECT_REQUIRED` |
| Multiple projects, no default | Exit: default project required | **Start**; explicit `project` on scoped tools |

## Capability gate contract (unchanged mechanism)

Tools register only when startup probes succeed. Gated tools are omitted from the MCP tool list
(not present with error-at-call-time).

## Backward compatibility guarantees

- Instances where REST project API already works: no extra web scrape; behavior unchanged.
- Explicit `OPENGROK_MCP_PROJECTS` / `OPENGROK_MCP_DEFAULT_PROJECT`: unchanged precedence.
- Operators who set `OPENGROK_MCP_PROJECT_SCRAPE=false` explicitly: scraping stays off (via compat shim).

## Breaking change notice (document in CHANGELOG)

- Default scraping when API discovery fails: **on** (was off). Operators relying on zero web-UI
  fetches on restricted instances must set `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE=true`.
- Startup no longer fails when default project is unset after multi-project discovery.
- Startup no longer fails when all search probes are unauthorized without a configured token.
