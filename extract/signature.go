package extract

import (
	"regexp"
	"strings"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// BODY_DELIMITERS maps a language to the character that marks the start of a body.
var BODY_DELIMITERS = map[types.Language]string{
	types.LanguageTypeScript: "{",
	types.LanguageJavaScript: "{",
	types.LanguagePython:     ":",
	types.LanguageRust:       "{",
	types.LanguageGo:         "{",
	types.LanguageJava:       "{",
}

// bodyNodeTypes are node types that represent body/block structures.
var bodyNodeTypes = []string{
	"block",
	"statement_block",
	"class_body",
	"interface_body",
	"enum_body",
}

// GetBodyDelimiter returns the character that marks the start of a body block.
func GetBodyDelimiter(language types.Language) string {
	return BODY_DELIMITERS[language]
}

// findBodyDelimiterPos finds the position of the body delimiter in text,
// skipping delimiters inside nested brackets/parens/generics and strings.
func findBodyDelimiterPos(text string, delimiter string) int {
	if delimiter == "" {
		return -1
	}

	parenDepth := 0
	bracketDepth := 0
	angleDepth := 0
	inString := false
	var stringChar byte

	for i := 0; i < len(text); i++ {
		char := text[i]
		var prevChar byte
		if i > 0 {
			prevChar = text[i-1]
		}

		// Track string literals to avoid matching inside them
		if (char == '"' || char == '\'' || char == '`') && prevChar != '\\' {
			if !inString {
				inString = true
				stringChar = char
			} else if char == stringChar {
				inString = false
			}
			continue
		}

		if inString {
			continue
		}

		// Track nested structures
		switch char {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '<':
			// Only count as generic bracket if followed by identifier or another <
			// This helps avoid matching comparison operators like <, <=, <<
			if i+1 < len(text) && isAngleOpen(text[i+1]) {
				angleDepth++
			}
		case '>':
			// Only decrement if we're tracking angle brackets
			if angleDepth > 0 {
				angleDepth--
			}
		}

		// Only match delimiter at depth 0
		if char == delimiter[0] && parenDepth == 0 && bracketDepth == 0 && angleDepth == 0 {
			return i
		}
	}

	return -1
}

// isAngleOpen reports whether the char could start an angle/generic context
// (identifier char, `<`, `>`, or a space), mirroring the TS regex /[A-Za-z_<]/.
func isAngleOpen(next byte) bool {
	return next == '_' || next == '<' || next == '>' || next == ' ' ||
		('A' <= next && next <= 'Z') || ('a' <= next && next <= 'z')
}

// tryExtractSignatureFromBody extracts the signature using the AST body field.
// It looks for a 'body' or block-like child and returns everything before it.
// The bool result reports whether a body node was found.
func tryExtractSignatureFromBody(node *gotreesitter.Node, code string, language types.Language, lang *gotreesitter.Language) (string, bool) {
	if lang == nil {
		return "", false
	}

	var bodyNode *gotreesitter.Node
	if bodyNode = node.ChildByFieldName("body", lang); bodyNode == nil {
		for _, child := range node.Children() {
			if hasType(child, lang, bodyNodeTypes...) {
				bodyNode = child
				break
			}
		}
	}
	if bodyNode == nil {
		return "", false
	}

	sig := strings.TrimSpace(code[node.StartByte():bodyNode.StartByte()])

	// For Python, remove trailing colon
	if language == types.LanguagePython && strings.HasSuffix(sig, ":") {
		sig = sig[:len(sig)-1]
	}

	// For arrow functions, remove trailing =>
	if strings.HasSuffix(sig, "=>") {
		sig = strings.TrimSpace(sig[:len(sig)-2])
	}

	return cleanSignature(sig), true
}

// extractFunctionSignature extracts the signature for function/method entities.
// It tries the AST body field first, then falls back to text-based extraction.
func extractFunctionSignature(node *gotreesitter.Node, language types.Language, code string, lang *gotreesitter.Language) string {
	if sig, ok := tryExtractSignatureFromBody(node, code, language, lang); ok {
		return sig
	}

	nodeText := code[node.StartByte():node.EndByte()]
	delimiter := BODY_DELIMITERS[language]
	delimPos := findBodyDelimiterPos(nodeText, delimiter)

	if delimPos == -1 {
		// No body delimiter found - might be a declaration without a body
		return cleanSignature(nodeText)
	}

	return cleanSignature(strings.TrimSpace(nodeText[:delimPos]))
}

// extractClassSignature extracts the signature for class/interface entities.
func extractClassSignature(node *gotreesitter.Node, language types.Language, code string, lang *gotreesitter.Language) string {
	if sig, ok := tryExtractSignatureFromBody(node, code, language, lang); ok {
		return sig
	}

	nodeText := code[node.StartByte():node.EndByte()]
	delimiter := BODY_DELIMITERS[language]
	delimPos := findBodyDelimiterPos(nodeText, delimiter)

	if delimPos == -1 {
		// No body - return first line or full text
		firstNewline := strings.IndexByte(nodeText, '\n')
		if firstNewline != -1 {
			return cleanSignature(nodeText[:firstNewline])
		}
		return cleanSignature(nodeText)
	}

	return cleanSignature(strings.TrimSpace(nodeText[:delimPos]))
}

// extractTypeSignature extracts the signature for type/enum entities.
// It extracts until '=' or '{' (or ':' for Python), whichever comes first.
func extractTypeSignature(node *gotreesitter.Node, language types.Language, code string) string {
	nodeText := code[node.StartByte():node.EndByte()]

	equalsPos := strings.IndexByte(nodeText, '=')
	bracePos := findBodyDelimiterPos(nodeText, "{")
	colonPos := -1
	if language == types.LanguagePython {
		colonPos = findBodyDelimiterPos(nodeText, ":")
	}

	// Find the earliest delimiter
	delimPos := -1
	if equalsPos != -1 {
		delimPos = equalsPos
	}
	if bracePos != -1 && (delimPos == -1 || bracePos < delimPos) {
		delimPos = bracePos
	}
	if colonPos != -1 && (delimPos == -1 || colonPos < delimPos) {
		delimPos = colonPos
	}

	if delimPos == -1 {
		// No delimiter found - return first line or full text
		firstNewline := strings.IndexByte(nodeText, '\n')
		if firstNewline != -1 {
			return cleanSignature(nodeText[:firstNewline])
		}
		return cleanSignature(nodeText)
	}

	return cleanSignature(strings.TrimSpace(nodeText[:delimPos]))
}

// extractImportExportSignature extracts the full import/export statement.
func extractImportExportSignature(node *gotreesitter.Node, code string) string {
	return cleanSignature(code[node.StartByte():node.EndByte()])
}

var (
	lineBreakRe  = regexp.MustCompile(`[\r\n]+`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// cleanSignature normalizes a signature:
// - Collapses multiple whitespace to a single space
// - Normalizes multi-line to single line
// - Trims leading/trailing whitespace
func cleanSignature(sig string) string {
	sig = lineBreakRe.ReplaceAllString(sig, " ")
	sig = whitespaceRe.ReplaceAllString(sig, " ")
	return strings.TrimSpace(sig)
}

// ExtractSignature extracts the signature of an entity from its AST node.
// It mirrors the TS `Effect<string, never>`: it never fails and returns a
// plain string. When the native grammar is unavailable it degrades to
// text-only extraction.
func ExtractSignature(node *gotreesitter.Node, entityType types.EntityType, language types.Language, code string) string {
	lang := nativeLang(language)

	switch entityType {
	case types.EntityTypeFunction, types.EntityTypeMethod:
		return extractFunctionSignature(node, language, code, lang)
	case types.EntityTypeClass, types.EntityTypeInterface:
		return extractClassSignature(node, language, code, lang)
	case types.EntityTypeType, types.EntityTypeEnum:
		return extractTypeSignature(node, language, code)
	case types.EntityTypeImport, types.EntityTypeExport:
		return extractImportExportSignature(node, code)
	default:
		// Fallback: extract first line
		nodeText := code[node.StartByte():node.EndByte()]
		firstNewline := strings.IndexByte(nodeText, '\n')
		if firstNewline != -1 {
			return cleanSignature(nodeText[:firstNewline])
		}
		return cleanSignature(nodeText)
	}
}
