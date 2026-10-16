package model

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Dump renders p in the stable text form used by golden tests and `loom __dump --text`.
func (p *Project) Dump() string {
	var b strings.Builder
	b.WriteString("root " + p.Root + "\n")
	for _, f := range p.Files {
		b.WriteString("file " + f + "\n")
	}
	writeVars(&b, "", "var", p.Vars)
	writeVars(&b, "", "override", p.Overrides)

	for _, name := range p.Order {
		t := p.Targets[name]
		if t == nil {
			continue
		}
		head := "target"
		if t.Kind == KindTask {
			head = "task"
		}
		b.WriteString(head + " " + t.Name + " @" + t.Pos.String())
		writeDoc(&b, t.Doc)
		if len(t.Params) > 0 {
			parts := make([]string, len(t.Params))
			for i, pr := range t.Params {
				if pr.Default == nil {
					parts[i] = pr.Name
				} else {
					parts[i] = pr.Name + "=" + strconv.Quote(*pr.Default)
				}
			}
			b.WriteString("  params: " + strings.Join(parts, " ") + "\n")
		}
		writeChildren(&b, t.Prereqs, t.OrderOnly, t.Vars, t.Recipe)
	}

	for _, r := range p.Patterns {
		pats := make([]string, len(r.Targets))
		for i, pt := range r.Targets {
			pats[i] = pt.String()
		}
		b.WriteString("pattern @" + r.Pos.String() + " targets=" + strings.Join(pats, " "))
		writeDoc(&b, r.Doc)
		writeChildren(&b, r.Prereqs, r.OrderOnly, r.Vars, r.Recipe)
	}

	for _, g := range p.GoBlocks {
		b.WriteString("go @" + g.Pos.String() + "\n")
		writeBody(&b, g.Body)
	}
	return b.String()
}

// writeDoc terminates a header line, appending doc=<quoted> when non-empty.
func writeDoc(b *strings.Builder, doc string) {
	if doc != "" {
		b.WriteString(" doc=" + strconv.Quote(doc))
	}
	b.WriteString("\n")
}

func writeChildren(b *strings.Builder, prereqs, orderOnly []string, vars map[string]string, rec *Recipe) {
	if len(prereqs) > 0 {
		b.WriteString("  prereqs: " + list(prereqs) + "\n")
	}
	if len(orderOnly) > 0 {
		b.WriteString("  orderonly: " + list(orderOnly) + "\n")
	}
	writeVars(b, "  ", "var", vars)
	if rec != nil {
		b.WriteString("  recipe " + rec.Lang.String())
		if rec.Shell != nil {
			b.WriteString(" shell=" + strconv.Quote(strings.Join(rec.Shell, " ")))
		}
		b.WriteString(" @" + rec.Pos.String() + "\n")
		writeBody(b, rec.Body)
	}
}

func writeVars(b *strings.Builder, indent, label string, vars map[string]string) {
	for _, k := range slices.Sorted(maps.Keys(vars)) {
		b.WriteString(indent + label + " " + k + " = " + strconv.Quote(vars[k]) + "\n")
	}
}

func writeBody(b *strings.Builder, body string) {
	if body == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		if line == "" {
			b.WriteString("  |\n")
		} else {
			b.WriteString("  | " + line + "\n")
		}
	}
}

// list joins items, quoting only those containing a space, tab or double quote.
func list(items []string) string {
	out := make([]string, len(items))
	for i, s := range items {
		if strings.ContainsAny(s, " \t\"") {
			out[i] = strconv.Quote(s)
		} else {
			out[i] = s
		}
	}
	return strings.Join(out, " ")
}
