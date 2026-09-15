package extract

import (
	"fmt"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// LoadQuery returns the compiled entity-extraction query for a language,
// compiling and caching it on first use. It returns (nil, nil) when no query
// exists for the language.
func LoadQuery(language types.Language) (CompiledQuery, error) {
	queryCacheMu.RLock()
	cached := queryCache[language]
	queryCacheMu.RUnlock()

	if cached != nil {
		return cached, nil
	}

	pattern, ok := QueryPatterns[language]
	if !ok {
		return nil, nil
	}

	lang, err := parser.GetLanguage(string(language))
	if err != nil {
		return nil, &QueryLoadError{
			Language: language,
			Message:  "failed to load grammar: " + err.Error(),
			Cause:    err,
		}
	}
	if lang == nil {
		return nil, &QueryLoadError{
			Language: language,
			Message:  "no grammar available for language " + string(language),
		}
	}

	q, err := gotreesitter.NewQuery(pattern, lang)
	if err != nil {
		return nil, &QueryLoadError{
			Language: language,
			Message:  fmt.Sprintf("failed to compile query: %s", err.Error()),
			Cause:    err,
		}
	}

	queryCacheMu.Lock()
	queryCache[language] = q
	queryCacheMu.Unlock()
	return q, nil
}

// LoadQuerySync returns the cached query for a language, or nil if it has not
// been loaded yet.
func LoadQuerySync(language types.Language) CompiledQuery {
	queryCacheMu.RLock()
	defer queryCacheMu.RUnlock()
	return queryCache[language]
}

// ClearQueryCache drops all compiled queries from the cache.
func ClearQueryCache() {
	queryCacheMu.Lock()
	queryCache = map[types.Language]CompiledQuery{}
	queryCacheMu.Unlock()
}

// ExecuteQuery runs the query against a tree, starting from startNode (or the
// tree root when nil), and returns the flattened result.
func ExecuteQuery(query CompiledQuery, tree *gotreesitter.Tree, startNode *gotreesitter.Node) (result QueryResult, err error) {
	if query == nil || tree == nil {
		return QueryResult{}, nil
	}

	defer func() {
		if r := recover(); r != nil {
			var cause error
			var ok bool
			if cause, ok = r.(error); !ok {
				cause = fmt.Errorf("%v", r)
			}
			err = &QueryExecutionError{
				Message: cause.Error(),
				Cause:   cause,
			}
		}
	}()

	node := startNode
	if node == nil {
		node = tree.RootNode()
	}

	matches := query.ExecuteNode(node, tree.Language(), tree.Source())

	out := make([]QueryMatch, 0, len(matches))
	all := make([]QueryCapture, 0, len(matches))
	for _, m := range matches {
		caps := make([]QueryCapture, 0, len(m.Captures))
		for _, c := range m.Captures {
			caps = append(caps, QueryCapture{
				Name:         c.Name,
				Node:         c.Node,
				PatternIndex: m.PatternIndex,
			})
		}
		out = append(out, QueryMatch{PatternIndex: m.PatternIndex, Captures: caps})
		all = append(all, caps...)
	}
	return QueryResult{Matches: out, Captures: all}, nil
}

// GetCapturesByName returns all captures with the given name.
func GetCapturesByName(result QueryResult, captureName string) []QueryCapture {
	var out []QueryCapture
	for _, c := range result.Captures {
		if c.Name == captureName {
			out = append(out, c)
		}
	}
	return out
}

// GetEntityMatches returns the matches that captured an entity (@item).
func GetEntityMatches(result QueryResult) []QueryMatch {
	var out []QueryMatch
	for _, m := range result.Matches {
		for _, c := range m.Captures {
			if c.Name == "item" {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// ExtractEntityFromMatch extracts the structured entity view of a match. It
// returns false when the match has no @item or @name capture.
func ExtractEntityFromMatch(match QueryMatch) (EntityExtraction, bool) {
	var itemNode *gotreesitter.Node
	var nameNode *gotreesitter.Node
	for _, c := range match.Captures {
		switch c.Name {
		case "item":
			if itemNode == nil {
				itemNode = c.Node
			}
		case "name":
			if nameNode == nil {
				nameNode = c.Node
			}
		}
	}
	if itemNode == nil || nameNode == nil {
		return EntityExtraction{}, false
	}

	extraction := EntityExtraction{ItemNode: itemNode, NameNode: nameNode}
	for _, c := range match.Captures {
		switch c.Name {
		case "context":
			extraction.ContextNodes = append(extraction.ContextNodes, c.Node)
		case "annotation":
			extraction.AnnotationNodes = append(extraction.AnnotationNodes, c.Node)
		}
	}
	return extraction, true
}

// HasQueryForLanguage reports whether a query pattern exists for the language.
func HasQueryForLanguage(language types.Language) bool {
	_, ok := QueryPatterns[language]
	return ok
}
