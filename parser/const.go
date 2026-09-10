package parser

import (
	"fmt"
	"path/filepath"
)

type extension struct {
	name     string
	language string
}

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
	ext := filepath.Ext(filePath)
	if language, ok := extensions[ext]; ok {
		return language
	}
	fmt.Sprint("another languague detect or something off")
	return ""
}

func GetGrammar(language string) string {
	switch language {
	case "typescript":
		return "tree-sitter-typescript/tree-sitter-tsx.wasm"
	case "javascript":
		return "tree-sitter-javascript/tree-sitter-jsx.wasm"
	case "python":
		return "tree-sitter-python/tree-sitter-python.wasm"
	case "rust":
		return "tree-sitter-rust/tree-sitter-rust.wasm"
	case "go":
		return "tree-sitter-go/tree-sitter-go.wasm"
	case "java":
		return "tree-sitter-java/tree-sitter-java.wasm"
	default:
		return ""
	}
}
