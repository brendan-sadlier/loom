package syntax

import (
	"fmt"
	"strconv"
)

// Pos is a 1-based line/column (byte column) in a named file.
// A zero Pos means "unknown".
type Pos struct {
	File string
	Line int
	Col  int
}

// IsValid reports whether p refers to a real source line.
func (p Pos) IsValid() bool { return p.Line > 0 }

// String returns "file:line:col", "file:line" when Col is 0, or "?" when p is invalid.
func (p Pos) String() string {
	if !p.IsValid() {
		return "?"
	}
	s := p.File + ":" + strconv.Itoa(p.Line)
	if p.Col > 0 {
		s += ":" + strconv.Itoa(p.Col)
	}
	return s
}

// Diagnostic is a user-facing error anchored to a position.
type Diagnostic struct {
	Pos  Pos
	Msg  string
	Hint string // optional; cli prints it on a following "  hint: ..." line
}

// Error returns "pos: msg". Hint is deliberately excluded so tests can assert
// the full string and cli controls hint rendering.
func (d *Diagnostic) Error() string { return d.Pos.String() + ": " + d.Msg }

// Errorf builds a Diagnostic. The message must not end with punctuation.
func Errorf(pos Pos, format string, args ...any) *Diagnostic {
	return &Diagnostic{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}
