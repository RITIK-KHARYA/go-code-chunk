package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// oversizedBlockComment builds a Go /* ... */ block comment whose body is
// `lines` lines of `words` repetitions of "word " (4 NWS each), yielding a
// multi-line comment of known NWS per line. The first line carries the "/* "
// prefix and the last line carries the " */" suffix, so total NWS is
// 4*words*lines + 4.
func oversizedBlockComment(words, lines int) string {
	body := make([]string, lines)
	for i := range body {
		body[i] = strings.Repeat("word ", words)
	}
	return "/* " + strings.Join(body, "\n") + " */"
}

// assertNoChunkExceeds fails the test if any chunk's NWS exceeds maxSize. It is
// the core contract the chunker promises (modulo genuinely unsplittable single
// lines, handled by TestSingleUnsplittableOversizedLineIsAllowedExceeds).
func assertNoChunkExceeds(t *testing.T, chunks []types.Chunk, maxSize int) {
	t.Helper()
	for i, c := range chunks {
		nws := CountNws(c.Text)
		t.Logf("chunk %d NWS=%d (maxSize=%d) text=%q", i, nws, maxSize, c.Text)
		if nws > maxSize {
			t.Errorf("chunk %d NWS=%d exceeds maxSize=%d; text=%q", i, nws, maxSize, c.Text)
		}
	}
}

// chunkTexts returns a slice of each chunk's text.
func chunkTexts(chunks []types.Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = c.Text
	}
	return out
}

// containsNode reports whether nodes contains target by tree-sitter pointer
// identity.
func containsNode(nodes []types.SyntaxNode, target types.SyntaxNode) bool {
	tgt := tsNode(target)
	for _, n := range nodes {
		if tsNode(n) == tgt {
			return true
		}
	}
	return false
}

// TestSplitOversizedGroupLineSplitsLeadingComment is a white-box test of the
// internal window structure: a group [oversizedComment, declaration] must
// produce multiple partial windows for the comment, with the declaration
// absorbed into the last one (so it stays attached to the comment's final
// line-split chunk) instead of being reattached wholesale to the first
// sub-window.
func TestSplitOversizedGroupLineSplitsLeadingComment(t *testing.T) {
	const maxSize = 200
	comment := oversizedBlockComment(15, 8) // 484 NWS
	src := "package p\n\n" + comment + "\n\nvar x = 1\n"

	rootNode, _, _, err := parseSource("test.go", src, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone, Language: types.LanguageGo,
	})
	if err != nil {
		t.Fatal(err)
	}
	cumsum := PreprocessNwsCumsum(src)
	lang, _ := parser.GetLanguage("go")
	groups := groupWithLeadingComments(childrenOf(rootNode), types.LanguageGo, lang)

	// Find the [comment, var_declaration] group.
	var group []types.SyntaxNode
	for _, g := range groups {
		if len(g) == 2 && isCommentNode(g[0], types.LanguageGo, lang) {
			group = g
			break
		}
	}
	if group == nil {
		t.Fatal("no [comment, decl] group found")
	}
	windows := splitOversizedGroup(group, src, cumsum, maxSize, types.LanguageGo)
	if len(windows) < 2 {
		t.Fatalf("expected the oversized comment to be split into >=2 windows, got %d", len(windows))
	}

	// Every emitted window must have Size <= maxSize (no oversized glue).
	for i, w := range windows {
		if w.Size > maxSize {
			t.Errorf("window %d Size=%d exceeds maxSize %d", i, w.Size, maxSize)
		}
	}

	// The first window must be a partial (line-split) window carrying the
	// comment node; the LAST window must be partial and, when reconstructed,
	// contain BOTH the comment's final lines and the declaration (the
	// declaration absorbed into the last split so it stays glued to the
	// comment's final line-split chunk).
	first := windows[0]
	if !isPartialWindow(first) || len(first.LineRanges) == 0 {
		t.Errorf("first window should be partial with LineRanges, got IsPartialNode=%v LineRanges=%v",
			first.IsPartialNode, first.LineRanges)
	}
	if !containsNode(first.Nodes, group[0]) {
		t.Errorf("first window should carry the comment node")
	}
	last := windows[len(windows)-1]
	if !isPartialWindow(last) {
		t.Errorf("last window should be partial so the declaration is rebuilt via LineRanges, got IsPartialNode=%v",
			last.IsPartialNode)
	}
	if !containsNode(last.Nodes, group[0]) {
		t.Errorf("last window should carry the comment node so the comment stays attached to the declaration")
	}
	lastText := RebuildText(last, src).Text
	if !strings.Contains(lastText, "word") {
		t.Errorf("last window text should contain the comment's final lines; got %q", lastText)
	}
	if !strings.Contains(lastText, "var x = 1") {
		t.Errorf("last window text should contain the declaration (absorbed into the last split); got %q", lastText)
	}
}

