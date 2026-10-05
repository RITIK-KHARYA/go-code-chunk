package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/scope"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// goDepsSrc has three sibling functions padded (MaxChunkSize 1000) so each
// lands in its own chunk, mirroring the proven goSiblingsSrc windowing shape.
// target calls helper once, other twice, and itself recursively once; the
// direct recursion must not appear as a dependency.
const goDepsSrc = `package main

func helper() {
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

func other() {
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

func target() {
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
	_ = helper()
	_ = other(1)
	_ = other(2)
	target()
}
`

func depHas(deps []types.DependencyInfo, name string) (types.DependencyInfo, bool) {
	for _, d := range deps {
		if d.Name == name {
			return d, true
		}
	}
	return types.DependencyInfo{}, false
}

// TestChunkDependenciesResolved asserts target's chunk lists its two called
// siblings once each (with signature and type) and omits its own recursive
// call.
func TestChunkDependenciesResolved(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 1000}
	rootNode, scopeTree, language, err := parseSource("deps.go", goDepsSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, goDepsSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}

	target := findEntityName(t, scopeTree, "target")
	helper := findEntityName(t, scopeTree, "helper")
	other := findEntityName(t, scopeTree, "other")

	var targetChunk types.Chunk
	found := false
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, target.ByteRange) {
			targetChunk = ch
			found = true
		}
	}
	if !found {
		t.Fatal("no chunk fully covers target")
	}

	targetEnt := entityInfoInChunk(t, targetChunk, target.Name)
	deps := targetEnt.Dependencies

	helperDep, ok := depHas(deps, "helper")
	if !ok {
		t.Fatalf("target deps = %+v, want helper dependency", deps)
	}
	if helperDep.Signature != helper.Signature || helperDep.Type != types.EntityTypeFunction {
		t.Errorf("helper dep = %+v, want sig %q/type function", helperDep, helper.Signature)
	}
	if helperDep.Filepath != "" {
		t.Errorf("helper dep = %+v, want empty Filepath for a same-file dependency", helperDep)
	}

	otherDep, ok := depHas(deps, "other")
	if !ok {
		t.Fatalf("target deps = %+v, want other dependency", deps)
	}
	if otherDep.Signature != other.Signature || otherDep.Type != types.EntityTypeFunction {
		t.Errorf("other dep = %+v, want sig %q/type function", otherDep, other.Signature)
	}

	if _, isSelf := depHas(deps, "target"); isSelf {
		t.Errorf("target deps = %+v, want no self-reference to target", deps)
	}

	// other is called twice; exactly one dep must be reported.
	count := 0
	for _, d := range deps {
		if d.Name == "other" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("other appears %d times in target deps, want exactly 1: %+v", count, deps)
	}

	// The contextualized text must render the dependency without a file suffix
	// for a same-file call.
	if !strings.Contains(targetChunk.ContextualizedText, "// Dependencies: helper(...), other(...)") {
		t.Errorf("contextualized text missing same-file deps line:\n%s", targetChunk.ContextualizedText)
	}

	// helper chunk calls nothing and must report no dependencies.
	var helperChunk types.Chunk
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, helper.ByteRange) {
			helperChunk = ch
		}
	}
	helperEnt := entityInfoInChunk(t, helperChunk, helper.Name)
	if len(helperEnt.Dependencies) != 0 {
		t.Errorf("helper entity deps = %+v, want none", helperEnt.Dependencies)
	}
}

// goTypeConvSrc defines a named Go type (MyInt) and a function that converts to
// it via MyInt(x), which parses as a call_expression whose function child is a
// type_identifier. Before the ExtractName fix, MyInt was indexed under
// ByName["<anonymous>"], so the conversion edge was silently dropped.
const goTypeConvSrc = `package main

type MyInt int

func asInt(x int) MyInt {
	return MyInt(x)
}
`

