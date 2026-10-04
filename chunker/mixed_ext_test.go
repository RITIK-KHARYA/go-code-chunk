package chunker

import (
	"testing"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// jsHelperSrc defines helper via an export; jsCallerSrc calls it. Mirrors the
// bug-report scenario where a mixed-case extension (.Jsx) was silently dropped
// from the project index, so an all-lowercase caller.js could not resolve
// helper (the silent cross-file corruption path).
const jsHelperSrc = `export function helper() { return 42; }
`

const jsCallerSrc = `function caller() { return helper(); }
`

// callerHelperDep scans a batch's caller.js result for the `caller` entity and
// returns its resolved `helper` dependency (ok=false when absent).
func callerHelperDep(results []types.BatchResult) (types.DependencyInfo, bool) {
	for i := range results {
		if results[i].Filepath != "caller.js" {
			continue
		}
		for _, ch := range results[i].Chunks {
			for _, e := range ch.Context.Entities {
				if e.Name == "caller" {
					return depHas(e.Dependencies, "helper")
				}
			}
		}
	}
	return types.DependencyInfo{}, false
}

// TestSilentCrossFileDependencyMiss is the headline regression for the
// mixed-case extension bug: a helper defined in a non-lowercase file must be
// indexed and resolvable by an all-lowercase caller, exactly matching the
// all-lowercase baseline. Before the fix, utils.Jsx was silently skipped from
// the project index, so caller.js resolved 0 dependencies (vs 1 in the
// lowercase run).
func TestSilentCrossFileDependencyMiss(t *testing.T) {
	c := NewCodeChunker()

	mixed := []types.FileInput{
		{Filepath: "utils.Jsx", Code: jsHelperSrc},
		{Filepath: "caller.js", Code: jsCallerSrc},
	}
	lower := []types.FileInput{
		{Filepath: "utils.jsx", Code: jsHelperSrc},
		{Filepath: "caller.js", Code: jsCallerSrc},
	}

	resMixed, err := c.ChunkBatch(mixed, types.BatchOptions{})
	if err != nil {
		t.Fatalf("ChunkBatch mixed: %v", err)
	}
	resLower, err := c.ChunkBatch(lower, types.BatchOptions{})
	if err != nil {
		t.Fatalf("ChunkBatch lower: %v", err)
	}

	for _, r := range resMixed {
		if r.Error != nil {
			t.Errorf("mixed run %s errored: %v", r.Filepath, r.Error)
		}
	}
	for _, r := range resLower {
		if r.Error != nil {
			t.Errorf("lower run %s errored: %v", r.Filepath, r.Error)
		}
	}

	depLower, okLower := callerHelperDep(resLower)
	if !okLower {
		t.Fatalf("lower baseline: caller did not resolve helper dep; baseline assumption failed")
	}
	if depLower.Filepath != "utils.jsx" {
		t.Fatalf("lower baseline helper dep Filepath = %q, want %q", depLower.Filepath, "utils.jsx")
	}

	depMixed, okMixed := callerHelperDep(resMixed)
	if !okMixed {
		t.Fatalf("mixed run: caller did not resolve helper dep; this is the silent cross-file corruption the fix prevents (lowercase baseline resolves it)")
	}
	if depMixed.Filepath != "utils.Jsx" {
		t.Errorf("mixed run helper dep Filepath = %q, want %q", depMixed.Filepath, "utils.Jsx")
	}
}
