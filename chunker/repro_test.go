package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

func TestReproPartialMergeBug(t *testing.T) {
	const maxSize = 200
	lines := make([]string, 8)
	for i := range lines {
		lines[i] = strings.Repeat("word ", 12+i) // 48..76 NWS per line
	}
	src := "package p\n\n/* " + strings.Join(lines, "\n") + " */\n\nvar x = 1\n"

	rootNode, scopeTree, language, err := parseSource("test.go", src, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone, Language: types.LanguageGo,
	})
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := ChunkCode(rootNode, src, scopeTree, language, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range chunks {
		nws := CountNws(c.Text)
		t.Logf("chunk %d NWS=%d (maxSize=%d) text=%q", i, nws, maxSize, c.Text)
		if nws > maxSize {
			t.Errorf("chunk %d NWS=%d exceeds maxSize=%d; text=%q", i, nws, maxSize, c.Text)
		}
	}
}

// TestReproNewExprDeps is the bug-report reproduction, now with hard
// assertions: JS/TS constructor calls (`new Foo()`) used to be dropped from
// dependency resolution because `new_expression` was absent from
// extract.CallNodeTypes and calledName had no case for it. Run for both
// TypeScript and JavaScript since the fix touches both languages' map entries.
func TestReproNewExprDeps(t *testing.T) {
	src := "class EventEmitter {}\n" +
		"class Logger {\n" +
		"    constructor() {\n" +
		"        this.emitter = new EventEmitter();\n" +
		"    }\n" +
		"}\n" +
		"function makeLogger() {\n" +
		"    return new Logger();\n" +
		"}\n"
	for _, fx := range []struct {
		name string
		lang types.Language
		file string
	}{
		{"typescript", types.LanguageTypeScript, "test.ts"},
		{"javascript", types.LanguageJavaScript, "test.js"},
	} {
		t.Run(fx.name, func(t *testing.T) {
			c := &CodeChunker{}
			results, err := c.Chunk(fx.file, src, types.ChunkOptions{Language: fx.lang})
			if err != nil {
				t.Fatal(err)
			}

			makeLoggerEnt := findChunkEntity(t, results, "makeLogger")
			dep, ok := depHas(makeLoggerEnt.Dependencies, "Logger")
			if !ok {
				t.Fatalf("makeLogger deps = %+v, want Logger (from `new Logger()`)", makeLoggerEnt.Dependencies)
			}
			if dep.Type != types.EntityTypeClass {
				t.Errorf("Logger dep type = %q, want %q", dep.Type, types.EntityTypeClass)
			}

			// `new EventEmitter()` lives inside Logger's constructor method. The
			// constructor method's enclosingName is "constructor" (not "Logger"),
			// so the call must NOT be filtered as a self-reference.
			ctorEnt := findChunkEntity(t, results, "constructor")
			if _, ok := depHas(ctorEnt.Dependencies, "EventEmitter"); !ok {
				t.Fatalf("constructor deps = %+v, want EventEmitter (from `new EventEmitter()`)", ctorEnt.Dependencies)
			}
		})
	}
}

// findChunkEntity scans every chunk in the results for an entity entry with the
// given name, returning the first match.
func findChunkEntity(t *testing.T, results []types.Chunk, name string) types.ChunkEntityInfo {
	t.Helper()
	for _, r := range results {
		for _, e := range r.Context.Entities {
			if e.Name == name {
				return e
			}
		}
	}
	t.Fatalf("no chunk entity named %q in %d chunks", name, len(results))
	return types.ChunkEntityInfo{}
}
