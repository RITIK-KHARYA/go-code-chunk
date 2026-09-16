package chunker

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

func TestRebuildTextEmptyWindow(t *testing.T) {
	got := RebuildText(types.ASTWindow{}, "some code")
	if got.Text != "" {
		t.Errorf("text = %q, want empty", got.Text)
	}
	if got.ByteRange != (types.ByteRange{}) {
		t.Errorf("byteRange = %+v, want zero", got.ByteRange)
	}
	if got.LineRange != (types.LineRange{}) {
		t.Errorf("lineRange = %+v, want zero", got.LineRange)
	}
}

func TestRebuildTextNormalWindow(t *testing.T) {
	src := "package main\n\nfunc alpha() int {\n\treturn 1\n}\n\nfunc beta() int {\n\treturn 2\n}\n"
	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	root := types.SyntaxNode(tree.RootNode())
	functions := childrenOf(root)
	if len(functions) < 2 {
		t.Fatalf("expected at least 2 top-level nodes, got %d", len(functions))
	}

	// Window covering the first two top-level declarations.
	window := types.ASTWindow{
		Nodes: []types.SyntaxNode{functions[0], functions[1]},
	}

	startByte := int(tsNode(functions[0]).StartByte())
	endByte := int(tsNode(functions[1]).EndByte())
	startLine := int(tsNode(functions[0]).StartPoint().Row)
	endLine := int(tsNode(functions[1]).EndPoint().Row)

	got := RebuildText(window, src)
	if got.Text != src[startByte:endByte] {
		t.Errorf("text mismatch:\n got %q\nwant %q", got.Text, src[startByte:endByte])
	}
	if got.ByteRange != (types.ByteRange{Start: startByte, End: endByte}) {
		t.Errorf("byteRange = %+v, want %+v", got.ByteRange, types.ByteRange{Start: startByte, End: endByte})
	}
	if got.LineRange != (types.LineRange{Start: startLine, End: endLine}) {
		t.Errorf("lineRange = %+v, want %+v", got.LineRange, types.LineRange{Start: startLine, End: endLine})
	}
}

func TestRebuildTextFromLineRanges(t *testing.T) {
	src := "package p\n\nfunc f() {\n\treturn\n}\n"
	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	window := types.ASTWindow{
		Nodes:         []types.SyntaxNode{types.SyntaxNode(tree.RootNode())},
		IsPartialNode: new(true),
		LineRanges:    []types.LineRange{{Start: 1, End: 3}},
	}

	got := RebuildText(window, src)
	wantText := src[10:30] // lines 1..3 inclusive: "\nfunc f() {\n\treturn\n"
	if got.Text != wantText {
		t.Errorf("text = %q, want %q", got.Text, wantText)
	}
	if got.ByteRange != (types.ByteRange{Start: 10, End: 30}) {
		t.Errorf("byteRange = %+v, want {10 30}", got.ByteRange)
	}
	if got.LineRange != (types.LineRange{Start: 1, End: 3}) {
		t.Errorf("lineRange = %+v, want {1 3}", got.LineRange)
	}
}

func TestRebuildTextFromLineRangesEndOfFile(t *testing.T) {
	src := "package p\n\nfunc f() {\n\treturn\n}" // last line "}" has no trailing newline
	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	window := types.ASTWindow{
		Nodes:         []types.SyntaxNode{types.SyntaxNode(tree.RootNode())},
		IsPartialNode: new(true),
		LineRanges:    []types.LineRange{{Start: 4, End: 4}},
	}

	got := RebuildText(window, src)
	// Line 4 ("}") has no following line; endByte falls back to len(code).
	if got.Text != "}" {
		t.Errorf("text = %q, want %q", got.Text, "}")
	}
	if got.ByteRange != (types.ByteRange{Start: 30, End: 31}) {
		t.Errorf("byteRange = %+v, want {30 31}", got.ByteRange)
	}
	if got.LineRange != (types.LineRange{Start: 4, End: 4}) {
		t.Errorf("lineRange = %+v, want {4 4}", got.LineRange)
	}
}

func TestBuildLineStartsTable(t *testing.T) {
	got := buildLineStartsTable("ab\nc\nd")
	want := []int{0, 3, 5}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lineStarts[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
