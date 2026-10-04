package chunker

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

func TestReproPartialMergeBug(t *testing.T) {
	const maxSize = 200
	lines := make([]string, 8)
	for i := range lines {
		lines[i] = strings.Repeat("word ", 12+i) // 48..76 NWS per line
	}
	src := "package p\n\n/* " + strings.Join(lines, "\n") + " */\n\nvar x = 1\n"

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
	for i, c := range chunks {
		nws := CountNws(c.Text)
		t.Logf("chunk %d NWS=%d (maxSize=%d) text=%q", i, nws, maxSize, c.Text)
		if nws > maxSize {
			t.Errorf("chunk %d NWS=%d exceeds maxSize=%d; text=%q", i, nws, maxSize, c.Text)
		}
	}
}
