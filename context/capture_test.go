package chunkcontext

import (
	"testing"

	"github.com/odvcencio/gotreesitter"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// resolveCapturesFor resolves captures across the given byte range of src.
func resolveCapturesFor(t *testing.T, src string, start, end int) []types.CapturedVariable {
	t.Helper()
	_, tree := buildTreeFor(t, src, "go")
	return ResolveCaptures(types.ByteRange{Start: start, End: end}, tree, src, types.LanguageGo)
}

func findCapture(t *testing.T, captures []types.CapturedVariable, name string) types.CapturedVariable {
	t.Helper()
	for _, cv := range captures {
		if cv.Name == name {
			return cv
		}
	}
	t.Fatalf("captured variable %q not found in %v", name, captures)
	return types.CapturedVariable{}
}

func capturesLen(t *testing.T, captures []types.CapturedVariable, want int) {
	t.Helper()
	if len(captures) != want {
		t.Errorf("captures length = %d, want %d (%v)", len(captures), want, captures)
	}
}

// TestCaptureParamsAndInferredLocals asserts a closure capturing a typed param
// and an inferred local reports both with their types and declaration lines.
func TestCaptureParamsAndInferredLocals(t *testing.T) {
	src := `package p

func outer(base int) {
	count := 0
	emit := func() {
		_ = base
		_ = count
	}
	_ = emit
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 2)

	base := findCapture(t, captures, "base")
	if base.Type != "int" {
		t.Errorf("base type = %q, want %q", base.Type, "int")
	}
	if base.DeclaredAt.Start != 2 || base.DeclaredAt.End != 2 {
		t.Errorf("base DeclaredAt = %+v, want row 2", base.DeclaredAt)
	}

	count := findCapture(t, captures, "count")
	if count.Type != "int" {
		t.Errorf("count type = %q, want %q", count.Type, "int")
	}
	if count.DeclaredAt.Start != 3 || count.DeclaredAt.End != 3 {
		t.Errorf("count DeclaredAt = %+v, want row 3", count.DeclaredAt)
	}
}

// TestCaptureExplicitTypeAndPointer asserts explicit var types and &T{} literals
// produce their expected type strings.
func TestCaptureExplicitTypeAndPointer(t *testing.T) {
	src := `package p

func f() {
	var msg string = "hi"
	list := []int{1, 2}
	obj := &Widget{}
	_ = func() {
		_ = msg
		_ = list
		_ = obj
	}
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 3)

	if cv := findCapture(t, captures, "msg"); cv.Type != "string" {
		t.Errorf("msg type = %q, want %q", cv.Type, "string")
	}
	if cv := findCapture(t, captures, "list"); cv.Type != "[]int" {
		t.Errorf("list type = %q, want %q", cv.Type, "[]int")
	}
	if cv := findCapture(t, captures, "obj"); cv.Type != "*Widget" {
		t.Errorf("obj type = %q, want %q", cv.Type, "*Widget")
	}
}

// TestCaptureSelfDeclarationsNotCaptured asserts locals and params declared
// inside the closure itself are never reported as captures.
func TestCaptureSelfDeclarationsNotCaptured(t *testing.T) {
	src := `package p

func f() {
	_ = func(own int) {
		local := 1
		_ = own
		_ = local
	}
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 0)
}

// TestCaptureShadowing asserts binding respects lexical shadowing: a closure
// with its own x does not capture the outer x, while a sibling closure without
// it does.
func TestCaptureShadowing(t *testing.T) {
	src := `package p

func f() {
	x := 1
	g := func() {
		x := 2
		_ = x
	}
	h := func() {
		_ = x
	}
	_ = g
	_ = h
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 1)
	cv := findCapture(t, captures, "x")
	if cv.Type != "int" {
		t.Errorf("x type = %q, want %q", cv.Type, "int")
	}
	if cv.DeclaredAt.Start != 3 {
		t.Errorf("x DeclaredAt = %+v, want row 3 (outer declaration)", cv.DeclaredAt)
	}
}

// TestCaptureNestedClosure asserts a closure nested inside another closure
// captures locals of its enclosing closure.
func TestCaptureNestedClosure(t *testing.T) {
	src := `package p

func f() {
	_ = func() {
		y := 2
		_ = func() { _ = y }
	}
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 1)
	cv := findCapture(t, captures, "y")
	if cv.Type != "int" || cv.DeclaredAt.Start != 4 {
		t.Errorf("y = (%q, %+v), want (int, row 4)", cv.Type, cv.DeclaredAt)
	}
}

// TestCaptureReceiver asserts a method receiver referenced from a closure is
// reported with the receiver type.
func TestCaptureReceiver(t *testing.T) {
	src := `package p

type S struct{ N int }

func (rec *S) M() {
	_ = func() { _ = rec }
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 1)
	cv := findCapture(t, captures, "rec")
	if cv.Type != "*S" {
		t.Errorf("rec type = %q, want %q", cv.Type, "*S")
	}
	if cv.DeclaredAt.Start != 4 {
		t.Errorf("rec DeclaredAt = %+v, want row 4", cv.DeclaredAt)
	}
}

// TestCaptureUnresolvedDropped asserts a reference that binds to no
// declaration in an enclosing scope is not reported.
func TestCaptureUnresolvedDropped(t *testing.T) {
	src := `package p

func f() {
	_ = func() { _ = globalThing }
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 0)
}

// TestCaptureDedupesAcrossClosures asserts a variable captured by several
// closures is reported once.
func TestCaptureDedupesAcrossClosures(t *testing.T) {
	src := `package p

func f() {
	msg := "hi"
	_ = func() { _ = msg }
	_ = func() { _ = msg }
}
`
	captures := resolveCapturesFor(t, src, 0, len(src))
	capturesLen(t, captures, 1)
	if cv := findCapture(t, captures, "msg"); cv.Type != "string" {
		t.Errorf("msg type = %q, want %q", cv.Type, "string")
	}
}

// TestCaptureDeclaredOutsideChunkRange asserts a split-window scenario: when a
// chunk covers only the closure, a variable declared earlier in the enclosing
// function (outside the chunk) is still reported as captured.
func TestCaptureDeclaredOutsideChunkRange(t *testing.T) {
	src := `package p

func f() {
	x := 1
	_ = func() { _ = x }
}
`
	closure := findClosureRange(t, src)
	captures := resolveCapturesFor(t, src, closure.Start, closure.End)
	capturesLen(t, captures, 1)
	cv := findCapture(t, captures, "x")
	if cv.Type != "int" {
		t.Errorf("x type = %q, want %q", cv.Type, "int")
	}
	if cv.DeclaredAt.Start != 3 || cv.DeclaredAt.Start >= closure.Start {
		t.Errorf("x DeclaredAt = %+v, want row 3 outside chunk", cv.DeclaredAt)
	}
}

// findClosureRange locates the first func_literal node's byte range.
func findClosureRange(t *testing.T, src string) types.ByteRange {
	t.Helper()
	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	st, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var found *gotreesitter.Node
	lang := p.Language()
	var rec func(n *gotreesitter.Node)
	rec = func(n *gotreesitter.Node) {
		if n == nil || found != nil {
			return
		}
		if n.Type(lang) == "func_literal" {
			found = n
			return
		}
		for _, c := range n.Children() {
			rec(c)
		}
	}
	rec(st.RootNode())
	if found == nil {
		t.Fatal("no func_literal found")
	}
	return types.ByteRange{Start: int(found.StartByte()), End: int(found.EndByte())}
}

// TestUnsupportedLanguageReturnsNil asserts non-Go languages report no
// captures yet.
func TestUnsupportedLanguageReturnsNil(t *testing.T) {
	src := "function add(a: number): number {\n  return a + 1;\n}\n"
	_, tree := buildTreeFor(t, src, "typescript")
	captures := ResolveCaptures(types.ByteRange{Start: 0, End: len(src)}, tree, src, types.LanguageTypeScript)
	if len(captures) != 0 {
		t.Errorf("unexpected captures for typescript: %v", captures)
	}
}
