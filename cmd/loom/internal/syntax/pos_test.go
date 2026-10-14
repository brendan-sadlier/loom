package syntax

import (
	"errors"
	"testing"
)

func TestPosString(t *testing.T) {
	tests := []struct {
		name string
		pos  Pos
		want string
	}{
		{"full", Pos{File: "Loomfile", Line: 3, Col: 7}, "Loomfile:3:7"},
		{"no column", Pos{File: "Loomfile", Line: 3}, "Loomfile:3"},
		{"zero", Pos{}, "?"},
		{"file only", Pos{File: "Loomfile"}, "Loomfile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pos.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPosIsValid(t *testing.T) {
	if (Pos{}).IsValid() {
		t.Error("zero Pos reported valid")
	}
	if !(Pos{File: "f", Line: 1}).IsValid() {
		t.Error("Pos with Line 1 reported invalid")
	}
}

func TestDiagnosticError(t *testing.T) {
	d := Errorf(Pos{File: "Loomfile", Line: 1, Col: 6}, "unterminated %q: missing %q", "$(", ")")
	d.Hint = "close the call"
	const want = `Loomfile:1:6: unterminated "$(": missing ")"`
	if got := d.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	var err error = d
	var target *Diagnostic
	if !errors.As(err, &target) || target != d {
		t.Error("errors.As failed to recover *Diagnostic")
	}
}
