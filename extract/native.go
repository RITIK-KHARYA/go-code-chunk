package extract

import (
	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// nativeLang resolves a types.Language to its native tree-sitter grammar.
// It returns nil for unsupported languages; callers must degrade gracefully.
func nativeLang(language types.Language) *gotreesitter.Language {
	lang, _ := parser.GetLanguage(string(language))
	return lang
}

// hasType reports whether the node's type is one of the given types.
func hasType(n *gotreesitter.Node, lang *gotreesitter.Language, nodeTypes ...string) bool {
	if n == nil || lang == nil {
		return false
	}
	t := n.Type(lang)
	for _, want := range nodeTypes {
		if t == want {
			return true
		}
	}
	return false
}

// previousNamedSibling returns the previous named sibling of n, or nil if none.
// It skips anonymous/punctuation sibling nodes.
func previousNamedSibling(n *gotreesitter.Node) *gotreesitter.Node {
	current := n.PrevSibling()
	for current != nil && !current.IsNamed() {
		current = current.PrevSibling()
	}
	return current
}

// namedChildren returns all named children of n.
func namedChildren(n *gotreesitter.Node) []*gotreesitter.Node {
	count := n.NamedChildCount()
	children := make([]*gotreesitter.Node, 0, count)
	for i := 0; i < count; i++ {
		if child := n.NamedChild(i); child != nil {
			children = append(children, child)
		}
	}
	return children
}
