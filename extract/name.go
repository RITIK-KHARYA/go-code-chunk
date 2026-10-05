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

	// Go type_declaration: the name lives on the grandchild
	// type_spec.name / type_alias.name (type_identifier), neither of which the
	// shallow loops below reach. Only single-spec declarations are named; a
	// grouped `type ( A int; B string )` or `type ( A = int; B = string )`
	// emits one entity for the whole group, so naming it after the first spec
	// would silently mislabel it — leave those as "<anonymous>" via the
	// (false) return below. Both spec kinds (`type_spec` for `type Foo int`
	// and `type_alias` for `type Foo = Bar`) expose a `name` field, so count
	// and pick from either.
	if hasType(node, lang, "type_declaration") {
		specCount := 0
		for _, child := range node.Children() {
			if hasType(child, lang, "type_spec", "type_alias") {
				specCount++
			}
		}
		if specCount == 1 {
			for _, child := range node.Children() {
				if hasType(child, lang, "type_spec", "type_alias") {
					if nameNode := child.ChildByFieldName("name", lang); nameNode != nil {
						return nameNode.Text([]byte(code)), true
					}
				}
			}
		}
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
