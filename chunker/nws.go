package chunker

import (
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// NwsCumsum is the cumulative sum array for O(1) NWS range queries.
// cumsum[i] = count of non-whitespace chars in code[0..i-1].
type NwsCumsum []uint32

// CountNws counts non-whitespace characters in a string. It is the Go port of
// the TS `countNws`, which counts characters NOT matched by JS `\s`.
func CountNws(text string) int {
	n := 0
	for _, r := range text {
		if !isRegexWhitespace(r) {
			n++
		}
	}
	return n
}

// isRegexWhitespace reports whether r matches the JS `\s` regex class
// (`[\f\n\r\t\v \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff]`).
func isRegexWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004',
		'\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a',
		'\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return false
}

// PreprocessNwsCumsum builds a cumulative sum array for O(1) NWS range
// queries. The resulting array has length len(code)+1 and
// cumsum[i] = count of non-whitespace characters in code[0..i-1].
//
// A character is whitespace when its code point is <= 32 (space, tab,
// newline, CR, and the other C0 control characters), mirroring the TS
// `charCodeAt(i) <= 32` check.
func PreprocessNwsCumsum(code string) NwsCumsum {
	cumsum := make(NwsCumsum, len(code)+1)
	// cumsum[0] is already 0 by default.
	var count uint32
	for i := 0; i < len(code); i++ {
		if code[i] > 32 {
			count++
		}
		cumsum[i+1] = count
	}
	return cumsum
}

// GetNwsCountFromCumsum returns the NWS count for the range [start, end) in
// O(1) using the precomputed cumulative sum array.
func GetNwsCountFromCumsum(cumsum NwsCumsum, start, end int) int {
	return int(cumsum[end]) - int(cumsum[start])
}

// GetNwsCountForNode returns the NWS count for an AST node in O(1) using the
// precomputed cumulative sum array.
func GetNwsCountForNode(node types.SyntaxNode, cumsum NwsCumsum) int {
	return GetNwsCountFromCumsum(cumsum, int(tsNode(node).StartByte()), int(tsNode(node).EndByte()))
}
