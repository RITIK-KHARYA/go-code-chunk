package extract

import (
	"strings"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// importSourceNodeTypes are node types that represent import source/path by language.
var importSourceNodeTypes = []string{
	"string",
	"string_literal",
	"interpreted_string_literal", // Go
	"source",                     // Some grammars use this field name
}

// stripQuotes removes surrounding quotes from a string.
func stripQuotes(s string) string {
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`")) {
		return s[1 : len(s)-1]
	}
	return s
}

// extractRustUsePath returns the import path for a Rust use declaration.
// For 'std::collections::{HashMap, HashSet}' it returns 'std::collections'.
func extractRustUsePath(node *gotreesitter.Node, lang *gotreesitter.Language, code string) string {
	if node == nil {
		return ""
	}

	switch node.Type(lang) {
	case "use_list", "use_wildcard":
		// If it's a use_list, the parent path holds the actual source
		return ""
	case "scoped_use_list":
		// path::{...} -> the scoped_identifier child is the path part
		for _, child := range node.Children() {
			if hasType(child, lang, "scoped_identifier", "identifier") {
				return child.Text([]byte(code))
			}
		}
		if pathChild := node.ChildByFieldName("path", lang); pathChild != nil {
			return pathChild.Text([]byte(code))
		}
		return ""
	case "scoped_identifier":
		// For scoped_identifier, check if the last part is a use_list
		children := node.Children()
		if n := len(children); n > 0 {
			if last := children[n-1]; last != nil && last.Type(lang) == "use_list" {
				if pathChild := node.ChildByFieldName("path", lang); pathChild != nil {
					return pathChild.Text([]byte(code))
				}
			}
		}
		return node.Text([]byte(code))
	default:
		return node.Text([]byte(code))
	}
}

// goImportSpecs returns the import_spec nodes of a Go import_declaration,
// whether from a block (import (...)) or a single import.
func goImportSpecs(node *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node {
	var specs []*gotreesitter.Node
	for _, child := range node.Children() {
		switch child.Type(lang) {
		case "import_spec_list":
			for _, spec := range child.Children() {
				if spec.Type(lang) == "import_spec" {
					specs = append(specs, spec)
				}
			}
		case "import_spec":
			specs = append(specs, child)
		}
	}
	return specs
}

// importSpecPath returns the import path of a Go import_spec, quotes stripped.
func importSpecPath(spec *gotreesitter.Node, lang *gotreesitter.Language, code string) string {
	if pathField := spec.ChildByFieldName("path", lang); pathField != nil {
		return stripQuotes(pathField.Text([]byte(code)))
	}
	if lit := firstNamedChildOfType(spec, lang, "interpreted_string_literal"); lit != nil {
		return stripQuotes(lit.Text([]byte(code)))
	}
	return spec.Text([]byte(code))
}

// importSourceString returns the source string child of a JS/TS import
// statement, quotes stripped, or "" when absent.
func importSourceString(node *gotreesitter.Node, lang *gotreesitter.Language, code string) string {
	for _, child := range node.Children() {
		if child.Type(lang) == "string" {
			return stripQuotes(child.Text([]byte(code)))
		}
	}
	return ""
}

// dottedNameText returns the text of a dotted_name node, descending into
// aliased_import nodes (whose module is their first child).
func dottedNameText(n *gotreesitter.Node, lang *gotreesitter.Language, code string) (string, bool) {
	var dn *gotreesitter.Node
	switch n.Type(lang) {
	case "dotted_name":
		dn = n
	case "aliased_import":
		dn = firstNamedChildOfType(n, lang, "dotted_name")
	}
	if dn == nil {
		return "", false
	}
	return dn.Text([]byte(code)), true
}

// pythonFromImportModule returns the module path of an import_from_statement
// and the remaining child nodes (the imported symbols).
func pythonFromImportModule(node *gotreesitter.Node, lang *gotreesitter.Language, code string) (string, []*gotreesitter.Node) {
	if mn := node.ChildByFieldName("module_name", lang); mn != nil {
		module := mn.Text([]byte(code))
		var body []*gotreesitter.Node
		for _, child := range node.Children() {
			if child == mn {
				continue
			}
			body = append(body, child)
		}
		return module, body
	}

	// Fallback: the first dotted_name child is the module.
	module := ""
	var body []*gotreesitter.Node
	for _, child := range node.Children() {
		if module == "" && child.Type(lang) == "dotted_name" {
			module = child.Text([]byte(code))
			continue
		}
		body = append(body, child)
	}
	return module, body
}

// ExtractImportSource extracts the import source path from an import AST node.
// The bool result reports whether a source was found.
func ExtractImportSource(node *gotreesitter.Node, language types.Language, code string) (string, bool) {
	lang := nativeLang(language)
	if lang == nil {
		return "", false
	}

	// Try the 'source' field first (common in many grammars)
	if sourceField := node.ChildByFieldName("source", lang); sourceField != nil {
		return stripQuotes(sourceField.Text([]byte(code))), true
	}

	switch language {
	case types.LanguageTypeScript, types.LanguageJavaScript:
		// Look for a string literal child (the 'from "..."' part)
		if src := importSourceString(node, lang, code); src != "" {
			return src, true
		}

	case types.LanguagePython:
		// import_from_statement: the module is the source
		if node.Type(lang) == "import_from_statement" {
			if module, _ := pythonFromImportModule(node, lang, code); module != "" {
				return module, true
			}
		}
		// import_statement: the module is the (dotted) import name.
		if name := node.ChildByFieldName("name", lang); name != nil {
			if txt, ok := dottedNameText(name, lang, code); ok {
				return txt, true
			}
			return name.Text([]byte(code)), true
		}
		// Fallback: descend into children for dotted_name / aliased_import
		for _, child := range node.Children() {
			if txt, ok := dottedNameText(child, lang, code); ok {
				return txt, true
			}
		}

	case types.LanguageRust:
		// For 'use path::to::item', extract the path
		if argument := node.ChildByFieldName("argument", lang); argument != nil {
			return extractRustUsePath(argument, lang, code), true
		}
		// Fallback: look for children that could be paths
		for _, child := range node.Children() {
			if hasType(child, lang, "scoped_identifier", "identifier", "use_wildcard") {
				return extractRustUsePath(child, lang, code), true
			}
		}

	case types.LanguageGo:
		// For 'import "path"', look through the import_spec(s)
		for _, spec := range goImportSpecs(node, lang) {
			if path := importSpecPath(spec, lang, code); path != "" {
				return path, true
			}
		}
		// Direct string literal child (some Go grammars)
		for _, child := range node.Children() {
			if child.Type(lang) == "interpreted_string_literal" {
				return stripQuotes(child.Text([]byte(code))), true
			}
		}

	case types.LanguageJava:
		// For 'import package.Class', look for scoped_identifier
		for _, child := range node.Children() {
			if child.Type(lang) == "scoped_identifier" {
				return child.Text([]byte(code)), true
			}
		}
	}

	// Fallback: look for any string-like child
	for _, child := range node.Children() {
		if hasType(child, lang, importSourceNodeTypes...) {
			return stripQuotes(child.Text([]byte(code))), true
		}
	}

	return "", false
}

// importSymbolEntity builds an import entity from a symbol node.
func importSymbolEntity(node *gotreesitter.Node, name, source string) types.ExtractedEntity {
	return types.ExtractedEntity{
		Type:      types.EntityTypeImport,
		Name:      name,
		Signature: name,
		ByteRange: types.ByteRange{Start: int(node.StartByte()), End: int(node.EndByte())},
		LineRange: types.LineRange{Start: int(node.StartPoint().Row), End: int(node.EndPoint().Row)},
		Source:    &source,
		Node:      node,
	}
}

// firstNamedChildOfType returns the first named child of the given node type,
// or nil when absent.
func firstNamedChildOfType(n *gotreesitter.Node, lang *gotreesitter.Language, typ string) *gotreesitter.Node {
	for _, child := range n.Children() {
		if child.IsNamed() && child.Type(lang) == typ {
			return child
		}
	}
	return nil
}

// ExtractImportSymbols extracts one entity per imported symbol from an import
// AST node. Name holds the symbol and Source holds the module path. It
// reconstructs the behavior of the TS extractImportSymbols module.
func ExtractImportSymbols(node *gotreesitter.Node, language types.Language, code string) []types.ExtractedEntity {
	lang := nativeLang(language)
	if lang == nil || node == nil {
		return nil
	}

	switch language {
	case types.LanguageGo:
		return extractGoImportSymbols(node, lang, code)
	case types.LanguagePython:
		return extractPythonImportSymbols(node, lang, code)
	case types.LanguageRust:
		return extractRustImportSymbols(node, lang, code)
	case types.LanguageTypeScript, types.LanguageJavaScript:
		return extractJSImportSymbols(node, lang, code)
	default:
		if source, ok := ExtractImportSource(node, language, code); ok {
			return []types.ExtractedEntity{importSymbolEntity(node, source, source)}
		}
		return nil
	}
}

func extractGoImportSymbols(node *gotreesitter.Node, lang *gotreesitter.Language, code string) []types.ExtractedEntity {
	var entities []types.ExtractedEntity
	for _, spec := range goImportSpecs(node, lang) {
		path := importSpecPath(spec, lang, code)
		name := path
		if nameField := spec.ChildByFieldName("name", lang); nameField != nil {
			name = nameField.Text([]byte(code))
		}
		entities = append(entities, importSymbolEntity(spec, name, path))
	}
	return entities
}

func extractPythonImportSymbols(node *gotreesitter.Node, lang *gotreesitter.Language, code string) []types.ExtractedEntity {
	var entities []types.ExtractedEntity

	switch node.Type(lang) {
	case "import_statement":
		for _, child := range node.Children() {
			if txt, ok := dottedNameText(child, lang, code); ok {
				entities = append(entities, importSymbolEntity(child, txt, txt))
			}
		}

	case "import_from_statement":
		module, body := pythonFromImportModule(node, lang, code)
		for _, child := range body {
			if txt, ok := dottedNameText(child, lang, code); ok {
				entities = append(entities, importSymbolEntity(child, txt, module))
				continue
			}
			if child.Type(lang) == "wildcard_import" {
				entities = append(entities, importSymbolEntity(child, "*", module))
			}
		}
	}

	return entities
}

func extractRustImportSymbols(node *gotreesitter.Node, lang *gotreesitter.Language, code string) []types.ExtractedEntity {
	arg := node.ChildByFieldName("argument", lang)
	if arg == nil {
		for _, child := range node.Children() {
			if child.IsNamed() {
				arg = child
				break
			}
		}
	}
	if arg == nil {
		return nil
	}

	switch arg.Type(lang) {
	case "scoped_use_list":
		path := extractRustUsePath(arg, lang, code)
		var items []*gotreesitter.Node
		for _, child := range arg.Children() {
			if child.Type(lang) == "use_list" {
				items = child.Children()
			}
		}
		var entities []types.ExtractedEntity
		for _, item := range items {
			if !item.IsNamed() {
				continue
			}
			entities = append(entities, importSymbolEntity(item, item.Text([]byte(code)), path))
		}
		if len(entities) == 0 {
			txt := arg.Text([]byte(code))
			entities = append(entities, importSymbolEntity(arg, txt, txt))
		}
		return entities

	case "use_list":
		var entities []types.ExtractedEntity
		for _, item := range arg.Children() {
			if !item.IsNamed() {
				continue
			}
			entities = append(entities, importSymbolEntity(item, item.Text([]byte(code)), ""))
		}
		if len(entities) == 0 {
			txt := arg.Text([]byte(code))
			entities = append(entities, importSymbolEntity(arg, txt, txt))
		}
		return entities

	default:
		txt := arg.Text([]byte(code))
		return []types.ExtractedEntity{importSymbolEntity(arg, txt, txt)}
	}
}

func extractJSImportSymbols(node *gotreesitter.Node, lang *gotreesitter.Language, code string) []types.ExtractedEntity {
	source := importSourceString(node, lang, code)

	var clause *gotreesitter.Node
	for _, child := range node.Children() {
		if child.Type(lang) == "import_clause" {
			clause = child
			break
		}
	}
	if clause == nil {
		if source != "" {
			return []types.ExtractedEntity{importSymbolEntity(node, source, source)}
		}
		return nil
	}

	var entities []types.ExtractedEntity
	for _, child := range clause.Children() {
		switch child.Type(lang) {
		case "named_imports":
			for _, spec := range child.Children() {
				if spec.Type(lang) != "import_specifier" {
					continue
				}
				name := ""
				if nameField := spec.ChildByFieldName("name", lang); nameField != nil {
					name = nameField.Text([]byte(code))
				} else {
					name = spec.Text([]byte(code))
				}
				entities = append(entities, importSymbolEntity(spec, name, source))
			}
		case "namespace_import":
			name := ""
			if id := firstNamedChildOfType(child, lang, "identifier"); id != nil {
				name = id.Text([]byte(code))
			} else {
				name = child.Text([]byte(code))
			}
			entities = append(entities, importSymbolEntity(child, name, source))
		case "identifier":
			entities = append(entities, importSymbolEntity(child, child.Text([]byte(code)), source))
		}
	}
	return entities
}
