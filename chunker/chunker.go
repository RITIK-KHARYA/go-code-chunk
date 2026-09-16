// Package chunker splits source code into semantic chunks with context.
//
// This file is the Go port of the original TS `chunk/index.ts`.
package chunker

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/RITIK-KHARYA/go-code-chunk/extract"
	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/scope"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// tsNode converts a types.SyntaxNode to the underlying *gotreesitter.Node so
// its methods are accessible.
func tsNode(n types.SyntaxNode) *gotreesitter.Node {
	return (*gotreesitter.Node)(n)
}

// ============================================================================
// Errors
// ============================================================================

// ChunkError is returned when chunking fails.
type ChunkError struct {
	Message string
	Cause   error
}

func (e *ChunkError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying cause.
func (e *ChunkError) Unwrap() error {
	return e.Cause
}

// NewChunkError builds a ChunkError with a message and underlying cause.
func NewChunkError(message string, cause error) *ChunkError {
	return &ChunkError{Message: message, Cause: cause}
}

// ============================================================================
// Window assignment
// ============================================================================

// greedyAssignWindows accumulates nodes until maxSize is reached, recursing
// into oversized nodes. It is the Go port of the TS generator
// `greedyAssignWindows`; instead of yielding it returns all windows.
func greedyAssignWindows(nodes []types.SyntaxNode, code string, cumsum NwsCumsum, maxSize int) []types.ASTWindow {
	var windows []types.ASTWindow
	current := types.ASTWindow{Nodes: []types.SyntaxNode{}, Ancestors: []types.SyntaxNode{}}

	for _, node := range nodes {
		nodeSize := GetNwsCountForNode(node, cumsum)

		// Check if node fits in current window
		if current.Size+nodeSize <= maxSize {
			current.Nodes = append(current.Nodes, node)
			current.Size += nodeSize
		} else if nodeSize > maxSize {
			// Node is oversized - need to handle specially
			if len(current.Nodes) > 0 {
				current.Ancestors = GetAncestors(current.Nodes)
				windows = append(windows, current)
				current = types.ASTWindow{Nodes: []types.SyntaxNode{}, Ancestors: []types.SyntaxNode{}}
			}

			// Try to subdivide the node if it has children
			if !IsLeafNode(node) {
				windows = append(windows, greedyAssignWindows(childrenOf(node), code, cumsum, maxSize)...)
			} else {
				// Leaf node that's oversized - split at line boundaries
				windows = append(windows, splitOversizedLeafByLines(node, code, maxSize)...)
			}
		} else {
			// Node doesn't fit but isn't oversized - start new window
			if len(current.Nodes) > 0 {
				current.Ancestors = GetAncestors(current.Nodes)
				windows = append(windows, current)
			}
			current = types.ASTWindow{
				Nodes:     []types.SyntaxNode{node},
				Ancestors: []types.SyntaxNode{},
				Size:      nodeSize,
			}
		}
	}

	// Yield final window if it has content
	if len(current.Nodes) > 0 {
		current.Ancestors = GetAncestors(current.Nodes)
		windows = append(windows, current)
	}

	return windows
}

// splitOversizedLeafByLines splits an oversized leaf node at line boundaries.
// It is the Go port of the TS generator `splitOversizedLeafByLines`.
func splitOversizedLeafByLines(node types.SyntaxNode, code string, maxSize int) []types.ASTWindow {
	startByte := int(tsNode(node).StartByte())
	text := code[startByte:int(tsNode(node).EndByte())]
	lines := strings.Split(text, "\n")

	ancestors := GetAncestors([]types.SyntaxNode{node})
	baseLines := strings.Count(code[:startByte], "\n")

	var windows []types.ASTWindow
	var currentChunk strings.Builder
	currentSize := 0
	beforeChunk := 0   // newlines inside text before the current chunk
	chunkNewlines := 0 // newlines inside the current chunk

	emit := func(last bool) {
		if currentChunk.Len() == 0 {
			return
		}
		start := baseLines + beforeChunk
		end := start + chunkNewlines
		if !last {
			// Non-final chunks end with a trailing newline, whose line is
			// already counted inside the chunk; LineRange.End is inclusive.
			end--
		}
		windows = append(windows, types.ASTWindow{
			Nodes:         []types.SyntaxNode{node},
			Ancestors:     ancestors,
			Size:          currentSize,
			IsPartialNode: new(true),
			LineRanges:    []types.LineRange{{Start: start, End: end}},
		})
		beforeChunk += chunkNewlines
		currentChunk.Reset()
	}

	for i, line := range lines {
		lineNws := CountNws(line)
		lineWithNewline := line
		if i < len(lines)-1 {
			lineWithNewline += "\n"
		}

		if currentSize+lineNws <= maxSize {
			currentChunk.WriteString(lineWithNewline)
			currentSize += lineNws
		} else {
			emit(false)
			currentChunk.WriteString(lineWithNewline)
			currentSize = lineNws
			chunkNewlines = 0
		}

		if i < len(lines)-1 {
			chunkNewlines++
		}
	}

	emit(true)

	return windows
}

// childrenOf returns the direct children of an AST node.
func childrenOf(root types.SyntaxNode) []types.SyntaxNode {
	var children []types.SyntaxNode
	rootNode := tsNode(root)
	for i := 0; i < rootNode.ChildCount(); i++ {
		if child := rootNode.Child(i); child != nil {
			children = append(children, child)
		}
	}
	return children
}

// ============================================================================
// Context
// ============================================================================

// buildContext builds the chunk context from the scope tree. It is the Go
// port of the TS `buildContext`.
func buildContext(text types.RebuiltText, scopeTree types.ScopeTree, options types.ChunkOptions, filepath *string, language *types.Language) types.ChunkContext {
	_ = text.ByteRange // TODO: consumed by context lookups once the context package is wired up

	// TODO(context): wire up the context package once it is implemented.
	//   entities := context.GetEntitiesInRange(br, scopeTree)
	//   scope    := context.GetScopeForRange(br, scopeTree)
	//   siblings := context.GetSiblings(br, scopeTree, context.SiblingsOptions{Detail: string(options.SiblingDetail), MaxSiblings: 3})
	//   imports  := context.GetRelevantImports(entities, scopeTree, options.FilterImports)
	var entities []types.ChunkEntityInfo
	var scopeList []types.EntityInfo
	var siblings []types.SiblingInfo
	var imports []types.ImportInfo

	return types.ChunkContext{
		Filepath: filepath,
		Language: language,
		Scope:    scopeList,
		Entities: entities,
		Siblings: siblings,
		Imports:  imports,
	}
}

// emptyContext returns the context used when contextMode is "none".
func emptyContext() types.ChunkContext {
	return types.ChunkContext{
		Scope:    []types.EntityInfo{},
		Entities: []types.ChunkEntityInfo{},
		Siblings: []types.SiblingInfo{},
		Imports:  []types.ImportInfo{},
	}
}

// resolveOptions merges the caller options with defaults, mirroring the TS
// `{ ...DEFAULT_CHUNK_OPTIONS, ...options, language }` spread. Zero-valued
// fields fall back to defaults.
func resolveOptions(options types.ChunkOptions, language types.Language) types.ChunkOptions {
	opts := types.DefaultChunkOptions()

	if options.MaxChunkSize != 0 {
		opts.MaxChunkSize = options.MaxChunkSize
	}
	if options.ContextMode != "" {
		opts.ContextMode = options.ContextMode
	}
	if options.SiblingDetail != "" {
		opts.SiblingDetail = options.SiblingDetail
	}
	opts.FilterImports = options.FilterImports
	if options.OverlapLines != 0 {
		opts.OverlapLines = options.OverlapLines
	}
	opts.Language = language

	return opts
}

// computeOverlapText returns the trailing overlapLines lines of prevText.
func computeOverlapText(prevText string, overlapLines int) string {
	if prevText == "" || overlapLines <= 0 {
		return ""
	}
	lines := strings.Split(prevText, "\n")
	n := min(overlapLines, len(lines))
	return strings.Join(lines[len(lines)-n:], "\n")
}

// ============================================================================
// Core chunking
// ============================================================================

// processWindows runs the shared chunking pipeline: it preprocesses the NWS
// cumsum, greedily assigns nodes to windows, merges adjacent windows, then
// rebuilds each window into a chunk and yields it, in order, to yield.
// streaming=true reports TotalChunks as -1 (unknown) like the TS generator.
func processWindows(rootNode types.SyntaxNode, code string, scopeTree types.ScopeTree, language types.Language, options types.ChunkOptions, filepath *string, streaming bool, yield func(types.Chunk) error) error {
	opts := resolveOptions(options, language)

	cumsum := PreprocessNwsCumsum(code)
	children := childrenOf(rootNode)
	rawWindows := greedyAssignWindows(children, code, cumsum, opts.MaxChunkSize)
	mergedWindows := MergeAdjacentWindows(rawWindows, MergeOptions{MaxSize: opts.MaxChunkSize})

	totalChunks := len(mergedWindows)
	if streaming {
		totalChunks = -1
	}

	var prevText string
	for i, w := range mergedWindows {
		text := RebuildText(w, code)

		var context types.ChunkContext
		if opts.ContextMode == types.ContextModeNone {
			context = emptyContext()
		} else {
			context = buildContext(text, scopeTree, opts, filepath, &language)
		}

		var overlapText string
		if opts.OverlapLines > 0 && prevText != "" {
			overlapText = computeOverlapText(prevText, opts.OverlapLines)
		}

		chunk := types.Chunk{
			Text:               text.Text,
			ContextualizedText: FormatChunkWithContext(text.Text, context, overlapText),
			ByteRange:          text.ByteRange,
			LineRange:          text.LineRange,
			Context:            context,
			Index:              i,
			TotalChunks:        totalChunks,
		}
		if err := yield(chunk); err != nil {
			return err
		}
		prevText = text.Text
	}
	return nil
}

// ChunkCode chunks source code into pieces with context. It is the Go port of
// the TS `chunk` function and takes pre-parsed input.
func ChunkCode(rootNode types.SyntaxNode, code string, scopeTree types.ScopeTree, language types.Language, options types.ChunkOptions, filepath *string) ([]types.Chunk, error) {
	var chunks []types.Chunk
	err := processWindows(rootNode, code, scopeTree, language, options, filepath, false, func(chunk types.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// StreamChunks streams chunks as they are generated. It is the Go port of the
// TS async generator `streamChunks`. totalChunks is -1 since the total count
// is unknown while streaming.
func StreamChunks(rootNode types.SyntaxNode, code string, scopeTree types.ScopeTree, language types.Language, options types.ChunkOptions, filepath *string) <-chan types.Chunk {
	chunks := make(chan types.Chunk)
	go func() {
		defer close(chunks)
		_ = processWindows(rootNode, code, scopeTree, language, options, filepath, true, func(chunk types.Chunk) error {
			chunks <- chunk
			return nil
		})
	}()
	return chunks
}

// ============================================================================
// Chunker interface implementation
// ============================================================================

// CodeChunker implements types.Chunker by parsing source code and delegating
// to ChunkCode / StreamChunks.
type CodeChunker struct{}

var _ types.Chunker = (*CodeChunker)(nil)

// NewCodeChunker returns a Chunker implementation.
func NewCodeChunker() *CodeChunker {
	return &CodeChunker{}
}

// parseSource detects the language, parses the source and builds the scope tree.
func parseSource(filepath, source string, opts types.ChunkOptions) (types.SyntaxNode, types.ScopeTree, types.Language, error) {
	langName := ""
	if opts.Language != "" {
		langName = string(opts.Language)
	} else {
		langName = parser.DetectLanguage(filepath)
	}
	if langName == "" {
		return nil, types.ScopeTree{}, "", fmt.Errorf("unable to detect language for %q", filepath)
	}

	p, err := parser.NewParser(langName)
	if err != nil {
		return nil, types.ScopeTree{}, "", err
	}
	tree, err := p.Parse([]byte(source))
	if err != nil {
		return nil, types.ScopeTree{}, "", err
	}

	language := types.Language(langName)
	entities := extract.ExtractEntitiesByNodeTypes(tree.RootNode(), language, source)
	scopeTree := scope.BuildScopeTreeFromEntities(entities)

	return types.SyntaxNode(tree.RootNode()), scopeTree, language, nil
}

// Chunk parses the source and chunks it.
func (c *CodeChunker) Chunk(filepath, source string, opts types.ChunkOptions) ([]types.Chunk, error) {
	rootNode, scopeTree, language, err := parseSource(filepath, source, opts)
	if err != nil {
		return nil, NewChunkError("Failed to chunk code", err)
	}
	return ChunkCode(rootNode, source, scopeTree, language, opts, &filepath)
}

// Stream parses the source and streams chunks one by one.
func (c *CodeChunker) Stream(filepath, source string, opts types.ChunkOptions) (<-chan types.Chunk, <-chan error) {
	chunks := make(chan types.Chunk)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errCh)

		rootNode, scopeTree, language, err := parseSource(filepath, source, opts)
		if err != nil {
			errCh <- NewChunkError("Failed to chunk code", err)
			return
		}
		if err := processWindows(rootNode, source, scopeTree, language, opts, &filepath, true, func(chunk types.Chunk) error {
			chunks <- chunk
			return nil
		}); err != nil {
			errCh <- err
		}
	}()

	return chunks, errCh
}

// ChunkBatch chunks multiple files with limited concurrency.
func (c *CodeChunker) ChunkBatch(files []types.FileInput, opts types.BatchOptions) ([]types.BatchResult, error) {
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	results := make([]types.BatchResult, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	var completed atomic.Int32

	for i, f := range files {
		wg.Add(1)
		go func(i int, f types.FileInput) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			fileOpts := opts.ChunkOptions
			if f.Options != nil {
				fileOpts = *f.Options
			}

			chunks, err := c.Chunk(f.Filepath, f.Code, fileOpts)
			results[i] = types.BatchResult{Filepath: f.Filepath, Chunks: chunks, Error: err}

			if opts.OnProgress != nil {
				done := int(completed.Add(1))
				opts.OnProgress(done, len(files), f.Filepath, err == nil)
			}
		}(i, f)
	}

	wg.Wait()
	return results, nil
}

// ChunkBatchStream chunks multiple files and streams results as they complete.
// It honours the same concurrency and progress semantics as ChunkBatch, but
// results arrive in completion order via a channel.
func (c *CodeChunker) ChunkBatchStream(files []types.FileInput, opts types.BatchOptions) (<-chan types.BatchResult, <-chan error) {
	results := make(chan types.BatchResult)
	errCh := make(chan error, 1)

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	go func() {
		defer close(results)
		defer close(errCh)

		var completed atomic.Int32
		sem := make(chan struct{}, concurrency)

		var wg sync.WaitGroup
		for _, f := range files {
			wg.Add(1)
			go func(f types.FileInput) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				fileOpts := opts.ChunkOptions
				if f.Options != nil {
					fileOpts = *f.Options
				}

				chunks, err := c.Chunk(f.Filepath, f.Code, fileOpts)
				results <- types.BatchResult{Filepath: f.Filepath, Chunks: chunks, Error: err}

				if opts.OnProgress != nil {
					done := int(completed.Add(1))
					opts.OnProgress(done, len(files), f.Filepath, err == nil)
				}
			}(f)
		}

		wg.Wait()
	}()

	return results, errCh
}

// ============================================================================
// TEMPORARY STUBS
//
// The declarations below are placeholders so this package compiles standalone.
// They mirror the TS sibling modules. DELETE each block once the real
// implementation is added (each is marked with its target file):
//
//	FormatChunkWithContext   -> context/format
//	buildContext lookups     -> context/*
// ============================================================================

// TODO(context/format.go): replace with the real FormatChunkWithContext from
// context/format.
func FormatChunkWithContext(text string, ctx types.ChunkContext, overlapText string) string {
	return text
}
