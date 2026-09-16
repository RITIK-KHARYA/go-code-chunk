package chunker

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

func TestCountNws(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"a b c", 3},
		{"\t\n\r\v\f ", 0},
		{"  spaced  ", 6},
		{"a\u00a0b", 2},          // NBSP counts as whitespace
		{"\u2007\u200a", 0},      // figure space / punctuation space range
		{"hello\u2028world", 10}, // line separator
		{"caf\u00e9", 4},         // non-ASCII rune is non-whitespace
		{"a\u0085b", 3},          // NEL is NOT JS \s, so non-whitespace (counted)
		{"\ufeffleading", 7},     // BOM counts as whitespace
	}

	for _, tc := range cases {
		if got := CountNws(tc.text); got != tc.want {
			t.Errorf("CountNws(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

func TestPreprocessNwsCumsum(t *testing.T) {
	code := "abc 12\nx"
	//      byte:  a b c  ' '  1  2 '\n' x
	//      nws:   1 2 3   3   4  5  5    6
	cumsum := PreprocessNwsCumsum(code)

	if len(cumsum) != len(code)+1 {
		t.Fatalf("len(cumsum) = %d, want %d", len(cumsum), len(code)+1)
	}
	expected := []uint32{0, 1, 2, 3, 3, 4, 5, 5, 6}
	for i, want := range expected {
		if cumsum[i] != want {
			t.Errorf("cumsum[%d] = %d, want %d", i, cumsum[i], want)
		}
	}
}

func TestGetNwsCountFromCumsum(t *testing.T) {
	cumsum := PreprocessNwsCumsum("a b c")

	want := []struct {
		start, end int
		want       int
	}{
		{0, 1, 1}, // "a"
		{0, 5, 3}, // "a b c"
		{1, 2, 0}, // " "
		{4, 5, 1}, // "c"
		{2, 3, 1}, // "b"
	}
	for _, tc := range want {
		if got := GetNwsCountFromCumsum(cumsum, tc.start, tc.end); got != tc.want {
			t.Errorf("GetNwsCountFromCumsum([%d,%d)) = %d, want %d", tc.start, tc.end, got, tc.want)
		}
	}
}

func TestGetNwsCountForNode(t *testing.T) {
	src := "func alpha() {\n\treturn 0\n}\n"
	p, err := parser.NewParser("go")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	cumsum := PreprocessNwsCumsum(src)
	root := types.SyntaxNode(tree.RootNode())

	// Whole-file NWS count must equal the total defined in the cumsum.
	got := GetNwsCountForNode(root, cumsum)
	want := int(cumsum[len(cumsum)-1])
	if got != want {
		t.Errorf("GetNwsCountForNode(root) = %d, want %d", got, want)
	}

	// A node's NWS count must never exceed the whole file's.
	for i := 0; i < tsNode(root).ChildCount(); i++ {
		child := tsNode(root).Child(i)
		if child == nil {
			continue
		}
		if c := GetNwsCountForNode(child, cumsum); c < 0 || c > want {
			t.Errorf("node NWS count %d out of range [0,%d]", c, want)
		}
	}
}
