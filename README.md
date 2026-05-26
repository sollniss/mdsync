# mdsync

Keeps code snippets in Markdown files synchronized with their source files by replacing fenced code blocks in place.

## Install

```
go install github.com/sollniss/mdsync/cmd/mdsync@latest
```

## Usage

```
mdsync <file> [file...]
```

Each file is updated in place. Add a directive comment before any fenced code block and mdsync will replace the block's content with the current content of the referenced source file.

## Directive syntax

````markdown
<!-- MDSYNC:<lang> from:<source-file> [attributes] -->
```go
// replaced on next run
```
````

The language tag after `MDSYNC:` sets the fence language. An optional `<!-- MDSYNC-END -->` closing marker is recognized but not required.

## Attributes

| Attribute | Example | Description |
|---|---|---|
| `from:` | `from:main.go` | Source file to extract from (required) |
| `start-at:` | `start-at:"func Foo"` | Begin extraction after the first matching line |
| `start-offset:` | `start-offset:-1` | Shift start N lines (negative = include lines before match) |
| `end-at:` | `end-at:"^}"` | Stop extraction at first matching line |
| `end-offset:` | `end-offset:2` | Stop after the Nth match of the end pattern |
| `skip-match:` | `skip-match:"//nolint"` | Drop every line matching the pattern |
| `skip-after:` | `skip-after:"^import" count:3` | Skip N lines after each match |
| `skip-between:` | `skip-between:"// BEGIN":"// END"` | Drop lines between two patterns |
| `tab-size:` | `tab-size:4` | Convert leading tabs to N spaces (default 2; `0` strips tabs; negative preserves tabs) |
| `region:` | `region:myFunc` | Extract a named `#region`/`#endregion` block (shortcut for `start-at`/`end-at`) |

Patterns are Go regular expressions.

## Examples

### Embed a whole file

````markdown
<!-- MDSYNC:go from:internal/config/config.go -->
```go
```
````

### Extract a single function

````markdown
<!-- MDSYNC:go from:server.go start-at:"func handleRequest" end-at:"^}" -->
```go
```
````

`start-at` matches the function signature line and begins extraction on the next line. `end-at:"^}"` stops at the first line that is just a closing brace.

### Include the matched line itself

Use a negative `start-offset` to walk the start back before the match:

````markdown
<!-- MDSYNC:go from:server.go start-at:"func handleRequest" start-offset:-1 end-at:"^}" end-offset:1 -->
```go
```
````

`start-offset:-1` includes the line before the match (the function signature). `end-offset:1` includes the closing brace.

### Skip annotated sections

````markdown
<!-- MDSYNC:go from:example.go skip-between:"// BEGIN OMIT":"// END OMIT" -->
```go
```
````

Lines between `// BEGIN OMIT` and `// END OMIT` are dropped; the rest of the file is included.

### Skip the import block

````markdown
<!-- MDSYNC:go from:main.go start-at:"^package" start-offset:-1 skip-after:"^import" count:5 -->
```go
```
````

`skip-after:"^import" count:5` drops the 5 lines following each line that starts with `import`.

### Named regions

Most editors support `#region`/`#endregion` markers for folding. Use `region:` to extract one by name:

````markdown
<!-- MDSYNC:go from:server.go region:handleRequest -->
```go
```
````

This is equivalent to `start-at:"#region handleRequest" end-at:"#endregion"`. In the source file:

```go
// #region handleRequest
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // ...
}
// #endregion
```

The marker lines themselves are excluded from the output.

### Tab-indented sources

Most markdown renderers discard tab characters, collapsing indentation. Use `tab-size:` to convert leading tabs to spaces:

````markdown
<!-- MDSYNC:go from:server.go start-at:"func handleRequest" end-at:"^}" end-offset:1 tab-size:4 -->
```go
```
````

Each leading tab becomes 4 spaces. Omitting `tab-size:` defaults to 2 spaces per tab. Use `tab-size:0` to strip leading tabs entirely, or a negative value to preserve them as-is.
