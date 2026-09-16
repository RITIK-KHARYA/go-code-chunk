package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// oversizedLeafSrc is a Go source whose block comment alone exceeds the
// maxSize used in the tests, so splitOversizedLeafByLines must split it.
var oversizedLeafSrc = func() string {
	lines := make([]string, 8)
	for i := range lines {
		lines[i] = strings.Repeat("word ", 12+i) // 48..76 NWS per line, well under 150
	}
	return "package p\n\n/* " + strings.Join(lines, "\n") + " */\n\nvar x = 1\n"
}()

// findFirstComment walks the tree and returns the first node of type "comment".
func findFirstComment(t *testing.T, root *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	t.Helper()

	var found *gotreesitter.Node
	var walk func(n *gotreesitter.Node)
	walk = func(n *gotreesitter.Node) {
		if found != nil {
			return
		}
		if n.Type(lang) == "comment" {
			found = n
			return
		}
		for i := 0; i < n.ChildCount(); i++ {
			if child := n.Child(i); child != nil {
				walk(child)
			}
		}
	}
	walk(root)

	if found == nil {
		t.Fatal("no comment node found")
	}
	return found
}

func TestSplitOversizedLeafByLines(t *testing.T) {
	const maxSize = 150

	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(oversizedLeafSrc))
	if err != nil {
		t.Fatal(err)
	}
	lang, err := parser.GetLanguage("go")
	if err != nil {
		t.Fatal(err)
	}

	comment := findFirstComment(t, tree.RootNode(), lang)
	if !IsLeafNode(comment) {
		t.Fatal("expected the block comment to be a leaf node")
	}

	cumsum := PreprocessNwsCumsum(oversizedLeafSrc)
	totalNws := GetNwsCountForNode(comment, cumsum)
	if totalNws <= maxSize {
		t.Fatalf("setup error: comment NWS = %d, want > %d", totalNws, maxSize)
	}

	windows := splitOversizedLeafByLines(comment, oversizedLeafSrc, maxSize)
	if len(windows) < 2 {
		t.Fatalf("windows = %d, want at least 2 splits", len(windows))
	}

	lineStarts := buildLineStartsTable(oversizedLeafSrc)
	startLine := int(comment.StartPoint().Row)
	endLine := int(comment.EndPoint().Row)

	var rebuilt strings.Builder
	var gotNws int
	var prevEnd = -1
	for i, w := range windows {
		if w.Size > maxSize {
			t.Errorf("window %d size = %d, exceeds maxSize %d", i, w.Size, maxSize)
		}
		gotNws += w.Size
		if len(w.LineRanges) != 1 {
			t.Fatalf("window %d: lineRanges = %d, want 1", i, len(w.LineRanges))
		}

		r := w.LineRanges[0]
		if i == 0 && r.Start != startLine {
			t.Errorf("window 0 starts at line %d, want %d", r.Start, startLine)
		}
		if i == len(windows)-1 && r.End != endLine {
			t.Errorf("last window ends at line %d, want %d", r.End, endLine)
		}
		if prevEnd >= 0 && r.Start != prevEnd+1 {
			t.Errorf("windows not contiguous: previous ended at %d, next starts at %d", prevEnd, r.Start)
		}
		prevEnd = r.End

		rebuilt.WriteString(RebuildText(w, oversizedLeafSrc).Text)
	}

	if gotNws != totalNws {
		t.Errorf("sum of window sizes = %d, want comment NWS %d", gotNws, totalNws)
	}
	wantText := oversizedLeafSrc[lineStarts[startLine]:lineStarts[endLine+1]]
	if rebuilt.String() != wantText {
		t.Errorf("rebuilt text does not reconstruct the comment span")
	}
}

func TestChunkCodeMatchesStreamChunks(t *testing.T) {
	const src = "package p\n\nfunc foo() { return 1 }\n\nfunc bar() { return 2 }\n"

	rootNode, scopeTree, language, err := parseSource("test.go", src, types.ChunkOptions{})
	if err != nil {
		t.Fatal(err)
	}

	options := types.ChunkOptions{}
	chunks, err := ChunkCode(rootNode, src, scopeTree, language, options, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	var streamed []types.Chunk
	for ch := range StreamChunks(rootNode, src, scopeTree, language, options, nil) {
		streamed = append(streamed, ch)
	}

	if len(chunks) != len(streamed) {
		t.Fatalf("ChunkCode produced %d chunks, StreamChunks %d", len(chunks), len(streamed))
	}
	for i := range chunks {
		if chunks[i].Text != streamed[i].Text {
			t.Errorf("chunk %d text mismatch between ChunkCode and StreamChunks", i)
		}
		if chunks[i].Index != i {
			t.Errorf("chunk %d Index = %d, want %d", i, chunks[i].Index, i)
		}
		if chunks[i].TotalChunks != len(chunks) {
			t.Errorf("chunk %d TotalChunks = %d, want %d", i, chunks[i].TotalChunks, len(chunks))
		}
	}
	if streamed[0].TotalChunks != -1 {
		t.Errorf("streamed TotalChunks = %d, want -1", streamed[0].TotalChunks)
	}
}
