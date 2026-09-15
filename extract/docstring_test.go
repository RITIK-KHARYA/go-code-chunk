package extract

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

func TestExtractDocstring(t *testing.T) {
	cases := []struct {
		name, lang, src, pattern, want string
	}{
		{
			name:    "go line comment",
			lang:    "go",
			src:     "package main\n\n// adds two numbers\nfunc add(a, b int) int {\n\treturn a + b\n}\n",
			pattern: "(function_declaration) @f",
			want:    "adds two numbers",
		},
		{
			name:    "rust multi-line doc",
			lang:    "rust",
			src:     "/// Adds two numbers\n/// Returns the sum.\nfn add(a: i32, b: i32) -> i32 {\n\ta + b\n}\n",
			pattern: "(function_item) @f",
			want:    "Adds two numbers\nReturns the sum.",
		},
		{
			name:    "typescript jsdoc",
			lang:    "typescript",
			src:     "/** Sums two numbers. */\nfunction sum(a: number, b: number) {\n\treturn a + b\n}\n",
			pattern: "(function_declaration) @f",
			want:    "Sums two numbers.",
		},
		{
			name:    "java javadoc",
			lang:    "java",
			src:     "public class Test {\n\t/** Adds two numbers. */\n\tpublic int add(int a, int b) {\n\t\treturn a + b\n\t}\n}\n",
			pattern: "(method_declaration) @m",
			want:    "Adds two numbers.",
		},
		{
			name:    "python docstring",
			lang:    "python",
			src:     "def foo():\n\t\"\"\"Docstring here.\"\"\"\n\treturn 1\n",
			pattern: "(function_definition) @f",
			want:    "Docstring here.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, node := firstCaptured(t, tc.lang, tc.src, tc.pattern)
			defer tree.Release()

			got, ok := ExtractDocstring(node, types.Language(tc.lang), tc.src)
			if !ok || got != tc.want {
				t.Fatalf("ExtractDocstring = %q, %v; want %q, true", got, ok, tc.want)
			}
		})
	}
}

func TestExtractDocstringNotFound(t *testing.T) {
	src := "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}\n"
	tree, node := firstCaptured(t, "go", src, "(function_declaration) @f")
	defer tree.Release()

	if got, ok := ExtractDocstring(node, types.LanguageGo, src); ok {
		t.Fatalf("ExtractDocstring = %q, true; want not found", got)
	}
}

func TestIsDocComment(t *testing.T) {
	cases := []struct {
		text     string
		language types.Language
		want     bool
	}{
		{"/** Sums. */", types.LanguageTypeScript, true},
		{"/** Sums. */", types.LanguageJava, true},
		{"/*** not doc */", types.LanguageTypeScript, false},
		{"/* plain */", types.LanguageTypeScript, false},
		{"/// Adds", types.LanguageRust, true},
		{"//! Inner", types.LanguageRust, true},
		{"// sometimes", types.LanguageRust, false},
		{"// adds", types.LanguageGo, true},
		{`"""doc"""`, types.LanguagePython, true},
		{`r'''raw doc'''`, types.LanguagePython, true},
		{"# not doc", types.LanguagePython, false},
	}

	for _, tc := range cases {
		if got := IsDocComment(tc.text, tc.language); got != tc.want {
			t.Errorf("IsDocComment(%q, %s) = %v, want %v", tc.text, tc.language, got, tc.want)
		}
	}
}

func TestParseDocstring(t *testing.T) {
	cases := []struct {
		name, text string
		language   types.Language
		want       string
	}{
		{
			name:     "jsdoc multi-line",
			text:     "/**\n * Sums numbers.\n * Takes two ints.\n */",
			language: types.LanguageTypeScript,
			want:     "Sums numbers.\nTakes two ints.",
		},
		{
			name:     "python dedent",
			text:     "'''\n\tfoo\n\tbar\n'''",
			language: types.LanguagePython,
			want:     "foo\nbar",
		},
		{
			name:     "python raw docstring",
			text:     `r"""raw\ncontent"""`,
			language: types.LanguagePython,
			want:     `raw\ncontent`,
		},
		{
			name:     "rust inner doc",
			text:     "//! first\n//! second",
			language: types.LanguageRust,
			want:     "first\nsecond",
		},
		{
			name:     "go comment",
			text:     "// first\n//second",
			language: types.LanguageGo,
			want:     "first\nsecond",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseDocstring(tc.text, tc.language); got != tc.want {
				t.Fatalf("ParseDocstring = %q, want %q", got, tc.want)
			}
		})
	}
}
