// Package eval turns a parsed Loomfile plus its environment into a model.Project.
package eval

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

// Options are the complete inputs to Evaluate. Tests inject every field.
type Options struct {
	File      string            // path to the Loomfile as given (relative or absolute)
	Overrides map[string]string // CLI VAR=val
	Environ   []string          // snapshot of os.Environ(); nil = empty environment
	Funcs     map[string]Func   // nil = DefaultFuncs()
	LoomPath  string            // value of the $loom built-in (absolute path of the running binary)
	GOOS      string            // "" = runtime.GOOS
	GOARCH    string            // "" = runtime.GOARCH
}

// fileState is per-file evaluation state. Directives like `shell` do not leak across includes.
type fileState struct {
	dir      string   // absolute OS path of the file's directory (include resolution)
	rel      string   // slash path relative to root, used as Pos.File
	shell    []string // from `shell` in this file; nil = embedded
	shellPos syntax.Pos
}

type evaluator struct {
	opts   Options
	root   string // absolute, forward slashes
	rootOS string // absolute, OS separators
	scope  *scope
	funcs  map[string]Func
	proj   *model.Project
	stack  []string              // absolute paths being evaluated (cycle detection)
	seen   map[string]bool       // absolute paths already included (include once)
	decls  map[string]*declState // by target name, or by canonical pattern key (always contains "{stem}")
	pats   map[string]*model.PatternRule
}

// Evaluate reads opts.File and all includes and returns the expanded project.
// Errors are *syntax.Diagnostic where a position is known.
func Evaluate(opts Options) (*model.Project, error) {
	abs, err := filepath.Abs(opts.File)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", opts.File, err)
	}
	rootOS := filepath.Dir(abs)
	root := filepath.ToSlash(rootOS)
	sc, err := newScope(opts, root)
	if err != nil {
		return nil, err
	}
	funcs := opts.Funcs
	if funcs == nil {
		funcs = DefaultFuncs()
	}
	ev := &evaluator{
		opts:   opts,
		root:   root,
		rootOS: rootOS,
		scope:  sc,
		funcs:  funcs,
		proj:   &model.Project{Root: root, Order: []string{}, Targets: map[string]*model.Target{}},
		seen:   map[string]bool{},
		decls:  map[string]*declState{},
		pats:   map[string]*model.PatternRule{},
	}
	if err := ev.evalFile(abs, syntax.Pos{}); err != nil {
		return nil, err
	}
	ev.proj.Vars = maps.Clone(sc.vars)
	if len(opts.Overrides) > 0 {
		ev.proj.Overrides = maps.Clone(opts.Overrides)
	}
	return ev.proj, nil
}

// evalFile evaluates one file inline. includePos is zero for the root Loomfile.
func (ev *evaluator) evalFile(abs string, includePos syntax.Pos) error {
	if i := slices.Index(ev.stack, abs); i >= 0 {
		chain := make([]string, 0, len(ev.stack)-i+1)
		for _, p := range ev.stack[i:] {
			chain = append(chain, ev.relName(p))
		}
		chain = append(chain, ev.relName(abs))
		return syntax.Errorf(includePos, "include cycle: %s", strings.Join(chain, " -> "))
	}
	if ev.seen[abs] {
		return nil
	}
	rel := ev.relName(abs)
	src, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !includePos.IsValid() {
			return &syntax.Diagnostic{Pos: syntax.Pos{File: ev.opts.File}, Msg: "file not found", Hint: "use -f FILE or -C DIR"}
		}
		return fmt.Errorf("read %s: %w", rel, err)
	}
	f, err := syntax.Parse(rel, src)
	if err != nil {
		return err
	}
	ev.seen[abs] = true
	ev.proj.Files = append(ev.proj.Files, filepath.ToSlash(abs))
	ev.stack = append(ev.stack, abs)
	defer func() { ev.stack = ev.stack[:len(ev.stack)-1] }()
	return ev.evalItems(f.Items, &fileState{dir: filepath.Dir(abs), rel: rel})
}

func (ev *evaluator) evalItems(items []syntax.Item, fs *fileState) error {
	for _, it := range items {
		var err error
		switch it := it.(type) {
		case *syntax.Assign:
			err = ev.evalAssign(it)
		case *syntax.TargetDecl:
			err = ev.evalTarget(it, fs)
		case *syntax.TaskDecl:
			err = ev.evalTask(it, fs)
		case *syntax.Include:
			err = ev.evalInclude(it, fs)
		case *syntax.ShellDir:
			err = ev.evalShell(it, fs)
		case *syntax.GoBlock:
			ev.proj.GoBlocks = append(ev.proj.GoBlocks, &model.GoBlock{Pos: it.Body.Pos, Body: it.Body.Text})
		case *syntax.If:
			err = syntax.Errorf(it.Pos(), `"if" is not implemented until Phase 09`)
		case *syntax.Dotenv:
			err = syntax.Errorf(it.Pos(), `"dotenv" is not implemented until Phase 11`)
		default:
			panic(fmt.Sprintf("eval: unknown item %T", it))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (ev *evaluator) evalAssign(a *syntax.Assign) error {
	value, err := ev.expand(a.Value)
	if err != nil {
		return err
	}
	return ev.scope.assign(a.P, a.Name, a.Op, value)
}

func (ev *evaluator) evalShell(d *syntax.ShellDir, fs *fileState) error {
	if fs.shellPos.IsValid() {
		return syntax.Errorf(d.P, "shell already set for this file at %s", fs.shellPos)
	}
	var args []string
	for _, w := range d.Args {
		items, err := ev.expandList(w)
		if err != nil {
			return err
		}
		args = append(args, items...)
	}
	if len(args) == 0 || args[0] == "" {
		return syntax.Errorf(d.P, "shell requires a program name")
	}
	fs.shell, fs.shellPos = args, d.P
	return nil
}

// evalInclude resolves paths relative to the including file. A glob matching nothing
// is a no-op; a missing literal path is an error (§3.7).
func (ev *evaluator) evalInclude(inc *syntax.Include, fs *fileState) error {
	for _, w := range inc.Paths {
		items, err := ev.expandList(w)
		if err != nil {
			return err
		}
		for _, item := range items {
			p := filepath.Join(fs.dir, filepath.FromSlash(item))
			var files []string
			if isGlob(item) {
				files, err = doublestar.FilepathGlob(p)
				if err != nil {
					return syntax.Errorf(w.Pos, "invalid glob pattern %q: %v", item, err)
				}
				slices.Sort(files)
			} else {
				if _, err := os.Stat(p); err != nil {
					if errors.Is(err, os.ErrNotExist) {
						return syntax.Errorf(w.Pos, "included file %q not found", item)
					}
					return fmt.Errorf("include %s: %w", item, err)
				}
				files = []string{p}
			}
			for _, f := range files {
				if err := ev.evalFile(f, w.Pos); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// relName returns abs as a slash path relative to the project root.
func (ev *evaluator) relName(abs string) string {
	rel, err := filepath.Rel(ev.rootOS, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}
