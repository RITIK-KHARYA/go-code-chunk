package chunker

import (
	"github.com/RITIK-KHARYA/go-code-chunk/extract"
	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// BuildProjectIndex parses and extracts every file once, producing a
// project-wide entity index keyed by entity name. It is meant to be called
// once at the top-level orchestrator (ChunkBatch / ChunkBatchStream) and then
// passed down into buildContext/dependency resolution, not rebuilt per chunk.
// Import and export entities are excluded: they are not repo-local call
// targets.
func BuildProjectIndex(files []types.FileInput, opts types.BatchOptions) (*types.ProjectIndex, error) {
	index := &types.ProjectIndex{ByName: make(map[string][]types.ProjectEntity)}
	for _, f := range files {
		fileOpts := opts.ChunkOptions
		if f.Options != nil {
			fileOpts = *f.Options
		}
		if err := indexFile(index, f.Filepath, f.Code, fileOpts); err != nil {
			return nil, err
		}
	}
	return index, nil
}

// indexFile parses one file and folds its code entities into the index.
func indexFile(index *types.ProjectIndex, filepath, code string, opts types.ChunkOptions) error {
	langName := string(opts.Language)
	if langName == "" {
		langName = parser.DetectLanguage(filepath)
	}
	if langName == "" {
		return nil
	}

	p, err := parser.NewParser(langName)
	if err != nil {
		return err
	}
	tree, err := p.Parse([]byte(code))
	if err != nil {
		return err
	}

	language := types.Language(langName)
	entities := extract.ExtractEntitiesByNodeTypes(tree.RootNode(), language, code)
	for _, e := range entities {
		if e.Type == types.EntityTypeImport || e.Type == types.EntityTypeExport {
			continue
		}
		index.ByName[e.Name] = append(index.ByName[e.Name], types.ProjectEntity{Entity: e, Filepath: filepath})
	}
	return nil
}

// singleFileIndex builds a one-file project index from an already-extracted
// scope tree, used by the single-file entry points (ChunkCode without a
// project, CodeChunker.Chunk/Stream).
func singleFileIndex(scopeTree types.ScopeTree, filepath string) *types.ProjectIndex {
	index := &types.ProjectIndex{ByName: make(map[string][]types.ProjectEntity)}
	for _, e := range scopeTree.AllEntities {
		if e.Type == types.EntityTypeImport || e.Type == types.EntityTypeExport {
			continue
		}
		index.ByName[e.Name] = append(index.ByName[e.Name], types.ProjectEntity{Entity: e, Filepath: filepath})
	}
	return index
}
