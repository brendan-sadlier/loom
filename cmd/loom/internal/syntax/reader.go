package syntax

import (
	"errors"
	"io"
	"strings"
)

// span maps a byte offset in a logical line's joined text back to a physical line.
type span struct {
	off      int // offset in the joined text where this physical piece starts
	physLine int // 0-based physical line index
	physCol  int // 1-based column in that physical line of the piece's first byte
}

// line is one logical line: possibly several physical lines joined by "\" continuation.
type line struct {
	pos    Pos    // first physical line, col 1
	text   string // joined, comment-stripped, trailing whitespace trimmed, leading whitespace kept
	indent string // leading whitespace of the FIRST physical line, verbatim
	first  int    // 0-based index of first physical line
	last   int    // 0-based index of last physical line consumed
	spans  []span // always non-empty; spans[0].off == 0
}

// posAt maps an offset in l.text back to its physical file position.
// Offsets after a "\#" escape are off by one per escape on the same logical
// line, because the escape shrinks the text; columns are a diagnostic aid only.
func (l line) posAt(off int) Pos {
	if len(l.spans) == 0 {
		return Pos{File: l.pos.File, Line: l.pos.Line, Col: off + 1}
	}

	s := l.spans[0]
	for _, sp := range l.spans[1:] {
		if sp.off > off {
			break
		}
		s = sp
	}
	return Pos{File: l.pos.File, Line: s.physLine + 1, Col: s.physCol + off - s.off}
}

type reader struct {
	file  string
	lines []string // physical lines, "\r\n" and "\n" both normalised, no terminators
	i     int      // next physical line to read
}

func newReader(file string, src []byte) *reader {
	r := &reader{file: file}
	if len(src) == 0 {
		return r
	}

	r.lines = strings.Split(string(src), "\n")
	if r.lines[len(r.lines)-1] == "" {
		r.lines = r.lines[:len(r.lines)-1]
	}
	for i, ln := range r.lines {
		r.lines[i] = strings.TrimSuffix(ln, "\r")
	}
	return r
}

func (r *reader) eof() bool {
	return r.i >= len(r.lines)
}

// peekRaw returns the physical line at r.i. It panics at EOF
func (r *reader) peekRaw() string {
	return r.lines[r.i]
}

func (r *reader) skipRaw() {
	r.i++
}

func (r *reader) raw() []string {
	return r.lines
}

func (r *reader) seek(i int) {
	r.i = i
}

// isContinued reports whether s ends in a single "\" (not "\\").
func isContinued(s string) bool {
	n := len(s)
	return n >= 1 && s[n-1] == '\\' && (n < 2 || s[n-2] != '\\')
}

// next returns one logical line. It returns io.EOF when no lines remain.
// Continuation is applied before comment stripping, as in Make.
func (r *reader) next() (line, error) {
	if r.eof() {
		return line{}, io.EOF
	}
	first := r.i
	cur := r.lines[r.i]
	r.i++

	l := line{
		pos:    Pos{File: r.file, Line: first + 1, Col: 1},
		indent: cur[:len(cur)-len(strings.TrimLeft(cur, " \t"))],
		first:  first,
		spans:  []span{{off: 0, physLine: first, physCol: 1}},
	}

	var b strings.Builder
	for isContinued(cur) {
		cur = strings.TrimRight(cur[:len(cur)-1], " \t")
		if r.eof() {
			break // trailing "\" on the last line: drop it, nothing to join
		}
		b.WriteString(cur)
		b.WriteByte(' ')
		nxt := r.lines[r.i]
		trimmed := strings.TrimLeft(nxt, " \t")
		l.spans = append(l.spans, span{off: b.Len(), physLine: r.i, physCol: len(nxt) - len(trimmed) + 1})
		cur = trimmed
		r.i++
	}
	b.WriteString(cur)
	l.last = r.i - 1

	text, err := stripComment(b.String(), l.pos)
	if err != nil {
		var d *Diagnostic
		if errors.As(err, &d) {
			d.Pos = l.posAt(d.Pos.Col - l.pos.Col)
		}
		return line{}, err
	}
	l.text = strings.TrimRight(text, " \t")
	return l, nil
}

// stripComment removes an unquoted, unescaped "#" and everything after it.
// "#" is not a comment inside "..." or inside $( ... ) parentheses; "\#" becomes "#".
// It returns a Diagnostic for an unterminated quote.
func stripComment(s string, base Pos) (string, error) {
	var b strings.Builder
	inQuote := false
	quoteOff := 0
	depth := 0 // nesting of $( ... ), counting inner plain parentheses
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote:
			if c == '\\' && i+1 < len(s) && s[i+1] == '"' {
				b.WriteString(`\"`)
				i++
				continue
			}
			if c == '"' {
				inQuote = false
			}
			b.WriteByte(c)
		case c == '"':
			inQuote, quoteOff = true, i
			b.WriteByte(c)
		case c == '$' && i+1 < len(s) && s[i+1] == '$':
			b.WriteString("$$") // escaped dollar must not open $( on the next byte
			i++
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			depth++
			b.WriteString("$(")
			i++
		case depth > 0 && c == '(':
			depth++
			b.WriteByte(c)
		case depth > 0 && c == ')':
			depth--
			b.WriteByte(c)
		case depth == 0 && c == '\\' && i+1 < len(s) && s[i+1] == '#':
			b.WriteByte('#')
			i++
		case depth == 0 && c == '#':
			return strings.TrimRight(b.String(), " \t"), nil
		default:
			b.WriteByte(c)
		}
	}
	if inQuote {
		return "", Errorf(Pos{File: base.File, Line: base.Line, Col: base.Col + quoteOff}, "unterminated quoted string")
	}
	return strings.TrimRight(b.String(), " \t"), nil
}
