package chunkcontext

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/RITIK-KHARYA/go-code-chunk/parser"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// closureNodeType returns the anonymous-closure node type for a language, or
// "" when closure capture resolution is not yet supported for that language.
func closureNodeType(language types.Language) string {
	switch language {
	case types.LanguageGo:
		return "func_literal"
	default:
		return ""
	}
}

// scopeBoundaryTypes are the node types that introduce a new variable scope.
// Only function-like nodes and blocks are tracked; condition- and loop-clause
// scopes merge into their enclosing block (best-effort).
var scopeBoundaryTypes = map[string]bool{
	"function_declaration": true,
	"method_declaration":   true,
	"func_literal":         true,
	"block":                true,
}

// ResolveCaptures finds the variables captured by closures fully contained
// within chunkRange. A variable is considered captured when it is declared in
// an enclosing scope (param, local, or receiver) that lies outside the closure
// itself, mirroring Go closure semantics. It is best-effort: types are inferred
// heuristically and "" denotes an unknown type. Not yet supported languages
// return nil.
func ResolveCaptures(chunkRange types.ByteRange, scopeTree types.ScopeTree, code string, language types.Language) []types.CapturedVariable {
	closureType := closureNodeType(language)
	if closureType == "" {
		return nil
	}
	lang, err := parser.GetLanguage(string(language))
	if err != nil || lang == nil {
		return nil
	}

	collector := &captureCollector{lang: lang, src: []byte(code), closureType: closureType}
	seen := map[*gotreesitter.Node]bool{}
	for _, root := range scopeTree.Root {
		node := root.Entity.Node
		if node == nil {
			continue
		}
		n := (*gotreesitter.Node)(node)
		if int(n.EndByte()) <= chunkRange.Start || int(n.StartByte()) >= chunkRange.End {
			continue
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		collector.walk(n)
	}

	declByScope := map[*gotreesitter.Node][]captureDecl{}
	for _, d := range collector.decls {
		declByScope[d.scope] = append(declByScope[d.scope], d)
	}

	var byName []types.CapturedVariable
	seenName := map[string]bool{}
	for _, closure := range collector.closures {
		cStart := int(closure.StartByte())
		cEnd := int(closure.EndByte())
		if cStart < chunkRange.Start || cEnd > chunkRange.End {
			continue
		}
		closureSeen := map[string]bool{}
		for _, ref := range collector.refs {
			if int(ref.start) < cStart || int(ref.start) >= cEnd {
				continue
			}
			if closureSeen[ref.name] || seenName[ref.name] {
				continue
			}
			decl, ok := bind(ref, declByScope)
			if !ok {
				continue
			}
			if decl.scope != nil && decl.scope.StartByte() >= closure.StartByte() && decl.scope.EndByte() <= closure.EndByte() {
				continue
			}
			closureSeen[ref.name] = true
			seenName[ref.name] = true
			byName = append(byName, types.CapturedVariable{
				Name:       ref.name,
				Type:       decl.typ,
				DeclaredAt: decl.declaredAt,
			})
		}
	}
	return byName
}

// captureDecl is a variable declaration recorded during collection.
type captureDecl struct {
	name       string
	typ        string
	start      uint32
	scope      *gotreesitter.Node
	declaredAt types.LineRange
}

// captureRef is a variable reference recorded during collection.
type captureRef struct {
	name   string
	start  uint32
	scopes []*gotreesitter.Node // enclosing scope chain, innermost first
}

// bind resolves a reference to the nearest enclosing scope declaring a
// variable of the same name, mirroring lexical shadowing.
func bind(ref captureRef, declByScope map[*gotreesitter.Node][]captureDecl) (captureDecl, bool) {
	for _, scopeNode := range ref.scopes {
		decls, ok := declByScope[scopeNode]
		if !ok {
			continue
		}
		for i := len(decls) - 1; i >= 0; i-- {
			if decls[i].name == ref.name {
				return decls[i], true
			}
		}
	}
	return captureDecl{}, false
}

// captureCollector walks the AST of an enclosing entity collecting
// declarations, references, and closures.
type captureCollector struct {
	lang        *gotreesitter.Language
	src         []byte
	closureType string
	decls       []captureDecl
	refs        []captureRef
	closures    []*gotreesitter.Node
}

// walk traverses the node tree, recording declarations, references, and
// closures (in source order).
func (c *captureCollector) walk(root *gotreesitter.Node) {
	stack := make([]*gotreesitter.Node, 0, 32)
	var rec func(n *gotreesitter.Node)
	rec = func(n *gotreesitter.Node) {
		if n == nil {
			return
		}
		t := n.Type(c.lang)
		if t == c.closureType {
			c.closures = append(c.closures, n)
		}
		if scopeBoundaryTypes[t] {
			stack = append(stack, n)
		}
		c.process(n, stack)
		for _, child := range n.Children() {
			rec(child)
		}
		if scopeBoundaryTypes[t] {
			stack = stack[:len(stack)-1]
		}
	}
	rec(root)
}

// process records declarations and references for a single node.
func (c *captureCollector) process(n *gotreesitter.Node, stack []*gotreesitter.Node) {
	var scope *gotreesitter.Node
	if len(stack) > 0 {
		scope = stack[len(stack)-1]
	}
	switch n.Type(c.lang) {
	case "parameter_declaration":
		typ := c.typeText(n.ChildByFieldName("type", c.lang))
		for child := 0; child < n.ChildCount(); child++ {
			if n.FieldNameForChild(child, c.lang) == "name" {
				c.addDecl(n.Child(child), typ, scope)
			}
		}
	case "var_spec":
		var names []*gotreesitter.Node
		for child := 0; child < n.ChildCount(); child++ {
			if n.FieldNameForChild(child, c.lang) == "name" {
				names = append(names, n.Child(child))
			}
		}
		if len(names) == 0 {
			return
		}
		typ := c.typeText(n.ChildByFieldName("type", c.lang))
		exprs := c.rhsExprs(n.ChildByFieldName("value", c.lang))
		for i, name := range names {
			t := typ
			if t == "" && i < len(exprs) {
				t = c.inferType(exprs[i])
			}
			c.addDecl(name, t, scope)
		}
	case "short_var_declaration":
		left := n.ChildByFieldName("left", c.lang)
		if left == nil {
			return
		}
		var names []*gotreesitter.Node
		for child := 0; child < left.ChildCount(); child++ {
			if left.Child(child).Type(c.lang) == "identifier" {
				names = append(names, left.Child(child))
			}
		}
		if len(names) == 0 {
			return
		}
		exprs := c.rhsExprs(n.ChildByFieldName("right", c.lang))
		for i, name := range names {
			t := ""
			if i < len(exprs) {
				t = c.inferType(exprs[i])
			}
			c.addDecl(name, t, scope)
		}
	case "identifier":
		if c.isDeclName(n) {
			return
		}
		name := n.Text(c.src)
		if name == "_" || name == "" {
			return
		}
		scopes := make([]*gotreesitter.Node, len(stack))
		copy(scopes, stack)
		c.refs = append(c.refs, captureRef{name: name, start: n.StartByte(), scopes: scopes})
	}
}

// addDecl records a declaration unless it is blank or unnamed.
func (c *captureCollector) addDecl(name *gotreesitter.Node, typ string, scope *gotreesitter.Node) {
	if name == nil {
		return
	}
	txt := name.Text(c.src)
	if txt == "_" || txt == "" {
		return
	}
	nameStart := name.StartPoint()
	nameEnd := name.EndPoint()
	c.decls = append(c.decls, captureDecl{
		name:  txt,
		typ:   typ,
		start: name.StartByte(),
		scope: scope,
		declaredAt: types.LineRange{
			Start: int(nameStart.Row),
			End:   int(nameEnd.Row),
		},
	})
}

// isDeclName reports whether the identifier is the name position of a
// declaration construct (parameter, var spec, or short-var left side) rather
// than a reference.
func (c *captureCollector) isDeclName(n *gotreesitter.Node) bool {
	p := n.Parent()
	if p == nil {
		return false
	}
	switch p.Type(c.lang) {
	case "parameter_declaration", "var_spec":
		for child := 0; child < p.ChildCount(); child++ {
			if p.Child(child) == n && p.FieldNameForChild(child, c.lang) == "name" {
				return true
			}
		}
		return false
	case "expression_list":
		gp := p.Parent()
		return gp != nil && gp.Type(c.lang) == "short_var_declaration" && gp.ChildByFieldName("left", c.lang) == p
	}
	return false
}

// typeText returns the source text of an explicit type node, or "".
func (c *captureCollector) typeText(tp *gotreesitter.Node) string {
	if tp == nil {
		return ""
	}
	return tp.Text(c.src)
}

// rhsExprs returns the named expressions in an expression list, or nil.
func (c *captureCollector) rhsExprs(node *gotreesitter.Node) []*gotreesitter.Node {
	if node == nil {
		return nil
	}
	count := node.NamedChildCount()
	exprs := make([]*gotreesitter.Node, 0, count)
	for i := range count {
		if ch := node.NamedChild(i); ch != nil {
			exprs = append(exprs, ch)
		}
	}
	return exprs
}

// inferType derives a best-effort type for an initializer expression.
func (c *captureCollector) inferType(expr *gotreesitter.Node) string {
	if expr == nil {
		return ""
	}
	switch expr.Type(c.lang) {
	case "int_literal", "hex_literal", "octal_literal", "binary_literal", "rune_literal":
		return "int"
	case "float_literal", "hexadecimal_float_literal":
		return "float64"
	case "imaginary_literal":
		return "complex128"
	case "interpreted_string_literal", "raw_string_literal":
		return "string"
	case "unary_expression":
		operand := expr.ChildByFieldName("operand", c.lang)
		if operand == nil {
			return ""
		}
		inner := c.inferType(operand)
		if inner == "" {
			return ""
		}
		if hasPrefixByte(c.src, operand.StartByte()) {
			return "*" + inner
		}
		return inner
	case "composite_literal":
		if tp := expr.ChildByFieldName("type", c.lang); tp != nil {
			return tp.Text(c.src)
		}
		return ""
	case "call_expression":
		fn := expr.ChildByFieldName("function", c.lang)
		if fn == nil {
			return ""
		}
		if fn.Type(c.lang) == "identifier" {
			if t := c.lookupType(fn.Text(c.src), fn.StartByte()); t != "" {
				return t
			}
		}
		return fn.Text(c.src)
	case "identifier":
		return c.lookupType(expr.Text(c.src), expr.StartByte())
	case "type_identifier", "slice_type", "map_type", "array_type", "pointer_type", "qualified_type", "function_type":
		return expr.Text(c.src)
	}
	return ""
}

// hasPrefixByte reports whether the source byte immediately before start is &.
func hasPrefixByte(src []byte, start uint32) bool {
	if start == 0 || int(start-1) >= len(src) {
		return false
	}
	return src[start-1] == '&'
}

// lookupType returns the type of a prior declaration of name that encloses
// offset, or "" when unknown.
func (c *captureCollector) lookupType(name string, offset uint32) string {
	for i := len(c.decls) - 1; i >= 0; i-- {
		d := c.decls[i]
		if d.name != name || d.typ == "" || int(d.start) >= int(offset) {
			continue
		}
		if d.scope == nil || (d.scope.StartByte() <= offset && offset <= d.scope.EndByte()) {
			return d.typ
		}
	}
	return ""
}
