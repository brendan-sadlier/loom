package syntax

import (
	"errors"
	"io"
	"testing"
)

func TestStripComment(t *testing.T) {
	tests := []struct{ in, want string }{
		{`x := a #c`, `x := a`},
		{`x := "a # b" # c`, `x := "a # b"`},
		{`x := $(shell echo '#') # c`, `x := $(shell echo '#')`},
		{`x := $(f (a#b)) #c`, `x := $(f (a#b))`},
		{`x := a \# b`, `x := a # b`},
		{`x := "a \" # b" #c`, `x := "a \" # b"`},
		{`x := $$(notcall) # c`, `x := $$(notcall)`},
		{`# whole line`, ``},
		{`a: b \\`, `a: b \\`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := stripComment(tt.in, Pos{File: "f", Line: 1, Col: 1})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripCommentUnterminatedQuote(t *testing.T) {
	_, err := stripComment(`x := "abc`, Pos{File: "Loomfile", Line: 1, Col: 1})
	const want = "Loomfile:1:6: unterminated quoted string"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestContinuation(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantText  string
		wantLast  int
		wantLines int // logical lines in total
	}{
		{"join", "a: b \\\n   c\n", "a: b c", 1, 1},
		{"three lines", "a: b \\\n  c \\\n\td\n", "a: b c d", 2, 1},
		{"double backslash", "a: b \\\\\nc\n", `a: b \\`, 0, 2},
		{"crlf", "x := a\r\n", "x := a", 0, 1},
		{"lone cr", "x := a\r", "x := a", 0, 1},
		{"trailing backslash at eof", "a: b \\", "a: b", 0, 1},
		{"continuation then comment", "a: b \\\n  c # d\n", "a: b c", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReader("f", []byte(tt.src))
			l, err := r.next()
			if err != nil {
				t.Fatal(err)
			}
			if l.text != tt.wantText {
				t.Errorf("text = %q, want %q", l.text, tt.wantText)
			}
			if l.pos != (Pos{File: "f", Line: 1, Col: 1}) {
				t.Errorf("pos = %v, want f:1:1", l.pos)
			}
			if l.first != 0 || l.last != tt.wantLast {
				t.Errorf("first,last = %d,%d, want 0,%d", l.first, l.last, tt.wantLast)
			}
			n := 1
			for !r.eof() {
				if _, err := r.next(); err != nil {
					t.Fatal(err)
				}
				n++
			}
			if n != tt.wantLines {
				t.Errorf("logical lines = %d, want %d", n, tt.wantLines)
			}
		})
	}
}

func TestReader(t *testing.T) {
	src := "  x := 1\r\n\n\ta: b \\\n    cc\n"
	r := newReader("Loomfile", []byte(src))

	if got := len(r.raw()); got != 4 {
		t.Fatalf("raw lines = %d, want 4", got)
	}
	if got := r.peekRaw(); got != "  x := 1" {
		t.Errorf("peekRaw = %q", got)
	}

	l, err := r.next()
	if err != nil {
		t.Fatal(err)
	}
	if l.text != "  x := 1" || l.indent != "  " {
		t.Errorf("line 1 = %q indent %q", l.text, l.indent)
	}

	r.skipRaw() // blank line
	l, err = r.next()
	if err != nil {
		t.Fatal(err)
	}
	if l.text != "\ta: b cc" || l.indent != "\t" || l.first != 2 || l.last != 3 {
		t.Errorf("line 3 = %+v", l)
	}
	// "cc" starts at offset 6 in "\ta: b cc" and at column 5 of physical line 4.
	if got, want := l.posAt(6), (Pos{File: "Loomfile", Line: 4, Col: 5}); got != want {
		t.Errorf("posAt(6) = %v, want %v", got, want)
	}
	if got, want := l.posAt(1), (Pos{File: "Loomfile", Line: 3, Col: 2}); got != want {
		t.Errorf("posAt(1) = %v, want %v", got, want)
	}

	if !r.eof() {
		t.Error("expected eof")
	}
	if _, err := r.next(); !errors.Is(err, io.EOF) {
		t.Errorf("next at eof = %v, want io.EOF", err)
	}

	r.seek(0)
	if r.eof() || r.peekRaw() != "  x := 1" {
		t.Error("seek(0) did not rewind")
	}
}

func TestReaderQuoteErrorOnContinuedLine(t *testing.T) {
	r := newReader("Loomfile", []byte("x := a \\\n  \"bc\n"))
	_, err := r.next()
	const want = "Loomfile:2:3: unterminated quoted string"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestReaderEmpty(t *testing.T) {
	if r := newReader("f", nil); !r.eof() {
		t.Error("empty source not at eof")
	}
}
