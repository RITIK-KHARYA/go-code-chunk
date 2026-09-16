package chunker

import (
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// GetAncestors returns unique ancestor nodes for a set of nodes, walking the
// parent chain of every node in order. It is the Go port of the TS
// `getAncestors`, deduplicating by node pointer identity.
func GetAncestors(nodes []types.SyntaxNode) []types.SyntaxNode {
	seen := make(map[*gotreesitter.Node]struct{})
	var ancestors []types.SyntaxNode

	for _, node := range nodes {
		for current := tsNode(node).Parent(); current != nil; current = current.Parent() {
			if _, ok := seen[current]; ok {
				continue
			}
			seen[current] = struct{}{}
			ancestors = append(ancestors, current)
		}
	}

	return ancestors
}
