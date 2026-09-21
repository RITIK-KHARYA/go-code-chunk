package chunkcontext

import (
	"strconv"
	"strings"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// FormatChunkWithContext is the single, canonical formatter for contextualized
// chunks. Every output path (ChunkCode, CodeChunker, batches, CLI) goes
// through this one function; there is no parallel formatting logic elsewhere.
//
// The output always begins with the file/language header (even when no other
// context resolved) and then, for every entity fully contained in the chunk,
// injects that entity's own annotation block immediately above its source
// text. Annotation uses "//" comment syntax matching the source file, never
// "#".
func FormatChunkWithContext(chunk types.Chunk, ctx types.ChunkContext) string {
	var b strings.Builder

	filepath := ""
	if ctx.Filepath != nil {
		filepath = *ctx.Filepath
	}
	b.WriteString("// File: ")
	b.WriteString(filepath)
	b.WriteString("\n")

	language := ""
	if ctx.Language != nil {
		language = string(*ctx.Language)
	}
	b.WriteString("// Language: ")
	b.WriteString(language)
	b.WriteString("\n")

	if len(ctx.Captures) > 0 {
		captureNames := make([]string, 0, len(ctx.Captures))
		for _, cv := range ctx.Captures {
			if cv.Type != "" {
				captureNames = append(captureNames, cv.Name+"("+cv.Type+")")
			} else {
				captureNames = append(captureNames, cv.Name)
			}
		}
		b.WriteString("// Captures: ")
		b.WriteString(strings.Join(captureNames, ", "))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if ctx.OverlapText != "" {
		b.WriteString("// ...\n")
		b.WriteString(ctx.OverlapText)
		b.WriteString("\n// ---\n")
	}

	b.WriteString(injectEntityBlocks(chunk.Text, chunk.LineRange.Start, filepath, ctx.Entities))
	return b.String()
}

// injectEntityBlocks inserts each entity's annotation block into text above
// the entity's own source. Blocks climb over the contiguous "//" doc-comment
// run directly above the entity's declaration line so the annotation reads
// above the whole group. Accurate even with nested entities because each block
// is placed at its entity's own line.
func injectEntityBlocks(text string, chunkStartLine int, filepath string, entities []types.ChunkEntityInfo) string {
	if len(entities) == 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	insertions := make(map[int][]string)

	for _, e := range entities {
		if e.LineRange == nil {
			continue
		}
		local := e.LineRange.Start - chunkStartLine
		if local < 0 || local >= len(lines) {
			continue
		}
		block := entityBlockLines(e, filepath)
		if len(block) == 0 {
			continue
		}
		idx := local
		for idx > 0 && isSourceComment(lines[idx-1]) {
			idx--
		}
		insertions[idx] = append(insertions[idx], block...)
	}

	if len(insertions) == 0 {
		return text
	}

	var out strings.Builder
	out.Grow(len(text) + 64*len(insertions))
	for i, line := range lines {
		if blocks, ok := insertions[i]; ok {
			for _, blk := range blocks {
				out.WriteString(blk)
				out.WriteString("\n")
			}
		}
		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

// isSourceComment reports whether the line is a "//" comment line.
func isSourceComment(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), "//")
}

// entityBlockLines renders one entity's annotation block. Lines with no data
// are omitted entirely per the "omit line if none" rule; a valid entity always
// yields at least the Scope line (its breadcrumb includes itself).
func entityBlockLines(e types.ChunkEntityInfo, filepath string) []string {
	var lines []string

	if len(e.Scope) > 0 {
		names := make([]string, 0, len(e.Scope))
		for _, s := range e.Scope {
			names = append(names, s.Name)
		}
		lines = append(lines, "// Scope: "+strings.Join(names, " > "))
	}

	if len(e.Dependencies) > 0 {
		deps := make([]string, 0, len(e.Dependencies))
		for _, d := range e.Dependencies {
			deps = append(deps, dependencyString(d, filepath))
		}
		lines = append(lines, "// Dependencies: "+strings.Join(deps, ", "))
	}

	if len(e.Imports) > 0 {
		imports := make([]string, 0, len(e.Imports))
		for _, im := range e.Imports {
			imports = append(imports, im.Name)
		}
		lines = append(lines, "// Imports used: "+strings.Join(imports, ", "))
	}

	if len(e.Siblings) > 0 {
		sibs := make([]string, 0, len(e.Siblings))
		for _, s := range e.Siblings {
			sibs = append(sibs, siblingString(s))
		}
		lines = append(lines, "// Siblings: "+strings.Join(sibs, ", "))
	}

	return lines
}

// dependencyString renders one dependency. A cross-file dependency (defined in
// a different file than the chunk's own) is annotated with its defining file
// in brackets; a same-file dependency omits it.
func dependencyString(d types.DependencyInfo, filepath string) string {
	s := d.Name
	if d.Signature != "" {
		s += "(...)"
	}
	if d.Filepath != "" && d.Filepath != filepath {
		s += " [" + d.Filepath + "]"
	}
	return s
}

// siblingString renders one sibling entry as "name (position, distance N)".
func siblingString(s types.SiblingInfo) string {
	return s.Name + " (" + string(s.Position) + ", distance " + strconv.Itoa(s.Distance) + ")"
}
