package types

type Language string

// ----------------------------------------------------
// RANGES
// ----------------------------------------------------

type LineRange struct {
	start int
	end   int
}

type ByteRange struct {
	Start int // 0-indexed, inclusive
	End   int // 0-indexed, exclusive
}

type EntityType string

const (
	EntityTypeFunction  EntityType = "function"
	EntityTypeMethod    EntityType = "method"
	EntityTypeClass     EntityType = "class"
	EntityTypeInterface EntityType = "interface"
	EntityTypeType      EntityType = "type"
	EntityTypeEnum      EntityType = "enum"
	EntityTypeImport    EntityType = "import"
	EntityTypeExport    EntityType = "export"
)

const (
	LanguageTypeScript Language = "typescript"
	LanguageJavaScript Language = "javascript"
	LanguagePython     Language = "python"
	LanguageRust       Language = "rust"
	LanguageGo         Language = "go"
	LanguageJava       Language = "java"
)

// ============================================================================
// Parsing
// ============================================================================

// ParseError holds error information from parsing.
type ParseError struct {
	Message     string
	Recoverable bool
}

func (e *ParseError) Error() string {
	return e.Message
}

type ParseResult struct {
	Tree   any // replace with sitter type later or binding type
	Errors []*ParseError
}

type SyntaxNode any
type SyntaxTree any

// ============================================================================
// Extracted Entities & Scope
// ============================================================================

// ExtractedEntity is an entity extracted from the AST (function, class, etc.).
type ExtractedEntity struct {
	Type      EntityType
	Name      string
	Signature string
	Docstring *string // nil when absent
	ByteRange ByteRange
	LineRange LineRange
	Parent    *string // nil when top-level
	Node      SyntaxNode
	Source    *string // only for import entities; nil otherwise
}

// ScopeNode is a node in the scope tree.
type ScopeNode struct {
	Entity   ExtractedEntity
	Children []*ScopeNode
	Parent   *ScopeNode
}

// ScopeTree represents the scope hierarchy of a file.
type ScopeTree struct {
	Root        []*ScopeNode
	Imports     []ExtractedEntity
	Exports     []ExtractedEntity
	AllEntities []ExtractedEntity
}

// ============================================================================
// Entity / Sibling / Import Info
// ============================================================================

// EntityInfo holds basic information about an entity for context.
type EntityInfo struct {
	Name      string
	Type      EntityType
	Signature *string // optional
}

// ChunkEntityInfo extends EntityInfo with extra context for entities inside a chunk.
type ChunkEntityInfo struct {
	EntityInfo
	Docstring *string    // optional
	LineRange *LineRange // optional
	IsPartial bool
}

// SiblingInfo holds information about a sibling entity.
type SiblingInfo struct {
	Name     string
	Type     EntityType
	Position SiblingPosition // "before" | "after"
	Distance int
}

type SiblingPosition string

const (
	SiblingPositionBefore SiblingPosition = "before"
	SiblingPositionAfter  SiblingPosition = "after"
)

// ImportInfo holds information about an import statement.
type ImportInfo struct {
	Name        string
	Source      string
	IsDefault   bool
	IsNamespace bool
}

// ============================================================================
// Chunk Context & Chunk
// ============================================================================

// ChunkContext holds contextual information for a chunk.
type ChunkContext struct {
	Filepath   *string // optional
	Language   *Language
	Scope      []EntityInfo
	Entities   []ChunkEntityInfo
	Siblings   []SiblingInfo
	Imports    []ImportInfo
	ParseError *ParseError // optional / recoverable
}

// Chunk is a chunk of source code with context.
type Chunk struct {
	Text               string
	ContextualizedText string // for embeddings / RAG
	ByteRange          ByteRange
	LineRange          LineRange
	Context            ChunkContext
	Index              int // 0-based
	TotalChunks        int
}

// ============================================================================
// Options
// ============================================================================

type ContextMode string

const (
	ContextModeNone    ContextMode = "none"
	ContextModeMinimal ContextMode = "minimal"
	ContextModeFull    ContextMode = "full"
)

type SiblingDetail string

const (
	SiblingDetailNone       SiblingDetail = "none"
	SiblingDetailNames      SiblingDetail = "names"
	SiblingDetailSignatures SiblingDetail = "signatures"
)

// ChunkOptions configures chunking behaviour.
// Zero values are intentional defaults (see DefaultChunkOptions).
type ChunkOptions struct {
	MaxChunkSize  int           // default 1500
	ContextMode   ContextMode   // default "full"
	SiblingDetail SiblingDetail // default "signatures"
	FilterImports bool          // default false
	Language      Language      // empty = auto-detect
	OverlapLines  int           // default 0
}

// DefaultChunkOptions returns sensible defaults matching the original TS.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		MaxChunkSize:  1500,
		ContextMode:   ContextModeFull,
		SiblingDetail: SiblingDetailSignatures,
		// FilterImports: false
		// OverlapLines:  0
		// Language:      ""
	}
}

// ============================================================================
// Chunker Interface
// ============================================================================

// Chunker is the interface for a chunker instance.
type Chunker interface {
	Chunk(filepath, source string, opts ChunkOptions) ([]Chunk, error)

	// Stream yields chunks one by one. In pure Go you typically return
	// a channel or use an iterator-style helper.
	Stream(filepath, source string, opts ChunkOptions) (<-chan Chunk, <-chan error)

	ChunkBatch(files []FileInput, opts BatchOptions) ([]BatchResult, error)

	// ChunkBatchStream is left as a channel-based API for simplicity.
	ChunkBatchStream(files []FileInput, opts BatchOptions) (<-chan BatchResult, <-chan error)
}
