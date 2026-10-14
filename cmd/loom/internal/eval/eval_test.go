package eval

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
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

type asg struct {
	op    syntax.AssignOp
	value string
}

func TestAssignPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		env       []string
		overrides map[string]string
		ops       []asg
		want      string
		wantErr   string
	}{
		{"env then set", []string{"a=E"}, nil, []asg{{syntax.OpSet, "F"}}, "F", ""},
		{"env then default", []string{"a=E"}, nil, []asg{{syntax.OpDefault, "F"}}, "E", ""},
		{"default no env", nil, nil, []asg{{syntax.OpDefault, "F"}}, "F", ""},
		{"set then default", nil, nil, []asg{{syntax.OpSet, "F"}, {syntax.OpDefault, "G"}}, "F", ""},
		{"override wins", nil, map[string]string{"a": "C"},
			[]asg{{syntax.OpSet, "F"}, {syntax.OpAppend, "G"}, {syntax.OpDefault, "H"}}, "C", ""},
		{"append to empty", nil, nil, []asg{{syntax.OpSet, ""}, {syntax.OpAppend, "x"}}, "x", ""},
		{"append", nil, nil, []asg{{syntax.OpSet, "x"}, {syntax.OpAppend, "y z"}}, "x y z", ""},
		{"append undefined", nil, nil, []asg{{syntax.OpAppend, "x"}}, "",
			`Loomfile:1:1: cannot append to undefined variable "a"`},
	}
	pos := syntax.Pos{File: "Loomfile", Line: 1, Col: 1}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := newScope(Options{Environ: tt.env, Overrides: tt.overrides}, "/r")
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range tt.ops {
				if err = s.assign(pos, "a", a.op, a.value); err != nil {
					break
				}
			}
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := s.vars["a"]; got != tt.want {
				t.Errorf("a = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAssignRejected(t *testing.T) {
	s, _ := newScope(Options{}, "/r")
	pos := syntax.Pos{File: "Loomfile", Line: 1, Col: 1}
	for name, want := range map[string]string{
		"os":  `Loomfile:1:1: cannot assign to built-in variable "os"`,
		"out": `Loomfile:1:1: "out" is reserved for recipes and cannot be assigned`,
	} {
		if err := s.assign(pos, name, syntax.OpSet, "x"); err == nil || err.Error() != want {
			t.Errorf("assign %s: %v, want %q", name, err, want)
		}
		if err := s.assignTarget(map[string]string{}, pos, name, syntax.OpSet, "x"); err == nil || err.Error() != want {
			t.Errorf("assignTarget %s: %v, want %q", name, err, want)
		}
	}
}

func TestAssignTarget(t *testing.T) {
	s, _ := newScope(Options{Environ: []string{"e=ENV"}, Overrides: map[string]string{"o": "C"}}, "/r")
	s.vars["g"] = "global"
	pos := syntax.Pos{File: "Loomfile", Line: 1, Col: 1}
	vars := map[string]string{}
	steps := []struct {
		name string
		op   syntax.AssignOp
		val  string
	}{
		{"e", syntax.OpDefault, "file"}, // env not consulted
		{"g", syntax.OpAppend, "more"},  // falls back to global
		{"g", syntax.OpAppend, "again"}, // now appends to target var
		{"o", syntax.OpSet, "ignored"},  // override wins: not stored
		{"d", syntax.OpSet, "1"},
		{"d", syntax.OpDefault, "2"},
	}
	for _, st := range steps {
		if err := s.assignTarget(vars, pos, st.name, st.op, st.val); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]string{"e": "file", "g": "global more again", "d": "1"}
	if !maps.Equal(vars, want) {
		t.Errorf("vars = %v, want %v", vars, want)
	}
	if s.vars["g"] != "global" {
		t.Errorf("global mutated: %q", s.vars["g"])
	}
	err := s.assignTarget(vars, pos, "nope", syntax.OpAppend, "x")
	if err == nil || err.Error() != `Loomfile:1:1: cannot append to undefined variable "nope"` {
		t.Errorf("err = %v", err)
	}
}

func TestEvalAssign(t *testing.T) {
	ev := testEvaluator(t, nil)
	pos := syntax.Pos{File: "Loomfile", Line: 1, Col: 1}
	self := syntax.Word{Pos: syntax.Pos{File: "Loomfile", Line: 1, Col: 6}, Segs: []syntax.Segment{syntax.VarRef{Name: "v"}}}

	err := ev.evalAssign(&syntax.Assign{P: pos, Name: "v", Op: syntax.OpSet, Value: self})
	if err == nil || err.Error() != `Loomfile:1:6: undefined variable "v"` {
		t.Fatalf("x := $x undefined: %v", err)
	}
	ev.scope.vars["v"] = "a"
	if err := ev.evalAssign(&syntax.Assign{P: pos, Name: "v", Op: syntax.OpAppend, Value: self}); err != nil {
		t.Fatal(err)
	}
	if ev.scope.vars["v"] != "a a" {
		t.Errorf("x += $x = %q, want %q", ev.scope.vars["v"], "a a")
	}
}

func evalSource(t *testing.T, src string) (*model.Project, error) {
	t.Helper()
	f, err := syntax.Parse("Loomfile", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ev := testEvaluator(t, nil)
	ev.proj = &model.Project{Order: []string{}, Targets: map[string]*model.Target{}}
	ev.decls = map[string]*declState{}
	ev.pats = map[string]*model.PatternRule{}
	fs := &fileState{rel: "Loomfile"}
	for _, it := range f.Items {
		switch it := it.(type) {
		case *syntax.Assign:
			err = ev.evalAssign(it)
		case *syntax.TargetDecl:
			err = ev.evalTarget(it, fs)
		case *syntax.TaskDecl:
			err = ev.evalTask(it, fs)
		default:
			t.Fatalf("unexpected item %T", it)
		}
		if err != nil {
			return ev.proj, err
		}
	}
	return ev.proj, nil
}

func TestDeclErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"a: {\n  x\n}\na: {\n  y\n}\n", `Loomfile:4:1: target "a" already has a recipe (declared at Loomfile:1:1)`},
		{"task t: x\nt: y\n", `Loomfile:2:1: "t" is declared as both a task and a file target (see Loomfile:1:1)`},
		{"a: @go\n", `Loomfile:1:1: attribute "@go" requires a recipe body on the same declaration`},
		{"a: b @go @bash {\n  x\n}\n", `Loomfile:1:1: conflicting attributes "@go" and "@bash"`},
		{"a%b%c: x\n", `Loomfile:1:1: pattern "a%b%c" has more than one wildcard`},
		{"a: {stem}.c\n", `Loomfile:1:4: "{stem}" is only allowed in pattern rules`},
		{"%.o lib: x\n", `Loomfile:1:1: cannot mix pattern and literal target names in one declaration`},
		{"a*: x\n", `Loomfile:1:1: target name "a*" contains glob characters`},
		{"task t a\ntask t b\n", `Loomfile:2:1: parameters for task "t" already declared at Loomfile:1:1`},
		{"%.o: {\n  x\n}\n{stem}.o: {\n  y\n}\n", `Loomfile:4:1: target "{stem}.o" already has a recipe (declared at Loomfile:1:1)`},
	}
	for _, tt := range tests {
		_, err := evalSource(t, tt.src)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%q:\n got %v\nwant %s", tt.src, err, tt.want)
		}
	}
}

