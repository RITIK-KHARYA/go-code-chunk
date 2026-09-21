package chunker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/scope"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// contextTestSrc is a small TypeScript file whose render method is nested
// inside the Widget class, giving a nested scope hierarchy to assert on.
const contextTestSrc = `class Widget {
  render(): string {
    const label = "widget";
    return label;
  }
}

function topLevel() {
  return 1;
}
`

// chunkContextFixture parses the nested-class fixture and chunks it.
func chunkContextFixture(t *testing.T, opts types.ChunkOptions) (types.ScopeTree, []types.Chunk) {
	t.Helper()
	rootNode, scopeTree, language, err := parseSource("widget.ts", contextTestSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, contextTestSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	return scopeTree, chunks
}

func findEntityName(t *testing.T, tree types.ScopeTree, name string) types.ExtractedEntity {
	t.Helper()
	for _, e := range tree.AllEntities {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("entity %q not extracted", name)
	return types.ExtractedEntity{}
}

func scopeHasEntity(scopeList []types.EntityInfo, name string) bool {
	for _, s := range scopeList {
		if s.Name == name {
			return true
		}
	}
	return false
}

func entitiesHaveEntry(entities []types.ChunkEntityInfo, name string, entityType types.EntityType) bool {
	for _, e := range entities {
		if e.Name == name && e.Type == entityType {
			return true
		}
	}
	return false
}

func rawEntitiesHaveEntry(entities []types.ExtractedEntity, name string, entityType types.EntityType) bool {
	for _, e := range entities {
		if e.Name == name && e.Type == entityType {
			return true
		}
	}
	return false
}

// entityInfoInChunk returns the chunk entity entry with the given name.
func entityInfoInChunk(t *testing.T, ch types.Chunk, name string) types.ChunkEntityInfo {
	t.Helper()
	for _, e := range ch.Context.Entities {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("chunk entities = %+v, want %q", ch.Context.Entities, name)
	return types.ChunkEntityInfo{}
}

// TestChunkContextCarriesRealScopeAndEntities asserts that a chunk covering a
// nested function carries the enclosing class in the function's own Scope and
// the function's own entry in Entities — real scope-tree data, not empty
// slices.
func TestChunkContextCarriesRealScopeAndEntities(t *testing.T) {
	scopeTree, chunks := chunkContextFixture(t, types.ChunkOptions{Language: types.LanguageTypeScript})

	render := findEntityName(t, scopeTree, "render")
	widget := findEntityName(t, scopeTree, "Widget")

	// Find a chunk that fully covers the inner render() function.
	var covered types.Chunk
	found := false
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, render.ByteRange) {
			covered = ch
			found = true
			break
		}
	}
	if !found {
		var ranges []string
		for i, ch := range chunks {
			ranges = append(ranges, fmt.Sprintf("chunk#%d %+v", i, ch.ByteRange))
		}
		t.Fatalf("no chunk fully covers %q %+v; chunks: %s", render.Name, render.ByteRange, strings.Join(ranges, ", "))
	}

	renderEnt := entityInfoInChunk(t, covered, render.Name)
	if !scopeHasEntity(renderEnt.Scope, widget.Name) {
		t.Errorf("render Scope = %+v, want it to contain the outer class %q", renderEnt.Scope, widget.Name)
	}
	if !entitiesHaveEntry(covered.Context.Entities, render.Name, types.EntityTypeMethod) {
		t.Errorf("chunk Entities = %+v, want entry %q (%s)", covered.Context.Entities, render.Name, types.EntityTypeMethod)
	}
}

// TestScopeBreadcrumbOuterToInner pins the scope-breadcrumb ordering: outer
// class first, inner method last, and full-containment filtering of entities.
func TestScopeBreadcrumbOuterToInner(t *testing.T) {
	scopeTree, _ := chunkContextFixture(t, types.ChunkOptions{Language: types.LanguageTypeScript})

	pos := strings.Index(contextTestSrc, "return label")
	if pos < 0 {
		t.Fatal("marker not found in source")
	}

	chain := scopeBreadcrumb(pos, scopeTree)
	if len(chain) != 2 {
		t.Fatalf("scope breadcrumb = %+v, want [Widget render] (outer to inner)", chain)
	}
	if chain[0].Name != "Widget" || chain[1].Name != "render" {
		t.Fatalf("scope breadcrumb = [%s %s], want outer-first [Widget render]",
			chain[0].Name, chain[1].Name)
	}
	if chain[0].Type != types.EntityTypeClass {
		t.Errorf("outer scope type = %q, want %q", chain[0].Type, types.EntityTypeClass)
	}
	if chain[1].Type != types.EntityTypeMethod {
		t.Errorf("inner scope type = %q, want %q", chain[1].Type, types.EntityTypeMethod)
	}

	// A range covering only render must include render itself but not the
	// wider class (full containment, not raw overlap).
	render := findEntityName(t, scopeTree, "render")
	ents := entitiesContainedIn(render.ByteRange, scopeTree)
	if !rawEntitiesHaveEntry(ents, render.Name, types.EntityTypeMethod) {
		t.Errorf("contained entities = %+v, want %q (%s)", ents, render.Name, types.EntityTypeMethod)
	}
	for _, e := range ents {
		if e.Name == "Widget" {
			t.Errorf("contained entities = %+v, want Widget excluded (class range is wider than the chunk)", ents)
		}
	}
}

// goNestingSrc is a Go file whose scope hierarchy is genuinely nested by byte
// range: the local type declaration lies inside the enclosing function's body,
// unlike Go methods which are top-level siblings of their struct type. The
// body is large enough that with a small maxSize the function is split into
// partial windows, and the local type falls into a window that starts inside
// the function.
const goNestingSrc = `package main

func outer() {
	a := "aaaaaaaaaaaaaaaaaaaaaa" + "aaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbb" + "bbbbbbbbbbbbbbbbbbbbbb"
	c := "cccccccccccccccccccccc" + "cccccccccccccccccccccc"
	d := "dddddddddddddddddddddd" + "dddddddddddddddddddddd"
	e := "eeeeeeeeeeeeeeeeeeeeee" + "eeeeeeeeeeeeeeeeeeeeee"
	f := "ffffffffffffffffffffff" + "ffffffffffffffffffffff"
	g := "gggggggggggggggggggggg" + "gggggggggggggggggggggg"
	h := "hhhhhhhhhhhhhhhhhhhhhh" + "hhhhhhhhhhhhhhhhhhhhhh"
	type inner struct {
		X int
	}
}
`

// TestChunkContextGoLocalTypeNesting asserts that a chunk covering a locally
// declared type carries the enclosing function in Scope and the local type
// itself in Entities.
func TestChunkContextGoLocalTypeNesting(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 300}
	rootNode, scopeTree, language, err := parseSource("nested.go", goNestingSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, goNestingSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	outer := findEntityName(t, scopeTree, "outer")

	var inner types.ExtractedEntity
	for _, e := range scopeTree.AllEntities {
		if e.Type == types.EntityTypeType {
			inner = e
			break
		}
	}
	if inner.Name == "" {
		t.Fatalf("no entity of type %q extracted; entities contain no local type", types.EntityTypeType)
	}

	var covered types.Chunk
	found := false
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, inner.ByteRange) {
			covered = ch
			found = true
			break
		}
	}
	if !found {
		var ranges []string
		for i, ch := range chunks {
			ranges = append(ranges, fmt.Sprintf("chunk#%d %+v", i, ch.ByteRange))
		}
		t.Fatalf("no chunk fully covers local type %q %+v; chunks: %s", inner.Name, inner.ByteRange, strings.Join(ranges, ", "))
	}

	innerEnt := entityInfoInChunk(t, covered, inner.Name)
	if !scopeHasEntity(innerEnt.Scope, outer.Name) {
		t.Errorf("local type Scope = %+v, want it to contain the outer function %q", innerEnt.Scope, outer.Name)
	}
	if !entitiesHaveEntry(covered.Context.Entities, inner.Name, types.EntityTypeType) {
		t.Errorf("chunk Entities = %+v, want entry %q (%s)", covered.Context.Entities, inner.Name, types.EntityTypeType)
	}
	for _, e := range covered.Context.Entities {
		if e.Name == outer.Name {
			t.Errorf("chunk Entities = %+v, want %q excluded (function range starts before this chunk)", covered.Context.Entities, outer.Name)
		}
	}
}

