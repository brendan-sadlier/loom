package execute

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/shell"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

type Options struct {
	Project *model.Project
	Graph   *graph.Graph
	Decider Decider
	Shell   shell.Runner
	Environ []string // OS env snapshot (eval.Env.Environ())
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	DryRun  bool
	Quiet   bool
	Color   bool
	Jobs    int // must be 1 in this phase
}

type Runner struct {
	o       Options
	rootOS  string
	loomDir string
	ran     int
}

// New validates o. Jobs must be 1 until Phase 05.
func New(o Options) (*Runner, error) {
	switch {
	case o.Project == nil, o.Graph == nil, o.Decider == nil, o.Shell == nil:
		return nil, errors.New("execute: Project, Graph, Decider and Shell are required")
	case o.Jobs != 1:
		return nil, fmt.Errorf("execute: Jobs = %d; parallel execution is not implemented until Phase 05", o.Jobs)
	}
	if o.Stdout == nil {
		o.Stdout = io.Discard
	}
	if o.Stderr == nil {
		o.Stderr = io.Discard
	}
	return &Runner{o: o, rootOS: filepath.FromSlash(o.Project.Root)}, nil
}

// Ran reports how many recipes executed (or would have, in dry-run).
func (r *Runner) Ran() int { return r.ran }

// Run executes the plan. It returns nil, *RecipeError, *EnvError, or *syntax.Diagnostic.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.preflight(); err != nil {
		return err
	}
	wouldRun := map[*graph.Node]bool{}
	for _, n := range r.o.Graph.Order {
		if n.IsSource() || n.Target.Recipe == nil {
			continue
		}
		d, err := r.o.Decider.Decide(ctx, n)
		if err != nil {
			return syntax.Errorf(n.Target.Pos, "%v", err)
		}
		if r.o.DryRun && !d.Dirty {
			// Make semantics: a dependent of a would-run node would also run.
			for _, p := range n.Prereqs {
				if wouldRun[p] {
					d.Dirty = true
					d.Changed = append(d.Changed, p.Name)
				}
			}
		}
		if !d.Dirty {
			continue
		}
		r.ran++
		rec := n.Target.Recipe
		if r.o.DryRun {
			wouldRun[n] = true
			echo(r.o.Stdout, n.Name, rec.Body, r.o.Color)
			continue
		}
		if err := r.runOne(ctx, n, d.Changed); err != nil {
			return err
		}
	}
	if r.ran == 0 && !r.o.Quiet {
		_, err := fmt.Fprintln(r.o.Stderr, "loom: nothing to do")
		if err != nil {
			return err
		}
	}
	return nil
}

// preflight rejects every recipe that cannot run before any recipe has side effects.
func (r *Runner) preflight() error {
	for _, n := range r.o.Graph.Order {
		if n.IsSource() || n.Target.Recipe == nil {
			continue
		}
		t, rec := n.Target, n.Target.Recipe
		switch {
		case rec.Lang == model.LangGo:
			return syntax.Errorf(rec.Pos, "Go recipes are not implemented until Phase 07")
		case rec.Lang == model.LangBash, rec.Lang == model.LangPwsh, rec.Shell != nil:
			return syntax.Errorf(rec.Pos, "external shells are not implemented until Phase 08")
		}
		if err := r.o.Shell.Check(n.Name, rec.Body); err != nil {
			var pe *shell.ParseError
			if errors.As(err, &pe) {
				// Bodies are verbatim after dedent, so the line maps exactly; column is approximate.
				pos := syntax.Pos{File: rec.Pos.File, Line: rec.Pos.Line + pe.Line - 1, Col: pe.Col}
				return syntax.Errorf(pos, "%s", pe.Msg)
			}
			return syntax.Errorf(rec.Pos, "%v", err)
		}
		for _, p := range t.Params {
			if p.Default == nil {
				return syntax.Errorf(t.Pos, "task %q requires parameter %q; passing parameters is not implemented until Phase 10", t.Name, p.Name)
			}
		}
	}
	return nil
}

func (r *Runner) runOne(ctx context.Context, n *graph.Node, changed []string) error {
	t, rec := n.Target, n.Target.Recipe
	if r.loomDir == "" {
		dir, err := ensureLoomDir(r.rootOS)
		if err != nil {
			return &EnvError{Msg: err.Error(), Hint: "check permissions on the project directory"}
		}
		r.loomDir = dir
	}

	params := map[string]string{}
	for _, p := range t.Params {
		params[p.Name] = *p.Default // preflight guarantees defaults
	}
	vars := map[string]string{}
	for _, layer := range []map[string]string{r.o.Project.Vars, t.Vars, r.o.Project.Overrides} {
		for k, v := range layer {
			vars[k] = v
		}
	}

	ctxPath, err := writeCtx(r.loomDir, newCtxFile(n, vars, params, changed, r.o.Project.Root, r.o.Jobs))
	if err != nil {
		return &EnvError{Msg: err.Error()}
	}
	defer func() { _ = os.Remove(ctxPath) }()

	env := recipeEnv(r.o.Environ,
		r.o.Project.Vars, t.Vars, params, r.o.Project.Overrides,
		automaticVars(n, changed),
		map[string]string{"LOOM_CTX": ctxPath},
	)
	if !r.o.Quiet {
		echo(r.o.Stdout, n.Name, rec.Body, r.o.Color)
	}
	err = r.o.Shell.Run(ctx, rec.Body, shell.RunOptions{
		Name:   rec.Pos.File + ":" + strconv.Itoa(rec.Pos.Line),
		Dir:    r.rootOS,
		Env:    env,
		Stdin:  r.o.Stdin,
		Stdout: r.o.Stdout,
		Stderr: r.o.Stderr,
	})
	if err != nil {
		return &RecipeError{Target: n.Name, Err: err}
	}
	return nil
}
