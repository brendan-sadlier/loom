package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/eval"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/execute"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/shell"
)

type runFlags struct {
	jobs                int
	dryRun, quiet, list bool
	file, dir, color    string
}

func run(ctx context.Context, stdio Stdio, f *runFlags, args []string) error {
	// Flag validation first: nothing is read or run with an unusable configuration.
	if f.jobs != 1 {
		return errors.New("parallel execution is not implemented until Phase 05; use -j 1")
	}
	color, err := resolveColor(f.color, stdio.Stderr)
	if err != nil {
		return err
	}
	if f.dir != "" {
		// The only permitted Chdir (§16): once, before evaluation.
		if err := os.Chdir(f.dir); err != nil {
			return fmt.Errorf("cannot change directory to %q: %w", f.dir, err)
		}
	}
	targets, overrides := parseArgs(args)
	environ := os.Environ()
	loomPath, _ := os.Executable()
	proj, err := eval.Evaluate(eval.Options{File: f.file, Overrides: overrides, Environ: environ, LoomPath: loomPath})
	if err != nil {
		return err
	}

	_, hasDefault := proj.Targets["default"]
	if f.list || (len(targets) == 0 && !hasDefault) {
		printList(stdio.Stdout, stdio.Stderr, proj)
		return nil
	}
	if len(targets) == 0 {
		targets = []string{"default"}
	}

	rootOS := filepath.FromSlash(proj.Root)
	g, err := graph.Build(proj, targets, graph.Options{Root: rootOS})
	if err != nil {
		return err
	}
	r, err := execute.New(execute.Options{
		Project: proj,
		Graph:   g,
		Decider: &execute.MtimeDecider{Root: rootOS},
		Shell:   shell.NewEmbedded(),
		Environ: environ,
		Stdin:   stdio.Stdin,
		Stdout:  stdio.Stdout,
		Stderr:  stdio.Stderr,
		DryRun:  f.dryRun,
		Quiet:   f.quiet,
		Color:   color,
		Jobs:    f.jobs,
	})
	if err != nil {
		return err
	}
	return r.Run(ctx)
}
