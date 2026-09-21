package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// langFor parses source with the given language name and returns the root
// node and the resolved native grammar.
func parseForTest(t *testing.T, langName, src string) (types.SyntaxNode, *gotreesitter.Language) {
	t.Helper()
	p, err := parser.NewParser(langName)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	lang, err := parser.GetLanguage(langName)
	if err != nil {
		t.Fatal(err)
	}
	return types.SyntaxNode(tree.RootNode()), lang
}

// nodeType returns the tree-sitter type name of node for lang.
func nodeType(t *testing.T, node types.SyntaxNode, lang *gotreesitter.Language) string {
	t.Helper()
	return tsNode(node).Type(lang)
}

func TestGroupWithLeadingComments(t *testing.T) {
	root, lang := parseForTest(t, "go", "package p\n\n// doc comment\nfunc alpha() {}\n\n// trailing\n")

	groups := groupWithLeadingComments(childrenOf(root), types.LanguageGo, lang)

	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3 ([package] [comment+func] [trailing comment])", len(groups))
	}

	if len(groups[0]) != 1 || nodeType(t, groups[0][0], lang) != "package_clause" {
		t.Errorf("groups[0] = %+v, want the package clause alone", groups[0])
	}

	if len(groups[1]) != 2 {
		t.Fatalf("groups[1] has %d nodes, want 2 (comment absorbed into its declaration)", len(groups[1]))
	}
	if nodeType(t, groups[1][0], lang) != "comment" {
		t.Errorf("groups[1][0] type = %q, want comment", nodeType(t, groups[1][0], lang))
	}
	if nodeType(t, groups[1][1], lang) != "function_declaration" {
		t.Errorf("groups[1][1] type = %q, want function_declaration", nodeType(t, groups[1][1], lang))
	}

	if len(groups[2]) != 1 || nodeType(t, groups[2][0], lang) != "comment" {
		t.Errorf("trailing comment should stay its own group, got %d nodes", len(groups[2]))
	}
}

func TestGroupWithLeadingCommentsNoComments(t *testing.T) {
	root, lang := parseForTest(t, "go", "package p\n\nfunc alpha() {}\n")

	groups := groupWithLeadingComments(childrenOf(root), types.LanguageGo, lang)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2 (one per node)", len(groups))
	}
	for i, g := range groups {
		if len(g) != 1 {
			t.Errorf("groups[%d] = %d nodes, want 1", i, len(g))
		}
	}
}

func TestGroupWithLeadingCommentsRunOfComments(t *testing.T) {
	root, lang := parseForTest(t, "go", "package p\n\n// one\n// two\n// three\nfunc alpha() {}\n")

	groups := groupWithLeadingComments(childrenOf(root), types.LanguageGo, lang)

	var merged []types.SyntaxNode
	for _, g := range groups {
		if len(g) > 1 {
			merged = g
			break
		}
	}
	if len(merged) != 4 {
		t.Fatalf("merged comment run has %d nodes, want 4 (3 comments + declaration)", len(merged))
	}
	for i := range 3 {
		if nodeType(t, merged[i], lang) != "comment" {
			t.Errorf("merged[%d] type = %q, want comment", i, nodeType(t, merged[i], lang))
		}
	}
	if nodeType(t, merged[3], lang) != "function_declaration" {
		t.Errorf("merged[3] type = %q, want function_declaration", nodeType(t, merged[3], lang))
	}
}

// TestGroupWithLeadingCommentsAcrossLanguages validates the per-language
// comment node type table: for every supported grammar, a leading comment
// node must be absorbed into the group of the declaration that follows it.
func TestGroupWithLeadingCommentsAcrossLanguages(t *testing.T) {
	cases := []struct {
		name        string
		lang        types.Language
		src         string
		commentType string
		declType    string
	}{
		{name: "go", lang: types.LanguageGo, src: "package p\n\n// doc\nfunc alpha() {}\n", commentType: "comment", declType: "function_declaration"},
		{name: "typescript", lang: types.LanguageTypeScript, src: "// doc\nfunction alpha(): void {}\n", commentType: "comment", declType: "function_declaration"},
		{name: "javascript", lang: types.LanguageJavaScript, src: "// doc\nfunction alpha() {}\n", commentType: "comment", declType: "function_declaration"},
		{name: "python", lang: types.LanguagePython, src: "# doc\ndef alpha():\n    pass\n", commentType: "comment", declType: "function_definition"},
		{name: "rust", lang: types.LanguageRust, src: "/// doc\nfn alpha() {\n}\n", commentType: "line_comment", declType: "function_item"},
		{name: "java", lang: types.LanguageJava, src: "// doc\nclass Alpha {\n}\n", commentType: "line_comment", declType: "class_declaration"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, lang := parseForTest(t, string(tc.lang), tc.src)
			groups := groupWithLeadingComments(childrenOf(root), tc.lang, lang)

			for _, g := range groups {
				if len(g) != 2 {
					continue
				}
				if nodeType(t, g[0], lang) != tc.commentType {
					t.Errorf("comment node type = %q, want %q", nodeType(t, g[0], lang), tc.commentType)
				}
				if nodeType(t, g[1], lang) != tc.declType {
					t.Errorf("following node type = %q, want %q", nodeType(t, g[1], lang), tc.declType)
				}
				return
			}
			t.Fatalf("no group merged the comment with its declaration; groups = %d", len(groups))
		})
	}
}

// sizedGoFunc builds a Go function whose body repeats stmt n times, giving a
// deterministic NWS size for threshold arithmetic.
func sizedGoFunc(name string, n int) string {
	return "func " + name + "() {\n" + strings.Repeat("\ta := \"aaaaaaaaaaaaaaaaaaaa\"\n", n) + "}\n"
}

