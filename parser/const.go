package parser

import (
	"path/filepath"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var extensions = map[string]string{
	".ts":   "typescript",
	".tsx":  "typescript",
	".mts":  "typescript",
	".cts":  "typescript",
	".js":   "javascript",
	".jsx":  "javascript",
	".mjs":  "javascript",
	".cjs":  "javascript",
	".py":   "python",
	".pyi":  "python",
	".rs":   "rust",
	".go":   "go",
	".java": "java",
}

func DetectLanguage(filePath string) string {
	return extensions[filepath.Ext(filePath)]
}

// GetLanguage returns the native tree-sitter grammar for a language name.
// It returns nil for unsupported languages.
func GetLanguage(language string) *gotreesitter.Language {
	switch language {
	case "typescript":
		return grammars.TsxLanguage()
	case "javascript":
		return grammars.JavascriptLanguage()
	case "python":
		return grammars.PythonLanguage()
	case "rust":
		return grammars.RustLanguage()
	case "go":
		return grammars.GoLanguage()
	case "java":
		return grammars.JavaLanguage()
	default:
		return nil
	}
}