// TestCanMergeRejectsPartialWithNormal pins the guard that prevents merging a
// partial (line-split) window with a normal (byte-slice) window. Without it,
// merging the two yields a window whose IsPartialNode is true but whose
// LineRanges is empty, which RebuildText reconstructs incorrectly (it would
// emit the whole partial node, producing an oversized chunk).
func TestCanMergeRejectsPartialWithNormal(t *testing.T) {
	partial := types.ASTWindow{Size: 30, IsPartialNode: new(true), LineRanges: []types.LineRange{{Start: 0, End: 1}}}
	normal := types.ASTWindow{Size: 30}

	// Both partial: merge allowed when sizes fit.
	if !CanMerge(partial, partial, 100) {
		t.Error("expected partial+partial merge allowed when sizes fit")
	}
	// Both normal: merge allowed when sizes fit.
	if !CanMerge(normal, normal, 100) {
		t.Error("expected normal+normal merge allowed when sizes fit")
	}
	// Mixed: merge rejected even when sizes fit.
	if CanMerge(partial, normal, 100) {
		t.Error("expected partial+normal merge rejected to keep reconstruction sound")
	}
	if CanMerge(normal, partial, 100) {
		t.Error("expected normal+partial merge rejected to keep reconstruction sound")
	}
	// Size overflow still rejected regardless of partialness.
	if CanMerge(normal, normal, 50) {
		t.Error("expected normal+normal merge rejected when sizes overflow")
	}
}

// TestSingleUnsplittableOversizedLineIsAllowedExceeds pins the documented
// fallback: a single oversized line that cannot be split at a line boundary
// exceeds MaxChunkSize (unavoidable), but the following declaration is NOT
// glued onto it (which would only inflate an already-oversized chunk).
func TestSingleUnsplittableOversizedLineIsAllowedExceeds(t *testing.T) {
	const maxSize = 200
	// A single-line /* ... */ comment far above maxSize: no internal newline
	// to split at, so splitOversizedLeafByLines emits one oversized window.
	src := "package p\n\n/* " + strings.Repeat("word ", 125) + " */\n\nvar x = 1\n"

	rootNode, scopeTree, language, err := parseSource("test.go", src, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone, Language: types.LanguageGo,
	})
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := ChunkCode(rootNode, src, scopeTree, language, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Exactly one chunk is allowed to exceed maxSize (the unsplittable
	// single-line comment). Find it.
	overCount := 0
	var oversizeChunk *types.Chunk
	for i := range chunks {
		if nws := CountNws(chunks[i].Text); nws > maxSize {
			overCount++
			oversizeChunk = &chunks[i]
		}
	}
	if overCount != 1 {
		t.Fatalf("expected exactly one oversized chunk (the unsplittable comment), got %d; chunks:\n%s",
			overCount, strings.Join(chunkTexts(chunks), "\n---\n"))
	}
	if !strings.Contains(oversizeChunk.Text, "/*") {
		t.Errorf("oversized chunk should be the comment; got %q", oversizeChunk.Text)
	}
	// The declaration must be its OWN chunk, not glued onto the oversized
	// comment chunk (which would inflate it further).
	if strings.Contains(oversizeChunk.Text, "var x = 1") {
		t.Errorf("declaration glued onto the oversized unsplittable comment chunk: %q", oversizeChunk.Text)
	}
	var hasDecl bool
	for _, c := range chunks {
		if strings.Contains(c.Text, "var x = 1") {
			hasDecl = true
			if dNws := CountNws(c.Text); dNws > maxSize {
				t.Errorf("declaration chunk NWS=%d exceeds maxSize=%d", dNws, maxSize)
			}
		}
	}
	if !hasDecl {
		t.Errorf("declaration missing from chunks:\n%s", strings.Join(chunkTexts(chunks), "\n---\n"))
	}
}

// TestRustOversizedDocComment covers a Rust /// line-comment run glued to a
// fn item, exercising the cross-language comment-node-type table: the fix
// routes oversized leading leaves through splitOversizedLeafByLines regardless
// of the grammar's comment node type name.
func TestRustOversizedDocComment(t *testing.T) {
	const maxSize = 200
	// 13 /// lines of ~16 NWS each = ~208 NWS, just over maxSize, and each
	// /// line is its own line_comment node.
	var doc strings.Builder
	for i := 0; i < 13; i++ {
		doc.WriteString("/// doc line word word word\n") // ~17 NWS/line
	}
	src := doc.String() + "\nfn alpha() {\n    42\n}\n"

	rootNode, scopeTree, language, err := parseSource("test.rs", src, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone, Language: types.LanguageRust,
	})
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := ChunkCode(rootNode, src, scopeTree, language, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNoChunkExceeds(t, chunks, maxSize)
}
