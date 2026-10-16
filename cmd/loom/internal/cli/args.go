package cli

import (
	"path"
	"strings"
)

// parseArgs splits positional args into targets and VAR=value overrides.
// Phase 10 extends this with task parameters.
func parseArgs(args []string) (targets []string, overrides map[string]string) {
	for _, a := range args {
		if overrideRE.MatchString(a) { // ^[A-Za-z_][A-Za-z0-9_]*=  (dump.go)
			k, v, _ := strings.Cut(a, "=")
			if overrides == nil {
				overrides = map[string]string{}
			}
			overrides[k] = v // later duplicates win, as in Make
			continue
		}
		// path.Clean is the identity on identifiers, so one rule covers tasks and files
		// and matches how eval cleaned declared names.
		targets = append(targets, path.Clean(a))
	}
	return targets, overrides
}
