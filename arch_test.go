package loom_test

import (
	"bufio"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Directories that are not part of the runtime module's packages.
var skipDirs = map[string]bool{"cmd": true, ".git": true, ".docs": true, ".github": true, "testdata": true}

func TestRuntimeIsStdlibOnly(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, s := range f.Imports {
			p, _ := strconv.Unquote(s.Path.Value)
			first, _, _ := strings.Cut(p, "/")
			if strings.Contains(first, ".") {
				t.Errorf("%s imports %q; the runtime module must be standard-library only", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeGoModHasNoRequires(t *testing.T) {
	f, err := os.Open("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(strings.TrimSpace(sc.Text()), "require") {
			t.Fatalf("go.mod contains a require directive: %q", sc.Text())
		}
	}
}
