# Why this project does not use WASM

A short note to help me (and readers of my blog) understand the tree-sitter / WASM
decision in one sitting.

## 1. The problem we solve

We split source code (`.go`, `.ts`, `.py`, ...) into meaningful chunks for AI / RAG
systems. To do that well we need the code's **structure** — where functions, classes,
interfaces, imports start and end. That is exactly what an **AST (Abstract Syntax
Tree)** gives us.

## 2. What tree-sitter is

Tree-sitter is a tool that turns source code into an AST very fast. It is written in
**C**. For each programming language there is a "grammar" — a file that describes how
that language parses.

- The core tree-sitter engine is a C library.
- Each language grammar is also compiled from C.

So historically, to use tree-sitter you needed a C toolchain at build time.

## 3. Why WASM grammars existed

WASM (WebAssembly) lets compiled code — like a tree-sitter grammar — run **in a
browser** safely and fast.

If you build a code tool that runs *in the browser* (for example a VS Code-like code
viewer, or an online playground), you cannot ship a native C library. So the
tree-sitter team compiles each grammar to a `.wasm` file, and a JavaScript library
called `web-tree-sitter` runs those WASM grammars in the browser.

That is the **only** place `.wasm` grammars are the natural choice:

```
Native C grammar  --->  compiled to .wasm  --->  runs in browser via web-tree-sitter
```

Our original codebase was a JavaScript library, so it referenced grammar files like
`tree-sitter-typescript/tree-sitter-tsx.wasm` — that is where those paths came from.

## 4. The Go port problem

When we rewrote the project in **Go**, the WASM grammar paths stopped making sense:

| Choice | What it means | The catch |
|---|---|---|
| Go + native tree-sitter (smacker bindings) | Real C library behind Go | Needs CGO + a C compiler (gcc) — makes builds hard for end users, esp. on Windows |
| Keep WASM grammars | Re-use those `.wasm` files from the JS version | Go must run a WASM runtime (wazero); available wrappers are early-stage and mostly useless for us |
| **Pure-Go tree-sitter (`gotreesitter`)** | Native Go implementation of the engine + grammars | None — this is what we picked |

`gotreesitter` implements the whole tree-sitter engine (parser, lexer, queries) in Go
and ships 200+ grammars bundled in the module. Import it, call it — done.

## 5. What we actually use

```
our code  ->  parser (parser/language.go)  ->  gotreesitter (pure Go)  ->  AST
                                                                          |
                                          queries extract functions / classes / ...
```

No `.wasm` files, no C compiler, no CGO.

## 6. Why this is great for our users

- `go build` / `go test` just works — even on Windows without gcc.
- We can **cross-compile** one binary for every OS/CPU (`GOOS=linux`,
  `GOOS=darwin`, `GOOS=windows`, even `GOOS=js GOARCH=wasm`) with a single command.
- No per-user setup, no "install a C compiler first".

## 7. When would we use WASM after all?

One day, if we want a **web demo / browser editor** that highlights or parses code on
the client side, we can generate WASM grammars again with `web-tree-sitter` — the
same grammars, just in the browser. Go and browser can share the same grammar set, so
it is a "frontend-only" decision, not a core one.

## One-line summary

WASM grammars exist to run tree-sitter in the **browser**. This is a Go library that
runs on servers, so we use a **pure-Go tree-sitter** — no WASM, no C compiler, and
easy builds on every OS.