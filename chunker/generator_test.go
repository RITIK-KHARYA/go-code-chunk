package chunker

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// sagaJSSrc and callerJSSrc form a two-file JS project: caller imports and
// calls an exported generator defined in saga. Before the
// generator_function_declaration fix, saga produced only an anonymous export
// entity (excluded from BuildProjectIndex.ByName), so the gen() call in
// caller stayed unresolved; after the fix, saga produces a sibling named
// function entity "gen" that the cross-file project index resolves.
const sagaJSSrc = "export function* gen() {}\n"

const callerJSSrc = "import { gen } from './saga.js'\nfunction caller() {\n  gen();\n}\n"

// mainJSSrc is the single-file form: a non-exported generator declaration
// followed by a caller that uses it. Before the fix the generator was
// silently dropped to zero entities, so caller.Dependencies was empty.
const mainJSSrc = "function* gen() {}\nfunction caller() {\n  gen();\n}\n"

// TestE2EGeneratorCrossFile exercises the full CodeChunker.ChunkBatch path
// (the shared project index built once here, then chunkWithProject per file,
// then enrichEntity -> ResolveDependencies) for the exported-generator form
// across two files. It pins both observable effects of the fix:
//
//  1. BuildProjectIndex now indexes a named "gen" function entity from
//     saga.js (its byte range is the generator declaration's body, [7,25)).
//  2. The caller entity's Dependencies slice resolves the gen() call to
//     that saga.js entity, with the right name/type/signature/filepath.
//
// Before the fix, saga.js produced only an export entity (excluded by
// project.go:51 from ByName), so steps (1) and (2) both fell through:
// ByName had no "gen" and caller.Dependencies was empty.
func TestE2EGeneratorCrossFile(t *testing.T) {
	files := []types.FileInput{
		{Filepath: "saga.js", Code: sagaJSSrc},
		{Filepath: "caller.js", Code: callerJSSrc},
	}
	c := NewCodeChunker()
	results, err := c.ChunkBatch(files, types.BatchOptions{
		ChunkOptions: types.ChunkOptions{Language: types.LanguageJavaScript},
	})
	if err != nil {
		t.Fatalf("ChunkBatch: %v", err)
	}
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("file %s errored: %v", r.Filepath, r.Error)
		}
	}

	// Part A: shared project index must contain the named generator function.
	idx, err := BuildProjectIndex(files, types.DefaultBatchOptions())
	if err != nil {
		t.Fatalf("BuildProjectIndex: %v", err)
	}
	gen := findProjectEntityByName(t, idx, "gen")
	if gen.Entity.Type != types.EntityTypeFunction {
		t.Errorf("ByName[\"gen\"].Entity.Type = %s, want function", gen.Entity.Type)
	}
	if gen.Entity.Name != "gen" {
		t.Errorf("ByName[\"gen\"].Entity.Name = %q, want \"gen\"", gen.Entity.Name)
	}
	if gen.Entity.Signature != "function* gen()" {
		t.Errorf("ByName[\"gen\"].Entity.Signature = %q, want \"function* gen()\"", gen.Entity.Signature)
	}
	if gen.Filepath != "saga.js" {
		t.Errorf("ByName[\"gen\"].Filepath = %q, want \"saga.js\"", gen.Filepath)
	}
	// Byte range of the inner generator_function_declaration in
	// "export function* gen() {}": starts after "export " (7), ends at the
	// declaration's end (25).
	if got, want := gen.Entity.ByteRange, (types.ByteRange{Start: 7, End: 25}); got.Start != want.Start || got.End != want.End {
		t.Errorf("ByName[\"gen\"].Entity.ByteRange = [%d,%d), want [%d,%d)", got.Start, got.End, want.Start, want.End)
	}

	// Part B: the caller entity's dependencies must include the cross-file gen.
	callerEnt := findChunkEntity(t, results, "caller.js", "caller")
	dep, ok := depHas(callerEnt.Dependencies, "gen")
	if !ok {
		t.Fatalf("caller.Dependencies = %+v, want a resolved gen dependency", callerEnt.Dependencies)
	}
	if dep.Type != types.EntityTypeFunction {
		t.Errorf("gen dep Type = %s, want function", dep.Type)
	}
	if dep.Signature != "function* gen()" {
		t.Errorf("gen dep Signature = %q, want \"function* gen()\"", dep.Signature)
	}
	if dep.Filepath != "saga.js" {
		t.Errorf("gen dep Filepath = %q, want \"saga.js\" (cross-file resolution)", dep.Filepath)
	}
}

