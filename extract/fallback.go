package extract

import (
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// stackItem is a single entry in the iterative traversal stack.
type stackItem struct {
	node       *gotreesitter.Node
	parentName *string
}

// walkAndExtract walks the AST depth-first using an explicit stack (no
// recursion, so deep trees do not overflow the call stack) and appends every
// matched entity to entities. entityNodes de-duplicates node processing so no
// node is visited twice.
func walkAndExtract(rootNode *gotreesitter.Node, language types.Language, lang *gotreesitter.Language, code string, entities *[]types.ExtractedEntity, entityNodes map[*gotreesitter.Node]struct{}) {
	stack := []stackItem{{node: rootNode}}

	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		node := item.node
		if node == nil {
			continue
		}

		nodeType := node.Type(lang)

		if IsEntityNodeType(nodeType, language) {
			if _, seen := entityNodes[node]; seen {
				continue
			}
			entityNodes[node] = struct{}{}

			entityType, ok := GetEntityType(nodeType)
			if !ok {
				continue
			}

			if entityType == types.EntityTypeImport {
				*entities = append(*entities, ExtractImportSymbols(node, language, code)...)
				continue
			}

			name := "<anonymous>"
			if extracted, ok := ExtractName(node, language, code); ok {
				name = extracted
			}

			signature := ExtractSignature(node, entityType, language, code)
			if signature == "" {
				signature = name
			}

			entity := types.ExtractedEntity{
				Type:      entityType,
				Name:      name,
				Signature: signature,
				ByteRange: types.ByteRange{Start: int(node.StartByte()), End: int(node.EndByte())},
				LineRange: types.LineRange{Start: int(node.StartPoint().Row), End: int(node.EndPoint().Row)},
				Parent:    item.parentName,
				Node:      node,
			}

			if docstring, ok := ExtractDocstring(node, language, code); ok {
				entity.Docstring = &docstring
			}

			*entities = append(*entities, entity)

			// Nested entities inherit this entity's name as their parent for
			// class-like scopes; otherwise they inherit ours.
			var newParentName *string
			switch entityType {
			case types.EntityTypeClass, types.EntityTypeInterface, types.EntityTypeFunction, types.EntityTypeMethod:
				newParentName = &name
			default:
				newParentName = item.parentName
			}

			// Push children in reverse order so they are processed in source
			// order (depth-first, left to right).
			children := namedChildren(node)
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, stackItem{node: children[i], parentName: newParentName})
			}
			continue
		}

		children := namedChildren(node)
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, stackItem{node: children[i], parentName: item.parentName})
		}
	}
}

// ExtractEntitiesByNodeTypes extracts entities by matching node types,
// traversing the tree iteratively. This is the fallback used when no query is
// available.
func ExtractEntitiesByNodeTypes(rootNode *gotreesitter.Node, language types.Language, code string) []types.ExtractedEntity {
	lang := nativeLang(language)
	if lang == nil || rootNode == nil {
		return nil
	}

	entities := []types.ExtractedEntity{}
	entityNodes := map[*gotreesitter.Node]struct{}{}
	walkAndExtract(rootNode, language, lang, code, &entities, entityNodes)
	return entities
}
