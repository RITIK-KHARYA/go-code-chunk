package chunker

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

func TestCanMerge(t *testing.T) {
	a := types.ASTWindow{Size: 40}
	b := types.ASTWindow{Size: 30}

	if !CanMerge(a, b, 70) {
		t.Error("expected merge allowed when sizes exactly equal maxSize")
	}
	if !CanMerge(a, b, 100) {
		t.Error("expected merge allowed when sizes below maxSize")
	}
	if CanMerge(a, b, 69) {
		t.Error("expected merge rejected when combined size exceeds maxSize")
	}
}

func TestMergeWindows(t *testing.T) {
	const src = "function alpha() { return 1 }\nfunction beta() { return 2 }\n"

	nodeA, nodeB, nodeAc := parseNodesForMerge(t, src)

	a := types.ASTWindow{
		Nodes:         []types.SyntaxNode{nodeA},
		Ancestors:     []types.SyntaxNode{nodeAc, nodeAc}, // duplicate ancestor
		Size:          4,
		IsPartialNode: new(true),
		LineRanges:    []types.LineRange{{Start: 0, End: 0}},
	}
	b := types.ASTWindow{
		Nodes:      []types.SyntaxNode{nodeB},
		Ancestors:  []types.SyntaxNode{nodeAc}, // same ancestor as a
		Size:       5,
		LineRanges: []types.LineRange{{Start: 1, End: 1}},
	}

	got := MergeWindows(a, b)

	if got.Size != 9 {
		t.Errorf("size = %d, want 9", got.Size)
	}
	if len(got.Nodes) != 2 {
		t.Errorf("nodes = %d, want 2", len(got.Nodes))
	}
	if got.Nodes[0] != nodeA || got.Nodes[1] != nodeB {
		t.Error("nodes not concatenated in order")
	}
	if len(got.Ancestors) != 1 {
		t.Errorf("ancestors = %d, want 1 (deduplicated)", len(got.Ancestors))
	}
	if got.Ancestors[0] != nodeAc {
		t.Error("deduplicated ancestor mismatch")
	}
	if got.IsPartialNode == nil || !*got.IsPartialNode {
		t.Error("isPartialNode should be OR of both windows")
	}
	if len(got.LineRanges) != 2 {
		t.Errorf("lineRanges = %d, want 2 when both windows have ranges", len(got.LineRanges))
	}
}

func TestMergeWindowsLineRangesNilWhenMissing(t *testing.T) {
	const src = "function alpha() { return 1 }\n"
	_, nodeB, nodeAc := parseNodesForMerge(t, src)

	a := types.ASTWindow{Nodes: []types.SyntaxNode{nodeB}, Ancestors: []types.SyntaxNode{nodeAc}, Size: 3}
	b := types.ASTWindow{Nodes: []types.SyntaxNode{nodeB}, Ancestors: []types.SyntaxNode{nodeAc}, Size: 3, LineRanges: []types.LineRange{{Start: 0, End: 0}}}

	got := MergeWindows(a, b)
	if got.LineRanges != nil {
		t.Errorf("lineRanges = %v, want nil when only one window has ranges", got.LineRanges)
	}
}

func TestMergeAdjacentWindows(t *testing.T) {
	windows := []types.ASTWindow{
		{Size: 4},
		{Size: 2},
		{Size: 30},
		{Size: 7},
	}

	// maxSize 6: first two merge (4+2=6), 30 stays, 7 stays.
	got := MergeAdjacentWindows(windows, MergeOptions{MaxSize: 6})
	if len(got) != 3 {
		t.Fatalf("windows = %d, want 3", len(got))
	}
	if got[0].Size != 6 {
		t.Errorf("merged size = %d, want 6", got[0].Size)
	}
	if got[1].Size != 30 {
		t.Errorf("second window size = %d, want 30", got[1].Size)
	}
	if got[2].Size != 7 {
		t.Errorf("third window size = %d, want 7", got[2].Size)
	}

	// maxSize 100: everything merges into one window.
	got = MergeAdjacentWindows(windows, MergeOptions{MaxSize: 100})
	if len(got) != 1 {
		t.Fatalf("windows = %d, want 1", len(got))
	}
	if got[0].Size != 43 {
		t.Errorf("merged size = %d, want 43", got[0].Size)
	}
}

func TestMergeAdjacentWindowsEmpty(t *testing.T) {
	got := MergeAdjacentWindows(nil, MergeOptions{MaxSize: 100})
	if len(got) != 0 {
		t.Fatalf("windows = %d, want 0", len(got))
	}
}

// parseNodesForMerge parses the source and returns two sibling nodes plus the
// shared ancestor node, so pointer identity can be exercised.
func parseNodesForMerge(t *testing.T, src string) (a, b, ancestor types.SyntaxNode) {
	t.Helper()

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

	return functions[0], functions[1], root
}
