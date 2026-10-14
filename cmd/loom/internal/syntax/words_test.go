package syntax

import (
	"testing"
)

// rawLine builds a single-physical-line line without comment stripping.
func rawLine(s string) line {
	return line{
		pos:   Pos{File: "Loomfile", Line: 1, Col: 1},
		text:  s,
		spans: []span{{off: 0, physLine: 0, physCol: 1}},
	}
}

func TestParseRaw(t *testing.T) {
	tests := []struct{ src, want string }{
		{`x := $name`, `[V:name]`},
		{`x := ${name}`, `[V:name]`},
		{`x := ${CC}x`, `[V:CC T:"x"]`},
		{`x := $bin/server$exe`, `[V:bin T:"/server" V:exe]`},
		{`x := $(fn)`, `[C:fn()]`},
		{`x := $(fn a, b)`, `[C:fn([T:"a"], [T:"b"])]`},
		{`x := $(f a,)`, `[C:f([T:"a"], [])]`},
		{`x := $(f $(g x), y)`, `[C:f([C:g([T:"x"])], [T:"y"])]`},
		{`x := $(glob **/*.go) go.mod go.sum`, `[C:glob([T:"**/*.go"]) T:" go.mod go.sum"]`},
		{`x := cost $$5`, `[T:"cost $5"]`},
		{`x := $$(notcall)`, `[T:"$(notcall)"]`},
		{`x := "a b" c`, `[T:"\"a b\" c"]`},
		{`x := "a \" b"`, `[T:"\"a \\\" b\""]`},
		{`x :=   padded   `, `[T:"padded"]`},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			w, err := parseRaw(rawLine(tt.src), tt.src[4:], 4)
			if err != nil {
				t.Fatal(err)
			}
			if got := dumpWord(w); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
			if w.Quoted {
				t.Error("raw word reported Quoted")
			}
		})
	}
}

func TestParseRawEmpty(t *testing.T) {
	w, err := parseRaw(rawLine("x :="), "", 4)
	if err != nil {
		t.Fatal(err)
	}
	if !w.IsEmpty() || dumpWord(w) != "[]" {
		t.Errorf("got %s, want []", dumpWord(w))
	}
}

func TestSegmentErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{`x := ${name`, `Loomfile:1:6: unterminated "${": missing "}"`},
		{`x := ${a-b}`, `Loomfile:1:6: invalid "$" sequence; use $name, ${name}, $(fn ...) or $$`},
		{`x := $(glob a`, `Loomfile:1:6: unterminated "$(": missing ")"`},
		{`x := $(1x)`, `Loomfile:1:6: expected function name after "$("`},
		{`x := $(a-b x)`, `Loomfile:1:6: expected function name after "$("`},
		{`x := $ y`, `Loomfile:1:6: invalid "$" sequence; use $name, ${name}, $(fn ...) or $$`},
		{`x := $`, `Loomfile:1:6: invalid "$" sequence; use $name, ${name}, $(fn ...) or $$`},
		{`x := "abc`, `Loomfile:1:6: unterminated quoted string`},
		{`x := $(f $ )`, `Loomfile:1:10: invalid "$" sequence; use $name, ${name}, $(fn ...) or $$`},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			_, err := parseRaw(rawLine(tt.src), tt.src[4:], 4)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLexWords(t *testing.T) {
	src := `a "b c" d$x$(f 1, "2,3")$$`
	words, err := lexWords(rawLine(src), src, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		dump string
		col  int
	}{
		{`[T:"a"]`, 1},
		{`Q[T:"b c"]`, 3},
		{`[T:"d" V:x C:f([T:"1"], [T:"\"2,3\""]) T:"$"]`, 9},
	}
	if len(words) != len(want) {
		t.Fatalf("got %d words, want %d", len(words), len(want))
	}
	for i, w := range want {
		if got := dumpWord(words[i]); got != w.dump {
			t.Errorf("word %d = %s, want %s", i, got, w.dump)
		}
		if words[i].Pos.Col != w.col {
			t.Errorf("word %d col = %d, want %d", i, words[i].Pos.Col, w.col)
		}
	}
}

func TestLexWordsQuoting(t *testing.T) {
	tests := []struct{ src, want string }{
		{`"a\"b"`, `Q[T:"a\"b"]`},
		{`x"y z"w`, `Q[T:"xy zw"]`},
		{`"$x y"`, `Q[V:x T:" y"]`},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			words, err := lexWords(rawLine(tt.src), tt.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(words) != 1 || dumpWord(words[0]) != tt.want {
				t.Fatalf("got %d words %v, want %s", len(words), words, tt.want)
			}
		})
	}
}

func TestLexWordsContinuedLinePos(t *testing.T) {
	l, err := newReader("Loomfile", []byte("a: b \\\n    c\n")).next()
	if err != nil {
		t.Fatal(err)
	}
	words, err := lexWords(l, l.text[2:], 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 2 {
		t.Fatalf("got %d words, want 2", len(words))
	}
	if got, want := words[1].Pos, (Pos{File: "Loomfile", Line: 2, Col: 5}); got != want {
		t.Errorf("pos = %v, want %v", got, want)
	}
}

func TestWordLiteral(t *testing.T) {
	if s, ok := (Word{Segs: []Segment{Text{S: "a"}}}).Literal(); !ok || s != "a" {
		t.Errorf("Literal = %q, %v", s, ok)
	}
	if _, ok := (Word{Segs: []Segment{VarRef{Name: "x"}}}).Literal(); ok {
		t.Error("VarRef word reported literal")
	}
	if s, ok := (Word{}).Literal(); !ok || s != "" {
		t.Errorf("empty Literal = %q, %v", s, ok)
	}
}
