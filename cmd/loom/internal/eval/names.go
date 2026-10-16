package eval

import (
	"path"
	"strings"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

// isGlob reports whether s contains glob metacharacters. Brace alternation is not
// supported, so "{" is deliberately not a glob character.
func isGlob(s string) bool { return strings.ContainsAny(s, "*?[") }

// cleanName canonicalises a file-target name: forward slashes, path.Clean, so that
// "./bin/app", "bin//app" and "bin/app" are the same target. Absolute and "../"
// names are allowed and kept. Task names are identifiers and never pass through here.
func cleanName(pos syntax.Pos, s string) (string, error) {
	if s == "" {
		return "", syntax.Errorf(pos, "empty target name")
	}
	if isGlob(s) {
		return "", syntax.Errorf(pos, "target name %q contains glob characters", s)
	}
	return path.Clean(s), nil
}
