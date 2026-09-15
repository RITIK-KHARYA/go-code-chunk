package extract

import "github.com/RITIK-KHARYA/go-code-chunk/types"

const typescriptQuery = `
(function_declaration
  name: (identifier) @name
  parameters: (formal_parameters) @context
  body: (statement_block) @item)

(method_definition
  name: (property_identifier) @name
  parameters: (formal_parameters) @context
  body: (statement_block) @item)

(class_declaration
  name: (type_identifier) @name
  body: (class_body) @item)

(interface_declaration
  name: (type_identifier) @name
  body: (interface_body) @item)

(type_alias_declaration
  name: (type_identifier) @name
  (_) @item)

(enum_declaration
  name: (identifier) @name
  body: (enum_body) @item)

(import_statement
  source: (string) @name) @item

(lexical_declaration
  (variable_declarator
    name: (identifier) @name) @item)
`

const javascriptQuery = `
(function_declaration
  name: (identifier) @name
  parameters: (formal_parameters) @context
  body: (statement_block) @item)

(method_definition
  name: (property_identifier) @name
  parameters: (formal_parameters) @context
  body: (statement_block) @item)

(class_declaration
  name: (identifier) @name
  body: (class_body) @item)

(import_statement
  source: (string) @name) @item

(lexical_declaration
  (variable_declarator
    name: (identifier) @name) @item)
`

const pythonQuery = `
(function_definition
  name: (identifier) @name
  parameters: (parameters) @context
  body: (block) @item)

(class_definition
  name: (identifier) @name
  body: (block) @item)

(import_statement
  (aliased_import
    name: (dotted_name) @name
    alias: (identifier) @name) @item)

(import_statement
  name: (dotted_name) @name) @item

(import_from_statement
  module_name: (dotted_name) @name) @item
`

const rustQuery = `
(function_item
  name: (identifier) @name
  parameters: (parameters) @context
  body: (block) @item)

(struct_item
  name: (type_identifier) @name
  body: (field_declaration_list) @item)

(enum_item
  name: (type_identifier) @name
  body: (enum_variant_list) @item)

(impl_item
  trait: (type_identifier) @name
  body: (declaration_list) @item)

(type_item
  name: (type_identifier) @name
  (_) @item)

(use_declaration
  argument: (scoped_use_list) @name) @item

(use_declaration
  argument: (scoped_identifier) @name) @item

(use_declaration
  argument: (identifier) @name) @item
`

const goQuery = `
(function_declaration
  name: (identifier) @name
  parameters: (parameter_list) @context
  body: (block) @item)

(method_declaration
  receiver: (parameter_list) @context
  name: (field_identifier) @name
  parameters: (parameter_list) @context
  body: (block) @item)

(type_declaration
  (type_spec
    name: (type_identifier) @name) @item)

(import_declaration
  (import_spec
    path: (interpreted_string_literal) @name) @item)
`

const javaQuery = `
(class_declaration
  name: (identifier) @name
  body: (class_body) @item)

(interface_declaration
  name: (identifier) @name
  body: (interface_body) @item)

(method_declaration
  name: (identifier) @name
  parameters: (formal_parameters) @context
  body: (block) @item)

(import_declaration
  (scoped_identifier) @name) @item
`

// QueryPatterns maps each supported language to its entity-extraction query.
// Patterns capture @item (the entity node), @name (its identifier), @context
// (auxiliary context nodes) and @annotation (doc/annotation nodes).
var QueryPatterns = map[types.Language]string{
	types.LanguageTypeScript: typescriptQuery,
	types.LanguageJavaScript: javascriptQuery,
	types.LanguagePython:     pythonQuery,
	types.LanguageRust:       rustQuery,
	types.LanguageGo:         goQuery,
	types.LanguageJava:       javaQuery,
}
