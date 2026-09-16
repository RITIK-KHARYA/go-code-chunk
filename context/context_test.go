package chunkcontext

import (
	"strings"
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/extract"
	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/scope"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

const testSrc = `package p

import (
	"fmt"
	"strings"
)

// Greeter greets.
type Greeter struct{}

// Greet greets a name.
func (g Greeter) Greet(name string) string {
	return fmt.Sprintf("Hello, %s", name)
}

func helper() string {
	return strings.TrimSpace(" x ")
}
`

// tsSrc scopes nest by byte range (class contains method), unlike Go where a
// method follows its type and is thus a sibling root.
const tsSrc = `class Widget {
  render(): string {
    return helper();
  }
}
function helper() {
  return 1;
}
`

func buildTreeFor(t *testing.T, src, lang string) (string, types.ScopeTree) {
	t.Helper()
	p, err := parser.NewParser(lang)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	syntaxTree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	entities := extract.ExtractEntitiesByNodeTypes(syntaxTree.RootNode(), types.Language(lang), src)
	tree := scope.BuildScopeTreeFromEntities(entities)
	if len(tree.Root) == 0 {
		t.Fatal("no scope roots extracted")
	}
	return src, tree
}

func buildTestTree(t *testing.T) (string, types.ScopeTree) {
	t.Helper()
	return buildTreeFor(t, testSrc, "go")
}

func findEntity(t *testing.T, tree types.ScopeTree, name string) types.ExtractedEntity {
	t.Helper()
	for _, e := range tree.AllEntities {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("entity %q not found", name)
	return types.ExtractedEntity{}
}

func TestGetScopeForRange(t *testing.T) {
	src, tree := buildTreeFor(t, tsSrc, "typescript")

	pos := strings.Index(src, "helper()")
	if pos < 0 {
		t.Fatal("marker not found in source")
	}
	chain := GetScopeForRange(types.ByteRange{Start: pos, End: pos + 1}, tree)

	got := []string{chain[0].Name, chain[1].Name}
	want := []string{"render", "Widget"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scope chain = %v, want %v", got, want)
		}
	}

	empty := GetScopeForRange(types.ByteRange{Start: len(src), End: len(src) + 1}, tree)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("GetScopeForRange(past end) = %v, want empty non-nil", empty)
	}

	srcGo, treeGo := buildTestTree(t)
	posGo := strings.Index(srcGo, "fmt.Sprintf")
	goChain := GetScopeForRange(types.ByteRange{Start: posGo, End: posGo + 1}, treeGo)
	if len(goChain) != 1 || goChain[0].Name != "Greet" {
		t.Fatalf("go scope chain = %+v, want [Greet] (no ancestor: method follows its type)", goChain)
	}
}

func TestGetEntitiesInRange(t *testing.T) {
	_, tree := buildTreeFor(t, tsSrc, "typescript")
	render := findEntity(t, tree, "render")

	result := GetEntitiesInRange(render.ByteRange, tree)

	if len(result) != 2 {
		t.Fatalf("overlap count = %d, want 2 (got %+v)", len(result), result)
	}
	if result[0].Name != "Widget" || !result[0].IsPartial {
		t.Fatalf("result[0] = %+v, want Widget partial", result[0])
	}
	if result[1].Name != "render" || result[1].IsPartial {
		t.Fatalf("result[1] = %+v, want render complete", result[1])
	}
	if result[1].LineRange == nil || result[1].Signature == nil {
		t.Fatalf("render info missing optional fields: %+v", result[1])
	}
}

func TestGetRelevantImports(t *testing.T) {
	_, tree := buildTestTree(t)

	all := GetRelevantImports(nil, tree, false)
	if len(all) != 2 || all[0].Name != "fmt" || all[1].Name != "strings" {
		t.Fatalf("all imports = %+v, want [fmt strings]", all)
	}
	if all[0].Source != "fmt" {
		t.Fatalf("fmt source = %q, want %q", all[0].Source, "fmt")
	}

	sig := "func (g Greeter) Greet(name string) string { return fmt.Sprintf(...) }"
	entities := []types.ChunkEntityInfo{{
		EntityInfo: types.EntityInfo{Name: "Greet", Type: types.EntityTypeMethod, Signature: &sig},
	}}
	filtered := GetRelevantImports(entities, tree, true)
	if len(filtered) != 1 || filtered[0].Name != "fmt" {
		t.Fatalf("filtered imports = %+v, want [fmt]", filtered)
	}

	if got := GetRelevantImports(nil, tree, true); len(got) != 0 {
		t.Fatalf("filtered with no entities = %+v, want empty", got)
	}

	if got := GetRelevantImports(nil, types.ScopeTree{}, true); len(got) != 0 {
		t.Fatalf("imports on empty tree = %+v, want empty", got)
	}
}

