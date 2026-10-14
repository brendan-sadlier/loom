package syntax

import (
	"fmt"
	"strconv"
	"strings"
)

// Dump returns a deterministic text form of f, used by golden tests.
func (f *File) Dump() string {
	var b strings.Builder
	b.WriteString("File " + f.Name + "\n")
	dumpItems(&b, f.Items, "")
	return b.String()
}

func dumpItems(b *strings.Builder, items []Item, ind string) {
	for _, it := range items {
		dumpItem(b, it, ind)
	}
}

func lc(p Pos) string { return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col) }

func dumpItem(b *strings.Builder, it Item, ind string) {
	b.WriteString(ind)
	switch x := it.(type) {
	case *Assign:
		b.WriteString("Assign " + lc(x.P) + " " + dumpAssign(x) + "\n")
	case *Include:
		b.WriteString("Include " + lc(x.P) + " " + dumpWords(x.Paths) + "\n")
	case *Dotenv:
		b.WriteString("Dotenv " + lc(x.P))
		if x.Path != nil {
			b.WriteString(" " + dumpWord(*x.Path))
		}
		b.WriteByte('\n')
	case *ShellDir:
		b.WriteString("Shell " + lc(x.P) + " " + dumpWords(x.Args) + "\n")
	case *GoBlock:
		b.WriteString("Go " + lc(x.P) + " body@" + lc(x.Body.Pos) + "\n")
		dumpBodyLines(b, x.Body, ind+"  ")
	case *TargetDecl:
		b.WriteString("Target " + lc(x.P) + " names=" + dumpWords(x.Names))
		if x.Var != nil {
			b.WriteString(" var=(" + dumpAssign(x.Var) + ")")
		}
		dumpTail(b, x.Prereqs, x.OrderOnly, x.Attrs, x.Doc)
		dumpBody(b, x.Body, ind)
	case *TaskDecl:
		b.WriteString("Task " + lc(x.P) + " name=" + x.Name)
		if len(x.Params) > 0 {
			ps := make([]string, len(x.Params))
			for i, p := range x.Params {
				ps[i] = p.Name
				if p.Default != nil {
					ps[i] += "=" + dumpWord(*p.Default)
				}
			}
			b.WriteString(" params=[" + strings.Join(ps, " ") + "]")
		}
		dumpTail(b, x.Prereqs, x.OrderOnly, x.Attrs, x.Doc)
		dumpBody(b, x.Body, ind)
	case *If:
		b.WriteString("If " + lc(x.P) + " cond=" + dumpExpr(x.Cond) + "\n")
		b.WriteString(ind + "  then:\n")
		dumpItems(b, x.Then, ind+"    ")
		if x.Else != nil {
			b.WriteString(ind + "  else:\n")
			dumpItems(b, x.Else, ind+"    ")
		}
	default:
		panic(fmt.Sprintf("syntax: Dump: unknown item %T", it))
	}
}

func dumpAssign(a *Assign) string {
	return a.Name + " " + string(a.Op) + " " + dumpWord(a.Value)
}

// dumpTail writes the optional fields shared by Target and Task, ending the line.
func dumpTail(b *strings.Builder, prereqs, orderOnly []Word, attrs []string, doc string) {
	if len(prereqs) > 0 {
		b.WriteString(" prereqs=" + dumpWords(prereqs))
	}
	if len(orderOnly) > 0 {
		b.WriteString(" orderonly=" + dumpWords(orderOnly))
	}
	if len(attrs) > 0 {
		b.WriteString(" attrs=[" + strings.Join(attrs, " ") + "]")
	}
	if doc != "" {
		b.WriteString(" doc=" + strconv.Quote(doc))
	}
	b.WriteByte('\n')
}

func dumpBody(b *strings.Builder, body *Body, ind string) {
	if body == nil {
		return
	}
	b.WriteString(ind + "  body@" + lc(body.Pos) + "\n")
	dumpBodyLines(b, *body, ind+"  ")
}

func dumpBodyLines(b *strings.Builder, body Body, ind string) {
	if body.Text == "" {
		return
	}
	for _, ln := range strings.Split(strings.TrimSuffix(body.Text, "\n"), "\n") {
		if ln == "" {
			b.WriteString(ind + "|\n")
		} else {
			b.WriteString(ind + "| " + ln + "\n")
		}
	}
}

func dumpWords(ws []Word) string {
	parts := make([]string, len(ws))
	for i, w := range ws {
		parts[i] = dumpWord(w)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func dumpWord(w Word) string {
	var b strings.Builder
	if w.Quoted {
		b.WriteByte('Q')
	}
	b.WriteByte('[')
	for i, s := range w.Segs {
		if i > 0 {
			b.WriteByte(' ')
		}
		switch s := s.(type) {
		case Text:
			b.WriteString("T:" + strconv.Quote(s.S))
		case VarRef:
			b.WriteString("V:" + s.Name)
		case Call:
			b.WriteString("C:" + s.Name + "(")
			for j, a := range s.Args {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(dumpWord(a))
			}
			b.WriteByte(')')
		}
	}
	b.WriteByte(']')
	return b.String()
}

func dumpExpr(e Expr) string {
	switch x := e.(type) {
	case Operand:
		return dumpWord(x.W)
	case Compare:
		return "(" + x.Op + " " + dumpWord(x.L) + " " + dumpWord(x.R) + ")"
	case Not:
		return "(! " + dumpExpr(x.X) + ")"
	case Binary:
		return "(" + x.Op + " " + dumpExpr(x.L) + " " + dumpExpr(x.R) + ")"
	default:
		panic(fmt.Sprintf("syntax: Dump: unknown expr %T", e))
	}
}
