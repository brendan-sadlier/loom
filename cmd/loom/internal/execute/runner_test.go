package execute

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/shell"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

type call struct {
	script string
	env    map[string]string
}

type fakeShell struct {
	calls  []call
	failOn string // script that fails
}

func (f *fakeShell) Check(string, string) error { return nil }

func (f *fakeShell) Run(_ context.Context, script string, o shell.RunOptions) error {
	env := map[string]string{}
	for _, kv := range o.Env {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	f.calls = append(f.calls, call{script, env})
	if script == f.failOn {
		return &shell.ExitError{Code: 3}
	}
	return nil
}

type fakeDecider map[string]bool // name -> dirty

func (d fakeDecider) Decide(_ context.Context, n *graph.Node) (Decision, error) {
	return Decision{Dirty: d[n.Name], Changed: names(n.Prereqs)}, nil
}

// chain builds src -> a -> b (file targets with recipes "A\n", "B\n").
func chain(t *testing.T) (*model.Project, *graph.Graph) {
	t.Helper()
	pos := syntax.Pos{File: "Loomfile", Line: 1, Col: 1}
	p := &model.Project{
		Root:    filepath.ToSlash(t.TempDir()),
		Vars:    map[string]string{"v": "project"},
		Targets: map[string]*model.Target{},
	}
	for _, d := range []struct {
		name, body string
		prereqs    []string
	}{
		{"a", "A\n", []string{"src"}},
		{"b", "B\n", []string{"a"}},
	} {
		p.Order = append(p.Order, d.name)
		p.Targets[d.name] = &model.Target{Name: d.name, Kind: model.KindFile, Pos: pos, Prereqs: d.prereqs,
			Recipe: &model.Recipe{Body: d.body, Pos: pos}}
	}
	g, err := graph.Build(p, []string{"b"}, graph.Options{Exists: func(n string) bool { return n == "src" }})
	if err != nil {
		t.Fatal(err)
	}
	return p, g
}

func newRunner(t *testing.T, p *model.Project, g *graph.Graph, d Decider, sh shell.Runner, mod func(*Options)) (*Runner, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	o := Options{Project: p, Graph: g, Decider: d, Shell: sh, Stdout: &out, Stderr: &errb, Jobs: 1}
	if mod != nil {
		mod(&o)
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	return r, &out, &errb
}

func TestRunOrderAndEnv(t *testing.T) {
	p, g := chain(t)
	sh := &fakeShell{}
	r, out, _ := newRunner(t, p, g, fakeDecider{"a": true, "b": true}, sh, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sh.calls) != 2 || sh.calls[0].script != "A\n" || sh.calls[1].script != "B\n" {
		t.Fatalf("calls = %+v", sh.calls)
	}
	env := sh.calls[1].env
	if env["out"] != "b" || env["in"] != "a" || env["deps"] != "a" || env["v"] != "project" || env["LOOM_CTX"] == "" {
		t.Fatalf("env = %v", env)
	}
	if _, err := os.Stat(env["LOOM_CTX"]); !os.IsNotExist(err) {
		t.Fatalf("ctx file not removed: %v", err)
	}
	if !strings.Contains(out.String(), "→ a\n  A\n→ b\n  B\n") {
		t.Fatalf("echo = %q", out)
	}
	if r.Ran() != 2 {
		t.Fatalf("Ran = %d", r.Ran())
	}
}

func TestRunSkipsCleanAndNothingToDo(t *testing.T) {
	p, g := chain(t)
	sh := &fakeShell{}
	r, out, errb := newRunner(t, p, g, fakeDecider{}, sh, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sh.calls) != 0 || out.Len() != 0 {
		t.Fatalf("ran %d, out %q", len(sh.calls), out)
	}
	if errb.String() != "loom: nothing to do\n" {
		t.Fatalf("stderr = %q", errb)
	}
}

func TestDryRunPropagates(t *testing.T) {
	p, g := chain(t)
	sh := &fakeShell{}
	r, out, _ := newRunner(t, p, g, fakeDecider{"a": true}, sh, func(o *Options) { o.DryRun, o.Quiet = true, true })
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sh.calls) != 0 {
		t.Fatal("dry-run executed a recipe")
	}
	if !strings.Contains(out.String(), "→ a") || !strings.Contains(out.String(), "→ b") {
		t.Fatalf("dry-run echo = %q", out)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(p.Root), ".loom")); !os.IsNotExist(err) {
		t.Fatal("dry-run created .loom")
	}
}

func TestRecipeFailureStops(t *testing.T) {
	p, g := chain(t)
	sh := &fakeShell{failOn: "A\n"}
	r, _, _ := newRunner(t, p, g, fakeDecider{"a": true, "b": true}, sh, nil)
	err := r.Run(context.Background())
	var re *RecipeError
	if !errors.As(err, &re) || re.Target != "a" || re.ExitCode() != 1 {
		t.Fatalf("err = %v", err)
	}
	if len(sh.calls) != 1 {
		t.Fatalf("build continued after failure: %d calls", len(sh.calls))
	}
}

func TestPreflightRejectsGo(t *testing.T) {
	p, g := chain(t)
	p.Targets["b"].Recipe.Lang = model.LangGo
	sh := &fakeShell{}
	r, _, _ := newRunner(t, p, g, fakeDecider{"a": true, "b": true}, sh, nil)
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Go recipes are not implemented until Phase 07") {
		t.Fatalf("err = %v", err)
	}
	if len(sh.calls) != 0 {
		t.Fatal("preflight must run before any recipe")
	}
}

func TestNewRejectsJobs(t *testing.T) {
	p, g := chain(t)
	if _, err := New(Options{Project: p, Graph: g, Decider: fakeDecider{}, Shell: &fakeShell{}, Jobs: 4}); err == nil {
		t.Fatal("Jobs=4 accepted")
	}
}