func TestGetSiblings(t *testing.T) {
	src, tree := buildTreeFor(t, tsSrc, "typescript")
	pos := strings.Index(src, "return 1")
	if pos < 0 {
		t.Fatal("marker not found in source")
	}
	br := types.ByteRange{Start: pos, End: pos + 1}

	siblings := GetSiblings(br, tree, SiblingOptions{Detail: "names"})
	if len(siblings) != 1 {
		t.Fatalf("siblings = %+v, want 1", siblings)
	}
	if siblings[0].Name != "Widget" || siblings[0].Position != types.SiblingPositionBefore || siblings[0].Distance != 1 {
		t.Fatalf("sibling[0] = %+v, want Widget before 1", siblings[0])
	}

	if got := GetSiblings(br, tree, SiblingOptions{Detail: "none"}); len(got) != 0 {
		t.Fatalf("detail none returned %+v, want empty", got)
	}

	t.Run("between roots", func(t *testing.T) {
		_, tree := buildTreeFor(t, tsSrc, "typescript")
		widget := findEntity(t, tree, "Widget")
		gap := types.ByteRange{Start: widget.ByteRange.End, End: widget.ByteRange.End}
		got := GetSiblings(gap, tree, SiblingOptions{Detail: "names"})
		if len(got) != 2 {
			t.Fatalf("gap siblings = %+v, want 2", got)
		}
		if got[0].Name != "Widget" || got[0].Position != types.SiblingPositionBefore || got[0].Distance != 2 {
			t.Fatalf("gap sibling[0] = %+v, want Widget before 2", got[0])
		}
		if got[1].Name != "helper" || got[1].Position != types.SiblingPositionAfter || got[1].Distance != 2 {
			t.Fatalf("gap sibling[1] = %+v, want helper after 2", got[1])
		}
	})

	t.Run("maxSiblings cap", func(t *testing.T) {
		n1 := scope.NewScopeNode(ent(types.EntityTypeFunction, "a", 0, 10), nil)
		n2 := scope.NewScopeNode(ent(types.EntityTypeFunction, "b", 20, 30), nil)
		n3 := scope.NewScopeNode(ent(types.EntityTypeFunction, "c", 40, 50), nil)
		n4 := scope.NewScopeNode(ent(types.EntityTypeFunction, "d", 60, 70), nil)
		tree := types.ScopeTree{Root: []*types.ScopeNode{n1, n2, n3, n4}}

		got := GetSiblings(types.ByteRange{Start: 35, End: 35}, tree, SiblingOptions{Detail: "names", MaxSiblings: 1})
		if len(got) != 2 {
			t.Fatalf("capped siblings = %+v, want 2", got)
		}
		if got[0].Name != "b" || got[0].Position != types.SiblingPositionBefore || got[0].Distance != 3 {
			t.Fatalf("before sibling = %+v, want b before 3", got[0])
		}
		if got[1].Name != "c" || got[1].Position != types.SiblingPositionAfter || got[1].Distance != 3 {
			t.Fatalf("after sibling = %+v, want c after 3", got[1])
		}
	})
}

func ent(t types.EntityType, name string, start, end int) types.ExtractedEntity {
	return types.ExtractedEntity{
		Type:      t,
		Name:      name,
		ByteRange: types.ByteRange{Start: start, End: end},
	}
}

func TestFormatChunkWithContext(t *testing.T) {
	fp := "src/app/components/Widget.tsx"
	sigClass := "export class Widget extends Component"
	sigRender := "render()"
	sigReact := "import React from 'react'"
	ctx := types.ChunkContext{
		Filepath: &fp,
		Language: &langTS,
		Scope: []types.EntityInfo{
			{Name: "Widget", Type: types.EntityTypeClass, Signature: &sigClass},
			{Name: "Component", Type: types.EntityTypeClass},
		},
		Entities: []types.ChunkEntityInfo{
			{EntityInfo: types.EntityInfo{Name: "Widget", Type: types.EntityTypeClass, Signature: &sigClass}},
			{EntityInfo: types.EntityInfo{Name: "render", Type: types.EntityTypeMethod, Signature: &sigRender}},
			{EntityInfo: types.EntityInfo{Name: "React", Type: types.EntityTypeImport, Signature: &sigReact}},
		},
		Siblings: []types.SiblingInfo{
			{Name: "setup", Type: types.EntityTypeFunction, Position: types.SiblingPositionBefore, Distance: 1},
			{Name: "teardown", Type: types.EntityTypeFunction, Position: types.SiblingPositionAfter, Distance: 1},
		},
		Imports: []types.ImportInfo{
			{Name: "react", Source: "react"},
			{Name: "mobx", Source: "mobx"},
		},
	}

	got := FormatChunkWithContext("export default widget", ctx, "overlap from prev")
	want := strings.Join([]string{
		"# app/components/Widget.tsx",
		"# Scope: Component > Widget",
		"# Defines: export class Widget extends Component, render()",
		"# Uses: react, mobx",
		"# After: setup",
		"# Before: teardown",
		"",
		"# ...",
		"overlap from prev",
		"# ---",
		"export default widget",
	}, "\n")
	if got != want {
		t.Fatalf("FormattedChunkWithContext:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	if got := FormatChunkWithContext("code", types.ChunkContext{}, ""); got != "code" {
		t.Fatalf("empty context = %q, want %q", got, "code")
	}
}

var langTS = types.LanguageTypeScript
