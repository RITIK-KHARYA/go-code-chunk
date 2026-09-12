package parser

import (
	"path/filepath"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type GrammerLoadError struct {
	lang  string
	cause error
}

func (e *GrammerLoadError) Error() string {
	return "failed to load grammar for " + e.lang + ": " + e.cause.Error()
}

var (
	GrammerCacheMutex sync.RWMutex
	GrammerCache      = map[string]*gotreesitter.Language{}
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
func LoadLanguage(language string) (*gotreesitter.Language, error) {
	switch language {
	case "typescript":
		return grammars.TsxLanguage(), nil
	case "javascript":
		return grammars.JavascriptLanguage(), nil
	case "python":
		return grammars.PythonLanguage(), nil
	case "rust":
		return grammars.RustLanguage(), nil
	case "go":
		return grammars.GoLanguage(), nil
	case "java":
		return grammars.JavaLanguage(), nil
	default:
		return nil, nil
	}
}

func GetLanguage(language string) (*gotreesitter.Language, error) {
	GrammerCacheMutex.RLock()
	cached, ok := GrammerCache[language]
	GrammerCacheMutex.RUnlock()

	if ok {
		return cached, nil
	}

	GrammerCacheMutex.Lock()
	defer GrammerCacheMutex.Unlock()

	// Double-check: another goroutine may have loaded it while we waited.
	if cached, ok := GrammerCache[language]; ok {
		return cached, nil
	}

	lang, err := LoadLanguage(language)
	if err != nil {
		return nil, err
	}
	if lang == nil {
		return nil, nil
	}
	GrammerCache[language] = lang
	return lang, nil
}

// for testing and debugging only
// creating function that will handle the empty cache

func ClearCache() {
	GrammerCacheMutex.Lock()
	defer GrammerCacheMutex.Unlock()
	GrammerCache = map[string]*gotreesitter.Language{}
}
