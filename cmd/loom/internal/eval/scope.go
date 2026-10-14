package eval

import (
	"maps"
	"regexp"
	"runtime"
	"slices"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

var builtinNames = []string{"os", "arch", "exe", "root", "loom"}
var automaticNames = []string{"out", "in", "deps", "changed", "stem", "LOOM_CTX"}

var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isBuiltin(name string) bool  { return slices.Contains(builtinNames, name) }
func isReserved(name string) bool { return slices.Contains(automaticNames, name) }

// commandLine is the pseudo-position for CLI overrides; it prints as "command line".
var commandLine = syntax.Pos{File: "command line"}

type scope struct {
	vars      map[string]string
	overrides map[string]string
	env       *Env
}

// newScope seeds built-ins then overrides. Overrides must be valid identifiers and
// must not name a built-in or reserved variable. root is absolute with forward slashes.
func newScope(opts Options, root string) (*scope, error) {
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := opts.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	exe := ""
	if goos == "windows" {
		exe = ".exe"
	}
	s := &scope{
		vars: map[string]string{
			"os":   goos,
			"arch": goarch,
			"exe":  exe,
			"root": root,
			"loom": opts.LoomPath,
		},
		overrides: make(map[string]string, len(opts.Overrides)),
		env:       NewEnv(opts.Environ),
	}
	// Sorted so the first reported error is deterministic.
	for _, name := range slices.Sorted(maps.Keys(opts.Overrides)) {
		switch {
		case !identRE.MatchString(name):
			return nil, syntax.Errorf(commandLine, "invalid variable name %q in command-line override", name)
		case isBuiltin(name):
			return nil, syntax.Errorf(commandLine, "cannot override built-in variable %q", name)
		case isReserved(name):
			return nil, syntax.Errorf(commandLine, "%q is reserved for recipes and cannot be assigned", name)
		}
		v := opts.Overrides[name]
		s.overrides[name] = v
		s.vars[name] = v
	}
	return s, nil
}