func TestDeclMerge(t *testing.T) {
	p, err := evalSource(t, `srcs := main.go util.go

# Build it.
bin/app: $srcs | bin
./bin/app: go.mod main.go
bin/app: CGO_ENABLED := 0
bin/app: {
    go build -o $out .
}

fmt lint: {
    go $out ./...
}

task bench pkg="./$os" count {
    x
}

%.o: ./%.c @bash {
    cc
}
{stem}.o: CC := gcc

task up: "a b" c
`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bin/app", "fmt", "lint", "bench", "up"}; !slices.Equal(p.Order, want) {
		t.Errorf("Order = %q, want %q", p.Order, want)
	}
	app := p.Targets["bin/app"]
	if !slices.Equal(app.Prereqs, []string{"main.go", "util.go", "go.mod"}) ||
		!slices.Equal(app.OrderOnly, []string{"bin"}) || app.Doc != "Build it." ||
		app.Pos.Line != 4 || app.Vars["CGO_ENABLED"] != "0" ||
		app.Recipe == nil || app.Recipe.Pos.Line != 8 || app.Recipe.Lang != model.LangSh {
		t.Errorf("bin/app = %+v recipe %+v", app, app.Recipe)
	}
	if p.Targets["fmt"].Recipe == p.Targets["lint"].Recipe {
		t.Error("multi-name targets share a *Recipe")
	}
	b := p.Targets["bench"]
	if b.Kind != model.KindTask || len(b.Params) != 2 || *b.Params[0].Default != "./linux" || b.Params[1].Default != nil {
		t.Errorf("bench = %+v", b)
	}
	if !slices.Equal(p.Targets["up"].Prereqs, []string{"a b", "c"}) {
		t.Errorf("up prereqs = %q", p.Targets["up"].Prereqs)
	}
	if len(p.Patterns) != 1 {
		t.Fatalf("patterns = %d", len(p.Patterns))
	}
	r := p.Patterns[0]
	if r.Targets[0].String() != "{stem}.o" || !slices.Equal(r.Prereqs, []string{"{stem}.c"}) ||
		r.Recipe.Lang != model.LangBash || r.Vars["CC"] != "gcc" {
		t.Errorf("pattern = %+v", r)
	}
}

