package extract

import "github.com/RITIK-KHARYA/go-code-chunk/types"

// EntityNodeTypes are the AST node types considered extractable entities per
// language.
var EntityNodeTypes = map[types.Language][]string{
	types.LanguageTypeScript: {
		"function_declaration",
		"method_definition",
		"class_declaration",
		"interface_declaration",
		"type_alias_declaration",
		"enum_declaration",
		"import_statement",
		"export_statement",
	},
	types.LanguageJavaScript: {
		"function_declaration",
		"method_definition",
		"class_declaration",
		"import_statement",
		"export_statement",
	},
	types.LanguagePython: {
		"function_definition",
		"class_definition",
		"import_statement",
		"import_from_statement",
	},
	types.LanguageRust: {
		"function_item",
		"impl_item",
		"struct_item",
		"enum_item",
		"trait_item",
		"type_item",
		"use_declaration",
	},
	types.LanguageGo: {
		"function_declaration",
		"method_declaration",
		"type_declaration",
		"import_declaration",
	},
	types.LanguageJava: {
		"method_declaration",
		"class_declaration",
		"interface_declaration",
		"enum_declaration",
		"import_declaration",
	},
}

// NodeTypeToEntityType maps an AST node type to its entity type.
var NodeTypeToEntityType = map[string]types.EntityType{
	// Functions
	"function_declaration":           types.EntityTypeFunction,
	"function_definition":            types.EntityTypeFunction,
	"function_item":                  types.EntityTypeFunction,
	"generator_function_declaration": types.EntityTypeFunction,
	"arrow_function":                 types.EntityTypeFunction,

	// Methods
	"method_definition":  types.EntityTypeMethod,
	"method_declaration": types.EntityTypeMethod,

	// Classes
	"class_declaration":          types.EntityTypeClass,
	"class_definition":           types.EntityTypeClass,
	"abstract_class_declaration": types.EntityTypeClass,

	// Interfaces
	"interface_declaration": types.EntityTypeInterface,
	"trait_item":            types.EntityTypeInterface,

	// Types
	"type_alias_declaration": types.EntityTypeType,
	"type_item":              types.EntityTypeType,
	"type_declaration":       types.EntityTypeType,
	"struct_item":            types.EntityTypeType,

	// Enums
	"enum_declaration": types.EntityTypeEnum,
	"enum_item":        types.EntityTypeEnum,

	// Imports
	"import_statement":      types.EntityTypeImport,
	"import_declaration":    types.EntityTypeImport,
	"import_from_statement": types.EntityTypeImport,
	"use_declaration":       types.EntityTypeImport,

	// Exports
	"export_statement": types.EntityTypeExport,

	// Impl blocks (Rust - treat as class-like)
	"impl_item": types.EntityTypeClass,
}

// IsEntityNodeType reports whether nodeType represents an entity for the
// given language.
func IsEntityNodeType(nodeType string, language types.Language) bool {
	for _, t := range EntityNodeTypes[language] {
		if t == nodeType {
			return true
		}
	}
	return false
}

// GetEntityType returns the entity type for a node type.
// The bool result reports whether a mapping exists.
func GetEntityType(nodeType string) (types.EntityType, bool) {
	entityType, ok := NodeTypeToEntityType[nodeType]
	return entityType, ok
}
