package syntax

import "strings"

type tokKind int

const (
	tokWord tokKind = iota
	tokLParen
	tokRParen
	tokNot
	tokAnd
	tokOr
	tokEq
	tokNe
	tokEOF
)

type exprTok struct {
	kind tokKind
	text string
	off  int // offset in l.text
}

// parseExpr parses the text between "if" and the trailing "{".
// s is l.text[off:off+len(s)].
func parseExpr(l line, s string, off int) (Expr, error) {
	p := &exprParser{l: l, toks: lexExpr(s, off)}
	e, err := p.or()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tokEOF {
		return nil, p.unexpected(t)
	}
	return e, nil
}

// lexExpr splits s into operator and word tokens. Words use the same
// quote and $( grouping as lexWords; "!" alone is tokNot, "!=" is tokNe.
// Lexing never fails; malformed words are reported by the parser.
func lexExpr(s string, off int) []exprTok {
	var toks []exprTok
	i := 0
	for {
		i = skipSpace(s, i)
		if i >= len(s) {
			return append(toks, exprTok{kind: tokEOF, off: off + len(s)})
		}
		if k, n := exprOp(s, i); n > 0 {
			toks = append(toks, exprTok{kind: k, text: s[i : i+n], off: off + i})
			i += n
			continue
		}
		j := exprWordEnd(s, i)
		toks = append(toks, exprTok{kind: tokWord, text: s[i:j], off: off + i})
		i = j
	}
}

// exprOp reports the operator token at s[i], if any.
func exprOp(s string, i int) (tokKind, int) {
	r := s[i:]
	switch {
	case strings.HasPrefix(r, "=="):
		return tokEq, 2
	case strings.HasPrefix(r, "!="):
		return tokNe, 2
	case strings.HasPrefix(r, "&&"):
		return tokAnd, 2
	case strings.HasPrefix(r, "||"):
		return tokOr, 2
	case r[0] == '!':
		return tokNot, 1
	case r[0] == '(':
		return tokLParen, 1
	case r[0] == ')':
		return tokRParen, 1
	}
	return 0, 0
}

// exprWordEnd is wordEnd that also stops at depth-0 operators other than a
// lone "!" (so "a!b" stays one word). s[i] is not an operator.
func exprWordEnd(s string, i int) int {
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
		if depth == 0 {
			if isSpace(c) {
				return i
			}
			if k, n := exprOp(s, i); n > 0 && k != tokNot {
				return i
			}
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
		}
	}
	return i
}

type exprParser struct {
	l    line
	toks []exprTok
	i    int
}

func (p *exprParser) peek() exprTok { return p.toks[p.i] }

func (p *exprParser) unexpected(t exprTok) error {
	if t.kind == tokEOF {
		return Errorf(p.l.posAt(t.off), "expected expression")
	}
	return Errorf(p.l.posAt(t.off), "unexpected token %q in expression", t.text)
}

// or := and {"||" and}
func (p *exprParser) or() (Expr, error) {
	e, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOr {
		p.advance()
		r, err := p.and()
		if err != nil {
			return nil, err
		}
		e = Binary{Op: "||", L: e, R: r}
	}
	return e, nil
}

// and := unary {"&&" unary}
func (p *exprParser) and() (Expr, error) {
	e, err := p.unary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokAnd {
		p.advance()
		r, err := p.unary()
		if err != nil {
			return nil, err
		}
		e = Binary{Op: "&&", L: e, R: r}
	}
	return e, nil
}

// unary := "!" unary | primary
func (p *exprParser) unary() (Expr, error) {
	if p.peek().kind == tokNot {
		p.advance()
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return Not{X: x}, nil
	}
	return p.primary()
}

// primary := operand [("=="|"!=") operand] | "(" expr ")"
func (p *exprParser) primary() (Expr, error) {
	t := p.peek()
	switch t.kind {
	case tokLParen:
		p.advance()
		e, err := p.or()
		if err != nil {
			return nil, err
		}
		if c := p.peek(); c.kind != tokRParen {
			return nil, Errorf(p.l.posAt(c.off), `expected ")"`)
		}
		p.advance()
		return e, nil
	case tokWord:
		p.advance()
		lw, err := p.word(t)
		if err != nil {
			return nil, err
		}
		op := p.peek()
		if op.kind != tokEq && op.kind != tokNe {
			return Operand{W: lw}, nil
		}
		p.advance()
		r := p.peek()
		if r.kind != tokWord {
			return nil, p.unexpected(r)
		}
		p.advance()
		rw, err := p.word(r)
		if err != nil {
			return nil, err
		}
		return Compare{Op: op.text, L: lw, R: rw}, nil
	default:
		return nil, p.unexpected(t)
	}
}

// advance consumes the current token unless it is EOF.
func (p *exprParser) advance() {
	if p.toks[p.i].kind != tokEOF {
		p.i++
	}
}

func (p *exprParser) word(t exprTok) (Word, error) {
	segs, quoted, err := parseSegments(p.l, t.text, t.off, false)
	if err != nil {
		return Word{}, err
	}
	return Word{Pos: p.l.posAt(t.off), Quoted: quoted, Segs: segs}, nil
}