// importsGetSource reports whether imports contains an entry with the given
// import source.
func importsGetSource(imports []types.ImportInfo, source string) bool {
	for _, im := range imports {
		if im.Source == source {
			return true
		}
	}
	return false
}

// goImportsSrc is a two-import Go file sized so greedy windowing splits the
// two functions into separate chunks: each function only references its own
// import, so each chunk must retain exactly one import.
const goImportsSrc = `package main

import (
	"fmt"
	"os"
)

func usesFmt() {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	c := "cccccccccccccccccccccccccccccccccccccccc"
	d := "dddddddddddddddddddddddddddddddddddddddd"
	e := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	f := "ffffffffffffffffffffffffffffffffffffffff"
	g := "gggggggggggggggggggggggggggggggggggggggg"
	h := "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh"
	i := "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"
	j := "jjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj"
	k := "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"
	l := "llllllllllllllllllllllllllllllllllllllll"
	m := "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm"
	n := "nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn"
	o := "oooooooooooooooooooooooooooooooooooooooo"
	p := "pppppppppppppppppppppppppppppppppppppppp"
	fmt.Println(a, b, c, d, e, f, g, h, i, j, k, l, m, n, o, p)
}

func usesOs() {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	c := "cccccccccccccccccccccccccccccccccccccccc"
	d := "dddddddddddddddddddddddddddddddddddddddd"
	e := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	f := "ffffffffffffffffffffffffffffffffffffffff"
	g := "gggggggggggggggggggggggggggggggggggggggg"
	h := "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh"
	i := "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"
	j := "jjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj"
	k := "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"
	l := "llllllllllllllllllllllllllllllllllllllll"
	p := os.Getenv("HOME")
	_ = p
}
`

