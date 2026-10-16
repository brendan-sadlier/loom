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

// checkAssignable rejects built-in and reserved names for file and target assignments.
func checkAssignable(pos syntax.Pos, name string) error {
	if isBuiltin(name) {
		return syntax.Errorf(pos, "cannot assign to built-in variable %q", name)
	}
	if isReserved(name) {
		return syntax.Errorf(pos, "%q is reserved for recipes and cannot be assigned", name)
	}
	return nil
}

// assign applies a file-level assignment. CLI overrides win over every operator.
func (s *scope) assign(pos syntax.Pos, name string, op syntax.AssignOp, value string) error {
	if err := checkAssignable(pos, name); err != nil {
		return err
	}
	if _, ok := s.overrides[name]; ok {
		return nil
	}
	switch op {
	case syntax.OpSet:
		s.vars[name] = value
	case syntax.OpDefault:
		if _, ok := s.vars[name]; ok {
			return nil
		}
		if v, ok := s.env.Lookup(name); ok {
			s.vars[name] = v
			return nil
		}
		s.vars[name] = value
	case syntax.OpAppend:
		cur, ok := s.vars[name]
		if !ok {
			return syntax.Errorf(pos, "cannot append to undefined variable %q", name)
		}
		s.vars[name] = appendValue(cur, value)
	default:
		panic("eval: unknown assignment operator " + string(op))
	}
	return nil
}

// assignTarget applies a target-specific assignment into vars. The OS environment is
// not consulted for "?="; overrides still win.
func (s *scope) assignTarget(vars map[string]string, pos syntax.Pos, name string, op syntax.AssignOp, value string) error {
	if err := checkAssignable(pos, name); err != nil {
		return err
	}
	if _, ok := s.overrides[name]; ok {
		return nil
	}
	switch op {
	case syntax.OpSet:
		vars[name] = value
	case syntax.OpDefault:
		if _, ok := vars[name]; !ok {
			vars[name] = value
		}
	case syntax.OpAppend:
		cur, ok := vars[name]
		if !ok {
			cur, ok = s.vars[name]
		}
		if !ok {
			return syntax.Errorf(pos, "cannot append to undefined variable %q", name)
		}
		vars[name] = appendValue(cur, value)
	default:
		panic("eval: unknown assignment operator " + string(op))
	}
	return nil
}

// appendValue joins with a single space, without a leading space when cur is empty.
func appendValue(cur, value string) string {
	if cur == "" {
		return value
	}
	return cur + " " + value
}
