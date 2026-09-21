package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RITIK-KHARYA/go-code-chunk/chunker"
	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// sourceExts lists the file extensions the CLI expands when given a directory.
var sourceExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true,
}

func main() {
	jsonOut := flag.Bool("json", false, "output raw JSON")
	contextOut := flag.Bool("context", false, "print contextualized text (file header + per-entity annotations + overlap + code)")
	maxSize := flag.Int("max-size", 0, "max chunk size (0 = default)")
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gochunk <file-or-dir> [...]")
		os.Exit(1)
	}

	files, err := collectFiles(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	c := chunker.NewCodeChunker()
	opts := types.ChunkOptions{}
	if *maxSize > 0 {
		opts.MaxChunkSize = *maxSize
	}

	// A multi-file run builds the project-wide entity index once and passes it
	// down, so calls resolve across files of the same project.
	results, err := c.ChunkBatch(files, types.BatchOptions{ChunkOptions: opts})
	if err != nil {
		fmt.Fprintln(os.Stderr, "chunk error:", err)
		os.Exit(1)
	}

	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(results)
		return
	}

	for _, res := range results {
		if res.Error != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", res.Filepath, res.Error)
			continue
		}
		for _, ch := range res.Chunks {
			fmt.Printf("--- chunk %d/%d (%s, lines %d-%d) ---\n",
				ch.Index+1, ch.TotalChunks, res.Filepath, ch.LineRange.Start, ch.LineRange.End)
			if *contextOut {
				fmt.Println(ch.ContextualizedText)
				continue
			}
			fmt.Println(ch.Text)
		}
	}
}

// collectFiles expands the given paths into file inputs. Directory arguments
// include all recognized source files directly inside them (non-recursive),
// sorted for deterministic output.
func collectFiles(paths []string) ([]types.FileInput, error) {
	var files []types.FileInput

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			code, err := os.ReadFile(p)
			if err != nil {
				return nil, err
			}
			files = append(files, types.FileInput{Filepath: p, Code: string(code)})
			continue
		}

		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if sourceExts[strings.ToLower(filepath.Ext(e.Name()))] {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			fp := filepath.Join(p, name)
			code, err := os.ReadFile(fp)
			if err != nil {
				return nil, err
			}
			files = append(files, types.FileInput{Filepath: fp, Code: string(code)})
		}
	}

	return files, nil
}
