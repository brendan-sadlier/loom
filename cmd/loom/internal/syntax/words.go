package syntax

import "strings"

// Word is interpolatable text: a header word, prerequisite, directive argument, or a
// whole assignment value. Adjacent Text segments are always merged.
type Word struct {
	Pos    Pos
	Quoted bool      // any part was inside "..." (words only; raw_text is never Quoted)
	Segs   []Segment // empty for an empty value
}

// Segment is one piece of a Word: Text, VarRef or Call.
type Segment interface{ seg() }

// Text is literal text; "$$" is already collapsed to "$".
type Text struct{ S string }

// VarRef is $name or ${name}.
type VarRef struct{ Name string }

// Call is $(name arg, arg, ...).
type Call struct {
	Name string
	Args []Word // each trimmed of surrounding whitespace, may be empty
}

func (Text) seg()   {}
func (VarRef) seg() {}
func (Call) seg()   {}

const (
	msgDollar      = `invalid "$" sequence; use $name, ${name}, $(fn ...) or $$`
	msgUntermBrace = `unterminated "${": missing "}"`
	msgUntermCall  = `unterminated "$(": missing ")"`
	msgCallName    = `expected function name after "$("`
	msgUntermQuote = `unterminated quoted string`
)

// IsEmpty reports whether w has no segments (an empty value).
func (w Word) IsEmpty() bool { return len(w.Segs) == 0 }

// Literal returns the text if w consists only of Text segments, else "" and false.
// An empty word is the empty literal.
func (w Word) Literal() (string, bool) {
	var b strings.Builder
	for _, s := range w.Segs {
		t, ok := s.(Text)
		if !ok {
			return "", false
		}
		b.WriteString(t.S)
	}
	return b.String(), true
}

// lexWords splits s into whitespace-separated words honouring "..." quoting and the
// grouping of $( ... ). Quote characters are removed; `\"` inside quotes yields `"`.
// s is l.text[off:off+len(s)]; positions come from l.posAt.
func lexWords(l line, s string, off int) ([]Word, error) {
	var words []Word
	i := 0
	for {
		for i < len(s) && isBlank(s[i]) {
			i++
		}
		if i >= len(s) {
			return words, nil
		}
		start := i
		i = wordEnd(s, i)
		segs, quoted, err := parseSegments(l, s[start:i], off+start, false)
		if err != nil {
			return nil, err
		}
		words = append(words, Word{Pos: l.posAt(off + start), Quoted: quoted, Segs: segs})
	}
}

// wordEnd returns the index of the first blank at quote depth 0 and $( depth 0.
// Unterminated quotes or calls run to the end; parseSegments reports them.
func wordEnd(s string, i int) int {
	inQuote := false
	depth := 0
	for ; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == '\\' && i+1 < len(s) && s[i+1] == '"' {
				i++
			} else if c == '"' {
				inQuote = false
			}
			continue
		}
		switch {
		case c == '"':
			inQuote = true
		case c == '$' && i+1 < len(s) && s[i+1] == '$':
			i++
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			depth++
			i++
		case depth > 0 && c == '(':
			depth++
		case depth > 0 && c == ')':
			depth--
		case depth == 0 && isBlank(c):
			return i
		}
	}
	return i
}

// parseRaw parses an entire assignment value as ONE Word. Quotes are kept literally.
// Leading and trailing whitespace are trimmed.
func parseRaw(l line, s string, off int) (Word, error) {
	a, b := trimBlank(s, 0, len(s))
	segs, _, err := parseSegments(l, s[a:b], off+a, true)
	if err != nil {
		return Word{}, err
	}
	return Word{Pos: l.posAt(off + a), Segs: segs}, nil
}

// parseSegments is the shared "$" grammar used by lexWords and parseRaw.
// keepQuotes controls whether '"' is emitted as text (raw) or removed (words).
// In raw mode `\"` is kept verbatim; in word mode it yields `"`.
func parseSegments(l line, s string, off int, keepQuotes bool) (segs []Segment, quoted bool, err error) {
	inQuote := false
	quoteOff := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case inQuote && c == '\\' && i+1 < len(s) && s[i+1] == '"':
			if keepQuotes {
				segs = appendText(segs, `\"`)
			} else {
				segs = appendText(segs, `"`)
			}
			i += 2
		case c == '"':
			if !inQuote {
				quoteOff = i
				quoted = true
			}
			inQuote = !inQuote
			if keepQuotes {
				segs = appendText(segs, `"`)
			}
			i++
		case c == '$':
			seg, next, err := parseDollar(l, s, off, i)
			if err != nil {
				return nil, false, err
			}
			segs = appendSeg(segs, seg)
			i = next
		default:
			j := i + 1
			for j < len(s) && s[j] != '"' && s[j] != '$' && s[j] != '\\' {
				j++
			}
			segs = appendText(segs, s[i:j])
			i = j
		}
	}
	if inQuote {
		return nil, false, Errorf(l.posAt(off+quoteOff), "%s", msgUntermQuote)
	}
	return segs, quoted && !keepQuotes, nil
}

