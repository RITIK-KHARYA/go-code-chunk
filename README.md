# go-code-chunk

Go library for semantic code chunking — split source code into meaningful, context-rich chunks for LLM embeddings, RAG systems, and AI-assisted code tools.

## Implementation Status

| Component                | Status                      | Details                                                                             |
| ------------------------ | --------------------------- | ----------------------------------------------------------------------------------- |
| **Types & Domain Model** | :white_check_mark: Complete | Structs and domain interfaces defined in `types/`                                   |
| **Parser Module**        | :white_check_mark: Complete | AST parsing via `gotreesitter` in `parser/`                                         |
| **Entity Extraction**    | :white_check_mark: Complete | Extractor query logic and language support in `extract/`                            |
| **Scope Tree**           | :white_check_mark: Complete | Hierarchical representation and nesting lookup in `scope/`                          |
| **Chunker**              | :warning: **Incomplete**    | Windowing and chunk generation exist in `chunker/` but are unstable/fail to compile |
| **CLI Tool**             | :x: **Not Started**         | The `cmd/go-ats/` directory is currently empty                                      |

## Features

- **Semantic parsing** (Complete) — AST-based extraction using tree-sitter
- **Entity extraction** (Complete) — Functions, methods, classes, interfaces, types, enums, imports, exports
- **Scope tree** (Complete) — Hierarchical representation of code structure
- **Contextual chunks** (Planned/Incomplete) — Each chunk carries its enclosing scope, siblings, and relevant imports
- **Batch processing** (Planned/Incomplete) — Concurrent multi-file chunking with progress callbacks
- **Multi-language** (Complete) — TypeScript, JavaScript, Python, Rust, Go, Java

## Domain Objects

| Concept         | Types                                       |
| --------------- | ------------------------------------------- |
| Ranges          | `LineRange`, `ByteRange`                    |
| Parse artifacts | `ParseError`, `ParseResult`, `SyntaxNode`   |
| Entities        | `ExtractedEntity`, `ScopeNode`, `ScopeTree` |
| Chunk           | `Chunk` + `ChunkContext`                    |

## Configuration (Planned)

`ChunkOptions` controls chunking behavior:

- `MaxChunkSize` (default: 1500)
- `ContextMode` — `none`, `minimal`, `full` (default)
- `SiblingDetail` — `none`, `names`, `signatures` (default)
- `FilterImports`, `OverlapLines`, `Language`

## Planned Interfaces

**Single-file:**

- `Chunker.Chunk()` — chunk a single file
- `Chunker.Stream()` — stream chunks via Go channels

**Batch:**

- `Chunker.ChunkBatch()` — process multiple files
- `Chunker.ChunkBatchStream()` — channel-based batch streaming

Batch processing uses `FileInput` → `BatchResult` with `BatchOptions` (concurrency, progress callback).

## Build

```bash
go mod tidy
go build ./... # Note: Currently fails compilation because Chunker is incomplete
```
