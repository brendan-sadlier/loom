package eval

import (
	"runtime"
	"slices"
	"strings"
)

// Env is an immutable snapshot of the process environment.
type Env struct {
	environ  []string
	foldCase bool
}

// NewEnv snapshots environ. The slice is copied.
func NewEnv(environ []string) *Env {
	return &Env{environ: slices.Clone(environ), foldCase: runtime.GOOS == "windows"}
}

func (e *Env) Lookup(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	for _, kv := range e.environ {
		if kv == "" {
			continue
		}
		// Start at 1: Windows has hidden entries like "=C:=C:\dir" whose key begins with "=".
		i := strings.IndexByte(kv[1:], '=')
		if i < 0 {
			continue
		}
		k, v := kv[:i+1], kv[i+2:]
		if k == name || (e.foldCase && strings.EqualFold(k, name)) {
			return v, true
		}
	}
	return "", false
}

// Environ returns the snapshot in original order (used by Phase 03 to build recipe env).
func (e *Env) Environ() []string { return slices.Clone(e.environ) }