// TestChunkTypeConversionDependencyResolved asserts that a Go type-conversion
// call (T(x)) resolves to the named type T as a dependency: the type entity
// must carry its real name in the scope tree and ByName index, and the calling
// function's chunk must list the type as a dependency.
func TestChunkTypeConversionDependencyResolved(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 2000}
	rootNode, scopeTree, language, err := parseSource("conv.go", goTypeConvSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, goTypeConvSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}

	// The type entity must be extractable by name (regression: was "<anonymous>").
	myInt := findEntityName(t, scopeTree, "MyInt")
	if myInt.Type != types.EntityTypeType {
		t.Fatalf("MyInt entity type = %s, want %s", myInt.Type, types.EntityTypeType)
	}
	asInt := findEntityName(t, scopeTree, "asInt")

	var asIntChunk types.Chunk
	found := false
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, asInt.ByteRange) {
			asIntChunk = ch
			found = true
		}
	}
	if !found {
		t.Fatal("no chunk fully covers asInt")
	}

	asIntEnt := entityInfoInChunk(t, asIntChunk, asInt.Name)
	myIntDep, ok := depHas(asIntEnt.Dependencies, myInt.Name)
	if !ok {
		t.Fatalf("asInt deps = %+v, want dependency on %q (type conversion)", asIntEnt.Dependencies, myInt.Name)
	}
	if myIntDep.Type != types.EntityTypeType {
		t.Errorf("MyInt dep type = %s, want %s", myIntDep.Type, types.EntityTypeType)
	}
}

// goAliasConvSrc defines a single Go type alias (Bytes = []byte) and a
// function that converts to it via Bytes(s). tree-sitter parses `type Bytes =
// []byte` with a `type_alias` child (not `type_spec`); before ExtractName
// counted `type_alias`, the alias entity was extracted under "<anonymous>",
// so ByName["Bytes"] missed and the Bytes(s) conversion edge was silently
// dropped. This is the alias analogue of TestChunkTypeConversionDependencyResolved.
const goAliasConvSrc = `package main

type Bytes = []byte

func toBytes(s string) Bytes {
	return Bytes(s)
}
`

// TestChunkTypeAliasConversionDependencyResolved asserts that a Go
// type-conversion call to a single type alias (A(x) where A is `type A =
// ...`) resolves to the alias entity as a dependency: the alias must carry its
// real name in the scope tree and ByName index (not "<anonymous>"), and the
// calling function's chunk must list the alias type as a dependency.
func TestChunkTypeAliasConversionDependencyResolved(t *testing.T) {
	opts := types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 2000}
	rootNode, scopeTree, language, err := parseSource("alias.go", goAliasConvSrc, opts)
	if err != nil {
		t.Fatalf("parseSource: %v", err)
	}
	chunks, err := ChunkCode(rootNode, goAliasConvSrc, scopeTree, language, opts, nil)
	if err != nil {
		t.Fatalf("ChunkCode: %v", err)
	}

	// The alias entity must be extractable by its real name (regression: was
	// "<anonymous>" before ExtractName counted the type_alias child).
	bytesAlias := findEntityName(t, scopeTree, "Bytes")
	if bytesAlias.Type != types.EntityTypeType {
		t.Fatalf("Bytes alias entity type = %s, want %s", bytesAlias.Type, types.EntityTypeType)
	}
	if bytesAlias.Signature != "type Bytes" {
		t.Errorf("Bytes alias signature = %q, want %q", bytesAlias.Signature, "type Bytes")
	}
	toBytes := findEntityName(t, scopeTree, "toBytes")

	var toBytesChunk types.Chunk
	found := false
	for _, ch := range chunks {
		if scope.RangeContains(ch.ByteRange, toBytes.ByteRange) {
			toBytesChunk = ch
			found = true
		}
	}
	if !found {
		t.Fatal("no chunk fully covers toBytes")
	}

	toBytesEnt := entityInfoInChunk(t, toBytesChunk, toBytes.Name)
	bytesDep, ok := depHas(toBytesEnt.Dependencies, bytesAlias.Name)
	if !ok {
		t.Fatalf("toBytes deps = %+v, want dependency on %q (alias type conversion)", toBytesEnt.Dependencies, bytesAlias.Name)
	}
	if bytesDep.Type != types.EntityTypeType {
		t.Errorf("Bytes dep type = %s, want %s", bytesDep.Type, types.EntityTypeType)
	}
	if bytesDep.Signature != "type Bytes" {
		t.Errorf("Bytes dep signature = %q, want %q", bytesDep.Signature, "type Bytes")
	}
}

