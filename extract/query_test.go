package extract

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

func parseQueryTree(t *testing.T, language, src string) *gotreesitter.Tree {
	t.Helper()
	p, err := parser.NewParser(language)
	if err != nil {
		t.Fatalf("NewParser(%q): %v", language, err)
	}
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", language, err)
	}
	return tree
}

func captureNames(t *testing.T, tree *gotreesitter.Tree, result QueryResult) map[string]int {
	t.Helper()
	src := tree.Source()
	names := map[string]int{}
	for _, c := range result.Captures {
		if c.Name != "name" {
			continue
		}
		if c.Node == nil {
			continue
		}
		names[c.Node.Text(src)]++
	}
	return names
}

func TestQueryPatternsCompile(t *testing.T) {
	for language := range QueryPatterns {
		q, err := LoadQuery(language)
		if err != nil {
			t.Fatalf("LoadQuery(%s): %v", language, err)
		}
		if q == nil {
			t.Fatalf("LoadQuery(%s): nil query without error", language)
		}
	}
}

func TestHasQueryForLanguage(t *testing.T) {
	for language := range QueryPatterns {
		if !HasQueryForLanguage(language) {
			t.Errorf("HasQueryForLanguage(%s) = false, want true", language)
		}
	}
	if HasQueryForLanguage(types.Language("brainfuck")) {
		t.Error("HasQueryForLanguage(brainfuck) = true, want false")
	}
}

func TestLoadQueryUnknownLanguage(t *testing.T) {
	q, err := LoadQuery(types.Language("brainfuck"))
	if err != nil {
		t.Fatalf("LoadQuery(brainfuck): unexpected error: %v", err)
	}
	if q != nil {
		t.Fatalf("LoadQuery(brainfuck) = %v, want nil", q)
	}
}

func TestLoadQueryCacheLifecycle(t *testing.T) {
	ClearQueryCache()
	defer ClearQueryCache()

	if q := LoadQuerySync(types.LanguageGo); q != nil {
		t.Fatalf("LoadQuerySync(go) = %v before load, want nil", q)
	}

	q, err := LoadQuery(types.LanguageGo)
	if err != nil {
		t.Fatalf("LoadQuery(go): %v", err)
	}
	if LoadQuerySync(types.LanguageGo) != q {
		t.Fatal("LoadQuerySync(go) after load = not the cached query")
	}

	ClearQueryCache()
	if q := LoadQuerySync(types.LanguageGo); q != nil {
		t.Fatalf("LoadQuerySync(go) after ClearQueryCache = %v, want nil", q)
	}

	if _, err := LoadQuery(types.LanguageGo); err != nil {
		t.Fatalf("reload LoadQuery(go): %v", err)
	}
}

func TestExecuteQueryGo(t *testing.T) {
	nodeLang := string(types.LanguageGo)
	tree := parseQueryTree(t, nodeLang, `
package main

import "fmt"

func greet(name string) string {
	return "hi " + name
}

type Person struct {
	Name string
}

func main() {
	fmt.Println(greet("x"))
}
`)
	defer tree.Release()

	q, err := LoadQuery(types.LanguageGo)
	if err != nil {
		t.Fatalf("LoadQuery(go): %v", err)
	}
	result, err := ExecuteQuery(q, tree, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}

	entities := GetEntityMatches(result)
	if len(entities) != 4 {
		t.Fatalf("len(GetEntityMatches) = %d, want 4", len(entities))
	}

	names := captureNames(t, tree, result)
	for _, want := range []string{"greet", "main", "Person", "\"fmt\""} {
		if names[want] == 0 {
			t.Errorf("name %q not captured, got %v", want, names)
		}
	}
}

func TestExecuteQueryPython(t *testing.T) {
	tree := parseQueryTree(t, "python", `
import numpy as np
from os import path

def greet(name):
	return "hi " + name

class Greeter:
	def hello(self):
		pass
`)
	defer tree.Release()

	q, err := LoadQuery(types.LanguagePython)
	if err != nil {
		t.Fatalf("LoadQuery(python): %v", err)
	}
	result, err := ExecuteQuery(q, tree, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}

	names := captureNames(t, tree, result)
	for _, want := range []string{"numpy", "np", "os", "greet", "Greeter", "hello"} {
		if names[want] == 0 {
			t.Errorf("name %q not captured, got %v", want, names)
		}
	}
}

func TestExecuteQueryTypeScript(t *testing.T) {
	tree := parseQueryTree(t, "typescript", `
import fs from 'fs';

interface User {
	id: number;
}

const name = 'x';

function greet(user: User): string {
	return 'hi';
}

class Greeter {
	greeting = 'hi';
}
`)
	defer tree.Release()

	q, err := LoadQuery(types.LanguageTypeScript)
	if err != nil {
		t.Fatalf("LoadQuery(typescript): %v", err)
	}
	result, err := ExecuteQuery(q, tree, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}

	names := captureNames(t, tree, result)
	for _, want := range []string{"'fs'", "User", "name", "greet", "Greeter"} {
		if names[want] == 0 {
			t.Errorf("name %q not captured, got %v", want, names)
		}
	}
}

