package execute

import (
	"runtime"
	"sort"
	"strings"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/graph"
)

// recipeEnv overlays layers (in order, later wins) onto base. On Windows an
// existing key that differs only by case is replaced.
func recipeEnv(base []string, layers ...map[string]string) []string {
	idx := map[string]int{} // canonical key -> position in out
	out := make([]string, 0, len(base)+16)
	canon := func(k string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(k)
		}
		return k
	}
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		if i, dup := idx[canon(k)]; dup {
			out[i] = kv
			continue
		}
		idx[canon(k)] = len(out)
		out = append(out, kv)
	}
	for _, layer := range layers {
		keys := make([]string, 0, len(layer))
		for k := range layer {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			kv := k + "=" + layer[k]
			if i, dup := idx[canon(k)]; dup {
				out[i] = kv
				continue
			}
			idx[canon(k)] = len(out)
			out = append(out, kv)
		}
	}
	return out
}

// automaticVars are injected for every recipe (§9).
func automaticVars(n *graph.Node, changed []string) map[string]string {
	deps := names(n.Prereqs)
	in := ""
	if len(deps) > 0 {
		in = deps[0]
	}
	return map[string]string{
		"out":     n.Name,
		"in":      in,
		"deps":    strings.Join(deps, " "),
		"changed": strings.Join(changed, " "),
		"stem":    n.Stem,
	}
}

func names(nodes []*graph.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}
