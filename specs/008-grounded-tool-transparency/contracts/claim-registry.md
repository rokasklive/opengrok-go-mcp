# Contract: Claim Registry (seed)

The registry (`internal/mcpserver/claims.go`) is the single source of truth. Below is the
**seed** grounded in OpenGrok `help.jsp` (recon-captured). Each claim must carry every
Evidence-required field from `data-model.md#entity-1` before it can be rendered; the
bijection test fails the build on any orphan. Columns abbreviated: ID · status · claim text ·
example · positive assertion / negative control · source · gate.

## Supported — query syntax

| id | status | agent_claim_text | example | positive / negative control | gate |
|----|--------|------------------|---------|------------------------------|------|
| `phrase` | supported | Wrap multi-word queries in quotes for an exact phrase | `"extends PaymentProcessor"` | matches the adjacent phrase / a bag-of-words control matches more, looser | live |
| `wildcard-multi` | supported | `*` matches multiple characters | `test*` | matches `testing` / exact `test` control differs | live |
| `wildcard-single` | supported | `?` matches a single character | `te?t` | matches `text`/`test` / `te*t` differs | live |
| `field-defs` | supported | `defs:Name` finds definitions | `defs:PaymentProcessor` | returns definition hits / a plain-text control returns non-defs | live |
| `field-refs` | supported | `refs:Name` finds references | `refs:processPayment` | returns reference hits / defs-only control differs | live |
| `field-path` | supported | `path:` restricts by file path | `path:src/api` | only in-path hits / out-of-path control excluded | live |
| `field-hist` | supported | `hist:` searches history (history mode only) | `hist:"null check"` | history hits / ignored-with-warning elsewhere | live |
| `field-type` | supported | `type:` filters by file type | `type:java` | only that type / other-type control excluded | live |
| `boolean` | supported | `AND OR NOT + -` combine terms | `foo AND bar -baz` | both present, baz absent / OR control differs | live |
| `fuzzy` | supported | `~` does fuzzy (edit-distance) match | `paymet~` | matches `payment` / exact control misses typo | live |
| `proximity` | supported | `"a b"~N` matches within N words | `"opengrok help"~10` | near-co-occurrence / `~0` control differs | live |
| `range` | supported | `[A TO B]` / `{A TO B}` range queries | `title:{Aida TO Carmen}` | in-range hits / out-of-range excluded | live |
| `regex` | supported | Regex via `/…/` enclosure | `/[mb]an/` | matches `man`/`ban` / literal `man` control differs | live |
| `path-regex` | conditional | `path:` honors regex only when value starts AND ends with `/` | `path:/ma[a-zA-Z]*/` | regex path match / unslashed control is literal. **condition:** path-field `/`-escape behavior | live |
| `leading-wildcard` | conditional | Leading `*`/`?` allowed | `*Processor` | matches suffix / **condition:** indexer not run with `-a` | live |

## Behavioral guarantees

| id | status | agent_claim_text | example | positive / negative control | gate |
|----|--------|------------------|---------|------------------------------|------|
| `auto-quote` | supported | Bare multi-word queries are auto-quoted as a phrase; set `tokenized=true` for bag-of-words | `extends Foo` → phrase | auto-quoted result ⊆ tokenized result | live |
| `projects-array` | supported | `projects:[…]` scopes to multiple projects | `projects:["a","b"]` | applied to scope / accepted by schema | always-on |
| `scalar-coercion` | supported | String-encoded scalars (`"10"`) are accepted | `before:"10"` | coerced to int, accepted | always-on |
| `default-project` | supported | Omitting `project` uses the configured default `<NAME>` | (omit project) | resolves to default, no discovery call | always-on |

## Unsupported / pitfalls (negative claims — verified by negative checks)

| id | status | agent_claim_text | example | negative assertion | gate |
|----|--------|------------------|---------|--------------------|------|
| `bare-regex` | unsupported | Bare regex without `/…/` is not regex and errors | `class.*extends` | returns `QUERY_PARSER_FAILED` (upstream 400) | live |
| `wildcard-in-phrase` | unsupported | Wildcards inside quoted phrases do not expand | `"foo* bar"` | `*` treated literally / not as wildcard | live |
| `inheritance` | limitation | No AST/inheritance/subclass query exists; approximate with text search | (n/a) | no operator yields semantic subclasses; documented as limitation | none (prose; justified exemption — no positive behavior to assert) |
| `call-graph` | limitation | No call-graph/caller query; `refs:` is ctags text, not semantic callers | (n/a) | documented limitation | none (justified exemption) |

**Notes.** `<NAME>` in `default-project` is rendered from the resolved
`Config.DefaultProject` at description-build time (FR-004). The `none`-gate limitation claims
are the only bijection exemptions and each carries a justification (no positive behavior to
assert); they are still rendered into descriptions and counted by the description contract.
The live table is illustrative — the authoritative set is `claims.go`, and the bijection test
is what guarantees this file and the code agree.
