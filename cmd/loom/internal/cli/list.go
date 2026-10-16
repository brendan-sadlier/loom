package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
)

// printList writes "Targets:" and one line per declared target. Patterns are omitted.
// Phase 10 adds parameters.
func printList(w, stderr io.Writer, p *model.Project) {
	names := make([]string, 0, len(p.Order))
	if _, ok := p.Targets["default"]; ok {
		names = append(names, "default")
	}
	for _, n := range p.Order {
		if n != "default" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		_, err := fmt.Fprintln(stderr, "No targets defined.")
		if err != nil {
			return
		}
		return
	}
	width := 8
	for _, n := range names {
		width = max(width, len(n)+2)
	}
	var b strings.Builder
	b.WriteString("Targets:\n")
	for _, n := range names {
		doc, _, _ := strings.Cut(p.Targets[n].Doc, "\n")
		line := fmt.Sprintf("  %-*s%s", width, n, doc)
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	_, _ = io.WriteString(w, b.String())
}
