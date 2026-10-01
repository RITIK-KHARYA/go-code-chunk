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
	cases := []struct {
		name, text, delimiter string
		want                  int
	}{
		{"paren", "func f(a, b int) int {", "{", 21},
		{"generic", "interface Box<T> {}", "{", 17},
		{"string", "const s = {\"}\"};", "{", 10},
		{"missing", "no delimiter here", "{", -1},
		{"negative-depth", "func f() ) {", "{", -1},
		{"le-not-generic", "a <= b {", "{", 7},
		{"dbl-lt-generic", "a << b {", "{", -1},
		{"py-colon-outside-params", "def f(a: int): {}", ":", 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := findBodyDelimiterPos(tc.text, tc.delimiter, types.LanguageGo); got != tc.want {
				t.Fatalf("findBodyDelimiterPos(%q, %q) = %d, want %d", tc.text, tc.delimiter, got, tc.want)
			}
		})
	}
}

// TestFindBodyDelimiterPosLanguage verifies the language-conditional handling
// of the single quote: in Rust, ' is a lifetime and must not be treated as a
// string delimiter; in JS/TS (and Go), ' is still tracked as a string
// delimiter so a '{' inside a single-quoted string is skipped.
func TestFindBodyDelimiterPosLanguage(t *testing.T) {
	cases := []struct {
		name, text, delimiter string
		language              types.Language
		want                  int
	}{
		// Rust: ' is a lifetime, so the body '{' is found even when a single
		// lifetime's closing ' sits after the '{' (the pre-fix bug returned -1).
		{"rust single lifetime", "Foo<'a>{", "{", types.LanguageRust, 7},
		{"rust two lifetimes", "Foo<'a, 'b>{", "{", types.LanguageRust, 11},
		{"rust generic no lifetime", "Foo<T>{", "{", types.LanguageRust, 6},
		{"rust fn lifetime text-mode", "f<'a>(x){", "{", types.LanguageRust, 8},
		// JS/TS: ' is a string delimiter, so a '{' inside a '...' string is
		// skipped and the real body '{' after it is found.
		{"js single-quote masks brace", "f('a{b'){", "{", types.LanguageJavaScript, 8},
		{"ts single-quote masks brace", "f('a{b'){", "{", types.LanguageTypeScript, 8},
		// Go: ' is tracked (rune/string literals), but no quoting interferes here.
		{"go paren delimiter", "func f(a, b int) int {", "{", types.LanguageGo, 21},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := findBodyDelimiterPos(tc.text, tc.delimiter, tc.language); got != tc.want {
				t.Fatalf("findBodyDelimiterPos(%q, %q, %s) = %d, want %d", tc.text, tc.delimiter, tc.language, got, tc.want)
			}
		})
	}
}

// TestRustLifetimeSignatures verifies the end-to-end signature extraction for
// Rust declarations containing lifetimes in their header.
func TestRustLifetimeSignatures(t *testing.T) {
	cases := []struct {
		name, src, pattern string
		et                 types.EntityType
		want               string
	}{
		{"struct single lifetime", "struct Foo<'a> { x: &'a i32 }\n",
			"(struct_item) @s", types.EntityTypeType, "struct Foo<'a>"},
		{"enum single lifetime", "enum Foo<'a> { A(&'a i32) }\n",
			"(enum_item) @s", types.EntityTypeEnum, "enum Foo<'a>"},
		{"struct two lifetimes", "struct Foo<'a, 'b> { x: &'a i32 }\n",
			"(struct_item) @s", types.EntityTypeType, "struct Foo<'a, 'b>"},
		{"struct no lifetime", "struct Foo<T> { x: T }\n",
			"(struct_item) @s", types.EntityTypeType, "struct Foo<T>"},
		{"struct multi-line single lifetime", "struct Foo<'a> {\n    x: &'a i32,\n}\n",
			"(struct_item) @s", types.EntityTypeType, "struct Foo<'a>"},
		{"enum multi-line single lifetime", "enum Color<'a> {\n    Red(&'a str),\n    Green,\n}\n",
			"(enum_item) @s", types.EntityTypeEnum, "enum Color<'a>"},
		{"type alias no brace", "type Foo = u32;\n",
			"(type_item) @t", types.EntityTypeType, "type Foo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, node := firstCaptured(t, "rust", tc.src, tc.pattern)
			defer tree.Release()
			got := ExtractSignature(node, tc.et, types.LanguageRust, tc.src)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
