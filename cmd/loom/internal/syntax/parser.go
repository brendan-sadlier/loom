package syntax

import (
	"os"
	"strings"
)

const msgCloser = `expected "}" , "} else {" or "} else if ... {"`

// Parse parses src as a Loomfile named name (used in positions and errors).
func Parse(name string, src []byte) (*File, error) {
	p := &parser{r: newReader(name, src), file: name}
	items, err := p.parseItems(false, "")
	if err != nil {
		return nil, err
	}
	return &File{Name: name, Items: items}, nil
}

// ParseFile reads and parses path. The file name in positions is path as given.
func ParseFile(path string) (*File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, src)
}

type parser struct {
	r       *reader
	file    string
	pending []string // doc-comment lines awaiting a target/task
}

// parseItems reads items until EOF or, when stopAtBrace is set, until a logical
// line starting with "}". In that case the reader is rewound to that line so the
// caller can re-read it with next().
func (p *parser) parseItems(stopAtBrace bool, blockIndent string) ([]Item, error) {
	_ = blockIndent // the closing "}" indent is checked by parseIf via closeLine
	var items []Item
	for !p.r.eof() {
		// Rule 2: comment lines are detected on the raw line, before stripping.
		if t := strings.TrimLeft(p.r.peekRaw(), " \t"); strings.HasPrefix(t, "#") {
			p.pending = append(p.pending, strings.TrimPrefix(t[1:], " "))
			p.r.skipRaw()
			continue
		}
		l, err := p.r.next()
		if err != nil {
			return nil, err
		}
		t := strings.TrimLeft(l.text, " \t")
		off := len(l.text) - len(t)
		switch {
		case t == "": // rule 1
			p.pending = nil
			continue
		case strings.HasPrefix(t, "}"): // rule 3
			if stopAtBrace {
				p.r.seek(l.first)
				return items, nil
			}
			return nil, Errorf(l.posAt(off), `unexpected "}"`)
		}
		it, err := p.parseItem(l)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// parseItem classifies a non-empty, non-comment, non-"}" logical line (rules 4–10).
func (p *parser) parseItem(l line) (Item, error) {
	t := strings.TrimLeft(l.text, " \t")
	off := len(l.text) - len(t)

	if _, _, _, ok := matchAssign(t); ok { // rule 4
		p.pending = nil
		return p.parseAssign(l, t, off)
	}
	switch firstWord(t) {
	case "task": // rule 5
		return p.parseTask(l)
	case "include", "dotenv", "shell": // rule 6
		p.pending = nil
		return p.parseDirective(l, firstWord(t))
	}
	switch firstWord(t) {
	case "task": // rule 5
		return p.parseTask(l)
	case "include", "dotenv", "shell": // rule 6
		p.pending = nil
		return p.parseDirective(l, firstWord(t))
	case "if": // rule 7
		p.pending = nil
		return p.parseIf(l)
	}
	if t == "go {" { // rule 8; "go" alone falls through to rule 10
		p.pending = nil
		return p.parseGoBlock(l)
	}
	if headerColon(t) >= 0 { // rule 9
		return p.parseTarget(l)
	}
	// Rule 10.
	if words, err := lexWords(l, t, off); err == nil {
		for i, w := range words {
			if lit, ok := w.Literal(); ok && !w.Quoted && lit == "{" && i != len(words)-1 {
				return nil, Errorf(w.Pos, `recipe body must start on the line after "{"`)
			}
		}
	}
	return nil, Errorf(l.posAt(off), "expected assignment, target declaration or directive")
}

// parseAssign parses text (which starts at off in l.text) as "name op value".
func (p *parser) parseAssign(l line, text string, off int) (*Assign, error) {
	name, op, v, ok := matchAssign(text)
	if !ok {
		return nil, Errorf(l.posAt(off), "expected assignment")
	}
	val, err := parseRaw(l, text[v:], off+v)
	if err != nil {
		return nil, err
	}
	return &Assign{P: l.posAt(off), Name: name, Op: op, Value: val}, nil
}

func (p *parser) parseTarget(l line) (*TargetDecl, error) {
	t := strings.TrimLeft(l.text, " \t")
	off := len(l.text) - len(t)
	ci := headerColon(t)

	names, err := lexWords(l, t[:ci], off)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, Errorf(l.posAt(off+ci), `expected target name before ":"`)
	}
	d := &TargetDecl{P: l.posAt(off), Names: names}

	right := t[ci+1:]
	rt := strings.TrimLeft(right, " \t")
	roff := off + ci + 1 + len(right) - len(rt)
	if _, _, _, ok := matchAssign(rt); ok {
		if strings.HasSuffix(rt, "{") {
			return nil, Errorf(l.posAt(roff+len(rt)-1), "target-specific variables cannot have a recipe body")
		}
		if d.Var, err = p.parseAssign(l, rt, roff); err != nil {
			return nil, err
		}
		d.Doc = p.takeDoc()
		return d, nil
	}

	hdr, err := parseDeps(l, right, off+ci+1, true)
	if err != nil {
		return nil, err
	}
	d.Prereqs, d.OrderOnly, d.Attrs = hdr.prereqs, hdr.orderOnly, hdr.attrs
	d.Doc = p.takeDoc()
	if hdr.hasBody {
		if d.Body, err = p.parseBodyAfter(l); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (p *parser) parseTask(l line) (*TaskDecl, error) {
	t := strings.TrimLeft(l.text, " \t")
	off := len(l.text) - len(t)
	s := t[len("task"):]
	base := off + len("task")
	d := &TaskDecl{P: l.posAt(off)}

	i := skipSpace(s, 0)
	tokEnd := i
	for tokEnd < len(s) && !isSpace(s[tokEnd]) && s[tokEnd] != ':' {
		tokEnd++
	}
	tok := s[i:tokEnd]
	switch {
	case tok == "" || tok == "{" || tok[0] == '@':
		return nil, Errorf(l.posAt(base+i), "expected task name")
	case !isIdent(tok):
		return nil, Errorf(l.posAt(base+i), "invalid task name %q: must match [A-Za-z_][A-Za-z0-9_]*", tok)
	}
	d.Name = tok
	i = tokEnd

	// Params: ident ["=" ("quoted" | word up to blank, ":" or "{")].
	seen := map[string]bool{}
	for {
		i = skipSpace(s, i)
		if i >= len(s) || !isIdentStart(s[i]) {
			break
		}
		start := i
		i = scanIdent(s, i)
		prm := Param{P: l.posAt(base + start), Name: s[start:i]}
		if seen[prm.Name] {
			return nil, Errorf(prm.P, "duplicate parameter %q", prm.Name)
		}
		seen[prm.Name] = true
		if i < len(s) && s[i] == '=' {
			k := i + 1
			end := defaultEnd(s, k)
			segs, quoted, err := parseSegments(l, s[k:end], base+k, false)
			if err != nil {
				return nil, err
			}
			prm.Default = &Word{Pos: l.posAt(base + k), Quoted: quoted, Segs: segs}
			i = end
		}
		d.Params = append(d.Params, prm)
	}

	rest := s[i:]
	roff := base + i
	allowPrereqs := false
	if strings.HasPrefix(rest, ":") {
		rest, roff, allowPrereqs = rest[1:], roff+1, true
		if rt := strings.TrimLeft(rest, " \t"); rt != "" {
			if _, _, _, ok := matchAssign(rt); ok {
				return nil, Errorf(l.posAt(roff+len(rest)-len(rt)), "tasks cannot have target-specific variables")
			}
		}
	}
	hdr, err := parseDeps(l, rest, roff, allowPrereqs)
	if err != nil {
		return nil, err
	}
	d.Prereqs, d.OrderOnly, d.Attrs = hdr.prereqs, hdr.orderOnly, hdr.attrs
	d.Doc = p.takeDoc()
	if hdr.hasBody {
		if d.Body, err = p.parseBodyAfter(l); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (p *parser) parseIf(l line) (*If, error) {
	return p.parseIfAt(l, len(l.text)-len(strings.TrimLeft(l.text, " \t")))
}

// parseIfAt parses an if header whose "if" keyword starts at l.text[at]. It is
// also used for "} else if", where l is the closing line of the previous block.
func (p *parser) parseIfAt(l line, at int) (*If, error) {
	pos := l.posAt(at)
	s := l.text[at+len("if"):]
	base := at + len("if")
	rt := strings.TrimRight(s, " \t")
	n := len(rt) - 1
	if n < 0 || rt[n] != '{' || (n > 0 && !isSpace(rt[n-1])) {
		return nil, Errorf(pos, `expected "{" at end of if line`)
	}
	cond, err := parseExpr(l, s[:n], base)
	if err != nil {
		return nil, err
	}
	x := &If{P: pos, Cond: cond}
	if x.Then, err = p.parseItems(true, l.indent); err != nil {
		return nil, err
	}
	p.pending = nil

	cl, off, err := p.closeLine(l.indent, pos)
	if err != nil {
		return nil, err
	}
	rest := cl.text[off+1:]
	switch f := strings.Fields(rest); {
	case len(f) == 0:
		return x, nil
	case len(f) == 2 && f[0] == "else" && f[1] == "{":
		if x.Else, err = p.parseItems(true, l.indent); err != nil {
			return nil, err
		}
		if x.Else == nil {
			x.Else = []Item{} // nil means "no else"
		}
		p.pending = nil
		cl2, off2, err := p.closeLine(l.indent, pos)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(cl2.text[off2+1:]) != "" {
			return nil, Errorf(cl2.posAt(off2), "%s", msgCloser)
		}
		return x, nil
	case f[0] == "else":
		j := skipSpace(cl.text, off+1) + len("else")
		if j < len(cl.text) && isSpace(cl.text[j]) {
			k := skipSpace(cl.text, j)
			if firstWord(cl.text[k:]) == "if" {
				sub, err := p.parseIfAt(cl, k)
				if err != nil {
					return nil, err
				}
				x.Else = []Item{sub}
				return x, nil
			}
		}
	}
	return nil, Errorf(cl.posAt(off), "%s", msgCloser)
}

// closeLine reads the "}" line that parseItems stopped at, checking its
// indentation. off is the offset of "}" in the returned line's text.
func (p *parser) closeLine(indent string, ifPos Pos) (line, int, error) {
	if p.r.eof() {
		return line{}, 0, Errorf(ifPos, `unterminated "if" block`)
	}
	cl, err := p.r.next()
	if err != nil {
		return line{}, 0, err
	}
	off := len(cl.text) - len(strings.TrimLeft(cl.text, " \t"))
	if cl.indent != indent {
		return line{}, 0, Errorf(cl.posAt(off), `"}" must be at the indentation of the matching "if"`)
	}
	return cl, off, nil
}

func (p *parser) parseGoBlock(l line) (*GoBlock, error) {
	off := len(l.text) - len(strings.TrimLeft(l.text, " \t"))
	b, next, err := readBody(p.file, p.r.raw(), l.last+1, l.indent, l.pos)
	if err != nil {
		return nil, err
	}
	p.r.seek(next)
	return &GoBlock{P: l.posAt(off), Body: b}, nil
}

// defaultEnd returns the end of a parameter default starting at s[k]: a quoted
// string (with \" escapes), or a run up to blank, ":" or "{". An unterminated
// quote runs to the end; parseSegments reports it.
func defaultEnd(s string, k int) int {
	if k < len(s) && s[k] == '"' {
		for j := k + 1; j < len(s); j++ {
			if s[j] == '\\' && j+1 < len(s) && s[j+1] == '"' {
				j++
			} else if s[j] == '"' {
				return j + 1
			}
		}
		return len(s)
	}
	j := k
	for j < len(s) && !isSpace(s[j]) && s[j] != ':' && s[j] != '{' {
		j++
	}
	return j
}

func (p *parser) parseDirective(l line, kw string) (Item, error) {
	t := strings.TrimLeft(l.text, " \t")
	off := len(l.text) - len(t)
	pos := l.posAt(off)
	words, err := lexWords(l, t[len(kw):], off+len(kw))
	if err != nil {
		return nil, err
	}
	if n := len(words); n > 0 {
		if lit, ok := words[n-1].Literal(); ok && !words[n-1].Quoted && lit == "{" {
			return nil, Errorf(words[n-1].Pos, "directives cannot have a body")
		}
	}
	switch kw {
	case "include":
		if len(words) == 0 {
			return nil, Errorf(pos, "include requires at least one path")
		}
		return &Include{P: pos, Paths: words}, nil
	case "dotenv":
		if len(words) > 1 {
			return nil, Errorf(pos, "dotenv takes at most one path")
		}
		d := &Dotenv{P: pos}
		if len(words) == 1 {
			d.Path = &words[0]
		}
		return d, nil
	default: // shell
		if len(words) == 0 {
			return nil, Errorf(pos, "shell requires a program name")
		}
		return &ShellDir{P: pos, Args: words}, nil
	}
}

// parseBodyAfter reads the body following header l and seeks the reader past it.
func (p *parser) parseBodyAfter(l line) (*Body, error) {
	b, next, err := readBody(p.file, p.r.raw(), l.last+1, l.indent, l.pos)
	if err != nil {
		return nil, err
	}
	p.r.seek(next)
	return &b, nil
}

func (p *parser) takeDoc() string {
	doc := strings.Join(p.pending, "\n")
	p.pending = nil
	return doc
}

// header is the right-hand side of a target or task header.
type header struct {
	prereqs, orderOnly []Word
	attrs              []string
	hasBody            bool
}

var validAttrs = map[string]bool{"sh": true, "go": true, "bash": true, "pwsh": true}

// parseDeps walks the words after a header colon. When allowPrereqs is false
// (task header without ":"), only attributes and "{" are accepted.
func parseDeps(l line, s string, off int, allowPrereqs bool) (header, error) {
	var h header
	words, err := lexWords(l, s, off)
	if err != nil {
		return h, err
	}
	pipe := false
	for i, w := range words {
		if lit, ok := w.Literal(); ok && !w.Quoted {
			switch {
			case lit == "|":
				if pipe {
					return h, Errorf(w.Pos, `only one "|" allowed`)
				}
				pipe = true
				continue
			case strings.HasPrefix(lit, "@"):
				if !validAttrs[lit[1:]] {
					return h, Errorf(w.Pos, "unknown attribute %q (expected @sh, @go, @bash or @pwsh)", lit)
				}
				if !contains(h.attrs, lit[1:]) {
					h.attrs = append(h.attrs, lit[1:])
				}
				continue
			case lit == "{":
				if i != len(words)-1 {
					return h, Errorf(w.Pos, `recipe body must start on the line after "{"`)
				}
				h.hasBody = true
				continue
			}
		}
		if !allowPrereqs {
			return h, Errorf(w.Pos, `expected ":" before prerequisites`)
		}
		if pipe {
			h.orderOnly = append(h.orderOnly, w)
		} else {
			h.prereqs = append(h.prereqs, w)
		}
	}
	return h, nil
}

// matchAssign matches ^[A-Za-z_][A-Za-z0-9_]*\s*(:=|\?=|\+=) and returns the
// name, operator and the offset just after the operator.
func matchAssign(s string) (name string, op AssignOp, valOff int, ok bool) {
	end := scanIdent(s, 0)
	if end == 0 {
		return "", "", 0, false
	}
	j := skipSpace(s, end)
	if j+2 <= len(s) {
		switch o := s[j : j+2]; o {
		case ":=", "?=", "+=":
			return s[:end], AssignOp(o), j + 2, true
		}
	}
	return "", "", 0, false
}

// headerColon returns the index of the first ":" not inside "..." or $( ... )
// and not followed by "=", or -1.
func headerColon(s string) int {
	inQuote := false
	depth := 0
	for i := 0; i < len(s); i++ {
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
		case depth == 0 && c == ':' && (i+1 >= len(s) || s[i+1] != '='):
			return i
		}
	}
	return -1
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

func skipSpace(s string, i int) int {
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
