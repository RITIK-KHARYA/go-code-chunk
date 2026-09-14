package extract

import (
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// nameNodeTypes are node types that represent identifiers/names by language.
// Order matters - first match wins.
var nameNodeTypes = []string{
	"name",
	"identifier",
	"type_identifier",
	"property_identifier",
}

// ExtractName extracts the name of an entity from its AST node.
// The bool result reports whether a name was found (mirrors the TS
// `string | null` return).
func ExtractName(node *gotreesitter.Node, language types.Language, code string) (string, bool) {
	lang := nativeLang(language)
	if lang == nil {
		return "", false
	}

	// Try to find a named child that is an identifier
	for _, nameType := range nameNodeTypes {
		if nameNode := node.ChildByFieldName(nameType, lang); nameNode != nil {
			return nameNode.Text([]byte(code)), true
		}
	}

	// Try to find any child with a name-like type
	for _, child := range node.Children() {
		if hasType(child, lang, nameNodeTypes...) {
			return child.Text([]byte(code)), true
		}
	}

	// For some languages, try the first identifier child
	for _, child := range node.Children() {
		if hasType(child, lang, "identifier", "type_identifier") {
			return child.Text([]byte(code)), true
		}
	}

	return "", false
}