func TestExecuteQueryRust(t *testing.T) {
	tree := parseQueryTree(t, "rust", `
use std::collections::HashMap;

struct Point {
	x: i32,
}

impl Point {
	fn dist(&self) -> i32 { 0 }
}

fn main() {}
`)
	defer tree.Release()

	q, err := LoadQuery(types.LanguageRust)
	if err != nil {
		t.Fatalf("LoadQuery(rust): %v", err)
	}
	result, err := ExecuteQuery(q, tree, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}

	names := captureNames(t, tree, result)
	for _, want := range []string{"std::collections::HashMap", "Point", "dist", "main"} {
		if names[want] == 0 {
			t.Errorf("name %q not captured, got %v", want, names)
		}
	}
}

func TestExecuteQueryJava(t *testing.T) {
	tree := parseQueryTree(t, "java", `
import java.util.List;

class Person {
	void greet(List<String> names) {
		return;
	}
}
`)
	defer tree.Release()

	q, err := LoadQuery(types.LanguageJava)
	if err != nil {
		t.Fatalf("LoadQuery(java): %v", err)
	}
	result, err := ExecuteQuery(q, tree, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}

	names := captureNames(t, tree, result)
	for _, want := range []string{"java.util.List", "Person", "greet"} {
		if names[want] == 0 {
			t.Errorf("name %q not captured, got %v", want, names)
		}
	}
}

func TestExecuteQueryStartNode(t *testing.T) {
	tree := parseQueryTree(t, "go", `
package main

func one() {}

func two() {}
`)
	defer tree.Release()

	q, err := LoadQuery(types.LanguageGo)
	if err != nil {
		t.Fatalf("LoadQuery(go): %v", err)
	}

	root := tree.RootNode()
	var target *gotreesitter.Node
	for i := 0; i < root.NamedChildCount(); i++ {
		if child := root.NamedChild(i); child != nil && child.Type(tree.Language()) == "function_declaration" && strings.Contains(child.Text(tree.Source()), "one") {
			target = child
			break
		}
	}
	if target == nil {
		t.Fatal("function_declaration 'one' not found")
	}

	full, err := ExecuteQuery(q, tree, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery(root): %v", err)
	}
	sub, err := ExecuteQuery(q, tree, target)
	if err != nil {
		t.Fatalf("ExecuteQuery(startNode): %v", err)
	}

	names := captureNames(t, tree, full)
	if names["one"] == 0 || names["two"] == 0 {
		t.Fatalf("full execution missing names, got %v", names)
	}
	subNames := captureNames(t, tree, sub)
	if subNames["one"] == 0 {
		t.Fatalf("sub execution missing 'one', got %v", subNames)
	}
	if subNames["two"] != 0 {
		t.Fatalf("sub execution should not include 'two', got %v", subNames)
	}
}

func TestExecuteQueryNilInputs(t *testing.T) {
	result, err := ExecuteQuery(nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteQuery(nil, nil, nil): %v", err)
	}
	if len(result.Matches) != 0 || len(result.Captures) != 0 {
		t.Fatalf("ExecuteQuery(nil, nil, nil) non-empty: %+v", result)
	}
}

func TestExtractEntityFromMatch(t *testing.T) {
	match := QueryMatch{
		PatternIndex: 0,
		Captures: []QueryCapture{
			{Name: "item", Node: &gotreesitter.Node{}},
			{Name: "name", Node: &gotreesitter.Node{}},
			{Name: "context", Node: &gotreesitter.Node{}},
		},
	}
	extraction, ok := ExtractEntityFromMatch(match)
	if !ok {
		t.Fatal("ExtractEntityFromMatch = false, want true")
	}
	if extraction.ItemNode == nil || extraction.NameNode == nil {
		t.Fatal("ItemNode/NameNode nil")
	}
	if len(extraction.ContextNodes) != 1 {
		t.Fatalf("len(ContextNodes) = %d, want 1", len(extraction.ContextNodes))
	}
	if len(extraction.AnnotationNodes) != 0 {
		t.Fatalf("len(AnnotationNodes) = %d, want 0", len(extraction.AnnotationNodes))
	}
}

func TestExtractEntityFromMatchMissingName(t *testing.T) {
	match := QueryMatch{
		Captures: []QueryCapture{
			{Name: "item", Node: &gotreesitter.Node{}},
		},
	}
	if _, ok := ExtractEntityFromMatch(match); ok {
		t.Fatal("ExtractEntityFromMatch without name = true, want false")
	}
}

func TestGetCapturesByName(t *testing.T) {
	result := QueryResult{
		Captures: []QueryCapture{
			{Name: "name"},
			{Name: "item"},
			{Name: "name"},
		},
	}
	names := GetCapturesByName(result, "name")
	if len(names) != 2 {
		t.Fatalf("len(GetCapturesByName) = %d, want 2", len(names))
	}
	if got := GetCapturesByName(result, "annotation"); len(got) != 0 {
		t.Fatalf("len(GetCapturesByName(annotation)) = %d, want 0", len(got))
	}
}
