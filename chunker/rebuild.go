package chunker

import (
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// buildLineStartsTable builds a lookup table where lineStarts[i] is the byte
// offset of line i. It is the Go port of the TS `buildLineStartsTable`.
func buildLineStartsTable(code string) []int {
	lineStarts := []int{0} // Line 0 starts at byte 0
	for i := 0; i < len(code); i++ {
		if code[i] == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	return lineStarts
}

// RebuildText rebuilds source text from an AST window. It is the Go port of
// the TS `rebuildText`, handling both normal windows (slice from first node
// start to last node end) and partial node windows (rebuild from line ranges).
func RebuildText(window types.ASTWindow, code string) types.RebuiltText {
	// Handle empty windows
	if len(window.Nodes) == 0 {
		return types.RebuiltText{}
	}

	// Handle partial node windows with line ranges
	if isPartialWindow(window) && len(window.LineRanges) > 0 {
		return rebuildFromLineRanges(window, code)
	}

	// Normal case: slice from first node start to last node end.
	// Use node positions directly for line numbers (0-indexed).
	firstNode := tsNode(window.Nodes[0])
	lastNode := tsNode(window.Nodes[len(window.Nodes)-1])
	if firstNode == nil || lastNode == nil {
		return types.RebuiltText{}
	}

	startByte := int(firstNode.StartByte())
	endByte := int(lastNode.EndByte())
	startLine := int(firstNode.StartPoint().Row)
	endLine := int(lastNode.EndPoint().Row)

	return types.RebuiltText{
		Text:      code[startByte:endByte],
		ByteRange: types.ByteRange{Start: startByte, End: endByte},
		LineRange: types.LineRange{Start: startLine, End: endLine},
	}
}

// rebuildFromLineRanges rebuilds text from line ranges for partial nodes.
// It is the Go port of the TS `rebuildFromLineRanges`.
func rebuildFromLineRanges(window types.ASTWindow, code string) types.RebuiltText {
	lineRanges := window.LineRanges
	if len(lineRanges) == 0 {
		return types.RebuiltText{}
	}
	lineStarts := buildLineStartsTable(code)

	// Get the overall line range
	firstRange := lineRanges[0]
	lastRange := lineRanges[len(lineRanges)-1]

	startLine := firstRange.Start
	endLine := lastRange.End

	// Calculate byte offsets from line numbers
	startByte := 0
	if startLine < len(lineStarts) {
		startByte = lineStarts[startLine]
	}
	// End byte is start of line after endLine, or end of file
	endByte := len(code)
	if endLine+1 < len(lineStarts) {
		endByte = lineStarts[endLine+1]
	}

	// Clamp so the slice is always within bounds (TS slice clamps silently).
	if startByte > len(code) {
		startByte = len(code)
	}
	if endByte > len(code) {
		endByte = len(code)
	}
	if startByte > endByte {
		startByte = endByte
	}

	return types.RebuiltText{
		Text:      code[startByte:endByte],
		ByteRange: types.ByteRange{Start: startByte, End: endByte},
		LineRange: types.LineRange{Start: startLine, End: endLine},
	}
}

// isPartialWindow reports whether the window carries a partial-node flag.
func isPartialWindow(window types.ASTWindow) bool {
	return window.IsPartialNode != nil && *window.IsPartialNode
}
