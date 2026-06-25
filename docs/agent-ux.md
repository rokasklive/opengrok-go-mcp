# Agent UX Guidelines

This document is about **authoring** the agent-facing surface: tool
descriptions, warning text, error messages, schema field names, and examples.
For how a *consuming* agent should use these tools in practice, see
[`docs/agent-usage-patterns.md`](agent-usage-patterns.md).

The normative source for the contract is
[`.specify/memory/constitution.md`](../.specify/memory/constitution.md)
(Principle I). For the field-level contract (inputs, outputs, pagination,
truncation, citations, errors), see [`docs/tool-contracts.md`](tool-contracts.md).

---

## A Cold Agent Should Know What To Do

Assume the agent reading a tool description has no project context, no prior
session history, and has never seen this server before. Tool names,
descriptions, input schemas, and examples are its only orientation.

Practical consequences:

- Name the tool after what it does, not after an internal module. `search_code`
  is clear; `fts_dispatch` is not.
- State the scope in the description. "Searches the full-text index for a
  given project" tells the agent when to reach for this tool. "Performs a
  search" does not.
- State what the tool does NOT cover. If `search_implementations` returns
  candidate references rather than language-semantic implementers, say so in
  the description — not just in `docs/limitations.md`.
- Do not rely on description prose to carry schema semantics. When a field has
  a non-obvious interaction (e.g. `project`, `projects`, and
  `allow_all_projects` are mutually exclusive), make that explicit in the
  field's `description` or `oneOf` constraint, not only in a markdown doc.
- Label experimental behavior at the tool level. If `gateway` mode may change
  before 1.0, put that in the tool description, not only in the README.

---

## Tool Descriptions Must Teach Usage

A good tool description answers three questions:

1. **What does it do?** — the primary operation in plain terms.
2. **When should the agent reach for it?** — the specific situation it handles
   best.
3. **What does it NOT do?** — the boundary, especially where an agent might
   overestimate capability.

The third point is the most commonly omitted. Omitting it is how an agent ends
up treating `search_implementations` as a semantic call-graph query and
trusting those results without verification.

For heuristic or best-effort operations, name the limitation in the description
itself. "Returns candidate reference matches — not guaranteed implementers" is
short enough to fit in a description and prevents a class of downstream
hallucination.

For structural searches (`list_symbols`, `get_file_context`), note relevant
page-local or context-budget constraints where they affect how results should
be interpreted.

## Keep Compact Transparent

Compact does not mean under-described. Compact tool descriptions may defer deep
catalogs to `opengrok://capabilities`, but field-level schema descriptions must
remain present and match the full surface. Do not strip optional-field prose for
`mode`, `sort`, `context_budget`, or similar controls to save bytes.

Boundedness is still required, but the primary metric is cost per successful
task in the eval harness. `ListTools`, schema bytes, and largest response bytes
are secondary anomaly checks. If a description grows too large, move edge-case
detail to the capability manifest; do not remove must-know ground truth from the
schema.

---

## Prefer Actionable Warnings

A warning that only reports a condition does not help the agent. A warning that
tells the agent what to do next does.

The test: if an agent reads this warning and cannot decide what to do
differently, the warning is not actionable enough.

Every warning should include at least one concrete next step:

- Which lever to pull (`path_prefix`, `file_type`, `project`, a more specific
  `query`, `mode=path`)
- Whether to paginate or narrow (when `total_hits > 500`, narrow — do not
  paginate blindly)
- Whether to verify locally (when attribution is uncertain, do not trust
  citations without checking)

### Good Vs Bad Warnings

```text
Bad:  "Too many results."
Good: "Search returned 1,247 hits; results are truncated. Narrow by path,
       symbol, or project, or request the next page."
```

Further examples:

```text
Bad:  "Kind filtering active."
Good: "kind=interface filter applied to page 1 only (12 of 38 results match).
       total_hits counts all definitions before filtering. Use next_cursor to
       continue or narrow path_prefix to reduce the unfiltered set."

Bad:  "Attribution uncertain."
Good: "Result paths could not be matched to a requested project.
       attribution_uncertain=true is set on affected results. Verify
       citation.url before treating these as authoritative."
```

---

## Prefer Narrowing Guidance Over Generic Errors

When a call fails or produces degraded results, name the lever that fixes it.
Do not say "search failed" when you can say "the query matched 0 results in
project `platform`; try `allow_all_projects=true` or broaden the query."

The narrowing levers available to callers are:

- `path_prefix` — restrict to a directory subtree
- `file_type` — restrict by extension (`.go`, `.ts`, `.java`)
- `project` / `projects` — scope to one project or a named subset
- `allow_all_projects` — search across all configured projects
- `query` — a more specific or differently-structured query string
- `mode=path` — switch to path-based search when you know part of a filename
- `page_size` — reduce per-call volume when response size is the constraint

Error messages should match this pattern: state what went wrong, then name the
lever. "Response exceeded the 32 MiB limit. Narrow the search with
`path_prefix` or request a specific file path with `read_file`." is better than
"Request too large."

---

## Good Vs Bad Tool Descriptions

The pair below is for a hypothetical description of `search_code`. The bad
version omits scope, omits the non-goal, and buries the key caveat in a second
sentence that an agent might skip.

**Bad:**

```text
Searches code in the OpenGrok index. Returns matching snippets.
Supports full-text and path modes.
```

**Good:**

```text
Full-text search over the OpenGrok index for a named project (or across all
configured projects with allow_all_projects=true). Returns file paths, line
numbers, matching snippets, and a citation.url per hit.

Use this to locate code patterns, identifiers, or string literals. Not for
finding AST-level relationships (extends, implements, call graphs) — OpenGrok
is a text index, not a language server. For symbol definitions, prefer
search_symbol_definitions; for implementations, use search_implementations
and treat results as candidate references, not semantic implementers.

When total_hits > 500, the warning field advises narrowing with path_prefix,
file_type, or a more specific query before paginating further.
```

The good version states the scope, names the output fields an agent cares about
(`citation.url`), tells the agent when to use a different tool, names the
semantic limit, and links the warning to a concrete next action.

---

## When To Include Examples

Include a concrete example payload in a field description when:

- The correct value is a non-obvious format (e.g. a Lucene query string, a
  cursor token, a ctags kind).
- The field interacts with other fields in a way that is hard to express as a
  type constraint (e.g. `project` vs `projects` vs `allow_all_projects`).
- The default is subtle and likely to be wrong without an example (e.g.
  `tokenized=false` means the server may auto-quote the query).

Keep examples short. A single realistic value is enough:

```text
path_prefix: "internal/auth/"   # restrict to a subtree
file_type: ".go"                # extension including the dot
query: "func.*Handler"          # Lucene regex syntax
```

Do not duplicate full schema shapes in examples. A schema block that restates
every field with placeholder values adds noise rather than clarity. Show only
the fields whose usage is non-obvious.
