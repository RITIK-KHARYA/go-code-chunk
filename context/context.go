// Package chunkcontext computes semantic context for a chunk from the scope
// tree and formats the final contextualized text. It is the Go port of the TS
// `context/index.ts` and `context/format.ts`.
package chunkcontext

import (
	"regexp"

	"github.com/RITIK-KHARYA/go-code-chunk/scope"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// identifierRe matches identifier-like tokens inside signatures for import
// filtering, mirroring the TS /\b[a-zA-Z_$][a-zA-Z0-9_$]*\b/g regex.
var identifierRe = regexp.MustCompile(`\b[a-zA-Z_$][a-zA-Z0-9_$]*\b`)

// GetScopeForRange returns the scope chain enclosing the start of the byte
// range, deepest scope first followed by its ancestors. It is the Go port of
// the TS `getScopeForRange`.
func GetScopeForRange(byteRange types.ByteRange, scopeTree types.ScopeTree) []types.EntityInfo {
	scopeNode := scope.FindScopeAtOffset(scopeTree, byteRange.Start)
	if scopeNode == nil {
		return []types.EntityInfo{}
	}

	chain := []types.EntityInfo{entityInfo(scopeNode.Entity)}
	for _, ancestor := range scope.GetAncestorChain(scopeNode) {
		chain = append(chain, entityInfo(ancestor.Entity))
	}
	return chain
}

// isEntityPartial reports whether the entity overlaps the range but is not
// fully contained within it.
func isEntityPartial(entity types.ExtractedEntity, byteRange types.ByteRange) bool {
	return entity.ByteRange.Start < byteRange.Start || entity.ByteRange.End > byteRange.End
}

// GetEntitiesInRange returns entities whose byte ranges overlap the given
// range, with partial-flag detection. It is the Go port of the TS
// `getEntitiesInRange`.
func GetEntitiesInRange(byteRange types.ByteRange, scopeTree types.ScopeTree) []types.ChunkEntityInfo {
	result := make([]types.ChunkEntityInfo, 0)
	for _, entity := range scopeTree.AllEntities {
		if entity.ByteRange.Start < byteRange.End && entity.ByteRange.End > byteRange.Start {
			result = append(result, chunkEntityInfo(entity, isEntityPartial(entity, byteRange)))
		}
	}
	return result
}

// getImportSource returns the import source from an import entity, or "" when
// absent.
func getImportSource(entity types.ExtractedEntity) string {
	if entity.Source != nil {
		return *entity.Source
	}
	return ""
}

// GetRelevantImports returns the imports relevant to a chunk, optionally
// filtered to those referenced by the chunk's entities. It is the Go port of
// the TS `getRelevantImports`.
func GetRelevantImports(entities []types.ChunkEntityInfo, scopeTree types.ScopeTree, filterImports bool) []types.ImportInfo {
	imports := scopeTree.Imports
	if len(imports) == 0 {
		return []types.ImportInfo{}
	}

	mapToImportInfo := func(entity types.ExtractedEntity) types.ImportInfo {
		return types.ImportInfo{Name: entity.Name, Source: getImportSource(entity)}
	}

	if !filterImports {
		result := make([]types.ImportInfo, 0, len(imports))
		for _, entity := range imports {
			result = append(result, mapToImportInfo(entity))
		}
		return result
	}

	usedNames := map[string]struct{}{}
	for _, entity := range entities {
		usedNames[entity.Name] = struct{}{}
		if entity.Signature != nil {
			for _, id := range identifierRe.FindAllString(*entity.Signature, -1) {
				usedNames[id] = struct{}{}
			}
		}
	}

	result := make([]types.ImportInfo, 0, len(imports))
	for _, importEntity := range imports {
		if _, ok := usedNames[importEntity.Name]; ok {
			result = append(result, mapToImportInfo(importEntity))
		}
	}
	return result
}

// entityInfo builds an EntityInfo with an optional signature pointer.
func entityInfo(entity types.ExtractedEntity) types.EntityInfo {
	info := types.EntityInfo{Name: entity.Name, Type: entity.Type}
	if entity.Signature != "" {
		sig := entity.Signature
		info.Signature = &sig
	}
	return info
}

// chunkEntityInfo builds a ChunkEntityInfo from an entity and its partial flag.
func chunkEntityInfo(entity types.ExtractedEntity, partial bool) types.ChunkEntityInfo {
	info := types.ChunkEntityInfo{
		EntityInfo: entityInfo(entity),
		IsPartial:  partial,
	}
	if entity.Docstring != nil {
		info.Docstring = entity.Docstring
	}
	info.LineRange = &entity.LineRange
	return info
}
