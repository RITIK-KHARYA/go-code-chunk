package chunker

import "github.com/RITIK-KHARYA/go-code-chunk/types"

// IsLeafNode reports whether a node has no children. It is the Go port of
// the TS `isLeafNode` from oversized.go.
func IsLeafNode(node types.SyntaxNode) bool {
	return tsNode(node).ChildCount() == 0
}
