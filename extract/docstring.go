package extract

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// CommentNodeTypes are the comment node types by language. Python also uses
// string literals as docstrings.
var CommentNodeTypes = map[types.Language][]string{
	types.LanguageTypeScript: {"comment", "multiline_comment"},
	types.LanguageJavaScript: {"comment", "multiline_comment"},
	types.LanguagePython:     {"comment", "string"},
	types.LanguageRust:       {"line_comment", "block_comment"},
	types.LanguageGo:         {"comment"},
	types.LanguageJava:       {"line_comment", "block_comment"},
}

// pythonStringTypes are Python docstring node types (triple-quoted strings).
var pythonStringTypes = []string{"string", "string_content"}

var jsDocStartRe = regexp.MustCompile(`^/\*\*[^*]`)

// IsDocComment reports whether a comment is a documentation comment
// (JSDoc, docstring, etc.).
func IsDocComment(commentText string, language types.Language) bool {
	trimmed := strings.TrimSpace(commentText)

	switch language {
	case types.LanguageTypeScript, types.LanguageJavaScript, types.LanguageJava:
		// JSDoc/Javadoc: starts with /** (but not /***+)
		return jsDocStartRe.MatchString(trimmed) || trimmed == "/**/"

	case types.LanguagePython:
		// Python docstrings: triple quotes
		return strings.HasPrefix(trimmed, `"""`) ||
			strings.HasPrefix(trimmed, `'''`) ||
			strings.HasPrefix(trimmed, `r"""`) ||
			strings.HasPrefix(trimmed, `r'''`)

	case types.LanguageRust:
		// Rust doc comments: /// (outer) or //! (inner)
		return strings.HasPrefix(trimmed, "///") || strings.HasPrefix(trimmed, "//!")

	case types.LanguageGo:
		// Go: any // comment immediately before a declaration is considered doc
		return strings.HasPrefix(trimmed, "//")

	default:
		return false
	}
}

// ParseDocstring parses and cleans up a docstring, removing comment markers
// and normalizing whitespace.
func ParseDocstring(text string, language types.Language) string {
	switch language {
	case types.LanguageTypeScript, types.LanguageJavaScript, types.LanguageJava:
		return parseJSDocStyle(text)
	case types.LanguagePython:
		return parsePythonDocstring(text)
	case types.LanguageRust:
		return parseRustDocComment(text)
	case types.LanguageGo:
		return parseGoComment(text)
	default:
		return strings.TrimSpace(text)
	}
}

// parseJSDocStyle parses /** ... */ style comments.
func parseJSDocStyle(text string) string {
	content := strings.TrimSpace(text)

	// Remove opening /** and closing */
	if strings.HasPrefix(content, "/**") {
		content = content[3:]
	}
	if strings.HasSuffix(content, "*/") {
		content = content[:len(content)-2]
	}

	// Split into lines and process each
	lines := strings.Split(content, "\n")
	processed := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Remove leading * from each line (common JSDoc style)
		if strings.HasPrefix(line, "*") {
			line = line[1:]
			// Remove one space after * if present
			if strings.HasPrefix(line, " ") {
				line = line[1:]
			}
		}
		processed = append(processed, line)
	}

	return strings.Join(trimEmptyLines(processed), "\n")
}

// parsePythonDocstring parses ”' ... ”' and """ ... """ docstrings.
func parsePythonDocstring(text string) string {
	content := strings.TrimSpace(text)

	// Handle raw strings
	if strings.HasPrefix(content, "r\"\"\"") || strings.HasPrefix(content, "r'''") {
		content = content[1:]
	}

	// Remove opening and closing quotes
	if strings.HasPrefix(content, `"""`) {
		content = content[3:]
		if strings.HasSuffix(content, `"""`) {
			content = content[:len(content)-3]
		}
	} else if strings.HasPrefix(content, "'''") {
		content = content[3:]
		if strings.HasSuffix(content, "'''") {
			content = content[:len(content)-3]
		}
	}

	// Split into lines
	lines := strings.Split(content, "\n")

	// Find minimum indentation (excluding empty lines)
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := leadingWhitespace(line)
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent == -1 {
		minIndent = 0
	}

	// Remove common indentation
	dedented := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			dedented = append(dedented, "")
			continue
		}
		if len(line) >= minIndent {
			line = line[minIndent:]
		}
		dedented = append(dedented, line)
	}

	return strings.Join(trimEmptyLines(dedented), "\n")
}

// parseRustDocComment parses /// and //! doc comments.
func parseRustDocComment(text string) string {
	lines := strings.Split(text, "\n")
	processed := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		content := trimmed

		// Remove /// or //! prefix
		if strings.HasPrefix(trimmed, "///") {
			content = trimmed[3:]
		} else if strings.HasPrefix(trimmed, "//!") {
			content = trimmed[3:]
		}

		// Remove one leading space if present
		if strings.HasPrefix(content, " ") {
			content = content[1:]
		}

		processed = append(processed, content)
	}

	return strings.Join(trimEmptyLines(processed), "\n")
}

