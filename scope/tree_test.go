package scope

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

func ent(t types.EntityType, name string, start, end int) types.ExtractedEntity {
	return types.ExtractedEntity{
		Type:      t,
		Name:      name,
		ByteRange: types.ByteRange{Start: start, End: end},
	}
}

func TestBuildScopeTreeFromEntities(t *testing.T) {
	entities := []types.ExtractedEntity{
		ent(types.EntityTypeImport, "fs", 0, 5),
		ent(types.EntityTypeExport, "util", 6, 12),
		ent(types.EntityTypeClass, "Outer", 10, 100),
		ent(types.EntityTypeMethod, "Outer.Run", 20, 50),
		ent(types.EntityTypeFunction, "freeFunc", 200, 250),
	}

	tree := BuildScopeTreeFromEntities(entities)

	if len(tree.Imports) != 1 || tree.Imports[0].Name != "fs" {
		t.Fatalf("imports = %v, want 1 import fs", tree.Imports)
	}
	if len(tree.Exports) != 1 || tree.Exports[0].Name != "util" {
		t.Fatalf("exports = %v, want 1 export util", tree.Exports)
	}
	if len(tree.Root) != 2 {
		t.Fatalf("roots = %d, want 2", len(tree.Root))
	}

	outer := tree.Root[0]
	if outer.Entity.Name != "Outer" {
		t.Fatalf("first root = %s, want Outer", outer.Entity.Name)
	}
	if len(outer.Children) != 1 || outer.Children[0].Entity.Name != "Outer.Run" {
		t.Fatalf("Outer children = %v, want 1 child Outer.Run", outer.Children)
	}
	if outer.Children[0].Parent != outer {
		t.Fatal("child parent link not set to Outer")
	}

	free := tree.Root[1]
	if free.Entity.Name != "freeFunc" {
		t.Fatalf("second root = %s, want freeFunc", free.Entity.Name)
	}
}

func TestFindScopeAtOffset(t *testing.T) {
	tree := BuildScopeTreeFromEntities([]types.ExtractedEntity{
		ent(types.EntityTypeClass, "Outer", 10, 100),
		ent(types.EntityTypeMethod, "Outer.Run", 20, 50),
	})

	if got := FindScopeAtOffset(tree, 30); got == nil || got.Entity.Name != "Outer.Run" {
		t.Fatalf("offset 30 -> %v, want Outer.Run", got)
	}
	if got := FindScopeAtOffset(tree, 70); got == nil || got.Entity.Name != "Outer" {
		t.Fatalf("offset 70 -> %v, want Outer", got)
	}
	if got := FindScopeAtOffset(tree, 5); got != nil {
		t.Fatalf("offset 5 -> %v, want nil", got)
	}
}

func TestGetAncestorChain(t *testing.T) {
	tree := BuildScopeTreeFromEntities([]types.ExtractedEntity{
		ent(types.EntityTypeClass, "Outer", 10, 100),
		ent(types.EntityTypeMethod, "Outer.Run", 20, 50),
	})
	method := tree.Root[0].Children[0]

	chain := GetAncestorChain(method)
	if len(chain) != 1 || chain[0].Entity.Name != "Outer" {
		t.Fatalf("ancestors = %v, want [Outer]", chain)
	}
	if chain[0].Parent != nil {
		t.Fatal("Outer should have no parent")
	}
}

func TestFlattenScopeTree(t *testing.T) {
	tree := BuildScopeTreeFromEntities([]types.ExtractedEntity{
		ent(types.EntityTypeClass, "Outer", 10, 100),
		ent(types.EntityTypeMethod, "Outer.Run", 20, 50),
		ent(types.EntityTypeFunction, "freeFunc", 200, 250),
	})

	flat := FlattenScopeTree(tree)
	if len(flat) != 3 {
		t.Fatalf("flattened = %d nodes, want 3", len(flat))
	}
	if flat[0].Entity.Name != "Outer" || flat[1].Entity.Name != "Outer.Run" || flat[2].Entity.Name != "freeFunc" {
		t.Fatalf("DFS order = %v, want [Outer Outer.Run freeFunc]", names(flat))
	}
}

func names(nodes []*types.ScopeNode) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.Entity.Name)
	}
	return out
}
