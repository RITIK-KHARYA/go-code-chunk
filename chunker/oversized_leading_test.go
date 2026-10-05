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

// chunkSourceLines returns the number of source lines a chunk's text spans,
// ignoring a single trailing newline so a one-line chunk like "x\n" reports 1.
func chunkSourceLines(text string) int {
	return strings.Count(strings.TrimRight(text, "\n"), "\n") + 1
}

// assertNoMultiLineOversizedChunk fails the test if any chunk whose NWS exceeds
// maxSize spans more than one source line. A single oversized source line is
// the documented unsplittable-line exception (see
// TestSingleUnsplittableOversizedLineIsAllowedExceeds); a multi-source-line
// oversized chunk is a real MaxChunkSize violation that the chunker must never
// emit.
func assertNoMultiLineOversizedChunk(t *testing.T, chunks []types.Chunk, maxSize int) {
	t.Helper()
	multi := 0
	for i, c := range chunks {
		nws := CountNws(c.Text)
		lines := chunkSourceLines(c.Text)
		t.Logf("chunk %d NWS=%d/%d srcLines=%d text=%q", i, nws, maxSize, lines, c.Text)
		if nws > maxSize && lines > 1 {
			multi++
			t.Errorf("chunk %d is a multi-source-line oversized chunk: NWS=%d maxSize=%d lines=%d text=%q",
				i, nws, maxSize, lines, c.Text)
		}
	}
	if multi != 0 {
		t.Fatalf("expected no multi-source-line oversized chunks, got %d", multi)
	}
}

// firstLineContaining returns the 0-indexed source line number of the first
// occurrence of substr in src, or -1 if absent.
func firstLineContaining(src, substr string) int {
	idx := strings.Index(src, substr)
	if idx < 0 {
		return -1
	}
	return strings.Count(src[:idx], "\n")
}

// TestAbsorbOversizeSameLineStringRejectsMultiLineOversizedChunk is the
// end-to-end regression test for the absorbIntoLastSplit over-extension bug.
// An oversized leading block comment line-split into partial windows must NOT
// absorb a following declaration whose first sub-window ends mid-line (here the
// `var`/`x =`/`"` header of `var x = "<huge string>"`), because
// rebuildFromLineRanges would then rebuild the WHOLE declaration line —
// including the oversized same-line string that belongs to a later window —
// producing a multi-source-line chunk that exceeds MaxChunkSize and overlaps
// the string's own window. The fix guards the absorb on
// trailingLineIsWhitespace so the absorb is skipped when non-whitespace
// sibling content follows the first sub-window's last node.
func TestAbsorbOversizeSameLineStringRejectsMultiLineOversizedChunk(t *testing.T) {
	const maxSize = 200
	comment := oversizedBlockComment(10, 6) // ~244 NWS, exceeds maxSize
	big := "var x = \"" + strings.Repeat("word ", 60) + "\"\n"
	src := "package p\n\n" + comment + "\n\n" + big

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
	assertNoMultiLineOversizedChunk(t, chunks, maxSize)
}

// TestAbsorbedCommentChunkDoesNotOverlapSameLineSibling pins the specific
// overlap the bug introduced. Before the fix, absorbIntoLastSplit extended the
// comment chunk's last LineRange.End to the var declaration's line, so the
// whole var line (including the oversized string) appeared in BOTH the
// absorbed comment chunk and the string's own window. After the fix the
// comment's last line-split chunk must end strictly before the var line and
// must not contain any of the declaration's text.
func TestAbsorbedCommentChunkDoesNotOverlapSameLineSibling(t *testing.T) {
	const maxSize = 200
	comment := oversizedBlockComment(10, 6)
	big := "var x = \"" + strings.Repeat("word ", 60) + "\"\n"
	src := "package p\n\n" + comment + "\n\n" + big

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

	varLine := firstLineContaining(src, "var x =")
	if varLine < 0 {
		t.Fatal("could not locate `var x =` in source")
	}

	var commentChunk *types.Chunk
	for i := range chunks {
		if strings.Contains(chunks[i].Text, "*/") {
			commentChunk = &chunks[i]
			break
		}
	}
	if commentChunk == nil {
		t.Fatal("no chunk containing the comment close */")
	}
	if commentChunk.LineRange.End >= varLine {
		t.Errorf("comment chunk LineRange.End=%d should be < var line %d; text=%q",
			commentChunk.LineRange.End, varLine, commentChunk.Text)
	}
	if strings.Contains(commentChunk.Text, "var x =") {
		t.Errorf("comment chunk leaked the var declaration line into the absorbed chunk: %q",
			commentChunk.Text)
	}
}

