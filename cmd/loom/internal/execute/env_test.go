package execute

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
)

func TestRecipeEnvLayers(t *testing.T) {
	got := recipeEnv(
		[]string{"A=base", "B=base", "A=dup", "junk", "=x"},
		map[string]string{"B": "l1", "Z": "l1", "C": "l1"},
		map[string]string{"C": "l2"},
	)
	want := []string{"A=dup", "B=l1", "C=l2", "Z=l1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRecipeEnvWindowsCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitive env keys are Windows-only")
	}
	got := recipeEnv([]string{"Path=a"}, map[string]string{"PATH": "b"})
	if want := []string{"PATH=b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAutomaticVars(t *testing.T) {
	n := &graph.Node{
		Name:    "out.txt",
		Target:  &model.Target{Name: "out.txt"},
		Prereqs: []*graph.Node{{Name: "a"}, {Name: "b"}},
	}
	got := automaticVars(n, []string{"b"})
	want := map[string]string{"out": "out.txt", "in": "a", "deps": "a b", "changed": "b", "stem": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
