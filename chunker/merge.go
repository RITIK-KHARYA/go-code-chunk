package chunker

import (
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// MergeOptions configures merging of adjacent windows.
type MergeOptions struct {
	// MaxSize is the maximum size of a merged window.
	MaxSize int
}

// CanMerge reports whether two windows can be merged without exceeding maxSize.
// It is the Go port of the TS `canMerge`.
func CanMerge(a, b types.ASTWindow, maxSize int) bool {
	return a.Size+b.Size <= maxSize
}

// MergeWindows merges two windows into one. It is the Go port of the TS
// `mergeWindows`:
//
//   - nodes are concatenated
//   - ancestors are concatenated, deduplicating by node pointer identity
//   - lineRanges are concatenated only when both windows have them
//   - size is the sum of both sizes
//   - isPartialNode is the OR of both flags
func MergeWindows(a, b types.ASTWindow) types.ASTWindow {
	// Combine nodes from both windows
	nodes := make([]types.SyntaxNode, 0, len(a.Nodes)+len(b.Nodes))
	nodes = append(nodes, a.Nodes...)
	nodes = append(nodes, b.Nodes...)

	// Combine ancestors, deduplicating by node pointer identity
	seen := make(map[*gotreesitter.Node]struct{}, len(a.Ancestors)+len(b.Ancestors))
	ancestors := make([]types.SyntaxNode, 0, len(a.Ancestors)+len(b.Ancestors))
	addAncestors := func(list []types.SyntaxNode) {
		for _, ancestor := range list {
			key := tsNode(ancestor)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			ancestors = append(ancestors, ancestor)
		}
	}
	addAncestors(a.Ancestors)
	addAncestors(b.Ancestors)

	// Combine line ranges if present on both
	var lineRanges []types.LineRange
	if len(a.LineRanges) > 0 && len(b.LineRanges) > 0 {
		lineRanges = make([]types.LineRange, 0, len(a.LineRanges)+len(b.LineRanges))
		lineRanges = append(lineRanges, a.LineRanges...)
		lineRanges = append(lineRanges, b.LineRanges...)
	}

	return types.ASTWindow{
		Nodes:         nodes,
		Ancestors:     ancestors,
		Size:          a.Size + b.Size,
		IsPartialNode: mergePartial(a.IsPartialNode, b.IsPartialNode),
		LineRanges:    lineRanges,
	}
}

// mergePartial computes the OR of two *bool flags. It follows the window
// convention where a non-partial window carries a nil pointer, so the result
// is nil unless either input is explicitly true.
func mergePartial(a, b *bool) *bool {
	if (a != nil && *a) || (b != nil && *b) {
		v := true
		return &v
	}
	return nil
}

// MergeAdjacentWindows merges windows that together fit within maxSize. It is
// the Go port of the TS generator `mergeAdjacentWindows`, returning a slice
// instead of yielding.
func MergeAdjacentWindows(windows []types.ASTWindow, opts MergeOptions) []types.ASTWindow {
	maxSize := opts.MaxSize
	merged := make([]types.ASTWindow, 0, len(windows))

	var current types.ASTWindow
	hasCurrent := false

	for _, window := range windows {
		if !hasCurrent {
			current = window
			hasCurrent = true
		} else if CanMerge(current, window, maxSize) {
			current = MergeWindows(current, window)
		} else {
			merged = append(merged, current)
			current = window
		}
	}

	if hasCurrent {
		merged = append(merged, current)
	}

	return merged
}
