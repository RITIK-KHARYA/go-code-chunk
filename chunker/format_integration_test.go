package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// annotateSrc has three top-level functions small enough that the whole file
// is a single chunk, so all three entities carry their own annotation blocks
// in the same contextualized output.
const annotateSrc = `package main

func alpha() int {
	return 1
}

func beta() int {
	return 2
}

func gamma() int {
	return 3
}
`

// TestPerEntityAnnotationsPresentForAll pins the core guarantee of the
// per-entity architecture: every entity in a chunk's context renders its own
// annotation block above its own source, and the file header appears exactly
// once per chunk.
func TestPerEntityAnnotationsPresentForAll(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo}
	rootNode, scopeTree, language, err := parseSource("annotate.go", annotateSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, annotateSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected a single chunk, got %d", len(chunks))
	}

	chunk := chunks[0]
	if len(chunk.Context.Entities) != 3 {
		t.Fatalf("chunk entities = %+v, want all 3", chunk.Context.Entities)
	}

	ct := chunk.ContextualizedText
	if count := strings.Count(ct, "// File:"); count != 1 {
		t.Errorf("contextualized text has %d \"// File:\" lines, want exactly 1:\n%s", count, ct)
	}
	if count := strings.Count(ct, "// Scope:"); count != len(chunk.Context.Entities) {
		t.Errorf("contextualized text has %d \"// Scope:\" blocks, want %d:\n%s",
			count, len(chunk.Context.Entities), ct)
	}

	// Each entity's annotation block must appear above its own declaration,
	// separated from it only by further "//" comment lines (its own block lines).
	lines := strings.Split(ct, "\n")
	for _, name := range []string{"alpha", "beta", "gamma"} {
		funcIdx := -1
		for i, ln := range lines {
			if strings.HasPrefix(strings.TrimSpace(ln), "func "+name+"(") {
				funcIdx = i
				break
			}
		}
		if funcIdx < 0 {
			t.Fatalf("declaration for %q not found in output:\n%s", name, ct)
		}
		// Climb back over contiguous comment lines: the block must be there.
		idx := funcIdx
		for idx > 0 && strings.HasPrefix(strings.TrimLeft(lines[idx-1], " \t"), "//") {
			idx--
		}
		above := strings.Join(lines[idx:funcIdx], "\n")
		if !strings.Contains(above, "// Scope: "+name) {
			t.Errorf("entity %q block not placed directly above its declaration; lines above:\n%s\nfull output:\n%s", name, above, ct)
		}
	}
}

// TestNoHashCommentPrefix guards against the "# " annotation syntax ever
// reappearing: in the new architecture every annotation line uses "//".
func TestNoHashCommentPrefix(t *testing.T) {
	fixtures := []struct {
		name string
		src  string
		opts types.ChunkOptions
	}{
		{"annotate", annotateSrc, types.ChunkOptions{Language: types.LanguageGo}},
		{"deps", goDepsSrc, types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 1000}},
		{"imports", goImportsSrc, types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 1000}},
		{"siblings", goSiblingsSrc, types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 1000}},
		{"capture", goCaptureSrc, types.ChunkOptions{Language: types.LanguageGo}},
		{"ts", contextTestSrc, types.ChunkOptions{Language: types.LanguageTypeScript}},
	}

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.name, func(t *testing.T) {
			rootNode, scopeTree, language, err := parseSource(fx.name+".src", fx.src, fx.opts)
			if err != nil {
				t.Fatalf("parseSource: %v", err)
			}
			chunks, err := ChunkCode(rootNode, fx.src, scopeTree, language, fx.opts, nil)
			if err != nil {
				t.Fatalf("ChunkCode: %v", err)
			}
			for _, ch := range chunks {
				for _, line := range strings.Split(ch.ContextualizedText, "\n") {
					if strings.HasPrefix(line, "# ") {
						t.Fatalf("chunk output contains a \"# \" line: %q\nfull output:\n%s", line, ch.ContextualizedText)
					}
				}
			}
		})
	}
}

// crossFileA defines mainA which calls helperB defined in crossFileB.
const crossFileA = `package main

func mainA() {
	return helperB(1)
}
`

const crossFileB = `package main

func helperB(n int) int {
	return n + 1
}
`

// TestCrossFileDependencies asserts that a batch chunk shared project index
// resolves an entity defined in another file: the dependency carries the other
// file's path and the contextualized text renders the [file] suffix.
func TestCrossFileDependencies(t *testing.T) {
	c := NewCodeChunker()
	files := []types.FileInput{
		{Filepath: "a.go", Code: crossFileA},
		{Filepath: "b.go", Code: crossFileB},
	}
	results, err := c.ChunkBatch(files, types.BatchOptions{ChunkOptions: types.ChunkOptions{Language: types.LanguageGo}})
	if err != nil {
		t.Fatalf("ChunkBatch: %v", err)
	}

	var aResult *types.BatchResult
	for i := range results {
		if results[i].Error != nil {
			t.Fatalf("file %s errored: %v", results[i].Filepath, results[i].Error)
		}
		if results[i].Filepath == "a.go" {
			aResult = &results[i]
		}
	}
	if aResult == nil {
		t.Fatalf("no result for a.go; results = %+v", results)
	}

	var mainAEnt types.ChunkEntityInfo
	var mainAChunk types.Chunk
	for _, ch := range aResult.Chunks {
		for _, e := range ch.Context.Entities {
			if e.Name == "mainA" {
				mainAEnt = e
				mainAChunk = ch
			}
		}
	}
	if mainAEnt.Name == "" {
		t.Fatalf("no mainA entity in a.go chunks; chunks = %+v", aResult.Chunks)
	}

	dep, ok := depHas(mainAEnt.Dependencies, "helperB")
	if !ok {
		t.Fatalf("mainA deps = %+v, want cross-file helperB", mainAEnt.Dependencies)
	}
	if dep.Filepath != "b.go" {
		t.Errorf("helperB dep Filepath = %q, want %q", dep.Filepath, "b.go")
	}
	if !strings.Contains(mainAChunk.ContextualizedText, "helperB(...) [b.go]") {
		t.Errorf("contextualized text missing cross-file dep \"helperB(...) [b.go]\":\n%s", mainAChunk.ContextualizedText)
	}

	// helperB in b.go must not resolve itself and must not list mainA.
	for _, ch := range aResultChunkEntities(t, results, "b.go").entities {
		if ch.Name != "helperB" {
			continue
		}
		if _, isSelf := depHas(ch.Dependencies, "helperB"); isSelf {
			t.Errorf("helperB deps = %+v, want no self-reference", ch.Dependencies)
		}
		if _, cross := depHas(ch.Dependencies, "mainA"); cross {
			t.Errorf("helperB deps = %+v, want no reference to mainA", ch.Dependencies)
		}
	}
}

type chunkEntitiesByFile struct {
	entities []types.ChunkEntityInfo
}

func aResultChunkEntities(t *testing.T, results []types.BatchResult, filepath string) chunkEntitiesByFile {
	t.Helper()
	var out chunkEntitiesByFile
	for i := range results {
		if results[i].Filepath != filepath {
			continue
		}
		for _, ch := range results[i].Chunks {
			out.entities = append(out.entities, ch.Context.Entities...)
		}
		return out
	}
	t.Fatalf("no result for %s", filepath)
	return out
}