// TestBuildProjectIndexGoTypeByName asserts the project-wide ByName index keys
// single-spec Go types under their real name (not "<anonymous>"), so the
// dependency resolver and other name-based consumers can look them up. A
// grouped declaration still collapses under "<anonymous>" — the documented
// single-spec gate behavior.
func TestBuildProjectIndexGoTypeByName(t *testing.T) {
	files := []types.FileInput{
		{Filepath: "single.go", Code: "package main\ntype MyInt int\n"},
		{Filepath: "grouped.go", Code: "package main\ntype (\n\tA int\n\tB string\n)\n"},
	}
	index, err := BuildProjectIndex(files, types.BatchOptions{
		ChunkOptions: types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 2000},
	})
	if err != nil {
		t.Fatalf("BuildProjectIndex: %v", err)
	}

	myIntGroup, ok := index.ByName["MyInt"]
	if !ok || len(myIntGroup) != 1 {
		t.Fatalf("ByName[MyInt] = %+v, want exactly 1 entry (was collapsed under <anonymous> before fix)", myIntGroup)
	}
	if myIntGroup[0].Entity.Name != "MyInt" {
		t.Errorf("ByName[MyInt][0].Name = %q, want %q", myIntGroup[0].Entity.Name, "MyInt")
	}
	if myIntGroup[0].Entity.Type != types.EntityTypeType {
		t.Errorf("ByName[MyInt][0].Type = %s, want %s", myIntGroup[0].Entity.Type, types.EntityTypeType)
	}
	if myIntGroup[0].Filepath != "single.go" {
		t.Errorf("ByName[MyInt][0].Filepath = %q, want %q", myIntGroup[0].Filepath, "single.go")
	}

	// Grouped declaration stays unnamed: one entity under "<anonymous>".
	grouped, hasGrouped := index.ByName["<anonymous>"]
	if !hasGrouped || len(grouped) != 1 {
		t.Fatalf("ByName[<anonymous>] = %+v, want exactly 1 grouped entry (grouped declarations stay unnamed)", grouped)
	}
	if grouped[0].Filepath != "grouped.go" {
		t.Errorf("ByName[<anonymous>][0].Filepath = %q, want %q", grouped[0].Filepath, "grouped.go")
	}
}

// TestBuildProjectIndexGoTypeAliasByName asserts the project-wide ByName index
// keys single-spec Go type aliases (`type Bytes = []byte`) under their real
// name (not "<anonymous>"), so the dependency resolver can look them up for
// conversion calls like Bytes(s). A grouped alias declaration still collapses
// under "<anonymous>" — the documented single-spec gate behavior. This is the
// alias analogue of TestBuildProjectIndexGoTypeByName.
func TestBuildProjectIndexGoTypeAliasByName(t *testing.T) {
	files := []types.FileInput{
		{Filepath: "alias_single.go", Code: "package main\ntype Bytes = []byte\n"},
		{Filepath: "alias_grouped.go", Code: "package main\ntype (\n\tA = int\n\tB = string\n)\n"},
	}
	index, err := BuildProjectIndex(files, types.BatchOptions{
		ChunkOptions: types.ChunkOptions{Language: types.LanguageGo, MaxChunkSize: 2000},
	})
	if err != nil {
		t.Fatalf("BuildProjectIndex: %v", err)
	}

	bytesGroup, ok := index.ByName["Bytes"]
	if !ok || len(bytesGroup) != 1 {
		t.Fatalf("ByName[Bytes] = %+v, want exactly 1 entry (alias was collapsed under <anonymous> before fix)", bytesGroup)
	}
	if bytesGroup[0].Entity.Name != "Bytes" {
		t.Errorf("ByName[Bytes][0].Name = %q, want %q", bytesGroup[0].Entity.Name, "Bytes")
	}
	if bytesGroup[0].Entity.Type != types.EntityTypeType {
		t.Errorf("ByName[Bytes][0].Type = %s, want %s", bytesGroup[0].Entity.Type, types.EntityTypeType)
	}
	if bytesGroup[0].Filepath != "alias_single.go" {
		t.Errorf("ByName[Bytes][0].Filepath = %q, want %q", bytesGroup[0].Filepath, "alias_single.go")
	}

	// Grouped alias declaration stays unnamed: one entity under "<anonymous>".
	grouped, hasGrouped := index.ByName["<anonymous>"]
	if !hasGrouped || len(grouped) != 1 {
		t.Fatalf("ByName[<anonymous>] = %+v, want exactly 1 grouped entry (grouped aliases stay unnamed)", grouped)
	}
	if grouped[0].Filepath != "alias_grouped.go" {
		t.Errorf("ByName[<anonymous>][0].Filepath = %q, want %q", grouped[0].Filepath, "alias_grouped.go")
	}
}
