package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// These tests guard the JS/TS `new_expression` dependency-resolution paths
// added alongside the bug fix. Each covers a constructor shape or behaviour
// not exercised by TestReproNewExprDeps or the pre-existing Go fixtures.

// TestNewExpressionMemberConstructor asserts qualified constructor calls
// resolve to the final property name, and that object/receiver names do not
// leak in as garbage dependencies: `new pkg.Bar()` -> Bar (not pkg), and a
// chained `new window.Bar.Baz()` -> Baz (not window, Bar, or the dotted text).
func TestNewExpressionMemberConstructor(t *testing.T) {
	src := "class Bar {}\n" +
		"class Baz {}\n" +
		"const pkg = { Bar, Baz };\n" +
		"const window = { Bar: { Baz } };\n" +
		"function makeA() { return new pkg.Bar(); }\n" +
		"function makeB() { return new window.Bar.Baz(); }\n"
	c := &CodeChunker{}
	results, err := c.Chunk("test.ts", src, types.ChunkOptions{Language: types.LanguageTypeScript})
	if err != nil {
		t.Fatalf("Chunk: %v\nsrc:\n%s", err, src)
	}
	a := findChunkEntity(t, results, "makeA")
	if _, ok := depHas(a.Dependencies, "Bar"); !ok {
		t.Fatalf("makeA deps = %+v, want Bar (new pkg.Bar() -> Bar)", a.Dependencies)
	}
	for _, d := range a.Dependencies {
		if d.Name == "pkg" {
			t.Errorf("makeA deps = %+v, want no garbage `pkg` receiver dependency", a.Dependencies)
		}
	}
	b := findChunkEntity(t, results, "makeB")
	if _, ok := depHas(b.Dependencies, "Baz"); !ok {
		t.Fatalf("makeB deps = %+v, want Baz (new window.Bar.Baz() -> Baz)", b.Dependencies)
	}
	for _, d := range b.Dependencies {
		if d.Name == "window" || d.Name == "Bar" {
			t.Errorf("makeB deps = %+v, want no garbage receiver name", b.Dependencies)
		}
	}
}

// TestNewExpressionNonIdentifierConstructorReturnsNoGarbage asserts constructor
// shapes that do not name a repo-local entity yield no garbage dependency: a
// parenthesized constructor `new (factory2())()` and an anonymous class
// `new class { m() {} }()`. (The nested `factory2()` plain call inside the
// parenthesised expression may legitimately resolve; only garbage names
// derived from the new_expression node itself are forbidden.)
func TestNewExpressionNonIdentifierConstructorReturnsNoGarbage(t *testing.T) {
	src := "function factory2() { return class Inner {}; }\n" +
		"function makeFromFactory() { return new (factory2())(); }\n" +
		"function makeFromAnonClass() { return new class { m() {} }(); }\n"
	c := &CodeChunker{}
	results, err := c.Chunk("test.ts", src, types.ChunkOptions{Language: types.LanguageTypeScript})
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}
	for _, name := range []string{"makeFromFactory", "makeFromAnonClass"} {
		ent := findChunkEntity(t, results, name)
		for _, d := range ent.Dependencies {
			if strings.Contains(d.Name, "(") && d.Name != "factory2" {
				t.Errorf("%s deps = %+v, want no garbage constructor-derived name", name, ent.Dependencies)
			}
		}
	}
}

// TestNewExpressionPlainCallStillResolves is the control that adding
// `new_expression` to the JS/TS CallNodeTypes did not perturb plain
// `call_expression` resolution. The pre-existing dependency fixtures only
// cover Go, so this is the first TS plain-call resolution assertion.
func TestNewExpressionPlainCallStillResolves(t *testing.T) {
	src := "class Logger {}\n" +
		"function makeLogger() { return new Logger(); }\n" +
		"function baby() { return makeLogger(); }\n"
	c := &CodeChunker{}
	results, err := c.Chunk("test.ts", src, types.ChunkOptions{Language: types.LanguageTypeScript})
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}
	baby := findChunkEntity(t, results, "baby")
	if _, ok := depHas(baby.Dependencies, "makeLogger"); !ok {
		t.Errorf("baby deps = %+v, want makeLogger (plain call regression)", baby.Dependencies)
	}
	makeLogger := findChunkEntity(t, results, "makeLogger")
	if _, ok := depHas(makeLogger.Dependencies, "Logger"); !ok {
		t.Errorf("makeLogger deps = %+v, want Logger (new call in same file)", makeLogger.Dependencies)
	}
}

// TestNewExpressionNoSelfReference asserts the existing self-reference filter
// (`name != enclosingName`) still applies to `new_expression`: a class that
// constructs itself inside its own constructor does NOT list itself as a
// dependency on the class entity, while the constructor method (enclosingName
// "constructor") legitimately resolves the surrounding class.
func TestNewExpressionNoSelfReference(t *testing.T) {
	src := "class Same {\n" +
		"  constructor() { this.other = new Same(); }\n" +
		"}\n"
	c := &CodeChunker{}
	results, err := c.Chunk("test.ts", src, types.ChunkOptions{Language: types.LanguageTypeScript})
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}
	ctor := findChunkEntity(t, results, "constructor")
	if _, ok := depHas(ctor.Dependencies, "Same"); !ok {
		t.Fatalf("constructor deps = %+v, want Same (the method enclosingName is \"constructor\", not \"Same\")", ctor.Dependencies)
	}
	same := findChunkEntity(t, results, "Same")
	if _, isSelf := depHas(same.Dependencies, "Same"); isSelf {
		t.Errorf("Same class deps = %+v, want no self-reference via its own constructor", same.Dependencies)
	}
}
