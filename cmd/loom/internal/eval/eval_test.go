package eval

import (
	"errors"
	"runtime"
	"slices"
	"strings"
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

func TestSplitList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{`a "b c" d`, []string{"a", "b c", "d"}},
		{`""`, []string{""}},
		{`x "y`, []string{"x", "y"}},
		{"a\tb", []string{"a", "b"}},
		{`"a \"q\" b"`, []string{`a "q" b`}},
		{``, nil},
		{`  a   b  `, []string{"a", "b"}},
	}
	for _, tt := range tests {
		if got := splitList(tt.in); !slices.Equal(got, tt.want) || len(got) != len(tt.want) {
			t.Errorf("splitList(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func testEvaluator(t *testing.T, funcs map[string]Func) *evaluator {
	t.Helper()
	s, err := newScope(Options{GOOS: "linux", GOARCH: "amd64"}, "/r")
	if err != nil {
		t.Fatal(err)
	}
	s.vars["x"] = "one two"
	return &evaluator{root: "/r", scope: s, funcs: funcs}
}

var wpos = syntax.Pos{File: "Loomfile", Line: 3, Col: 6}

func word(quoted bool, segs ...syntax.Segment) syntax.Word {
	return syntax.Word{Pos: wpos, Quoted: quoted, Segs: segs}
}

func TestExpand(t *testing.T) {
	var gotArgs []string
	ev := testEvaluator(t, map[string]Func{
		"echo": func(c *Call) (string, error) { gotArgs = c.Args; return strings.Join(c.Args, "|"), nil },
		"fail": func(c *Call) (string, error) { return "", errors.New("boom") },
	})

	got, err := ev.expand(word(false, syntax.Text{S: "a-"}, syntax.VarRef{Name: "x"}, syntax.Text{S: "-$os"}))
	if err != nil || got != "a-one two-$os" {
		t.Errorf("expand = %q, %v", got, err)
	}

	// Nested call args are expanded and trimmed.
	call := syntax.Call{Name: "echo", Args: []syntax.Word{
		word(false, syntax.Text{S: "  "}, syntax.VarRef{Name: "os"}, syntax.Text{S: " "}),
		word(false, syntax.Call{Name: "echo", Args: []syntax.Word{word(false, syntax.Text{S: " in "})}}),
	}}
	got, err = ev.expand(word(false, call))
	if err != nil || got != "linux|in" || !slices.Equal(gotArgs, []string{"linux", "in"}) {
		t.Errorf("nested call = %q, %v, args %q", got, err, gotArgs)
	}

	list, err := ev.expandList(word(false, syntax.VarRef{Name: "x"}))
	if err != nil || !slices.Equal(list, []string{"one", "two"}) {
		t.Errorf("expandList = %q, %v", list, err)
	}
	list, err = ev.expandList(word(true, syntax.VarRef{Name: "x"}))
	if err != nil || !slices.Equal(list, []string{"one two"}) {
		t.Errorf("expandList quoted = %q, %v", list, err)
	}

	errTests := []struct {
		w          syntax.Word
		want, hint string
	}{
		{word(false, syntax.VarRef{Name: "y"}), `Loomfile:3:6: undefined variable "y"`, hintUndefined},
		{word(false, syntax.Call{Name: "glob"}), `Loomfile:3:6: unknown function "glob"`, ""},
		{word(false, syntax.Call{Name: "fail"}), `Loomfile:3:6: $(fail): boom`, ""},
		{word(false, syntax.Call{Name: "echo", Args: []syntax.Word{word(false, syntax.VarRef{Name: "z"})}}),
			`Loomfile:3:6: undefined variable "z"`, hintUndefined},
	}
	for _, tt := range errTests {
		_, err := ev.expand(tt.w)
		var d *syntax.Diagnostic
		if !errors.As(err, &d) || err.Error() != tt.want || d.Hint != tt.hint {
			t.Errorf("err = %v (hint %q), want %q (hint %q)", err, hintOf(d), tt.want, tt.hint)
		}
	}
}

func hintOf(d *syntax.Diagnostic) string {
	if d == nil {
		return ""
	}
	return d.Hint
}
