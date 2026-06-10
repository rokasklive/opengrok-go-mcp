# Quickstart: Minimal Setup Surface

**Feature**: `002-minimal-setup-surface` | **Date**: 2026-06-10

## North star — base URL only

Minimal MCP client environment (stdio):

```json
{
  "environment": {
    "OPENGROK_MCP_BASE_URL": "https://your-opengrok-host/source/api/v1"
  }
}
```

Expected behavior on a typical reverse-proxied OpenGrok instance:

1. Server derives web UI URL from base URL.
2. Attempts `GET /projects/indexed`.
3. If that fails or is empty, fetches web UI once and parses the project picker.
4. Auto-sets default project when exactly one project is found.
5. Probes search/file capabilities and registers working tools.

## Add auth when the instance requires it

If startup logs the auth remediation message, set `OPENGROK_MCP_API_TOKEN` to the full
`Authorization` header value and restart:

```json
{
  "environment": {
    "OPENGROK_MCP_BASE_URL": "https://your-opengrok-host/source/api/v1",
    "OPENGROK_MCP_API_TOKEN": "Basic dXNlcjpwYXNz"
  }
}
```

Or Bearer:

```json
"OPENGROK_MCP_API_TOKEN": "Bearer <token>"
```

## Disable web-UI project discovery

For API-only or policy-constrained deployments:

```json
"OPENGROK_MCP_DISABLE_PROJECT_SCRAPE": "true"
```

When disabled and the REST project list is unavailable, set an explicit project list or default:

```json
"OPENGROK_MCP_PROJECTS": "project-a,project-b",
"OPENGROK_MCP_DEFAULT_PROJECT": "project-a"
```

## Agent workflow after base-URL-only startup

1. Call `list_projects` to see the startup-resolved snapshot.
2. If multiple projects and no default, pass `project` on search/read tools.
3. If search tools are missing from the tool list, check startup logs for auth or TLS guidance.

## Verification checklist (operator)

- [x] Server starts with only `OPENGROK_MCP_BASE_URL` set
- [x] `list_projects` returns discovered projects without manual project env vars
- [x] Search works when instance allows anonymous access, or after adding auth token
- [x] Startup log names auth env vars when probes return 401 without a token *(N/A: search probes succeed anonymously on this instance; auth remediation not triggered)*
- [x] No web-UI fetch when REST project API returns a non-empty list *(N/A: `/projects/indexed` returns 401 here, so scrape path exercised)*

**Verified 2026-06-10** against `https://opengrok.home/api/v1/` (base URL only): scrape discovered 6 projects; `list_projects` and `search_code` (project `opengrok-go-mcp`) succeeded without auth tokens.

Re-run the full config matrix anytime:

```bash
go build -o /tmp/opengrok-go-mcp ./cmd/opengrok-go-mcp
OPENGROK_MCP_BASE_URL=https://your-host/api/v1 python3 scripts/live-config-matrix.py
```

## Migration from previous setup

| Old config | New equivalent |
|---|---|
| `OPENGROK_MCP_DEFAULT_PROJECT` + base URL | Base URL only (if discovery finds projects) |
| `OPENGROK_MCP_PROJECT_SCRAPE=true` | Default behavior; remove the variable |
| `OPENGROK_MCP_PROJECT_SCRAPE=false` | `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE=true` |
| Four-variable example blocks in README | One-variable block + optional auth section |