// TestE2EGeneratorNonExported exercises the same production ChunkBatch path
// on a single file containing a non-exported generator declaration and a
// caller that uses it. Before the fix the generator node type was silently
// skipped, ByName contained no "gen", and the gen() call could not resolve;
// after the fix ByName["gen"] indexes the named function from main.js and
// caller.Dependencies carries the resolved in-file dependency.
func TestE2EGeneratorNonExported(t *testing.T) {
	files := []types.FileInput{
		{Filepath: "main.js", Code: mainJSSrc},
	}
	c := NewCodeChunker()
	results, err := c.ChunkBatch(files, types.BatchOptions{
		ChunkOptions: types.ChunkOptions{Language: types.LanguageJavaScript},
	})
	if err != nil {
		t.Fatalf("ChunkBatch: %v", err)
	}
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("file %s errored: %v", r.Filepath, r.Error)
		}
	}

	idx, err := BuildProjectIndex(files, types.DefaultBatchOptions())
	if err != nil {
		t.Fatalf("BuildProjectIndex: %v", err)
	}
	gen := findProjectEntityByName(t, idx, "gen")
	if gen.Entity.Type != types.EntityTypeFunction {
		t.Errorf("ByName[\"gen\"].Entity.Type = %s, want function", gen.Entity.Type)
	}
	if gen.Entity.Name != "gen" {
		t.Errorf("ByName[\"gen\"].Entity.Name = %q, want \"gen\"", gen.Entity.Name)
	}
	if gen.Entity.Signature != "function* gen()" {
		t.Errorf("ByName[\"gen\"].Entity.Signature = %q, want \"function* gen()\"", gen.Entity.Signature)
	}
	if gen.Filepath != "main.js" {
		t.Errorf("ByName[\"gen\"].Filepath = %q, want \"main.js\"", gen.Filepath)
	}
	// Byte range of "function* gen() {}" is [0,18).
	if got, want := gen.Entity.ByteRange, (types.ByteRange{Start: 0, End: 18}); got.Start != want.Start || got.End != want.End {
		t.Errorf("ByName[\"gen\"].Entity.ByteRange = [%d,%d), want [%d,%d)", got.Start, got.End, want.Start, want.End)
	}

	callerEnt := findChunkEntity(t, results, "main.js", "caller")
	dep, ok := depHas(callerEnt.Dependencies, "gen")
	if !ok {
		t.Fatalf("caller.Dependencies = %+v, want a resolved gen dependency", callerEnt.Dependencies)
	}
	if dep.Type != types.EntityTypeFunction {
		t.Errorf("gen dep Type = %s, want function", dep.Type)
	}
	if dep.Signature != "function* gen()" {
		t.Errorf("gen dep Signature = %q, want \"function* gen()\"", dep.Signature)
	}
	if dep.Filepath != "main.js" {
		t.Errorf("gen dep Filepath = %q, want \"main.js\" (same-file resolution)", dep.Filepath)
	}
}

// findProjectEntityByName returns the single ProjectEntity indexed under the
// given name, failing the test if absent or ambiguous (the generator
// declarations in these fixtures are unique by name).
func findProjectEntityByName(t *testing.T, idx *types.ProjectIndex, name string) types.ProjectEntity {
	t.Helper()
	group, ok := idx.ByName[name]
	if !ok || len(group) == 0 {
		t.Fatalf("ByName[%q] not found; index = %+v", name, idx.ByName)
	}
	if len(group) > 1 {
		t.Fatalf("ByName[%q] has %d entries, want exactly 1: %+v", name, len(group), group)
	}
	return group[0]
}

// findChunkEntity returns the ChunkEntityInfo with the given name from the
// result for the given filepath, failing the test if absent.
func findChunkEntity(t *testing.T, results []types.BatchResult, filepath, name string) types.ChunkEntityInfo {
	t.Helper()
	for _, r := range results {
		if r.Filepath != filepath {
			continue
		}
		for _, ch := range r.Chunks {
			for _, e := range ch.Context.Entities {
				if e.Name == name {
					return e
				}
			}
		}
	}
	t.Fatalf("entity %q not found in %s chunks", name, filepath)
	return types.ChunkEntityInfo{}
}
