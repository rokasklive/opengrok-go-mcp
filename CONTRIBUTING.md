# Contributing

opengrok-go-mcp uses GitHub Spec Kit for all non-trivial contributions.

Before implementation review, every meaningful change must have a corresponding
spec under `specs/` and must comply with `.specify/memory/constitution.md`.

This applies to:

- new MCP tools
- changes to existing tool inputs or outputs
- pagination, cursor, warning, citation, or response-shape changes
- configuration or environment variable changes
- transport/security behavior changes
- OpenGrok query behavior changes
- behavior that affects agent reliability, token usage, or compatibility

Small documentation fixes, typo fixes, formatting-only changes, dependency
metadata updates, and trivial test-only cleanups may be accepted without a full
Spec Kit workflow at maintainer discretion.

Contributions that arrive as direct code changes, forks, or patches are welcome,
but meaningful behavior changes must be accompanied by a spec or converted into
one before they can be reviewed for merge.

Implementation-first contributions are not eligible for merge review until they
are backed by a Spec Kit spec, implementation plan, and task breakdown.

## Maintainer Policy

The maintainer may refuse to review implementation-first changes when the
behavior is not clearly specified.

The purpose of this rule is not bureaucracy. It protects the MCP contract,
OpenGrok semantic honesty, backward compatibility, local-security assumptions,
and agent-facing reliability.

## Quick Start For Contributors

Build from source with Go 1.26.0 or later. CI and release builds use the Go
patch pinned in [`.go-version`](.go-version).

1. Read `AGENTS.md`.
2. Read `.specify/memory/constitution.md`.
3. For non-trivial changes, create or update `specs/FEATURE/spec.md`,
   `specs/FEATURE/plan.md`, and `specs/FEATURE/tasks.md`.
4. Implement against `tasks.md`.
5. Open a PR using `.github/PULL_REQUEST_TEMPLATE.md` (set Spec-impact and
   Docs-impact).

## What Counts As Trivial?

Examples that may skip the full Spec Kit workflow at maintainer discretion:

- typo and broken-link fixes
- formatting-only documentation changes
- comments-only clarifications with no behavior change
- dependency metadata updates with no runtime behavior change
- trivial test-only cleanups
