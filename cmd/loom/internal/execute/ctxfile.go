package execute

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
)

// ctxFile is the LOOM_CTX JSON schema (§9). Shared with the runtime via testdata/ctx.json.
type ctxFile struct {
	Target  string            `json:"target"`
	Kind    string            `json:"kind"`
	Out     string            `json:"out"`
	In      string            `json:"in"`
	Deps    []string          `json:"deps"`
	Changed []string          `json:"changed"`
	Stem    string            `json:"stem"`
	Params  map[string]string `json:"params"`
	Vars    map[string]string `json:"vars"`
	Root    string            `json:"root"`
	Jobs    int               `json:"jobs"`
}

// Slices and maps are never nil in the JSON ([] / {}), so Go readers can index freely.
func newCtxFile(n *graph.Node, vars, params map[string]string, changed []string, root string, jobs int) ctxFile {
	deps := names(n.Prereqs)
	in := ""
	if len(deps) > 0 {
		in = deps[0]
	}
	kind := ""
	if n.Target != nil {
		kind = n.Target.Kind.String()
	}
	if changed == nil {
		changed = []string{}
	}
	if vars == nil {
		vars = map[string]string{}
	}
	if params == nil {
		params = map[string]string{}
	}
	return ctxFile{
		Target: n.Name, Kind: kind, Out: n.Name, In: in,
		Deps: deps, Changed: changed, Stem: n.Stem,
		Params: params, Vars: vars, Root: root, Jobs: jobs,
	}
}

const loomDirName = ".loom"

// ensureLoomDir creates ROOT/.loom and ROOT/.loom/tmp and writes .loom/.gitignore
// containing "*\n" if absent. Returns the OS path of .loom.
func ensureLoomDir(rootOS string) (string, error) {
	dir := filepath.Join(rootOS, loomDirName)
	if err := os.MkdirAll(filepath.Join(dir, "tmp"), 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	gi := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gi); errors.Is(err, fs.ErrNotExist) {
		// Self-ignoring: the user's .gitignore is never touched (Decision Log 2026-10-13).
		if err := os.WriteFile(gi, []byte("*\n"), 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", gi, err)
		}
	} else if err != nil {
		return "", fmt.Errorf("stat %s: %w", gi, err)
	}
	return dir, nil
}

// writeCtx writes c to a new file in <loomDir>/tmp and returns its path.
// The caller removes it after the recipe finishes.
func writeCtx(loomDir string, c ctxFile) (string, error) {
	f, err := os.CreateTemp(filepath.Join(loomDir, "tmp"), "ctx-*.json")
	if err != nil {
		return "", fmt.Errorf("create context file: %w", err)
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(c); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write context file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close context file: %w", err)
	}
	return f.Name(), nil
}
