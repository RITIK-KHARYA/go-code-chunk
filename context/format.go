package chunkcontext

import (
	"strings"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// FormatChunkWithContext prepends semantic context (file path, scope chain,
// defined signatures, imports, siblings, and optional overlap) to the chunk
// text, producing a contextualized version optimized for embeddings. It is the
// Go port of the TS `formatChunkWithContext` from `context/format.ts`.
func FormatChunkWithContext(text string, ctx types.ChunkContext, overlapText string) string {
	var parts []string

	if ctx.Filepath != nil {
		segments := strings.Split(*ctx.Filepath, "/")
		if n := len(segments); n > 3 {
			segments = segments[n-3:]
		}
		parts = append(parts, "# "+strings.Join(segments, "/"))
	}

	if len(ctx.Scope) > 0 {
		scopeNames := make([]string, 0, len(ctx.Scope))
		for i := len(ctx.Scope) - 1; i >= 0; i-- {
			scopeNames = append(scopeNames, ctx.Scope[i].Name)
		}
		parts = append(parts, "# Scope: "+strings.Join(scopeNames, " > "))
	}

	var signatures []string
	for _, e := range ctx.Entities {
		if e.Signature != nil && e.Type != types.EntityTypeImport {
			signatures = append(signatures, *e.Signature)
		}
	}
	if len(signatures) > 0 {
		parts = append(parts, "# Defines: "+strings.Join(signatures, ", "))
	}

	if len(ctx.Imports) > 0 {
		n := min(10, len(ctx.Imports))
		importNames := make([]string, 0, n)
		for _, im := range ctx.Imports[:n] {
			importNames = append(importNames, im.Name)
		}
		parts = append(parts, "# Uses: "+strings.Join(importNames, ", "))
	}

	var beforeSiblings, afterSiblings []string
	for _, s := range ctx.Siblings {
		switch s.Position {
		case types.SiblingPositionBefore:
			beforeSiblings = append(beforeSiblings, s.Name)
		case types.SiblingPositionAfter:
			afterSiblings = append(afterSiblings, s.Name)
		}
	}
	if len(beforeSiblings) > 0 {
		parts = append(parts, "# After: "+strings.Join(beforeSiblings, ", "))
	}
	if len(afterSiblings) > 0 {
		parts = append(parts, "# Before: "+strings.Join(afterSiblings, ", "))
	}

	if len(parts) > 0 {
		parts = append(parts, "")
	}

	if overlapText != "" {
		parts = append(parts, "# ...", overlapText, "# ---")
	}

	parts = append(parts, text)
	return strings.Join(parts, "\n")
}
