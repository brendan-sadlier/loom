package eval

import (
	"errors"
	"fmt"
	"strings"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

const hintUndefined = `define it with ":=" before use, or use $(env NAME, default) for optional values`

// expand interpolates w into a single string. Diagnostics point at the start of w.
func (ev *evaluator) expand(w syntax.Word) (string, error) {
	var b strings.Builder
	for _, seg := range w.Segs {
		switch s := seg.(type) {
		case syntax.Text:
			b.WriteString(s.S)
		case syntax.VarRef:
			v, ok := ev.scope.vars[s.Name]
			if !ok {
				return "", &syntax.Diagnostic{Pos: w.Pos, Msg: fmt.Sprintf("undefined variable %q", s.Name), Hint: hintUndefined}
			}
			b.WriteString(v)
		case syntax.Call:
			fn, ok := ev.funcs[s.Name]
			if !ok {
				return "", syntax.Errorf(w.Pos, "unknown function %q", s.Name)
			}
			args := make([]string, len(s.Args))
			for i, a := range s.Args {
				v, err := ev.expand(a)
				if err != nil {
					return "", err
				}
				args[i] = strings.TrimSpace(v)
			}
			out, err := fn(&Call{Name: s.Name, Args: args, Pos: w.Pos, Root: ev.root, Env: ev.scope.env, Vars: ev.scope.vars})
			if err != nil {
				var d *syntax.Diagnostic
				if errors.As(err, &d) {
					return "", err
				}
				return "", syntax.Errorf(w.Pos, "$(%s): %v", s.Name, err)
			}
			b.WriteString(out)
		default:
			panic(fmt.Sprintf("eval: unknown segment %T", seg))
		}
	}
	return b.String(), nil
}

// expandList expands w and splits it into list items unless w is quoted.
func (ev *evaluator) expandList(w syntax.Word) ([]string, error) {
	s, err := ev.expand(w)
	if err != nil {
		return nil, err
	}
	if w.Quoted {
		return []string{s}, nil
	}
	return splitList(s), nil
}

// splitList splits s on unquoted whitespace. Double quotes group and are removed;
// `\"` inside quotes yields a literal quote; an unterminated quote runs to the end.
// A quoted empty string ("") yields an empty item.
func splitList(s string) []string {
	var items []string
	var cur strings.Builder
	inWord, inQuote := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote && c == '\\' && i+1 < len(s) && s[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			inQuote = !inQuote
			inWord = true
		case !inQuote && (c == ' ' || c == '\t'):
			if inWord {
				items = append(items, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		items = append(items, cur.String())
	}
	return items
}
