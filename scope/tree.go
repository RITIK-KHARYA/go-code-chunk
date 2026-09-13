// Package scope builds a hierarchical scope tree from extracted entities.
package scope

import (
	"sort"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// RangeContains - true if outer fully contains inner
func RangeContains(outer, inner types.ByteRange) bool {
	return outer.Start <= inner.Start && inner.End <= outer.End
}

// NewScopeNode - new node from entity + optional parentS
func NewScopeNode(entity types.ExtractedEntity, parent *types.ScopeNode) *types.ScopeNode {
	return &types.ScopeNode{
		Entity:   entity,
		Children: []*types.ScopeNode{},
		Parent:   parent,
	}
}

// find the deepest and most closet parent node for the given entity
// example:
// x -- parent and x2 is child node so x2's parent is x
// FindParentNode - deepest node whose range contains entity's range, DFS
func FindParentNode(roots []*types.ScopeNode, entity types.ExtractedEntity) *types.ScopeNode {
	var findInNode func(node *types.ScopeNode) *types.ScopeNode
	findInNode = func(node *types.ScopeNode) *types.ScopeNode {
		if !RangeContains(node.Entity.ByteRange, entity.ByteRange) {
			return nil
		}
		for _, child := range node.Children {
			if deeper := findInNode(child); deeper != nil {
				return deeper
			}
		}
		return node
	}

	for _, root := range roots {
		if found := findInNode(root); found != nil {
			return found
		}
	}
	return nil
}

// BuildScopeTreeFromEntities - build tree from extracted entities
func BuildScopeTreeFromEntities(entities []types.ExtractedEntity) types.ScopeTree {
	var imports, exports, scopeEntities []types.ExtractedEntity

	// separate imports, exports, and scope entities
	// later we sort them by byte range to build the tree
	for _, entity := range entities {
		switch entity.Type {
		case types.EntityTypeImport:
			imports = append(imports, entity)
		case types.EntityTypeExport:
			exports = append(exports, entity)
		default:
			scopeEntities = append(scopeEntities, entity)
		}
	}

	sorted := make([]types.ExtractedEntity, len(scopeEntities))
	copy(sorted, scopeEntities)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ByteRange.Start < sorted[j].ByteRange.Start
	})

	var root []*types.ScopeNode

	for _, entity := range sorted {
		parent := FindParentNode(root, entity)
		node := NewScopeNode(entity, parent)

		if parent != nil {
			parent.Children = append(parent.Children, node)
		} else {
			root = append(root, node)
		}
	}

	return types.ScopeTree{
		Root:        root,
		Imports:     imports,
		Exports:     exports,
		AllEntities: entities,
	}
}

// FindScopeAtOffset - deepest node containing byte offset
func FindScopeAtOffset(tree types.ScopeTree, offset int) *types.ScopeNode {
	var findInNode func(node *types.ScopeNode) *types.ScopeNode
	findInNode = func(node *types.ScopeNode) *types.ScopeNode {
		br := node.Entity.ByteRange
		if offset < br.Start || offset >= br.End {
			return nil
		}
		for _, child := range node.Children {
			if deeper := findInNode(child); deeper != nil {
				return deeper
			}
		}
		return node
	}

	for _, root := range tree.Root {
		if found := findInNode(root); found != nil {
			return found
		}
	}
	return nil
}

// GetAncestorChain - parent chain, immediate parent first, root last
func GetAncestorChain(node *types.ScopeNode) []*types.ScopeNode {
	var ancestors []*types.ScopeNode
	current := node.Parent
	for current != nil {
		ancestors = append(ancestors, current)
		current = current.Parent
	}
	return ancestors
}

// FlattenScopeTree - all nodes, DFS order
func FlattenScopeTree(tree types.ScopeTree) []*types.ScopeNode {
	var result []*types.ScopeNode
	var visit func(node *types.ScopeNode)
	visit = func(node *types.ScopeNode) {
		result = append(result, node)
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, root := range tree.Root {
		visit(root)
	}
	return result
}
