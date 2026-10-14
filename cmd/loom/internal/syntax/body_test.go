package syntax

import "testing"

func TestReadBody(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string // lines[0] is the header
		indent   string
		wantText string
		wantNext int
	}{
		{
			name:     "go body with nested braces",
			lines:    []string{"go {", "    func f() {", "        x()", "    }", "}"},
			wantText: "func f() {\n    x()\n}\n",
			wantNext: 5,
		},
		{
			name:     "blank lines kept inside, dropped at end",
			lines:    []string{"a: {", "    one", "", "  ", "    two", "", "\t", "}"},
			wantText: "one\n\n\ntwo\n",
			wantNext: 8,
		},
		{
			name:     "tab body under space header",
			lines:    []string{"    t: {", "    \tx", "    \t\ty", "    }"},
			indent:   "    ",
			wantText: "x\n\ty\n",
			wantNext: 4,
		},
		{
			name:     "deeper brace is not a terminator",
			lines:    []string{"a: {", "    }", "  }", "}", "after"},
			wantText: "  }\n}\n",
			wantNext: 4,
		},
		{
			name:     "empty body",
			lines:    []string{"a: {", "}"},
			wantText: "",
			wantNext: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := Pos{File: "Loomfile", Line: 1, Col: 1}
			b, next, err := readBody("Loomfile", tt.lines, 1, tt.indent, hdr)
			if err != nil {
				t.Fatal(err)
			}
			if b.Text != tt.wantText {
				t.Errorf("text = %q, want %q", b.Text, tt.wantText)
			}
			if next != tt.wantNext {
				t.Errorf("next = %d, want %d", next, tt.wantNext)
			}
			if want := (Pos{File: "Loomfile", Line: 2, Col: 1}); b.Pos != want {
				t.Errorf("pos = %v, want %v", b.Pos, want)
			}
		})
	}
}

func TestReadBodyErrors(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		indent string
		want   string
	}{
		{
			name:   "brace at wrong indentation",
			lines:  []string{"    t: {", "        a", "}"},
			indent: "    ",
			want:   "Loomfile:3:1: " + msgBodyIndent,
		},
		{
			name:  "unindented body line",
			lines: []string{"build: a {", "echo hi", "}"},
			want:  "Loomfile:2:1: " + msgBodyIndent,
		},
		{
			name:  "eof",
			lines: []string{"build: a {", "    echo hi"},
			want:  "Loomfile:1:1: unterminated recipe body",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := Pos{File: "Loomfile", Line: 1, Col: 1}
			_, _, err := readBody("Loomfile", tt.lines, 1, tt.indent, hdr)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}
