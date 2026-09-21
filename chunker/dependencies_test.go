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
