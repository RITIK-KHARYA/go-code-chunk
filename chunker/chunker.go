// Package chunker splits source code into semantic chunks with context.
//
// This file is the Go port of the original TS `chunk/index.ts`.
package chunker

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/RITIK-KHARYA/go-code-chunk/context"
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

// groupWithLeadingComments groups a flat list of AST children into atomic
// units: a run of consecutive comment nodes is merged with the next
// non-comment sibling, so a doc comment can never be separated from the
// declaration it documents by a window boundary. A trailing run of comments
// with no following non-comment node forms its own group; non-comment nodes
// each remain their own group. If lang is nil every child is kept as its own
// group (flat fallback).
func groupWithLeadingComments(children []types.SyntaxNode, language types.Language, lang *gotreesitter.Language) [][]types.SyntaxNode {
	var groups [][]types.SyntaxNode
	var run []types.SyntaxNode
	for _, child := range children {
		if isCommentNode(child, language, lang) {
			run = append(run, child)
			continue
		}
		if len(run) > 0 {
			run = append(run, child)
			groups = append(groups, run)
			run = nil
		} else {
			groups = append(groups, []types.SyntaxNode{child})
		}
	}
	if len(run) > 0 {
		groups = append(groups, run)
	}
	return groups
}

// isCommentNode reports whether node's tree-sitter type is a comment type for
// language. Comment type names live in extract.CommentNodeTypes, the same
// per-language table used to attach docstrings to entities.
func isCommentNode(node types.SyntaxNode, language types.Language, lang *gotreesitter.Language) bool {
	if node == nil || lang == nil {
		return false
	}
	return slices.Contains(extract.CommentNodeTypes[language], tsNode(node).Type(lang))
}

// nwsCountForGroup returns the total NWS count of every member of a group,
// treating the group as one indivisible unit.
func nwsCountForGroup(group []types.SyntaxNode, cumsum NwsCumsum) int {
	size := 0
	for _, node := range group {
		size += GetNwsCountForNode(node, cumsum)
	}
	return size
}