// parseGoComment parses // style comments.
func parseGoComment(text string) string {
	lines := strings.Split(text, "\n")
	processed := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		content := trimmed

		// Remove // prefix
		if strings.HasPrefix(trimmed, "//") {
			content = trimmed[2:]
		}

		// Remove one leading space if present
		if strings.HasPrefix(content, " ") {
			content = content[1:]
		}

		processed = append(processed, content)
	}

	return strings.Join(trimEmptyLines(processed), "\n")
}

// trimEmptyLines removes empty lines at the start and end of a slice.
func trimEmptyLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// leadingWhitespace returns the number of leading whitespace characters.
func leadingWhitespace(s string) int {
	count := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			count++
		} else {
			break
		}
	}
	return count
}

// getNodeText returns the source text covered by the node.
func getNodeText(node *gotreesitter.Node, code string) string {
	return code[node.StartByte():node.EndByte()]
}

// findPrecedingComments finds consecutive preceding doc comment nodes.
// The bool result reports whether any doc comment was found.
func findPrecedingComments(node *gotreesitter.Node, language types.Language, code string) (string, bool) {
	lang := nativeLang(language)
	if lang == nil {
		return "", false
	}

	commentTypes := CommentNodeTypes[language]
	var comments []string
	current := previousNamedSibling(node)

	// Walk backwards collecting consecutive comment nodes
	for current != nil {
		nodeType := current.Type(lang)

		if slices.Contains(commentTypes, nodeType) {
			// Trim trailing newline: some grammars (e.g. Rust line_comment)
			// include it in the comment node's range.
			text := strings.TrimRight(getNodeText(current, code), "\r\n")

			// For Python, string literals before a node are not docstrings
			if language == types.LanguagePython && slices.Contains(pythonStringTypes, nodeType) {
				break
			}

			if IsDocComment(text, language) {
				comments = append([]string{text}, comments...) // add to front (walking backwards)
				current = previousNamedSibling(current)
			} else {
				break
			}
		} else {
			break
		}
	}

	if len(comments) == 0 {
		return "", false
	}

	// Combine consecutive comments (for Rust /// style)
	combined := strings.Join(comments, "\n")
	return ParseDocstring(combined, language), true
}

// findPythonDocstring finds the Python docstring (first string literal in the
// function/class body).
func findPythonDocstring(node *gotreesitter.Node, language types.Language, code string) (string, bool) {
	lang := nativeLang(language)
	if lang == nil {
		return "", false
	}

	// Look for a block/body child
	var bodyNode *gotreesitter.Node
	if bodyNode = node.ChildByFieldName("body", lang); bodyNode == nil {
		for _, child := range namedChildren(node) {
			if child.Type(lang) == "block" {
				bodyNode = child
				break
			}
		}
	}
	if bodyNode == nil {
		return "", false
	}

	// Get the first statement in the body
	bodyChildren := namedChildren(bodyNode)
	if len(bodyChildren) == 0 {
		return "", false
	}
	firstChild := bodyChildren[0]

	// Check if it's an expression statement containing a string
	if firstChild.Type(lang) == "expression_statement" {
		exprChildren := namedChildren(firstChild)
		if len(exprChildren) > 0 {
			stringNode := exprChildren[0]
			if slices.Contains(pythonStringTypes, stringNode.Type(lang)) {
				text := getNodeText(stringNode, code)
				if IsDocComment(text, types.LanguagePython) {
					return ParseDocstring(text, types.LanguagePython), true
				}
			}
		}
	}

	// Direct string literal (shouldn't happen in valid Python, but handle it)
	if slices.Contains(pythonStringTypes, firstChild.Type(lang)) {
		text := getNodeText(firstChild, code)
		if IsDocComment(text, types.LanguagePython) {
			return ParseDocstring(text, types.LanguagePython), true
		}
	}

	return "", false
}

// ExtractDocstring extracts the docstring/documentation comment for an entity.
// The bool result reports whether a docstring was found:
//   - JSDoc (/** ... */) for TypeScript/JavaScript/Java
//   - Python docstrings (triple-quoted string as first statement in body)
//   - Rust doc comments (/// and //!)
//   - Go comments (// before declaration)
func ExtractDocstring(node *gotreesitter.Node, language types.Language, code string) (string, bool) {
	// For Python, first check for a docstring inside the body
	if language == types.LanguagePython {
		if docstring, ok := findPythonDocstring(node, language, code); ok {
			return docstring, true
		}
	}

	// Look for preceding comments
	return findPrecedingComments(node, language, code)
}
