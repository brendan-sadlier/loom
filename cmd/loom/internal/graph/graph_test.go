package graph

import (
	"errors"
	"strings"
	"testing"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

type decl struct {
	name    string
	prereqs []string
}

func project(decls ...decl) *model.Project {
	p := &model.Project{Targets: map[string]*model.Target{}}
	for i, d := range decls {
		p.Order = append(p.Order, d.name)
		p.Targets[d.name] = &model.Target{
			Name:    d.name,
			Pos:     syntax.Pos{File: "Loomfile", Line: i + 1, Col: 1},
			Prereqs: d.prereqs,
		}
	}
	return p
}

func none(string) bool { return false }

func orderOf(g *Graph) string {
	var s []string
	for _, n := range g.Order {
		s = append(s, n.Name)
	}
	return strings.Join(s, " ")
}

func TestDiamondOrder(t *testing.T) {
	p := project(decl{"a", []string{"b", "c"}}, decl{"b", []string{"d"}}, decl{"c", []string{"d"}}, decl{"d", nil})
	g, err := Build(p, []string{"a"}, Options{Exists: none})
	if err != nil {
		t.Fatal(err)
	}
	if got := orderOf(g); got != "d b c a" {
		t.Fatalf("Order = %q, want %q", got, "d b c a")
	}
	if g.Nodes["b"].Prereqs[0] != g.Nodes["c"].Prereqs[0] {
		t.Fatal("shared prerequisite d must be one node")
	}
}

func TestCycle(t *testing.T) {
	p := project(decl{"a", []string{"b"}}, decl{"b", []string{"a"}})
	_, err := Build(p, []string{"a"}, Options{Exists: none})
	want := "Loomfile:1:1: dependency cycle: a -> b -> a"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestNoRule(t *testing.T) {
	p := project(decl{"app", []string{"missing.c"}})
	_, err := Build(p, []string{"app"}, Options{Exists: none})
	want := `Loomfile:1:1: no rule to make "missing.c", needed by "app"`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestSourceLeaf(t *testing.T) {
	p := project(decl{"app", []string{"main.c"}})
	g, err := Build(p, []string{"app"}, Options{Exists: func(n string) bool { return n == "main.c" }})
	if err != nil {
		t.Fatal(err)
	}
	if n := g.Nodes["main.c"]; n == nil || !n.IsSource() {
		t.Fatalf("main.c should be a source node, got %+v", n)
	}
	if got := orderOf(g); got != "main.c app" {
		t.Fatalf("Order = %q", got)
	}
}

func TestRequestedUnknown(t *testing.T) {
	p := project(decl{"app", nil})
	for _, tc := range []struct {
		name   string
		exists bool
	}{{"nope", false}, {"stray.txt", true}} {
		_, err := Build(p, []string{tc.name}, Options{Exists: func(string) bool { return tc.exists }})
		var ute *UnknownTargetError
		if !errors.As(err, &ute) || ute.Name != tc.name {
			t.Errorf("%s: err = %v, want UnknownTargetError", tc.name, err)
		}
	}
}
