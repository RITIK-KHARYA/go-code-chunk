package parser

import (
	"strings"
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
		if GetLanguage(lang) == nil {
			t.Errorf("GetLanguage(%q) = nil, want non-nil", lang)
		}
	}
	if GetLanguage("brainfuck") != nil {
		t.Error("GetLanguage(brainfuck) = non-nil, want nil")
	}
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