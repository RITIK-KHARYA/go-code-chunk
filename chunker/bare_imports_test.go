package chunker

import (
	"os"
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/scope"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// TestChunkContextBareNameImports is the non-Go verification that the import
// matcher works off the bound identifier, not the qualifier-substring
// heuristic. TS/JS named imports (import { foo }) and Python from-imports
// (from m import foo) are used as bare names, so they could never have matched
// the old "pkg." strategy.
func TestChunkContextBareNameImports(t *testing.T) {
	cases := []struct {
		name        string
		file        string
		language    types.Language
		wantImports []string
	}{
		{
			name:        "typescript named imports",
			file:        "../testdata/bare_imports.ts",
			language:    types.LanguageTypeScript,
			wantImports: []string{"dflt", "helper", "fmt", "utils"},
		},
		{
			name:        "python from and module imports",
			file:        "../testdata/bare_imports.py",
			language:    types.LanguagePython,
			wantImports: []string{"h", "scale", "os", "os.path"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			code, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			opts := types.ChunkOptions{Language: tc.language}
			rootNode, scopeTree, language, err := parseSource(tc.file, string(code), opts)
			if err != nil {
				t.Fatalf("parseSource: %v", err)
			}
			chunks, err := ChunkCode(rootNode, string(code), scopeTree, language, opts, nil)
			if err != nil {
				t.Fatalf("ChunkCode: %v", err)
			}

			run := findEntityName(t, scopeTree, "run")

			var covered types.Chunk
			found := false
			for _, ch := range chunks {
				if scope.RangeContains(ch.ByteRange, run.ByteRange) {
					covered = ch
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("no chunk fully covers %q %+v", run.Name, run.ByteRange)
			}

			ent := entityInfoInChunk(t, covered, run.Name)
			if len(ent.Imports) != len(tc.wantImports) {
				t.Fatalf("run imports = %+v, want %v", ent.Imports, tc.wantImports)
			}
			for i, want := range tc.wantImports {
				if ent.Imports[i].Name != want {
					t.Fatalf("run imports[%d] = %q, want %q (full: %+v)", i, ent.Imports[i].Name, want, ent.Imports)
				}
			}

			wantLine := "// Imports used: " + strings.Join(tc.wantImports, ", ")
			if !strings.Contains(covered.ContextualizedText, wantLine) {
				t.Errorf("contextualized text missing %q:\n%s", wantLine, covered.ContextualizedText)
			}
		})
	}
}
