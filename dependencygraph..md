# Dependency Graph & Code Flow

How a source file becomes context-rich chunks: the function flow (which function lives in which file), plus the package dependency graph.

## End-to-end flow

```
source + filepath
  │  1. parser.DetectLanguage(filepath) ............ parser/const.go
  │  2. parser.NewParser(lang).Parse(source) ....... parser/language.go   (tree-sitter, NOT regex)
  │  3. extract.ExtractEntitiesByNodeTypes(root, …) extract/fallback.go   → []ExtractedEntity
  │  4. scope.BuildScopeTreeFromEntities(entities) . scope/tree.go        → ScopeTree
  │  5. chunker.processWindows(...) ................. chunker/chunker.go
  │       greedyAssignWindows → MergeAdjacentWindows → RebuildText → buildContext
  │       → chunkcontext.FormatChunkWithContext
  ▼
  []types.Chunk   (Text + ContextualizedText + Context)
```

## 1. Language detection — `parser/const.go` (+ `parser/language_test.go`)

- `DetectLanguage(filePath)` maps a file extension to a language name (go / ts / js / py / rs / java). Used when `ChunkOptions.Language` is empty.
- `GetLanguage` / `LoadLanguage` fetch the tree-sitter grammar from a bounded cache; `ClearCache` resets it.
- `GrammarLoadError` wraps failure to load an unsupported grammar.

## 2. Parsing — `parser/language.go` (+ `parser/language_test.go`)

- `Parser` wraps the `gotreesitter` (pure-Go tree-sitter) engine.
- `NewParser(language)` → `Parser.Parse(source)` → `*gotreesitter.Tree`.

> ⚠️ **Correction to the original draft:** the code is parsed with tree-sitter, not regex.
> Regex is used later, only inside extraction (names/signatures/import syntax) and to
> spot identifiers when filtering imports (`context/context.go`).

## 3. Entity extraction — `extract/`

- `fallback.go`: `ExtractEntitiesByNodeTypes` — the **production** walker. DFS over the AST; every node whose type is an entity type (`const.go`: `IsEntityNodeType` / `GetEntityType`) becomes an `ExtractedEntity`.
- Per-entity helpers:
  - `name.go` — `ExtractName`
  - `signature.go` — `ExtractSignature`
  - `docstring.go` — `ExtractDocstring`
  - `import.go` — `ExtractImportSymbols` / `ExtractImportSource`
  - `native.go` — language routing (asks the `parser` package for grammars)
- `query*.go` (`query.go`, `query_patterns.go`, `query_types.go`, `query_errors.go`) — the tree-sitter **query engine**; currently exercised only by tests. Production uses the fallback walker above.
- Output: `[]ExtractedEntity` `{Type, Name, Signature, ByteRange, LineRange, Parent, Docstring, Source, Node}`.

## 4. Scope tree — `scope/tree.go` (+ `scope/flow.md`)

- `BuildScopeTreeFromEntities` splits entities into imports / exports / scope entities, sorts scope entities by start byte, then for each entity calls `FindParentNode` (which uses `RangeContains`) and `NewScopeNode` to attach parent/child links. Result: `ScopeTree{Root, Imports, Exports, AllEntities}`.
- Tree queries: `FindScopeAtOffset` (deepest node at an offset), `GetAncestorChain` (immediate-parent-first breadcrumb), `FlattenScopeTree` (DFS order).
- `error-handling.go` (`BuildScopeTree`, `BuildScopeTreeSync`, `ScopeError`) is currently **not** called by the pipeline.

## 5. Window assignment & merging — `chunker/`

