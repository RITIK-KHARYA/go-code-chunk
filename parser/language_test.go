package parser

import (
	"strings"
	"sync"
	"testing"

	"github.com/odvcencio/gotreesitter"
)

func TestParserParse(t *testing.T) {
	p, err := NewParser("go")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}

	src := []byte(`package main

func add(a, b int) int {
	return a + b
}
`)
	tree, err := p.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Release()

	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("root has error: %s", root.SExpr(p.Language()))
	}

	q, err := gotreesitter.NewQuery(`(function_declaration name: (identifier) @name)`, p.Language())
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}
	matches := q.ExecuteNode(root, p.Language(), src)
	if len(matches) == 0 {
		t.Fatalf("expected at least one function match")
	}
	if got := matches[0].Captures[0].Text(src); got != "add" {
		t.Fatalf("captured name = %q, want %q", got, "add")
	}
}

func TestDetectLanguage(t *testing.T) {
	cases := map[string]string{
		"main.go":     "go",
		"app.tsx":     "typescript",
		"app.ts":      "typescript",
		"index.js":    "javascript",
		"main.py":     "python",
		"lib.rs":      "rust",
		"Main.java":   "java",
		"unknown.xyz": "",
	}
	for path, want := range cases {
		if got := DetectLanguage(path); got != want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestGetLanguageSupported(t *testing.T) {
	for _, lang := range []string{"typescript", "javascript", "python", "rust", "go", "java"} {
		l, err := GetLanguage(lang)
		if err != nil {
			t.Errorf("GetLanguage(%q) error = %v, want nil", lang, err)
		}
		if l == nil {
			t.Errorf("GetLanguage(%q) = nil, want non-nil", lang)
		}
	}

	l, err := GetLanguage("brainfuck")
	if err != nil {
		t.Errorf("GetLanguage(brainfuck) error = %v, want nil", err)
	}
	if l != nil {
		t.Errorf("GetLanguage(brainfuck) = non-nil, want nil")
	}
}

func TestGetLanguageConcurrent(t *testing.T) {
	ClearCache()
	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for _, lang := range []string{"go", "rust", "python", "java", "javascript", "typescript"} {
				if _, err := GetLanguage(lang); err != nil {
					t.Errorf("concurrent GetLanguage(%q) error = %v", lang, err)
				}
			}
			if l, _ := GetLanguage("brainfuck"); l != nil {
				t.Error("concurrent GetLanguage(brainfuck) = non-nil, want nil")
			}
		}()
	}
	wg.Wait()
	ClearCache()
}

func TestUnsupportedLanguage(t *testing.T) {
	_, err := NewParser("brainfuck")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("unexpected error: %v", err)
	}
}