func TestPrereqGlob(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.c", "b.c", "sub/c.c", "x.h"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ev := testEvaluator(t, nil)
	ev.rootOS = dir

	tests := []struct {
		pat  string
		want []string
	}{
		{"*.c", []string{"a.c", "b.c"}},
		{"**/*.c", []string{"a.c", "b.c", "sub/c.c"}},
		{"./sub/*.c", []string{"sub/c.c"}},
	}
	for _, tt := range tests {
		got, err := ev.prereqItems([]syntax.Word{word(false, syntax.Text{S: tt.pat})}, false)
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("%s = %q, %v; want %q", tt.pat, got, err, tt.want)
		}
	}

	// Mixed with literals: matches are spliced in place.
	got, err := ev.prereqItems([]syntax.Word{word(false, syntax.Text{S: "z *.h y"})}, false)
	if err != nil || !slices.Equal(got, []string{"z", "x.h", "y"}) {
		t.Errorf("splice = %q, %v", got, err)
	}

	// Pattern rules never glob.
	got, err = ev.prereqItems([]syntax.Word{word(false, syntax.Text{S: "{stem}.c"})}, true)
	if err != nil || !slices.Equal(got, []string{"{stem}.c"}) {
		t.Errorf("pattern = %q, %v", got, err)
	}

	// Error row 9.
	w := syntax.Word{Pos: syntax.Pos{File: "Loomfile", Line: 1, Col: 4}, Segs: []syntax.Segment{syntax.Text{S: "*.nothing"}}}
	_, err = ev.prereqItems([]syntax.Word{w}, false)
	var d *syntax.Diagnostic
	if !errors.As(err, &d) || err.Error() != `Loomfile:1:4: prerequisite pattern "*.nothing" matched no files` ||
		d.Hint != "use $(glob ...) for a list that may be empty" {
		t.Errorf("no match: %v (hint %q)", err, hintOf(d))
	}
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func evalTree(t *testing.T, files map[string]string) (*model.Project, error) {
	t.Helper()
	dir := writeTree(t, files)
	return Evaluate(Options{File: filepath.Join(dir, "Loomfile"), GOOS: "linux", GOARCH: "amd64"})
}

func TestEvaluateInclude(t *testing.T) {
	p, err := evalTree(t, map[string]string{
		"Loomfile":     "include tools/*.loom extra.loom\nall: $tool_out\n",
		"tools/a.loom": "tool_out := a.out\na.out: {\n    touch $out\n}\n",
		"tools/b.loom": "shell bash\ntask b {\n    echo b\n}\n",
		"extra.loom":   "include tools/none-*.loom tools/a.loom\ntask extra {\n    echo extra\n}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, f := range p.Files {
		files = append(files, strings.TrimPrefix(f, p.Root+"/"))
	}
	if !slices.Equal(files, []string{"Loomfile", "tools/a.loom", "tools/b.loom", "extra.loom"}) {
		t.Errorf("Files = %q", files)
	}
	if !slices.Equal(p.Order, []string{"a.out", "b", "extra", "all"}) {
		t.Errorf("Order = %q", p.Order)
	}
	if got := p.Targets["b"].Recipe.Shell; !slices.Equal(got, []string{"bash"}) {
		t.Errorf("b shell = %q", got)
	}
	if p.Targets["extra"].Recipe.Shell != nil || p.Targets["a.out"].Recipe.Shell != nil {
		t.Error("shell directive leaked across files")
	}
	if p.Targets["a.out"].Pos.String() != "tools/a.loom:2:1" {
		t.Errorf("a.out pos = %s", p.Targets["a.out"].Pos)
	}
	if !slices.Equal(p.Targets["all"].Prereqs, []string{"a.out"}) {
		t.Errorf("all prereqs = %q", p.Targets["all"].Prereqs)
	}
	if p.Overrides != nil || p.Vars["root"] != p.Root {
		t.Errorf("overrides %v, root var %q", p.Overrides, p.Vars["root"])
	}
}

func TestIncludeCycle(t *testing.T) {
	_, err := evalTree(t, map[string]string{"Loomfile": "include a.loom\n", "a.loom": "include Loomfile\n"})
	if err == nil || err.Error() != "a.loom:1:9: include cycle: Loomfile -> a.loom -> Loomfile" {
		t.Errorf("err = %v", err)
	}
}

func TestEvaluateErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"x := $(glob *.go)\n", `Loomfile:1:6: unknown function "glob"`},
		{"if $x {\n}\n", `Loomfile:1:1: "if" is not implemented until Phase 09`},
		{"dotenv\n", `Loomfile:1:1: "dotenv" is not implemented until Phase 11`},
		{"include missing.loom\n", `Loomfile:1:9: included file "missing.loom" not found`},
		{"shell bash\nshell zsh\n", `Loomfile:2:1: shell already set for this file at Loomfile:1:1`},
	}
	for _, tt := range tests {
		_, err := evalTree(t, map[string]string{"Loomfile": tt.src})
		if err == nil || err.Error() != tt.want {
			t.Errorf("%q:\n got %v\nwant %s", tt.src, err, tt.want)
		}
	}
}

func TestEvaluateMissingFile(t *testing.T) {
	_, err := Evaluate(Options{File: filepath.Join(t.TempDir(), "Loomfile")})
	var d *syntax.Diagnostic
	if !errors.As(err, &d) || d.Msg != "file not found" || d.Hint != "use -f FILE or -C DIR" {
		t.Errorf("err = %v", err)
	}
}
