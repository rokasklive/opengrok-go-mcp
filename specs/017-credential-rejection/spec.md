# Feature Specification: Stop At The First Rejected Credentials

**Feature Branch**: `fix/credential-rejection`

**Created**: 2026-10-06

**Status**: Implemented

**Input**: A user's directory account kept getting locked while the server was
configured with a wrong password.

## Context & Motivation

OpenGrok deployments commonly sit behind HTTP Basic authentication backed by
LDAP/AD (Apache or Tomcat realm). There, **every request carrying rejected
credentials is a failed login**, and a handful of failed logins locks the
account. The server made that likely:

```
startup config: projects API unavailable: ... 401 Unauthorized        ← 1
startup config: web-UI project scrape failed: GET /: ... 401           ← 2
opengrok capability search_code: disabled: ... 401                     ← 3
opengrok capability search_symbol_definitions: disabled: ... 401       ← 4
opengrok capability search_symbol_references: disabled: ... 401       ← 5
opengrok capability server_side_sort: disabled: ... deadline exceeded  ← 6 (in flight)
opengrok-go-mcp: check OpenGrok access: no search capabilities are available
```

Five or six failed logins per start — then the process **exited**, so MCP
clients (Claude Desktop, Cowork, Claude Code sessions) started it again. The
account was locked again right after every unlock. Nothing told the agent, or
the user, what was wrong: the tools simply vanished.

## Decisions

**D1 — stop all traffic at the first rejection.** An `http.RoundTripper`
installed by `NewClient` whenever credentials are configured records the first
response that rejects them and answers every later request — any endpoint, any
retry — with the same `*AuthRejectedError` without contacting the server.
Transport retries skip it. The rejection is kept for the life of the process.

**D2 — tell a rejection from an endpoint restriction.** A 401 rejects the
credentials when it carries a `WWW-Authenticate` challenge for the scheme that
was sent (what Apache/Tomcat send) or comes from search. OpenGrok's own
per-endpoint API-token check can answer a bare 401 with valid credentials —
the reason project discovery falls back to scraping — so a bare 401 elsewhere
does not trip. A 403 never does; without configured credentials there is no
login to protect and nothing is installed.

**D3 — stay up and report.** When the credentials are rejected during startup,
web-UI discovery and capability probes are skipped, and the server starts with
the search and read tools registered instead of exiting. Every call returns a
new `AUTH_REJECTED` error whose `suggestion` tells the agent to report the
problem to the user and not retry; `opengrok_projects`, served from a snapshot
without a request, reports it too. A rejection at runtime (a password expiring
mid-session) has the same effect from then on.

**D4 — restart is the deliberate retry.** The rejection is not persisted. The
error tells the user to fix the token and restart — or, if they reset their
password to match the token instead, just restart. One failed login per
deliberate start, instead of a stream of them.

## Contract Impact

- New error code `AUTH_REJECTED` with `details.http_status`, `details.path`,
  `details.rejected_at` (documented in `tool-contracts.md`).
- With rejected credentials the server no longer exits at startup; tools stay
  registered and fail with `AUTH_REJECTED` until restart.
- Healthy credentials, endpoint-only 401s, 403s and anonymous setups are
  unchanged.

## Verification

Unit:

- Client: after a challenged 401, `ListProjects`, `Search`, `FileContent` and
  `ScrapeProjects` all fail with the cached rejection; exactly one request
  reached the server despite a 3-attempt retry policy. Bare 401 on the projects
  API does not trip; bare 401 on search does; 403 does not; no credentials, no
  breaker; a shared `http.Client` is not modified.
- Startup: a rejecting server sees exactly one request, discovery skips the web
  UI, probing is skipped, no error is returned and the tools are registered.
  An endpoint-only 401 still falls back to web-UI discovery and probes normally.
- MCP: the error maps to `AUTH_REJECTED` through `*url.Error` wrapping;
  `opengrok_search` returns it as structured content with the remediation in
  the text; `ListProjects` reports it directly and through the caching backend.

End-to-end, real binary against a local server rejecting every login like
Apache (`401` + `WWW-Authenticate: Basic realm=...`): one request in total, the
process stays up, and `opengrok_projects`, `opengrok_search` and
`opengrok_read` all return `AUTH_REJECTED`.

## Out of Scope

- Persisting rejections across restarts (D4).
- Credential checks for anonymous deployments (no account to lock).
