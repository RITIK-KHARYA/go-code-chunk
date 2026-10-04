package chunkcontext

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// TestReproDotAndBlankImportBug reproduces the false positives caused by Go dot
// (Name=".") and blank (Name="_") imports slipping past the old guard, which
// only skipped "" and "*". With the bug, \b\.\b matched any "x.y" field/method
// access and \b_\b matched the ubiquitous blank identifier, annotating chunks
// that never reference the imported package. The guard now validates that the
// binding contains at least one letter or digit, so these (and any other
// punctuation-only binding) are skipped like the "*" wildcard.
func TestReproDotAndBlankImportBug(t *testing.T) {
	dot := []types.ExtractedEntity{{Type: types.EntityTypeImport, Name: ".", Source: new("fmt")}}
	blank := []types.ExtractedEntity{{Type: types.EntityTypeImport, Name: "_", Source: new("log")}}
	wildcard := []types.ExtractedEntity{{Type: types.EntityTypeImport, Name: "*", Source: new("utils")}}
	empty := []types.ExtractedEntity{{Type: types.EntityTypeImport, Name: "", Source: new("mod")}}

	// \b\.\b matched every "x.y"; now dot imports are skipped.
	t.Run("dot import not matched by field access", func(t *testing.T) {
		if got := GetImportsUsedInText("v := S{Field: 1}\nreturn v.Field", dot); len(got) != 0 {
			t.Fatalf("dot import = %+v, want empty (no fmt reference)", got)
		}
	})
	t.Run("dot import not matched by qualified access", func(t *testing.T) {
		if got := GetImportsUsedInText("a.b = 1", dot); len(got) != 0 {
			t.Fatalf("dot import = %+v, want empty", got)
		}
	})

	// \b_\b matched the blank identifier; now blank imports are skipped.
	t.Run("blank import not matched by blank identifier", func(t *testing.T) {
		if got := GetImportsUsedInText("_, err := doSomething()\n_ = err", blank); len(got) != 0 {
			t.Fatalf("blank import = %+v, want empty (no log reference)", got)
		}
	})

	// Regression guards for the previously-handled sentinels.
	t.Run("wildcard import still skipped", func(t *testing.T) {
		if got := GetImportsUsedInText("utils.helper(2)", wildcard); len(got) != 0 {
			t.Fatalf("wildcard import = %+v, want empty", got)
		}
	})
	t.Run("empty-name default import still skipped", func(t *testing.T) {
		if got := GetImportsUsedInText("mod.Foo()", empty); len(got) != 0 {
			t.Fatalf("empty-name import = %+v, want empty", got)
		}
	})

	// A real binding must still be reported even with discarded siblings present.
	t.Run("real binding still matches alongside skipped ones", func(t *testing.T) {
		imports := []types.ExtractedEntity{
			{Type: types.EntityTypeImport, Name: ".", Source: new("fmt")},
			{Type: types.EntityTypeImport, Name: "_", Source: new("log")},
			{Type: types.EntityTypeImport, Name: "*", Source: new("x")},
			{Type: types.EntityTypeImport, Name: "strings", Source: new("strings")},
		}
		got := GetImportsUsedInText("s := strings.TrimSpace(parts[0])", imports)
		if len(got) != 1 || got[0].Name != "strings" {
			t.Fatalf("imports = %+v, want [strings] only", got)
		}
	})

	// Bindings containing identifier characters stay matchable, including the
	// dotted Python module name (must not be broken by the new guard).
	t.Run("dotted python binding still works", func(t *testing.T) {
		imports := []types.ExtractedEntity{{Type: types.EntityTypeImport, Name: "os.path", Source: new("os.path")}}
		got := GetImportsUsedInText("return os.path.join(a, b)", imports)
		if len(got) != 1 || got[0].Name != "os.path" {
			t.Fatalf("os.path = %+v, want [os.path]", got)
		}
	})

	// An underscore-prefixed real binding (not the bare "_" blank) still matches
	// because it contains letter characters.
	t.Run("underscore-prefixed real binding still matches", func(t *testing.T) {
		imports := []types.ExtractedEntity{{Type: types.EntityTypeImport, Name: "_foo", Source: new("mod")}}
		got := GetImportsUsedInText("x := _foo.Bar()", imports)
		if len(got) != 1 || got[0].Name != "_foo" {
			t.Fatalf("_foo = %+v, want [_foo]", got)
		}
	})
}
