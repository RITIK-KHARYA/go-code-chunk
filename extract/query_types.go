package extract

import (
	"sync"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

var (
	queryCacheMu sync.RWMutex
	queryCache   = map[types.Language]CompiledQuery{}
)

// CompiledQuery is a compiled tree-sitter query.
type CompiledQuery = *gotreesitter.Query

// QueryMatch represents a single pattern match returned by a query.
type QueryMatch struct {
	PatternIndex int
	Captures     []QueryCapture
}

// QueryCapture is a single capture within a query match.
type QueryCapture struct {
	Name         string
	Node         *gotreesitter.Node
	PatternIndex int
}

// QueryResult is the complete result of running a query, containing both
// grouped matches and a flat list of all captures.
type QueryResult struct {
	Matches  []QueryMatch
	Captures []QueryCapture
}

// EntityExtraction is the result of extracting an entity from a query match.
type EntityExtraction struct {
	ItemNode        *gotreesitter.Node
	NameNode        *gotreesitter.Node
	ContextNodes    []*gotreesitter.Node
	AnnotationNodes []*gotreesitter.Node
}
