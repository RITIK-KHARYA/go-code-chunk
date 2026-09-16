package chunkcontext

import (
	"sort"

	"github.com/RITIK-KHARYA/go-code-chunk/scope"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// SiblingOptions controls sibling retrieval. Detail mirrors the TS
// 'none' | 'names' | 'signatures' union; a MaxSiblings of 0 means unlimited.
type SiblingOptions struct {
	Detail      string
	MaxSiblings int
}

// getSiblingNodes returns the sibling array for a scope node: the parent's
// children when the node has a parent, otherwise the root array.
func getSiblingNodes(node *types.ScopeNode, scopeTree types.ScopeTree) []*types.ScopeNode {
	if node.Parent != nil {
		return node.Parent.Children
	}
	return scopeTree.Root
}

// nodeToSiblingInfo converts a scope node to sibling info with a position and
// index distance from the current node.
func nodeToSiblingInfo(node *types.ScopeNode, position types.SiblingPosition, distance int) types.SiblingInfo {
	return types.SiblingInfo{
		Name:     node.Entity.Name,
		Type:     node.Entity.Type,
		Position: position,
		Distance: distance,
	}
}

// GetSiblings returns sibling entities for a byte range: the peers of the
// enclosing scope, positioned before/after by byte order and sorted by index
// distance, capped on each side by MaxSiblings. It is the Go port of the TS
// `getSiblings` from `sibling.ts`.
func GetSiblings(byteRange types.ByteRange, scopeTree types.ScopeTree, options SiblingOptions) []types.SiblingInfo {
	if options.Detail == "" || options.Detail == string(types.SiblingDetailNone) {
		return []types.SiblingInfo{}
	}

	currentNode := scope.FindScopeAtOffset(scopeTree, byteRange.Start)

	var siblings []*types.ScopeNode
	if currentNode != nil {
		siblings = getSiblingNodes(currentNode, scopeTree)
	} else {
		siblings = scopeTree.Root
	}

	currentIndex := -1
	if currentNode != nil {
		for i, sibling := range siblings {
			if sibling == currentNode {
				currentIndex = i
				break
			}
		}
	}

	result := make([]types.SiblingInfo, 0)
	for i, sibling := range siblings {
		if sibling == nil {
			continue
		}

		if currentNode != nil && sibling == currentNode {
			continue
		}

		if sibling.Entity.ByteRange.Start < byteRange.End && sibling.Entity.ByteRange.End > byteRange.Start {
			continue
		}

		position := types.SiblingPositionAfter
		if sibling.Entity.ByteRange.Start < byteRange.Start {
			position = types.SiblingPositionBefore
		}

		distance := 0
		if currentIndex >= 0 {
			distance = abs(i - currentIndex)
		} else if position == types.SiblingPositionBefore {
			distance = len(siblings) - i
		} else {
			distance = i + 1
		}

		result = append(result, nodeToSiblingInfo(sibling, position, distance))
	}

	sort.SliceStable(result, func(a, b int) bool {
		return result[a].Distance < result[b].Distance
	})

	if options.MaxSiblings > 0 {
		var before, after []types.SiblingInfo
		for _, s := range result {
			if s.Position == types.SiblingPositionBefore {
				before = append(before, s)
			} else {
				after = append(after, s)
			}
		}
		if len(before) > options.MaxSiblings {
			before = before[:options.MaxSiblings]
		}
		if len(after) > options.MaxSiblings {
			after = after[:options.MaxSiblings]
		}
		return append(before, after...)
	}

	return result
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
