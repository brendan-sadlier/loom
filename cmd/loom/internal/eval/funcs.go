package eval

import "github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"

// Func implements $(name arg, ...). Args are already expanded and trimmed.
type Func func(c *Call) (string, error)

// Call is the context passed to a Func. Phase 09 adds fields (shell runner, cwd).
type Call struct {
	Name string
	Args []string
	Pos  syntax.Pos
	Root string // project root, forward slashes
	Env  *Env
	Vars map[string]string // read-only view of current variables
}

// DefaultFuncs returns the built-in registry. Empty until Phase 09.
func DefaultFuncs() map[string]Func { return map[string]Func{} }