// TestChunkContextImportsFiltered asserts that each chunk's context imports
// list contains only the import the chunk actually uses, not the file's full
// import block.
func TestChunkContextImportsFiltered(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 1000}
	rootNode, scopeTree, language, err := parseSource("imports.go", goImportsSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, goImportsSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks to observe import filtering, got %d", len(chunks))
	}

	assertChunkImports := func(t *testing.T, chunk types.Chunk, entityName, usedSource, unusedSource string) {
		t.Helper()
		ent := entityInfoInChunk(t, chunk, entityName)
		if len(ent.Imports) != 1 {
			t.Fatalf("entity %q imports = %+v, want exactly [%s]; chunk text:\n%s", entityName, ent.Imports, usedSource, chunk.Text)
		}
		if !importsGetSource(ent.Imports, usedSource) {
			t.Errorf("entity %q imports = %+v, want to include %q", entityName, ent.Imports, usedSource)
		}
		if importsGetSource(ent.Imports, unusedSource) {
			t.Errorf("entity %q imports = %+v, want to exclude %q", entityName, ent.Imports, unusedSource)
		}
	}

	usesFmt := findEntityName(t, scopeTree, "usesFmt")
	usesOs := findEntityName(t, scopeTree, "usesOs")

	var fmtChunk, osChunk types.Chunk
	var fmtFound, osFound bool
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, usesFmt.ByteRange) {
			fmtChunk = ch
			fmtFound = true
		}
		if scope.RangeContains(ch.ByteRange, usesOs.ByteRange) {
			osChunk = ch
			osFound = true
		}
	}
	if !fmtFound {
		t.Fatal("no chunk fully covers usesFmt")
	}
	if !osFound {
		t.Fatal("no chunk fully covers usesOs")
	}

	assertChunkImports(t, fmtChunk, "usesFmt", "fmt", "os")
	assertChunkImports(t, osChunk, "usesOs", "os", "fmt")
}

// checkSibling asserts that sibs contains an entry for name carrying the
// given position and distance.
func checkSibling(t *testing.T, sibs []types.SiblingInfo, name string, pos types.SiblingPosition, distance int) {
	t.Helper()
	for _, s := range sibs {
		if s.Name == name {
			if s.Position != pos || s.Distance != distance {
				t.Errorf("sibling %q = %+v, want %s/%d", name, s, pos, distance)
			}
			return
		}
	}
	t.Errorf("siblings = %+v, want %q (%s/%d)", sibs, name, pos, distance)
}

// goSiblingsSrc has three top-level functions sized (MaxChunkSize 1000) so
// each lands in its own chunk, giving the middle function both a before and an
// after sibling.
const goSiblingsSrc = `package main

func first() {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	c := "cccccccccccccccccccccccccccccccccccccccc"
	d := "dddddddddddddddddddddddddddddddddddddddd"
	e := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	f := "ffffffffffffffffffffffffffffffffffffffff"
	g := "gggggggggggggggggggggggggggggggggggggggg"
	h := "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh"
	i := "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"
	j := "jjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj"
	k := "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"
	l := "llllllllllllllllllllllllllllllllllllllll"
	m := "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm"
	n := "nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn"
}

func middle() {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	c := "cccccccccccccccccccccccccccccccccccccccc"
	d := "dddddddddddddddddddddddddddddddddddddddd"
	e := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	f := "ffffffffffffffffffffffffffffffffffffffff"
	g := "gggggggggggggggggggggggggggggggggggggggg"
	h := "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh"
	i := "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"
	j := "jjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj"
	k := "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"
	l := "llllllllllllllllllllllllllllllllllllllll"
	m := "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm"
	n := "nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn"
}

func last() {
	a := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	c := "cccccccccccccccccccccccccccccccccccccccc"
	d := "dddddddddddddddddddddddddddddddddddddddd"
	e := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	f := "ffffffffffffffffffffffffffffffffffffffff"
	g := "gggggggggggggggggggggggggggggggggggggggg"
	h := "hhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh"
	i := "iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"
	j := "jjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjj"
	k := "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk"
	l := "llllllllllllllllllllllllllllllllllllllll"
	m := "mmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmmm"
	n := "nnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnn"
}
`

