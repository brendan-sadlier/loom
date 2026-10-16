package execute

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
)

// Reason explains a rebuild decision (ARCHITECTURE §6.3). Phase 04 adds the full set.
type Reason struct {
	Code   string // "task-always" | "output-missing" | "input-newer" (mtime only) | "forced" ...
	Detail string // e.g. the input path
}

type Decision struct {
	Dirty   bool
	Reasons []Reason
	Changed []string // normal prerequisites considered changed; feeds $changed
}

// Decider decides whether a node with a recipe must run. Called after all of the
// node's prerequisites have finished. Never called for source nodes or aggregates.
type Decider interface {
	Decide(ctx context.Context, n *graph.Node) (Decision, error)
}

// MtimeDecider is Make's rule: rebuild if the output is missing or any normal
// prerequisite is strictly newer. Deleted in Phase 04.
type MtimeDecider struct{ Root string }

func (d *MtimeDecider) Decide(_ context.Context, n *graph.Node) (Decision, error) {
	all := names(n.Prereqs)
	if n.Target.Kind == model.KindTask {
		return Decision{Dirty: true, Reasons: []Reason{{Code: "task-always"}}, Changed: all}, nil
	}

	out, err := os.Stat(d.path(n.Name))
	if errors.Is(err, fs.ErrNotExist) {
		return Decision{Dirty: true, Reasons: []Reason{{Code: "output-missing", Detail: n.Name}}, Changed: all}, nil
	}
	if err != nil {
		return Decision{}, fmt.Errorf("stat %q: %w", n.Name, err)
	}

	dec := Decision{Changed: []string{}}
	for _, p := range n.Prereqs {
		fi, err := os.Stat(d.path(p.Name))
		if errors.Is(err, fs.ErrNotExist) {
			if p.Target != nil && p.Target.Kind == model.KindTask {
				continue // tasks produce no file
			}
			return Decision{}, fmt.Errorf("prerequisite %q of %q does not exist after being built", p.Name, n.Name)
		}
		if err != nil {
			return Decision{}, fmt.Errorf("stat %q: %w", p.Name, err)
		}
		// Strictly newer: equal mtimes are up to date, as in Make.
		if fi.ModTime().After(out.ModTime()) {
			dec.Dirty = true
			dec.Reasons = append(dec.Reasons, Reason{Code: "input-newer", Detail: p.Name})
			dec.Changed = append(dec.Changed, p.Name)
		}
	}
	return dec, nil
}

func (d *MtimeDecider) path(name string) string {
	return filepath.Join(d.Root, filepath.FromSlash(name))
}
