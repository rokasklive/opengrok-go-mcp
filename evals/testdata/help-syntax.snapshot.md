# OpenGrok help.jsp query syntax snapshot

Source: `oracle/opengrok` `opengrok-web/src/main/webapp/help.jsp`
Upstream commit: `a31f1f45a8e50432b7a4418c9cffa90990cb862a`
Fetched: 2026-06-25

This is a normalized snapshot of the help.jsp query-syntax section used as the
authoring ground truth for feature 008 claim registry entries. Runtime behavior
is verified by the live conformance suite; the server does not scrape this file.

## Examples

- Definition search: `defs:setResourceMonitors`.
- Reference search scoped by path: `refs:sprintf path:usr/src/cmd/cmd-inet/usr.sbin`.
- Exact phrase search: `"foo ="` and `"Bill Joy"`.
- Required/prohibited phrases: `-"/usr/bin/perl" +"/bin/perl"`.
- Multi-character wildcard: `foo*`.
- Path phrase: `". c"`.
- Path regular expression: `path:/ma[a-zA-Z]*/`.
- File type restriction: `main type:c`.

## Query clauses

A query is a series of clauses. A clause may be prefixed by `+` or `-` to
require or prohibit it, or by `field:` to search a specific field.

A clause may be:

- A term.
- A phrase enclosed in double quotes, such as `"hello dolly"`.
- A nested query enclosed in parentheses.
- A boolean expression. Supported operators are `AND` (`&&`), `+`, `OR` (`||`),
  `NOT` (`!`), and `-`. Word operators must be all caps.

## Regex, wildcard, fuzzy, proximity, and range searches

- Regex uses `/.../` enclosure, for example `/[mb]an/`.
- `path:` escapes `/` by default, so regex path search is supported only when
  the search string starts and ends with `/`, for example `path:/ma[a-zA-Z]*/`.
- `?` performs a single-character wildcard search, for example `te?t`.
- `*` performs a multi-character wildcard search, for example `test*` or `te*t`.
- `*` and `?` may be the first character unless disabled by the indexer `-a`
  option.
- `~` performs fuzzy matching, for example `rcs~`.
- A phrase followed by `~N` performs proximity search, for example
  `"opengrok help"~10`.
- Range queries use `[A TO B]` for inclusive bounds and `{A TO B}` for exclusive
  bounds. Sorting is lexicographic.

## Escaping

OpenGrok supports escaping query-syntax characters with `\`.

Current special characters:

```text
+ - && || ! ( ) { } [ ] ^ " ~ * ? : \ /
```

Example: search for `(1+1):2` as `\(1\+1\)\:2`.

Analyzer note: indexed words are made up of alphanumeric and underscore
characters. Most other characters are treated as whitespace. Exceptions called
out by help.jsp include `@ $ % ^ & = ? . :`, which are mostly indexed as
separate words and may still need escaping when they are query syntax.

## Valid fields

- `full`: all text tokens in the index.
- `defs`: symbol definitions.
- `refs`: symbols such as methods, classes, functions, and variables.
- `path`: source file path. Use `/` as the divider. Exact paths should be
  quoted, for example `"src/mypath"`.
- `hist`: history log comments.
- `type`: analyzer/file-type scope such as C sources.

## Boosting and Lucene

Terms or phrases can be boosted with `^`, for example `help^4 opengrok`.
OpenGrok search is powered by Lucene; help.jsp links to Lucene query-parser and
regexp documentation for details.