// TestChunkContextSiblings asserts that the chunk covering the middle of three
// sibling functions carries the other two as siblings with correct
// Position/Distance.
func TestChunkContextSiblings(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 1000}
	rootNode, scopeTree, language, err := parseSource("siblings.go", goSiblingsSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, goSiblingsSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks to observe siblings, got %d", len(chunks))
	}

	first := findEntityName(t, scopeTree, "first")
	middle := findEntityName(t, scopeTree, "middle")
	last := findEntityName(t, scopeTree, "last")

	var midChunk, firstChunk types.Chunk
	var midFound, firstFound bool
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, first.ByteRange) {
			firstChunk = ch
			firstFound = true
		}
		if scope.RangeContains(ch.ByteRange, middle.ByteRange) {
			midChunk = ch
			midFound = true
		}
	}
	if !firstFound {
		t.Fatal("no chunk fully covers first")
	}
	if !midFound {
		var ranges []string
		for i, ch := range chunks {
			ranges = append(ranges, fmt.Sprintf("chunk#%d %+v", i, ch.ByteRange))
		}
		t.Fatalf("no chunk fully covers middle %+v; chunks: %s", middle.ByteRange, strings.Join(ranges, ", "))
	}

	// Each function must occupy its own window for the sibling check to be
	// unambiguous.
	if scope.RangeContains(midChunk.ByteRange, first.ByteRange) || scope.RangeContains(midChunk.ByteRange, last.ByteRange) {
		t.Fatalf("middle chunk %+v must not also cover first/last; chunks: %+v", midChunk.ByteRange, chunks)
	}

	middleEnt := entityInfoInChunk(t, midChunk, middle.Name)
	sibs := middleEnt.Siblings
	if len(sibs) != 2 {
		t.Fatalf("middle entity siblings = %+v, want exactly 2", sibs)
	}
	checkSibling(t, sibs, "first", types.SiblingPositionBefore, 1)
	checkSibling(t, sibs, "last", types.SiblingPositionAfter, 1)

	// The first chunk's entity should have no before siblings.
	firstEnt := entityInfoInChunk(t, firstChunk, first.Name)
	for _, s := range firstEnt.Siblings {
		if s.Position == types.SiblingPositionBefore {
			t.Errorf("first entity siblings = %+v, want no before entry", firstEnt.Siblings)
		}
	}
}

// goCaptureSrc has a single function small enough to be one window; a closure
// inside it captures a param, an inferred float, a pointer literal, and a
// string param, while its own param and local stay uncaught.
const goCaptureSrc = `package conn

func process(hub Hub, prefix string) {
	deadline := 0.5
	client := &Client{}
	emit := func(line string) {
		out := hub.Send
		out(prefix + line)
		_ = client
		_ = deadline
	}
	_ = emit
}
`

func captureHas(captures []types.CapturedVariable, name string) bool {
	for _, cv := range captures {
		if cv.Name == name {
			return true
		}
	}
	return false
}

func checkCapture(t *testing.T, captures []types.CapturedVariable, name, typ string) {
	t.Helper()
	for _, cv := range captures {
		if cv.Name == name {
			if cv.Type != typ {
				t.Errorf("captured %q type = %q, want %q", name, cv.Type, typ)
			}
			if cv.DeclaredAt.Start > cv.DeclaredAt.End {
				t.Errorf("captured %q has inverted DeclaredAt %+v", name, cv.DeclaredAt)
			}
			return
		}
	}
	t.Errorf("captures = %+v, want %q(%s)", captures, name, typ)
}

// TestChunkContextCaptures asserts that the chunk containing a closure reports
// the enclosing variables it references and skips its own params and locals,
// and that the contextualized text carries a Captures line.
func TestChunkContextCaptures(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo}
	rootNode, scopeTree, language, err := parseSource("capture.go", goCaptureSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, goCaptureSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected a single chunk, got %d", len(chunks))
	}

	captures := chunks[0].Context.Captures
	checkCapture(t, captures, "hub", "Hub")
	checkCapture(t, captures, "prefix", "string")
	checkCapture(t, captures, "client", "*Client")
	checkCapture(t, captures, "deadline", "float64")
	for _, self := range []string{"line", "out"} {
		if captureHas(captures, self) {
			t.Errorf("captures = %+v, want no %q (closure-local)", captures, self)
		}
	}

	wantLine := "// Captures: hub(Hub), prefix(string), client(*Client), deadline(float64)"
	if !strings.Contains(chunks[0].ContextualizedText, wantLine) {
		t.Errorf("contextualized text missing %q;\ngot:\n%s", wantLine, chunks[0].ContextualizedText)
	}
}