// TestChunkKeepsDocCommentWithDeclaration is the Phase 0 regression test: the
// doc comment alone would let byte-size windowing close out a window right at
// MaxChunkSize, but the comment and the function must land together.
func TestChunkKeepsDocCommentWithDeclaration(t *testing.T) {
	const maxSize = 300
	const fillStmts = 4
	const targetStmts = 8
	doc := "// doc comment for target function\n// second line of the doc comment\n// third line of the doc comment\n"

	fill := sizedGoFunc("fill", fillStmts)
	target := sizedGoFunc("target", targetStmts)
	src := "package p\n\n" + fill + "\n" + doc + "\n" + target

	fSize := CountNws(fill)
	cSize := CountNws(doc)
	tSize := CountNws(target)
	if fSize > maxSize || fSize+cSize > maxSize || fSize+cSize+tSize <= maxSize || cSize+tSize > maxSize {
		t.Fatalf("setup error: fill=%d comment=%d target=%d, want fill<=%d, fill+comment<=%d, fill+comment+target>%d, comment+target<=%d",
			fSize, cSize, tSize, maxSize, maxSize, maxSize, maxSize)
	}

	rootNode, scopeTree, language, err := parseSource("test.go", src, types.ChunkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := ChunkCode(rootNode, src, scopeTree, language, types.ChunkOptions{MaxChunkSize: maxSize}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var targetChunk *types.Chunk
	for i := range chunks {
		if strings.Contains(chunks[i].Text, "func target") {
			if targetChunk != nil {
				t.Fatalf("func target appears in more than one chunk")
			}
			targetChunk = &chunks[i]
		}
	}
	if targetChunk == nil {
		t.Fatal("failed to find a chunk containing func target")
	}
	if !strings.Contains(targetChunk.Text, "// doc comment for target function") {
		t.Fatalf("doc comment split from its declaration:\nchunk contents:\n%s", targetChunk.Text)
	}
	if !strings.Contains(targetChunk.Text, "// third line of the doc comment") {
		t.Fatalf("final doc comment line split from its declaration:\nchunk contents:\n%s", targetChunk.Text)
	}
}

// TestChunkKeepsCommentRunWithDeclaration asserts an entire run of consecutive
// comment lines stays glued to the declaration that follows it.
func TestChunkKeepsCommentRunWithDeclaration(t *testing.T) {
	const maxSize = 260
	const fillStmts = 3
	doc := "// a\n// b\n// c\n// d\n// e\n"

	fill := sizedGoFunc("fill", fillStmts)
	target := sizedGoFunc("target", 7)
	src := "package p\n\n" + fill + "\n\n" + doc + "\n" + target

	fSize := CountNws(fill)
	cSize := CountNws(doc)
	tSize := CountNws(target)
	if fSize > maxSize || fSize+cSize > maxSize || fSize+cSize+tSize <= maxSize || cSize+tSize > maxSize {
		t.Fatalf("setup error: fill=%d comment=%d target=%d for maxSize=%d", fSize, cSize, tSize, maxSize)
	}

	rootNode, scopeTree, language, err := parseSource("test.go", src, types.ChunkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := ChunkCode(rootNode, src, scopeTree, language, types.ChunkOptions{MaxChunkSize: maxSize}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var targetChunk *types.Chunk
	for i := range chunks {
		if strings.Contains(chunks[i].Text, "func target") {
			targetChunk = &chunks[i]
			break
		}
	}
	if targetChunk == nil {
		t.Fatal("failed to find a chunk containing func target")
	}
	for _, line := range []string{"// a", "// b", "// c", "// d", "// e"} {
		if !strings.Contains(targetChunk.Text, line) {
			t.Errorf("comment line %q split from its declaration:\nchunk contents:\n%s", line, targetChunk.Text)
		}
	}
}

// TestChunkKeepsRustDocComment asserts consecutive /// doc comment lines stay
// glued to the fn item they document (Rust uses line_comment node types).
func TestChunkKeepsRustDocComment(t *testing.T) {
	sizedFill := func(name string, n int) string {
		return "fn " + name + "() -> i32 {\n" + strings.Repeat("\tlet x = \"aaaaaaaaaaaaaaaaaaaa\";\n", n) + "}\n"
	}

	fill := sizedFill("fill", 6)
	doc := "/// doc line one\n/// doc line two\n"
	target := "fn target() -> i32 {\n    42\n}\n"
	src := fill + "\n" + doc + target

	fSize := CountNws(fill)
	cSize := CountNws(doc)
	tSize := CountNws(target)
	maxSize := fSize + cSize
	if fSize > maxSize || fSize+cSize > maxSize || fSize+cSize+tSize <= maxSize || cSize+tSize > maxSize {
		t.Fatalf("setup error: fill=%d comment=%d target=%d for maxSize=%d", fSize, cSize, tSize, maxSize)
	}

	rootNode, scopeTree, language, err := parseSource("test.rs", src, types.ChunkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := ChunkCode(rootNode, src, scopeTree, language, types.ChunkOptions{MaxChunkSize: maxSize}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var targetChunk *types.Chunk
	for i := range chunks {
		if strings.Contains(chunks[i].Text, "fn target") {
			targetChunk = &chunks[i]
			break
		}
	}
	if targetChunk == nil {
		t.Fatal("failed to find a chunk containing fn target")
	}
	if !strings.Contains(targetChunk.Text, "/// doc line one") || !strings.Contains(targetChunk.Text, "/// doc line two") {
		t.Fatalf("rust doc comment split from its declaration:\nchunk contents:\n%s", targetChunk.Text)
	}
}
