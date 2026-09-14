package extract

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// firstCaptured parses src and returns the tree (kept alive for node reads)
// plus the node for the first capture of pattern.
func firstCaptured(t *testing.T, language, src, pattern string) (*gotreesitter.Tree, *gotreesitter.Node) {
	t.Helper()
	p, err := parser.NewParser(language)
	if err != nil {
		t.Fatalf("NewParser(%q): %v", language, err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", language, err)
	}
	q, err := gotreesitter.NewQuery(pattern, p.Language())
	if err != nil {
		tree.Release()
		t.Fatalf("NewQuery(%q): %v", pattern, err)
	}
	matches := q.ExecuteNode(tree.RootNode(), p.Language(), []byte(src))
	if len(matches) == 0 || len(matches[0].Captures) == 0 {
		tree.Release()
		t.Fatalf("no captures for pattern %q", pattern)
	}
	return tree, matches[0].Captures[0].Node
}

func TestExtractName(t *testing.T) {
	cases := []struct {
		name, lang, src, pattern, want string
	}{
		{
			name:    "go function",
			lang:    "go",
			src:     "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}\n",
			pattern: "(function_declaration) @f",
			want:    "add",
		},
		{
			name:    "typescript function",
			lang:    "typescript",
			src:     "function greet<T>(name: string): string {\n\treturn name\n}\n",
			pattern: "(function_declaration) @f",
			want:    "greet",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, node := firstCaptured(t, tc.lang, tc.src, tc.pattern)
			defer tree.Release()

			got, ok := ExtractName(node, types.Language(tc.lang), tc.src)
			if !ok || got != tc.want {
				t.Fatalf("ExtractName = %q, %v; want %q, true", got, ok, tc.want)
			}
		})
	}
}

func TestExtractNameNotFound(t *testing.T) {
	// A bare block has no name-like child.
	src := "package main\n\nfunc main() {\n\tprint()\n}\n"
	tree, node := firstCaptured(t, "go", src, "(block) @b")
	defer tree.Release()

	if got, ok := ExtractName(node, types.LanguageGo, src); ok {
		t.Fatalf("ExtractName(block) = %q, true; want found=false", got)
	}
}

func TestExtractSignature(t *testing.T) {
	cases := []struct {
		name, lang, src, pattern string
		entityType               types.EntityType
		want                     string
	}{
		{
			name:       "go function",
			lang:       "go",
			src:        "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}\n",
			pattern:    "(function_declaration) @f",
			entityType: types.EntityTypeFunction,
			want:       "func add(a, b int) int",
		},
		{
			name:       "typescript function with generics",
			lang:       "typescript",
			src:        "function greet<T>(name: string): string {\n\treturn name\n}\n",
			pattern:    "(function_declaration) @f",
			entityType: types.EntityTypeFunction,
			want:       "function greet<T>(name: string): string",
		},
		{
			name:       "typescript class",
			lang:       "typescript",
			src:        "class Point {\n\tx = 0\n}\n",
			pattern:    "(class_declaration) @c",
			entityType: types.EntityTypeClass,
			want:       "class Point",
		},
		{
			name:       "python class strips colon",
			lang:       "python",
			src:        "class Foo:\n\tpass\n",
			pattern:    "(class_definition) @c",
			entityType: types.EntityTypeClass,
			want:       "class Foo",
		},
		{
			name:       "python method strips colon",
			lang:       "python",
			src:        "def bar(self):\n\treturn 1\n",
			pattern:    "(function_definition) @f",
			entityType: types.EntityTypeMethod,
			want:       "def bar(self)",
		},
		{
			name:       "rust type alias",
			lang:       "rust",
			src:        "type Foo = u32;\n",
			pattern:    "(type_item) @t",
			entityType: types.EntityTypeType,
			want:       "type Foo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, node := firstCaptured(t, tc.lang, tc.src, tc.pattern)
			defer tree.Release()

			got := ExtractSignature(node, tc.entityType, types.Language(tc.lang), tc.src)
			if got != tc.want {
				t.Fatalf("ExtractSignature = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractSignatureUnsupportedLanguageDegrades(t *testing.T) {
	// Signature extraction must never panic for an unsupported grammar.
	// With no known grammar or body delimiter it falls back to the full
	// (cleaned) node text.
	src := "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}\n"
	tree, node := firstCaptured(t, "go", src, "(function_declaration) @f")
	defer tree.Release()

	got := ExtractSignature(node, types.EntityTypeFunction, types.Language("brainfuck"), src)
	if want := "func add(a, b int) int { return a + b }"; got != want {
		t.Fatalf("text-only fallback = %q, want %q", got, want)
	}
}

func TestExtractImportSource(t *testing.T) {
	cases := []struct {
		name, lang, src, pattern, want string
	}{
		{
			name:    "go single import",
			lang:    "go",
			src:     "package main\n\nimport \"fmt\"\n",
			pattern: "(import_declaration) @i",
			want:    "fmt",
		},
		{
			name:    "go import block",
			lang:    "go",
			src:     "package main\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\n",
			pattern: "(import_declaration) @i",
			want:    "os",
		},
		{
			name:    "python from import",
			lang:    "python",
			src:     "from os import path\n",
			pattern: "(import_from_statement) @i",
			want:    "os",
		},
		{
			name:    "python import",
			lang:    "python",
			src:     "import numpy as np\n",
			pattern: "(import_statement) @i",
			want:    "numpy",
		},
		{
			name:    "rust use path",
			lang:    "rust",
			src:     "use std::collections::{HashMap, HashSet};\n",
			pattern: "(use_declaration) @u",
			want:    "std::collections",
		},
		{
			name:    "typescript import",
			lang:    "typescript",
			src:     "import { readFile } from \"node:fs\";\n",
			pattern: "(import_statement) @i",
			want:    "node:fs",
		},
		{
			name:    "java import",
			lang:    "java",
			src:     "import java.util.List;\n",
			pattern: "(import_declaration) @i",
			want:    "java.util.List",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, node := firstCaptured(t, tc.lang, tc.src, tc.pattern)
			defer tree.Release()

			got, ok := ExtractImportSource(node, types.Language(tc.lang), tc.src)
			if !ok || got != tc.want {
				t.Fatalf("ExtractImportSource = %q, %v; want %q, true", got, ok, tc.want)
			}
		})
	}
}

func TestGetBodyDelimiter(t *testing.T) {
	cases := map[types.Language]string{
		types.LanguageTypeScript: "{",
		types.LanguagePython:     ":",
		types.LanguageGo:         "{",
		types.LanguageJava:       "{",
	}
	for lang, want := range cases {
		if got := GetBodyDelimiter(lang); got != want {
			t.Errorf("GetBodyDelimiter(%s) = %q, want %q", lang, got, want)
		}
	}
}

func TestFindBodyDelimiterPos(t *testing.T) {
	// delimiter inside a parameter list must not match
	if got := findBodyDelimiterPos("func f(a, b int) int {", "{"); got != 21 {
		t.Fatalf("paren case = %d, want 21", got)
	}
	// delimiter inside generics must not match
	if got := findBodyDelimiterPos("interface Box<T> {}", "{"); got != 17 {
		t.Fatalf("generic case = %d, want 17", got)
	}
	// string literal containing a brace must not match
	if got := findBodyDelimiterPos("const s = {\"}\"};", "{"); got != 10 {
		t.Fatalf("string case = %d, want 10", got)
	}
	if got := findBodyDelimiterPos("no delimiter here", "{"); got != -1 {
		t.Fatalf("missing case = %d, want -1", got)
	}
}