// greedyAssignWindows accumulates groups until maxSize is reached, recursing
// into oversized groups. It is the Go port of the TS generator
// `greedyAssignWindows`; instead of yielding it returns all windows. Groups
// come from groupWithLeadingComments, so a comment and the declaration it
// documents are always assigned to windows as one atomic unit: the whole
// group is appended together, never partially. An oversized group flushes the
// current window, then splitOversizedGroup subdivides only its trailing node.
func greedyAssignWindows(nodes []types.SyntaxNode, code string, cumsum NwsCumsum, maxSize int, language types.Language) []types.ASTWindow {
	lang, _ := parser.GetLanguage(string(language))

	var windows []types.ASTWindow
	current := types.ASTWindow{Nodes: []types.SyntaxNode{}, Ancestors: []types.SyntaxNode{}}

	for _, group := range groupWithLeadingComments(nodes, language, lang) {
		groupSize := nwsCountForGroup(group, cumsum)

		// Check if group fits in current window
		if current.Size+groupSize <= maxSize {
			current.Nodes = append(current.Nodes, group...)
			current.Size += groupSize
		} else if groupSize > maxSize {
			// Group is oversized - need to handle specially. The group is
			// atomic, so it is subdivided as a whole rather than per node.
			if len(current.Nodes) > 0 {
				current.Ancestors = GetAncestors(current.Nodes)
				windows = append(windows, current)
				current = types.ASTWindow{Nodes: []types.SyntaxNode{}, Ancestors: []types.SyntaxNode{}}
			}

			windows = append(windows, splitOversizedGroup(group, code, cumsum, maxSize, language)...)
		} else {
			// Group doesn't fit but isn't oversized - start new window
			if len(current.Nodes) > 0 {
				current.Ancestors = GetAncestors(current.Nodes)
				windows = append(windows, current)
			}
			current = types.ASTWindow{
				Nodes:     group,
				Ancestors: []types.SyntaxNode{},
				Size:      groupSize,
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

// splitOversizedGroup subdivides an oversized atomic group while keeping its
// leading comments glued to the declaration they document. The group always
// ends with the non-comment node those comments precede; only that node is
// subdivided (recursing into children, or line-splitting a leaf), and the
// comments are re-attached to the first result window. A partial (line-split)
// window rebuilds text from LineRanges alone, so its first line range is
// expanded upward to cover the comments.
func splitOversizedGroup(group []types.SyntaxNode, code string, cumsum NwsCumsum, maxSize int, language types.Language) []types.ASTWindow {
	leading := group[:len(group)-1]
	top := group[len(group)-1]

	var windows []types.ASTWindow
	if !IsLeafNode(top) {
		windows = greedyAssignWindows(childrenOf(top), code, cumsum, maxSize, language)
	} else {
		windows = splitOversizedLeafByLines(top, code, maxSize)
	}
	if len(windows) == 0 {
		return nil
	}

	first := windows[0]
	window := types.ASTWindow{
		Nodes:         append(append([]types.SyntaxNode{}, leading...), first.Nodes...),
		Ancestors:     first.Ancestors,
		Size:          first.Size + nwsCountForGroup(leading, cumsum),
		IsPartialNode: first.IsPartialNode,
		LineRanges:    first.LineRanges,
	}
	if len(leading) > 0 && isPartialWindow(window) && len(window.LineRanges) > 0 {
		startLine := int(tsNode(leading[0]).StartPoint().Row)
		if startLine < window.LineRanges[0].Start {
			window.LineRanges[0].Start = startLine
		}
	}
	windows[0] = window
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

// buildContext builds the chunk context from the scope tree. Progress, named
// entities are resolved and annotated per-entity: every entity fully contained
// in the chunk gets its OWN Scope/Dependencies/Imports/Siblings computed at
// that entity's byte range (see enrichEntity). The result is a pure data
// struct rendered by chunkcontext.FormatChunkWithContext.
func buildContext(text types.RebuiltText, nodes []types.SyntaxNode, code string, scopeTree types.ScopeTree, options types.ChunkOptions, filepath *string, language *types.Language, project *types.ProjectIndex) types.ChunkContext {
	captures := chunkcontext.ResolveCaptures(text.ByteRange, scopeTree, code, *language)

	raw := entitiesContainedIn(text.ByteRange, scopeTree)
	entities := make([]types.ChunkEntityInfo, 0, len(raw))
	for _, e := range raw {
		entities = append(entities, enrichEntity(e, nodes, code, scopeTree, options, filepath, *language, project))
	}

	return types.ChunkContext{
		Filepath: filepath,
		Language: language,
		Entities: entities,
		Captures: captures,
	}
}

// enrichEntity computes the per-entity annotation data for one entity: its
// scope breadcrumb at the entity's own starting byte offset (NOT the chunk's),
// the repo-local calls made inside its body, the imports its own text uses,
// and its siblings. This is what fixes the old once-per-chunk lookup bug.
func enrichEntity(e types.ExtractedEntity, nodes []types.SyntaxNode, code string, scopeTree types.ScopeTree, options types.ChunkOptions, filepath *string, language types.Language, project *types.ProjectIndex) types.ChunkEntityInfo {
	info := chunkEntityInfoFor(e)

	info.Scope = scopeBreadcrumb(e.ByteRange.Start, scopeTree)
	info.Dependencies = ResolveDependencies(nodes, e.ByteRange, code, project, filepathOrEmpty(filepath), language, e.Name)
	info.Imports = chunkcontext.GetImportsUsedInText(entityText(code, e.ByteRange), scopeTree.Imports)
	info.Siblings = chunkcontext.GetSiblings(e.ByteRange, scopeTree,
		chunkcontext.SiblingOptions{Detail: string(options.SiblingDetail)})

	return info
}

// entityText returns the entity's own source slice, clamped to the file.
func entityText(code string, br types.ByteRange) string {
	start := min(br.Start, len(code))
	end := min(br.End, len(code))
	if start > end {
		start = end
	}
	return code[start:end]
}

// filepathOrEmpty returns the filepath string, or "" when absent.
func filepathOrEmpty(filepath *string) string {
	if filepath == nil {
		return ""
	}
	return *filepath
}

// scopeBreadcrumb returns the breadcrumb of entities enclosing offset, ordered
// outer to inner: the outermost ancestor first and the innermost scope last.
// It is derived from the scope tree, never fabricated.
func scopeBreadcrumb(offset int, scopeTree types.ScopeTree) []types.EntityInfo {
	node := scope.FindScopeAtOffset(scopeTree, offset)
	if node == nil {
		return []types.EntityInfo{}
	}

	// GetAncestorChain walks Parent pointers from the node upward, returning
	// the immediate parent first and the root last. Prepending the innermost
	// node yields an inner-first chain; reverse it into outer-first order.
	chain := []types.EntityInfo{entityInfoFor(node.Entity)}
	for _, ancestor := range scope.GetAncestorChain(node) {
		chain = append(chain, entityInfoFor(ancestor.Entity))
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// entitiesContainedIn returns every code entity in the scope tree whose byte
// range is fully contained within chunkRange. Import and export marker
// entities are excluded: they are not code entities and do not get per-entity
// annotation blocks.
func entitiesContainedIn(chunkRange types.ByteRange, scopeTree types.ScopeTree) []types.ExtractedEntity {
	var result []types.ExtractedEntity
	for _, entity := range scopeTree.AllEntities {
		if entity.Type == types.EntityTypeImport || entity.Type == types.EntityTypeExport {
			continue
		}
		if scope.RangeContains(chunkRange, entity.ByteRange) {
			result = append(result, entity)
		}
	}
	return result
}

// entityInfoFor converts an extracted entity into basic context info.
func entityInfoFor(entity types.ExtractedEntity) types.EntityInfo {
	info := types.EntityInfo{Name: entity.Name, Type: entity.Type}
	if entity.Signature != "" {
		sig := entity.Signature
		info.Signature = &sig
	}
	return info
}

// chunkEntityInfoFor converts an extracted entity into chunk-info form. A
// fully contained entity is never partial.
func chunkEntityInfoFor(entity types.ExtractedEntity) types.ChunkEntityInfo {
	info := types.ChunkEntityInfo{
		EntityInfo: entityInfoFor(entity),
		IsPartial:  false,
	}
	if entity.Docstring != nil {
		info.Docstring = entity.Docstring
	}
	lineRange := entity.LineRange
	info.LineRange = &lineRange
	return info
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
func processWindows(rootNode types.SyntaxNode, code string, scopeTree types.ScopeTree, language types.Language, options types.ChunkOptions, filepath *string, project *types.ProjectIndex, streaming bool, yield func(types.Chunk) error) error {
	opts := resolveOptions(options, language)

	cumsum := PreprocessNwsCumsum(code)
	children := childrenOf(rootNode)
	rawWindows := greedyAssignWindows(children, code, cumsum, opts.MaxChunkSize, language)
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
			// "none" keeps only the always-present file/language header.
			context = types.ChunkContext{Filepath: filepath, Language: &language}
		} else {
			context = buildContext(text, w.Nodes, code, scopeTree, opts, filepath, &language, project)
		}

		if opts.OverlapLines > 0 && prevText != "" {
			context.OverlapText = computeOverlapText(prevText, opts.OverlapLines)
		}

		chunk := types.Chunk{
			Text:        text.Text,
			ByteRange:   text.ByteRange,
			LineRange:   text.LineRange,
			Context:     context,
			Index:       i,
			TotalChunks: totalChunks,
		}
		chunk.ContextualizedText = chunkcontext.FormatChunkWithContext(chunk, chunk.Context)
		if err := yield(chunk); err != nil {
			return err
		}
		prevText = text.Text
	}
	return nil
}

// ChunkCode chunks source code into pieces with context. It is the Go port of
// the TS `chunk` function and takes pre-parsed input. Dependency resolution
// uses a single-file project index (this file only); use ChunkCodeWithProject
// to resolve calls against the whole project.
func ChunkCode(rootNode types.SyntaxNode, code string, scopeTree types.ScopeTree, language types.Language, options types.ChunkOptions, filepath *string) ([]types.Chunk, error) {
	return ChunkCodeWithProject(rootNode, code, scopeTree, language, options, filepath, singleFileIndex(scopeTree, filepathOrEmpty(filepath)))
}

// ChunkCodeWithProject is ChunkCode with an explicit project-wide entity index
// for cross-file dependency resolution. project may be shared across files and
// must be built once per batch (see BuildProjectIndex).
func ChunkCodeWithProject(rootNode types.SyntaxNode, code string, scopeTree types.ScopeTree, language types.Language, options types.ChunkOptions, filepath *string, project *types.ProjectIndex) ([]types.Chunk, error) {
	var chunks []types.Chunk
	err := processWindows(rootNode, code, scopeTree, language, options, filepath, project, false, func(chunk types.Chunk) error {
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
		_ = processWindows(rootNode, code, scopeTree, language, options, filepath, singleFileIndex(scopeTree, filepathOrEmpty(filepath)), true, func(chunk types.Chunk) error {
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
		if err := processWindows(rootNode, source, scopeTree, language, opts, &filepath, singleFileIndex(scopeTree, filepath), true, func(chunk types.Chunk) error {
			chunks <- chunk
			return nil
		}); err != nil {
			errCh <- err
		}
	}()

	return chunks, errCh
}

// chunkWithProject parses a single file and chunks it against a shared
// project-wide entity index.
func (c *CodeChunker) chunkWithProject(filepath, source string, opts types.ChunkOptions, project *types.ProjectIndex) ([]types.Chunk, error) {
	rootNode, scopeTree, language, err := parseSource(filepath, source, opts)
	if err != nil {
		return nil, NewChunkError("Failed to chunk code", err)
	}
	return ChunkCodeWithProject(rootNode, source, scopeTree, language, opts, &filepath, project)
}

// ChunkBatch chunks multiple files with limited concurrency. The project-wide
// entity index covering every file is built ONCE here and shared by all
// workers, so a call in one file can resolve an entity defined in another.
func (c *CodeChunker) ChunkBatch(files []types.FileInput, opts types.BatchOptions) ([]types.BatchResult, error) {
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	project, err := BuildProjectIndex(files, opts)
	if err != nil {
		return nil, err
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

			chunks, err := c.chunkWithProject(f.Filepath, f.Code, fileOpts, project)
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

	project, err := BuildProjectIndex(files, opts)
	if err != nil {
		errCh <- err
		close(results)
		close(errCh)
		return results, errCh
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

				chunks, err := c.chunkWithProject(f.Filepath, f.Code, fileOpts, project)
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
