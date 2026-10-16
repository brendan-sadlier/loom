package execute

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
)

func TestCtxFileMatchesFixture(t *testing.T) {
	n := &graph.Node{
		Name:    "bin/server",
		Target:  &model.Target{Name: "bin/server", Kind: model.KindFile},
		Prereqs: []*graph.Node{{Name: "main.go"}, {Name: "go.mod"}},
	}
	c := newCtxFile(n, map[string]string{"bin": "./bin"}, map[string]string{"pkg": "./..."},
		[]string{"main.go"}, "/abs/project", 8)

	gotJSON, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "ctx.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wantJSON, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ctx JSON mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestCtxFileNeverNull(t *testing.T) {
	c := newCtxFile(&graph.Node{Name: "t", Target: &model.Target{Kind: model.KindTask}}, nil, nil, nil, "/r", 1)
	b, _ := json.Marshal(c)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"deps", "changed", "params", "vars"} {
		if m[k] == nil {
			t.Errorf("%s is null in %s", k, b)
		}
	}
}

func TestEnsureLoomDir(t *testing.T) {
	root := t.TempDir()
	dir, err := ensureLoomDir(root)
	if err != nil {
		t.Fatal(err)
	}
	gi := filepath.Join(dir, ".gitignore")
	b, err := os.ReadFile(gi)
	if err != nil || string(b) != "*\n" {
		t.Fatalf(".gitignore = %q, %v", b, err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "tmp")); err != nil || !fi.IsDir() {
		t.Fatalf("tmp dir missing: %v", err)
	}

	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(gi, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureLoomDir(root); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(gi)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(old) {
		t.Fatal(".gitignore was rewritten on second call")
	}
}

func TestWriteCtx(t *testing.T) {
	dir, err := ensureLoomDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := writeCtx(dir, ctxFile{Target: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(p) != filepath.Join(dir, "tmp") {
		t.Fatalf("ctx file at %s, want under .loom/tmp", p)
	}
	var c ctxFile
	b, _ := os.ReadFile(p)
	if err := json.Unmarshal(b, &c); err != nil || c.Target != "x" {
		t.Fatalf("round-trip failed: %v %+v", err, c)
	}
}
