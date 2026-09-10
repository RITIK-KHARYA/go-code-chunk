package parser

import (
	cerrors "github.com/RITIK-KHARYA/go-code-chunk/errors"
	"github.com/odvcencio/gotreesitter"
)

// Parser wraps a tree-sitter grammar and parser runtime.
type Parser struct {
	lang   *gotreesitter.Language
	parser *gotreesitter.Parser
}

// NewParser creates a parser for the given language name (see GetLanguage).
// It returns UnsupportedLanguageError for languages without a grammar.
func NewParser(language string) (*Parser, error) {
	lang := GetLanguage(language)
	if lang == nil {
		return nil, cerrors.NewUnsupportedLanguageError(language)
	}
	return &Parser{
		lang:   lang,
		parser: gotreesitter.NewParser(lang),
	}, nil
}

// Language returns the underlying tree-sitter grammar.
func (p *Parser) Language() *gotreesitter.Language {
	return p.lang
}

// Parse parses source and returns the syntax tree.
func (p *Parser) Parse(source []byte) (*gotreesitter.Tree, error) {
	return p.parser.Parse(source)
}