// parseDollar parses the "$" construct at s[i] and returns the segment and the
// index just past it.
func parseDollar(l line, s string, off, i int) (Segment, int, error) {
	at := l.posAt(off + i)
	if i+1 >= len(s) {
		return nil, 0, Errorf(at, "%s", msgDollar)
	}
	switch c := s[i+1]; {
	case c == '$':
		return Text{S: "$"}, i + 2, nil
	case c == '{':
		j := strings.IndexByte(s[i+2:], '}')
		if j < 0 {
			return nil, 0, Errorf(at, "%s", msgUntermBrace)
		}
		name := s[i+2 : i+2+j]
		if !isIdent(name) {
			return nil, 0, Errorf(at, "%s", msgDollar)
		}
		return VarRef{Name: name}, i + 3 + j, nil
	case c == '(':
		return parseCall(l, s, off, i)
	case isIdentStart(c):
		end := scanIdent(s, i+1)
		return VarRef{Name: s[i+1 : end]}, end, nil
	default:
		return nil, 0, Errorf(at, "%s", msgDollar)
	}
}

// parseCall parses $(name args...) starting at the "$" at s[i].
func parseCall(l line, s string, off, i int) (Segment, int, error) {
	at := l.posAt(off + i)
	nameStart := i + 2
	nameEnd := scanIdent(s, nameStart)
	if nameEnd == nameStart {
		return nil, 0, Errorf(at, "%s", msgCallName)
	}
	if nameEnd < len(s) && s[nameEnd] != ')' && !isBlank(s[nameEnd]) {
		return nil, 0, Errorf(at, "%s", msgCallName)
	}

	// Find top-level commas and the matching ")"; quotes and nested parens group.
	var bounds [][2]int
	argStart := nameEnd
	closeAt := -1
	depth := 1
	inQuote := false
scan:
	for j := nameEnd; j < len(s); j++ {
		c := s[j]
		if inQuote {
			if c == '\\' && j+1 < len(s) && s[j+1] == '"' {
				j++
			} else if c == '"' {
				inQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inQuote = true
		case '$':
			if j+1 < len(s) && s[j+1] == '$' {
				j++
			} else if j+1 < len(s) && s[j+1] == '(' {
				depth++
				j++
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				bounds = append(bounds, [2]int{argStart, j})
				closeAt = j
				break scan
			}
		case ',':
			if depth == 1 {
				bounds = append(bounds, [2]int{argStart, j})
				argStart = j + 1
			}
		}
	}
	if closeAt < 0 {
		return nil, 0, Errorf(at, "%s", msgUntermCall)
	}

	call := Call{Name: s[nameStart:nameEnd]}
	if len(bounds) == 1 {
		if a, b := trimBlank(s, bounds[0][0], bounds[0][1]); a == b {
			return call, closeAt + 1, nil // $(fn) has no args
		}
	}
	for _, bd := range bounds {
		a, b := trimBlank(s, bd[0], bd[1])
		segs, _, err := parseSegments(l, s[a:b], off+a, true)
		if err != nil {
			return nil, 0, err
		}
		call.Args = append(call.Args, Word{Pos: l.posAt(off + a), Segs: segs})
	}
	return call, closeAt + 1, nil
}

func appendText(segs []Segment, s string) []Segment {
	if s == "" {
		return segs
	}
	if n := len(segs); n > 0 {
		if t, ok := segs[n-1].(Text); ok {
			segs[n-1] = Text{S: t.S + s}
			return segs
		}
	}
	return append(segs, Text{S: s})
}

func appendSeg(segs []Segment, s Segment) []Segment {
	if t, ok := s.(Text); ok {
		return appendText(segs, t.S)
	}
	return append(segs, s)
}

// trimBlank narrows [a,b) of s past leading and trailing spaces/tabs.
func trimBlank(s string, a, b int) (int, int) {
	for a < b && isBlank(s[a]) {
		a++
	}
	for b > a && isBlank(s[b-1]) {
		b--
	}
	return a, b
}

// isBlank is a space or tab. (body.go's isSpace arrives verbatim in P01-T04.)
func isBlank(c byte) bool { return c == ' ' || c == '\t' }

func isIdentStart(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isIdentChar(c byte) bool { return isIdentStart(c) || ('0' <= c && c <= '9') }

// scanIdent returns the end of the identifier starting at s[i], or i if none.
func scanIdent(s string, i int) int {
	if i >= len(s) || !isIdentStart(s[i]) {
		return i
	}
	j := i + 1
	for j < len(s) && isIdentChar(s[j]) {
		j++
	}
	return j
}

func isIdent(s string) bool { return s != "" && scanIdent(s, 0) == len(s) }
