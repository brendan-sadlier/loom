package eval

import (
	"errors"
	"runtime"
	"testing"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

func TestEnvLookup(t *testing.T) {
	e := NewEnv([]string{"A=1", "B=", "A=2", "=C:=C:\\x", "EQ=a=b", "junk"})
	tests := []struct {
		name, want string
		ok         bool
	}{
		{"A", "1", true}, // first wins
		{"B", "", true},
		{"EQ", "a=b", true},
		{"=C:", "C:\\x", true},
		{"missing", "", false},
		{"", "", false},
		{"junk", "", false},
	}
	for _, tt := range tests {
		got, ok := e.Lookup(tt.name)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Lookup(%q) = %q,%v want %q,%v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
	_, ok := e.Lookup("a")
	if want := runtime.GOOS == "windows"; ok != want {
		t.Errorf("Lookup(\"a\") ok = %v, want %v on %s", ok, want, runtime.GOOS)
	}
	if got := e.Environ(); len(got) != 6 || got[0] != "A=1" {
		t.Errorf("Environ() = %q", got)
	}
}

func TestNewScopeBuiltins(t *testing.T) {
	for _, tt := range []struct{ goos, exe string }{{"linux", ""}, {"windows", ".exe"}} {
		s, err := newScope(Options{GOOS: tt.goos, GOARCH: "arm64", LoomPath: "/bin/loom",
			Overrides: map[string]string{"x": "cli"}}, "/r")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"os": tt.goos, "arch": "arm64", "exe": tt.exe, "root": "/r", "loom": "/bin/loom", "x": "cli"}
		for k, v := range want {
			if s.vars[k] != v {
				t.Errorf("%s: vars[%q] = %q, want %q", tt.goos, k, s.vars[k], v)
			}
		}
		if len(s.vars) != len(want) || s.overrides["x"] != "cli" {
			t.Errorf("%s: vars %v overrides %v", tt.goos, s.vars, s.overrides)
		}
	}
	s, err := newScope(Options{}, "/r")
	if err != nil {
		t.Fatal(err)
	}
	if s.vars["os"] != runtime.GOOS || s.vars["arch"] != runtime.GOARCH {
		t.Errorf("default GOOS/GOARCH not applied: %v", s.vars)
	}
}

func TestNewScopeOverrideErrors(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"1x", `command line: invalid variable name "1x" in command-line override`},
		{"a-b", `command line: invalid variable name "a-b" in command-line override`},
		{"os", `command line: cannot override built-in variable "os"`},
		{"root", `command line: cannot override built-in variable "root"`},
		{"out", `command line: "out" is reserved for recipes and cannot be assigned`},
		{"LOOM_CTX", `command line: "LOOM_CTX" is reserved for recipes and cannot be assigned`},
	}
	for _, tt := range tests {
		_, err := newScope(Options{Overrides: map[string]string{tt.name: "v"}}, "/r")
		var d *syntax.Diagnostic
		if !errors.As(err, &d) {
			t.Fatalf("%s: err = %v, want *syntax.Diagnostic", tt.name, err)
		}
		if err.Error() != tt.want {
			t.Errorf("%s:\n got %s\nwant %s", tt.name, err, tt.want)
		}
	}
}
