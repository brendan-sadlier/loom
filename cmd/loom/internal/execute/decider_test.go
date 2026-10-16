package execute

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
)

var t0 = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func writeAt(t *testing.T, root, name string, mt time.Time) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func fileNode(name string, prereqs ...*graph.Node) *graph.Node {
	return &graph.Node{Name: name, Target: &model.Target{Name: name, Kind: model.KindFile}, Prereqs: prereqs}
}

func src(name string) *graph.Node { return &graph.Node{Name: name} }

func decide(t *testing.T, root string, n *graph.Node) Decision {
	t.Helper()
	d, err := (&MtimeDecider{Root: root}).Decide(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestMtimeOutputMissing(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "in", t0)
	d := decide(t, root, fileNode("out", src("in")))
	if !d.Dirty || d.Reasons[0].Code != "output-missing" || !reflect.DeepEqual(d.Changed, []string{"in"}) {
		t.Fatalf("got %+v", d)
	}
}

func TestMtimeInputNewer(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "out", t0)
	writeAt(t, root, "old", t0.Add(-time.Hour))
	writeAt(t, root, "new", t0.Add(2*time.Second))
	d := decide(t, root, fileNode("out", src("old"), src("new")))
	want := Decision{Dirty: true, Reasons: []Reason{{Code: "input-newer", Detail: "new"}}, Changed: []string{"new"}}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("got %+v, want %+v", d, want)
	}
}

func TestMtimeEqualIsClean(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "out", t0)
	writeAt(t, root, "in", t0)
	if d := decide(t, root, fileNode("out", src("in"))); d.Dirty {
		t.Fatalf("equal mtimes must be clean: %+v", d)
	}
}

func TestMtimeTaskAlways(t *testing.T) {
	n := &graph.Node{Name: "test", Target: &model.Target{Name: "test", Kind: model.KindTask}, Prereqs: []*graph.Node{src("a")}}
	d := decide(t, t.TempDir(), n)
	if !d.Dirty || d.Reasons[0].Code != "task-always" || !reflect.DeepEqual(d.Changed, []string{"a"}) {
		t.Fatalf("got %+v", d)
	}
}

func TestMtimeMissingPrereq(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "out", t0)
	task := &graph.Node{Name: "prep", Target: &model.Target{Name: "prep", Kind: model.KindTask}}

	// A task prerequisite has no file: skipped.
	if d := decide(t, root, fileNode("out", task)); d.Dirty {
		t.Fatalf("task prereq should be skipped: %+v", d)
	}
	// A missing file prerequisite is an error.
	_, err := (&MtimeDecider{Root: root}).Decide(context.Background(), fileNode("out", fileNode("gen")))
	if err == nil || !strings.Contains(err.Error(), `prerequisite "gen" of "out" does not exist after being built`) {
		t.Fatalf("err = %v", err)
	}
}
