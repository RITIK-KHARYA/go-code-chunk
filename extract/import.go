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
		for _, child := range node.Children() {
			if child.Type(lang) == "string" {
				return stripQuotes(child.Text([]byte(code))), true
			}
		}

	case types.LanguagePython:
		// For 'from X import Y', look for module_name field
		if moduleName := node.ChildByFieldName("module_name", lang); moduleName != nil {
			return moduleName.Text([]byte(code)), true
		}
		// For 'import X' style, try name field first (covers some grammars).
		// On this binding the field may point at an aliased_import; descend into
		// its dotted_name in that case.
		if name := node.ChildByFieldName("name", lang); name != nil {
			if name.Type(lang) == "aliased_import" {
				for _, sub := range name.Children() {
					if sub.Type(lang) == "dotted_name" {
						return sub.Text([]byte(code)), true
					}
				}
			}
			return name.Text([]byte(code)), true
		}
		// Fallback: descend into children for dotted_name / aliased_import
		for _, child := range node.Children() {
			switch child.Type(lang) {
			case "aliased_import":
				for _, sub := range child.Children() {
					if sub.Type(lang) == "dotted_name" {
						return sub.Text([]byte(code)), true
					}
				}
			case "dotted_name":
				return child.Text([]byte(code)), true
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
		// For 'import "path"', look for import_spec or interpreted_string_literal
		for _, child := range node.Children() {
			switch child.Type(lang) {
			case "import_spec":
				// Single import: import "fmt" -> has import_spec child
				if path := child.ChildByFieldName("path", lang); path != nil {
					return stripQuotes(path.Text([]byte(code))), true
				}
				// Fallback: look for string literal in import_spec
				for _, specChild := range child.Children() {
					if specChild.Type(lang) == "interpreted_string_literal" {
						return stripQuotes(specChild.Text([]byte(code))), true
					}
				}
			case "interpreted_string_literal":
				// Direct string literal (some Go grammars)
				return stripQuotes(child.Text([]byte(code))), true
			case "import_spec_list":
				// For import blocks: import ( "fmt" "os" )
				for _, spec := range child.Children() {
					if spec.Type(lang) == "import_spec" {
						if path := spec.ChildByFieldName("path", lang); path != nil {
							return stripQuotes(path.Text([]byte(code))), true
						}
					}
				}
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
