package extract

import (
	"reflect"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

type entityView struct {
	typ    types.EntityType
	name   string
	sign   string
	parent *string
	source *string
}

func collectViews(t *testing.T, language, src string) []entityView {
	t.Helper()

	p, err := parser.NewParser(language)
	if err != nil {
		t.Fatalf("NewParser(%q): %v", language, err)
	}

	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", language, err)
	}
	defer tree.Release()

	entities := ExtractEntitiesByNodeTypes(tree.RootNode(), types.Language(language), src)
	views := make([]entityView, 0, len(entities))
	for _, e := range entities {
		views = append(views, entityView{typ: e.Type, name: e.Name, sign: e.Signature, parent: e.Parent, source: e.Source})
	}
	return views
}

//go:fix inline
func strptr(s string) *string { return new(s) }

func TestExtractEntitiesByNodeTypesGo(t *testing.T) {
	src := `package main

import "fmt"

type Person struct {
	Name string
}

func (p *Person) Greet() string {
	return "hi"
}

func main() {
	fmt.Println("hello")
}
`

	want := []entityView{
		{typ: types.EntityTypeImport, name: "fmt", sign: "fmt", source: new("fmt")},
		{typ: types.EntityTypeType, name: "<anonymous>", sign: "type Person struct"},
		{typ: types.EntityTypeMethod, name: "Greet", sign: "func (p *Person) Greet() string"},
		{typ: types.EntityTypeFunction, name: "main", sign: "func main()"},
	}

	got := collectViews(t, "go", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesGoImportSymbols(t *testing.T) {
	src := `package main

import (
	"os"
	"path/filepath"
)
`
	want := []entityView{
		{typ: types.EntityTypeImport, name: "os", sign: "os", source: new("os")},
		{typ: types.EntityTypeImport, name: "path/filepath", sign: "path/filepath", source: new("path/filepath")},
	}

	got := collectViews(t, "go", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesGoAliasedImport(t *testing.T) {
	src := `package main

import jsonf "encoding/json"
`
	want := []entityView{
		{typ: types.EntityTypeImport, name: "jsonf", sign: "jsonf", source: new("encoding/json")},
	}

	got := collectViews(t, "go", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesTypeScriptNestedParents(t *testing.T) {
	src := `class Person {
  greet(): string {
    return 'hi';
  }
}
function helper() {}
`

	want := []entityView{
		{typ: types.EntityTypeClass, name: "Person", sign: "class Person"},
		{typ: types.EntityTypeMethod, name: "greet", sign: "greet(): string", parent: new("Person")},
		{typ: types.EntityTypeFunction, name: "helper", sign: "function helper()"},
	}

	got := collectViews(t, "typescript", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesPythonNestedParents(t *testing.T) {
	src := `class Greeter:
    def hello(self):
        return 'hi'

def top():
    pass
`

	want := []entityView{
		{typ: types.EntityTypeClass, name: "Greeter", sign: "class Greeter"},
		{typ: types.EntityTypeFunction, name: "hello", sign: "def hello(self)", parent: new("Greeter")},
		{typ: types.EntityTypeFunction, name: "top", sign: "def top()"},
	}

	got := collectViews(t, "python", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesRustImplParents(t *testing.T) {
	src := `struct Point {}

impl Point {
    fn dist(&self) -> i32 { 0 }
}

fn main() {}
`

	want := []entityView{
		{typ: types.EntityTypeType, name: "Point", sign: "struct Point"},
		{typ: types.EntityTypeClass, name: "Point", sign: "impl Point"},
		{typ: types.EntityTypeFunction, name: "dist", sign: "fn dist(&self) -> i32", parent: new("Point")},
		{typ: types.EntityTypeFunction, name: "main", sign: "fn main()"},
	}

	got := collectViews(t, "rust", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesPythonImports(t *testing.T) {
	src := `import numpy as np
from os import path, sep
from os.path import join as j
`

	// Aliased imports bind the alias ("as X"), since that is the identifier
	// source code subsequently uses; plain dotted names bind themselves.
	want := []entityView{
		{typ: types.EntityTypeImport, name: "np", sign: "np", source: new("numpy")},
		{typ: types.EntityTypeImport, name: "path", sign: "path", source: new("os")},
		{typ: types.EntityTypeImport, name: "sep", sign: "sep", source: new("os")},
		{typ: types.EntityTypeImport, name: "j", sign: "j", source: new("os.path")},
	}

	got := collectViews(t, "python", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesRustImportSymbols(t *testing.T) {
	src := `use std::collections::{HashMap, HashSet};
use std::io;
`
	want := []entityView{
		{typ: types.EntityTypeImport, name: "HashMap", sign: "HashMap", source: new("std::collections")},
		{typ: types.EntityTypeImport, name: "HashSet", sign: "HashSet", source: new("std::collections")},
		{typ: types.EntityTypeImport, name: "std::io", sign: "std::io", source: new("std::io")},
	}

	got := collectViews(t, "rust", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesTSImportSymbols(t *testing.T) {
	src := `import 'polyfill';
import d, { a, b } from 'x';
import * as ns from 'y';
import { format as fmt } from 'z';
`
	want := []entityView{
		{typ: types.EntityTypeImport, name: "polyfill", sign: "polyfill", source: new("polyfill")},
		{typ: types.EntityTypeImport, name: "d", sign: "d", source: new("x")},
		{typ: types.EntityTypeImport, name: "a", sign: "a", source: new("x")},
		{typ: types.EntityTypeImport, name: "b", sign: "b", source: new("x")},
		{typ: types.EntityTypeImport, name: "ns", sign: "ns", source: new("y")},
		// Aliased named imports bind the alias, the identifier code uses.
		{typ: types.EntityTypeImport, name: "fmt", sign: "fmt", source: new("z")},
	}

	got := collectViews(t, "typescript", src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entities = %+v, want %+v", got, want)
	}
}

func TestExtractEntitiesByNodeTypesUnknownLanguage(t *testing.T) {
	entities := ExtractEntitiesByNodeTypes(nil, types.Language("not-a-language"), "")
	if entities != nil {
		t.Fatalf("entities = %v, want nil", entities)
	}
}

func TestIsEntityNodeType(t *testing.T) {
	cases := []struct {
		nodeType string
		language types.Language
		want     bool
	}{
		{"function_declaration", types.LanguageGo, true},
		{"class_declaration", types.LanguageJava, true},
		{"class_declaration", types.LanguageGo, false},
		{"function_definition", types.LanguagePython, true},
		{"function_definition", types.LanguageTypeScript, false},
		{"not_a_node_type", types.LanguageGo, false},
	}
	for _, c := range cases {
		if got := IsEntityNodeType(c.nodeType, c.language); got != c.want {
			t.Errorf("IsEntityNodeType(%q, %s) = %v, want %v", c.nodeType, c.language, got, c.want)
		}
	}
}

func TestGetEntityType(t *testing.T) {
	cases := []struct {
		nodeType string
		want     types.EntityType
		ok       bool
	}{
		{"function_declaration", types.EntityTypeFunction, true},
		{"method_declaration", types.EntityTypeMethod, true},
		{"class_definition", types.EntityTypeClass, true},
		{"impl_item", types.EntityTypeClass, true},
		{"trait_item", types.EntityTypeInterface, true},
		{"use_declaration", types.EntityTypeImport, true},
		{"nope", "", false},
	}
	for _, c := range cases {
		got, ok := GetEntityType(c.nodeType)
		if got != c.want || ok != c.ok {
			t.Errorf("GetEntityType(%q) = (%s, %v), want (%s, %v)", c.nodeType, got, ok, c.want, c.ok)
		}
	}
}