- `nws.go`: `PreprocessNwsCumsum(code)` builds a cumulative sum over non-whitespace characters; `GetNwsCountFromCumsum` / `GetNwsCountForNode` answer size queries in O(1).
- `childrenOf(root)` (in `chunker.go`) — top-level AST children to pack.
- `greedyAssignWindows` (`chunker.go:63`) — greedily packs nodes up to `MaxChunkSize`; oversized leaves are split on line boundaries via `splitOversizedLeafByLines` + `IsLeafNode` (`oversized.go`).
- `merge.go`: `MergeAdjacentWindows` — `CanMerge` + `MergeWindows` fold adjacent windows that still fit within `MaxChunkSize`.
- `windows.go`: `GetAncestors` — deduplicated ancestor nodes of a window (largest-first ordering).

## 6. Rebuild + context per window — `chunker/` + `context/`

For each merged window:

- `rebuild.go`: `RebuildText(window, code)` uses `buildLineStartsTable` + `rebuildFromLineRanges` → `RebuiltText{Text, ByteRange, LineRange}`.
- `buildContext` (`chunker.go:195`):
  - `context/context.go` — `GetScopeForRange` (deepest scope → ancestors), `GetEntitiesInRange` (overlap + `IsPartial` flag), `GetRelevantImports` (identifier-regex import filtering).
  - `context/sibling.go` — `GetSiblings` (peers of the enclosing scope; before/after by byte order; sorted by distance; capped by `MaxSiblings`).
- `chunkcontext.FormatChunkWithContext` (`context/format.go`) — prepends `# path` (last 3 segments), `# Scope: parent > current`, `# Defines`, `# Uses`, sibling `# After` / `# Before`, then an optional overlap block (`# ...` → `# ---`), then the code → `ContextualizedText`.
- `ContextModeNone` → `emptyContext()`; otherwise `buildContext`. `computeOverlapText` re-attaches the trailing `OverlapLines` lines of the previous window.

## 7. Public API — `chunker/chunker.go`

- `ChunkCode(root, code, scopeTree, …)` — pre-parsed input, collects `[]types.Chunk`.
- `StreamChunks(...)` — channel-based streaming (`TotalChunks == -1` while running).
- `CodeChunker.Chunk` / `CodeChunker.Stream` — run `parseSource()` (steps 1–4) then `processWindows`.
- `CodeChunker.ChunkBatch` / `ChunkBatchStream` — semaphore + `WaitGroup` worker pool (default concurrency 10), `OnProgress` callback, results in completion order.

## Dependency graph (packages)

| Package | Depends on (internal) | Role |
| --- | --- | --- |
| `types` | — | central domain structs; leaf, imported by all |
| `parser` | — (only `gotreesitter` + stdlib) | language detection + AST parsing |
| `errors` | — | standalone error-wrapper types (package name `error`) |
| `extract` | `types`, `parser` | AST → `[]ExtractedEntity` |
| `scope` | `types` | entities → `ScopeTree` + queries |
| `context` | `types`, `scope` | chunk lookups + formatting |
| `chunker` | `types`, `parser`, `extract`, `scope`, `context` | windowing pipeline + public API |
| `cmd` | — | CLI (directory currently empty — not started) |

```
types  (leaf)
 ▲   ▲   ▲   ▲
 │   │   │   └─────── scope ──→ context
 │   │   │                        ▲   ▲
 │   │   └──────── extract ────────┘   │
 │   │                                 │
 │   └────────── parser ───────────────┤
 │                                     │
 └────────────── chunker ◄─────────────┘
                    │
                  cmd/  (CLI, not started)
```

External engine: `github.com/odvcencio/gotreesitter` — pure-Go tree-sitter bindings used by `parser`.

## Tests → source map

| Test file | Exercises |
| --- | --- |
| `parser/language_test.go` | language detection, grammar cache, parsing |
| `extract/fallback_test.go`, `query_test.go`, `extract_test.go`, `docstring_test.go` | entity extraction paths |
| `scope/tree_test.go` | scope tree build + queries |
| `context/context_test.go` | scope chain, entities-in-range, imports, siblings, formatting |
| `chunker/nws_test.go`, `windows_test.go`, `merge_test.go`, `rebuild_test.go`, `oversized_test.go` | each pipeline stage incl. `ChunkCode` vs `StreamChunks` parity |