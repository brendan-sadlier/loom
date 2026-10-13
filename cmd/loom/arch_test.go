package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	internalPrefix = "github.com/brendan-sadlier/loom/cmd/loom/internal/"
	runtimeModule  = "github.com/brendan-sadlier/loom"
	cliModule      = "github.com/brendan-sadlier/loom/cmd/loom"
)

// allowed mirrors the dependency table in ARCHITECTURE.md §2.
// nil means "may import any internal package".
var allowed = map[string][]string{
	"syntax":   {},
	"model":    {},
	"eval":     {"syntax", "model"},
	"graph":    {"model"},
	"state":    {"model"},
	"shell":    {},
	"gorecipe": {"model"},
	"execute":  {"graph", "state", "shell", "gorecipe", "model"},
	"watch":    {"model", "graph"},
	"cli":      nil,
}

func TestInternalImportRules(t *testing.T) {
	entries, err := os.ReadDir("internal")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkg := e.Name()
		rules, known := allowed[pkg]
		if !known {
			t.Errorf("internal/%s is not in ARCHITECTURE.md §2; add it to the table and to this test", pkg)
			continue
		}
		for _, imp := range importsOf(t, filepath.Join("internal", pkg)) {
			if imp == runtimeModule || (strings.HasPrefix(imp, runtimeModule+"/") && !strings.HasPrefix(imp, cliModule+"/")) {
				t.Errorf("internal/%s imports the runtime module %q; the CLI must not depend on it", pkg, imp)
			}
			if !strings.HasPrefix(imp, internalPrefix) {
				continue
			}
			dep, _, _ := strings.Cut(strings.TrimPrefix(imp, internalPrefix), "/")
			if rules != nil && !slices.Contains(rules, dep) {
				t.Errorf("internal/%s imports internal/%s, not allowed by ARCHITECTURE.md §2", pkg, dep)
			}
		}
	}
}

// importsOf returns the import paths of all non-test Go files under dir (recursively).
func importsOf(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, s := range f.Imports {
			p, err := strconv.Unquote(s.Path.Value)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