// TestAbsorbOversizeSameLineStringJavaScript confirms the language-agnostic
// guard holds across grammars: an oversized JS block comment preceding a
// single-line const whose initializer is a huge same-line string literal must
// not yield a multi-source-line oversized absorbed chunk.
func TestAbsorbOversizeSameLineStringJavaScript(t *testing.T) {
	const maxSize = 200
	comment := oversizedBlockComment(10, 6)
	big := "const x = \"" + strings.Repeat("word ", 60) + "\";\n"
	src := comment + "\n\n" + big

	rootNode, scopeTree, language, err := parseSource("test.js", src, types.ChunkOptions{
		MaxChunkSize: maxSize, ContextMode: types.ContextModeNone, Language: types.LanguageJavaScript,
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
	assertNoMultiLineOversizedChunk(t, chunks, maxSize)
}

// TestAbsorbStillHappensWhenDeclarationEndsAtEndOfLine pins the non-regression
// half of the fix: when the declaration's first sub-window ends at the end of
// its line (only whitespace follows, up to the newline), the absorb MUST still
// fire so the declaration stays glued to the comment's last line-split chunk.
// This is the case the commit's existing coverage exercised; the new guard must
// not regress it. Verified end-to-end via ChunkCode.
func TestAbsorbStillHappensWhenDeclarationEndsAtEndOfLine(t *testing.T) {
	const maxSize = 200
	comment := oversizedBlockComment(15, 8) // 484 NWS
	src := "package p\n\n" + comment + "\n\nvar x = 1\n"

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

	// The absorb must have fired: exactly one chunk contains BOTH the comment
	// close "*/" and the declaration "var x = 1".
	var attached *types.Chunk
	for i := range chunks {
		if strings.Contains(chunks[i].Text, "*/") && strings.Contains(chunks[i].Text, "var x = 1") {
			attached = &chunks[i]
			break
		}
	}
	if attached == nil {
		t.Fatalf("declaration not absorbed into the comment's last split; chunks:\n%s",
			strings.Join(chunkTexts(chunks), "\n---\n"))
	}
	// The absorb's Size sum is accurate (declaration's last node is end-of-line),
	// so the absorbed chunk must respect MaxChunkSize.
	if nws := CountNws(attached.Text); nws > maxSize {
		t.Errorf("absorbed chunk NWS=%d exceeds maxSize=%d; text=%q", nws, maxSize, attached.Text)
	}
	assertNoMultiLineOversizedChunk(t, chunks, maxSize)
}

// TestTrailingLineIsWhitespace exercises the guard helper directly. It returns
// true only when every character from the node's end byte up to (but not
// including) the next newline is whitespace.
func TestTrailingLineIsWhitespace(t *testing.T) {
	// nil node: nothing trailing, vacuously safe.
	if !trailingLineIsWhitespace(nil, "anything\n") {
		t.Error("nil node should be vacuously whitespace-only")
	}

	// var_spec `x = 1` ends right before a newline (end of line): safe.
	// identifier `a` inside call(a, b) ends mid-line followed by ", b)": unsafe.
	src := "package p\n\nvar x = 1\nvar y = call(a, b)\n"
	root, lang := parseForTest(t, "go", src)
	var varSpecX, argA types.SyntaxNode
	var walk func(n types.SyntaxNode)
	walk = func(n types.SyntaxNode) {
		nn := tsNode(n)
		if nn.Type(lang) == "var_spec" && varSpecX == nil {
			if txt := src[nn.StartByte():nn.EndByte()]; txt == "x = 1" {
				varSpecX = n
			}
		}
		if nn.Type(lang) == "identifier" && argA == nil {
			if txt := src[nn.StartByte():nn.EndByte()]; txt == "a" {
				argA = n
			}
		}
		for i := 0; i < nn.ChildCount(); i++ {
			if c := nn.Child(i); c != nil {
				walk(types.SyntaxNode(c))
			}
		}
	}
	walk(root)
	if varSpecX == nil {
		t.Fatal("var_spec `x = 1` not found")
	}
	if argA == nil {
		t.Fatal("identifier `a` not found")
	}
	if !trailingLineIsWhitespace(varSpecX, src) {
		t.Errorf("var_spec ending at end of line should be whitespace-only")
	}
	if trailingLineIsWhitespace(argA, src) {
		t.Errorf("identifier `a` followed by `, b)` should NOT be whitespace-only")
	}

	// Node at EOF with no trailing newline: trailing rest is empty, safe.
	srcEOF := "package p"
	rootEOF, _ := parseForTest(t, "go", srcEOF)
	if !trailingLineIsWhitespace(rootEOF, srcEOF) {
		t.Errorf("root at EOF with no trailing newline should be whitespace-only")
	}
}
