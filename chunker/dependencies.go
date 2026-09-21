package chunker

import (
	"github.com/RITIK-KHARYA/go-code-chunk/extract"
	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
	"github.com/odvcencio/gotreesitter"
)

// ResolveDependencies returns the repo-local entities called within the
// chunk's byte range, in call order, de-duplicated by name.
//
// Matching is name-based only against the project-wide index: each call is
// reduced to the bare called name (the method/function part of a dotted name)
// and matched by Name. There is no type resolution, so two same-named entities
// in different scopes are not disambiguated; a call prefers a match defined in
// the calling file and otherwise falls back to the first match across files. A
// single-file run (project built from just that file) legitimately resolves
// only that file's entities — that is correct, not a bug. Import/export
// entities are excluded: they are not repo-local calls. Calls whose node lies
// outside the given byte range are ignored, so a partial (line-split) window
// does not count calls from the rest of the enclosing function.
func ResolveDependencies(nodes []types.SyntaxNode, chunkRange types.ByteRange, code string, project *types.ProjectIndex, chunkFilepath string, language types.Language, enclosingName string) []types.DependencyInfo {
	callTypes := extract.CallNodeTypes[language]
	if len(callTypes) == 0 {
		return nil
	}
	lang, err := parser.GetLanguage(string(language))
	if err != nil || lang == nil {
		return nil
	}
	callTypeSet := make(map[string]bool, len(callTypes))
	for _, t := range callTypes {
		callTypeSet[t] = true
	}
	if project == nil || len(project.ByName) == 0 {
		return nil
	}

	src := []byte(code)
	var deps []types.DependencyInfo
	seen := make(map[string]bool)
	stack := make([]*gotreesitter.Node, 0)

	// Walk each chunk node depth-first with an explicit stack (no recursion),
	// matching the traversal style in extract/fallback.go's walkAndExtract.
	for _, s := range nodes {
		if s == nil {
			continue
		}
		node := (*gotreesitter.Node)(s)
		stack = append(stack, node)
		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if n == nil {
				continue
			}
			if n.StartByte() >= uint32(chunkRange.Start) && n.EndByte() <= uint32(chunkRange.End) && callTypeSet[n.Type(lang)] {
				name := calledName(n, lang, src)
				if name != "" && name != enclosingName && !seen[name] {
					if projectEntity, ok := matchEntity(project, name, chunkFilepath); ok {
						seen[name] = true
						deps = append(deps, toDependency(projectEntity.Entity, projectEntity.Filepath))
					}
				}
			}
			children := n.Children()
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		}
	}
	return deps
}

// matchEntity looks up an entity by name in the project index, preferring a
// definition in the calling file before falling back to the first match in
// index (file) order.
func matchEntity(project *types.ProjectIndex, name, chunkFilepath string) (types.ProjectEntity, bool) {
	group := project.ByName[name]
	if len(group) == 0 {
		return types.ProjectEntity{}, false
	}
	for _, pe := range group {
		if pe.Filepath == chunkFilepath {
			return pe, true
		}
	}
	return group[0], true
}

// calledName extracts the called function/method name from a call node, taking
// the final name of a dotted receiver form (obj.method -> "method").
func calledName(call *gotreesitter.Node, lang *gotreesitter.Language, src []byte) string {
	switch call.Type(lang) {
	case "call": // Python
		fn := call.ChildByFieldName("function", lang)
		if fn == nil {
			return ""
		}
		if fn.Type(lang) == "attribute" {
			if attr := fn.ChildByFieldName("attribute", lang); attr != nil {
				return attr.Text(src)
			}
			return ""
		}
		return fn.Text(src)
	case "method_invocation": // Java
		if name := call.ChildByFieldName("name", lang); name != nil {
			return name.Text(src)
		}
		return ""
	default: // call_expression (Go / TS / JS / Rust)
		fn := call.ChildByFieldName("function", lang)
		if fn == nil {
			return ""
		}
		if fn.Type(lang) == "identifier" {
			return fn.Text(src)
		}
		// selector_expression (Go), member_expression (TS/JS), field_expression
		// (Rust): take the final identifier after the last dot.
		children := fn.Children()
		for i := len(children) - 1; i >= 0; i-- {
			if t := children[i].Type(lang); t == "field_identifier" || t == "property_identifier" {
				return children[i].Text(src)
			}
		}
		for i := len(children) - 1; i >= 0; i-- {
			if children[i].IsNamed() {
				return children[i].Text(src)
			}
		}
		return fn.Text(src)
	}
}

// toDependency narrows an extracted entity to the context dependency form.
func toDependency(e types.ExtractedEntity, filepath string) types.DependencyInfo {
	return types.DependencyInfo{
		Name:      e.Name,
		Type:      e.Type,
		Signature: e.Signature,
		Docstring: e.Docstring,
		Filepath:  filepath,
	}
}
