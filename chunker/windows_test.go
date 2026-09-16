package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

func TestGetAncestorsEmpty(t *testing.T) {
	if got := GetAncestors(nil); len(got) != 0 {
		t.Fatalf("ancestors = %v, want empty", got)
	}
}

func TestGetAncestorsSiblings(t *testing.T) {
	const src = "package p\n\nfunc alpha() {}\n\nfunc beta() {}\n"
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

	got := GetAncestors(functions[:2])
	if len(got) != 1 {
		t.Fatalf("ancestors = %d, want 1 (shared parent deduplicated)", len(got))
	}
	if tsNode(got[0]) != tsNode(root) {
		t.Error("expected the single ancestor to be the root node")
	}
}

func TestGetAncestorsWalksEveryNode(t *testing.T) {
	const src = "package p\n\nfunc f() {\n\ta := 1\n}\n\nfunc g() {\n\tb := 2\n}\n"
	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	aNode := types.SyntaxNode(tree.NodeAtByte(uint32(strings.Index(src, "a :="))))
	bNode := types.SyntaxNode(tree.NodeAtByte(uint32(strings.Index(src, "b :="))))
	if aNode == nil || bNode == nil {
		t.Fatal("failed to resolve nodes at byte offsets")
	}

	got := GetAncestors([]types.SyntaxNode{aNode, bNode})

	// Expected: walk parent chains of both nodes in order, deduplicating.
	seen := make(map[*gotreesitter.Node]struct{})
	var expected []types.SyntaxNode
	for _, n := range []types.SyntaxNode{aNode, bNode} {
		for cur := tsNode(n).Parent(); cur != nil; cur = cur.Parent() {
			if _, ok := seen[cur]; ok {
				continue
			}
			seen[cur] = struct{}{}
			expected = append(expected, cur)
		}
	}

	if len(got) != len(expected) {
		t.Fatalf("ancestors = %d, want %d (chains of both nodes deduplicated)", len(got), len(expected))
	}
	for i := range expected {
		if tsNode(got[i]) != tsNode(expected[i]) {
			t.Errorf("ancestors[%d]: got %v, want %v", i, tsNode(got[i]).StartByte(), tsNode(expected[i]).StartByte())
		}
	}

	// Sanity: the file root must appear exactly once (deduplicated).
	root := tsNode(tree.RootNode())
	rootOccurrences := 0
	for _, anc := range got {
		if tsNode(anc) == root {
			rootOccurrences++
		}
	}
	if rootOccurrences != 1 {
		t.Errorf("file root appears %d times, want exactly 1", rootOccurrences)
	}
}

func TestGetAncestorsOrderedParentFirst(t *testing.T) {
	const src = "package p\n\nfunc f() {\n\ta := 1\n}\n"
	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	aNode := types.SyntaxNode(tree.NodeAtByte(uint32(strings.Index(src, "a :="))))
	got := GetAncestors([]types.SyntaxNode{aNode})

	if len(got) == 0 {
		t.Fatal("expected at least one ancestor")
	}
	if tsNode(got[0]) != tsNode(aNode).Parent() {
		t.Error("first ancestor should be the node's immediate parent")
	}

	// Each subsequent ancestor should be the previous one's parent.
	for i := 1; i < len(got); i++ {
		if tsNode(got[i]) != tsNode(got[i-1]).Parent() {
			t.Errorf("ancestors[%d] is not the parent of ancestors[%d]", i, i-1)
		}
	}
}
