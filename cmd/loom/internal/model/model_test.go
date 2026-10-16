package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

func TestParsePattern(t *testing.T) {
	tests := []struct {
		in      string
		want    Pattern
		ok      bool
		wantErr string
	}{
		{"bin/app", Pattern{}, false, ""},
		{"%.o", Pattern{Suffix: ".o"}, true, ""},
		{"{stem}.pb.go", Pattern{Suffix: ".pb.go"}, true, ""},
		{"src/%.c", Pattern{Prefix: "src/", Suffix: ".c"}, true, ""},
		{"gen/{stem}", Pattern{Prefix: "gen/"}, true, ""},
		{"a%b%c", Pattern{}, false, `pattern "a%b%c" has more than one wildcard`},
		{"{stem}-%", Pattern{}, false, `pattern "{stem}-%" has more than one wildcard`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok, err := ParsePattern(tt.in)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.ok || got != tt.want {
				t.Errorf("got %+v,%v want %+v,%v", got, ok, tt.want, tt.ok)
			}
			if ok && strings.Contains(tt.in, "%") && got.String() != strings.Replace(tt.in, "%", "{stem}", 1) {
				t.Errorf("String() = %q", got.String())
			}
		})
	}
}

func TestLangKindText(t *testing.T) {
	for _, n := range []string{"sh", "go", "bash", "pwsh"} {
		l, ok := ParseLang(n)
		if !ok {
			t.Fatalf("ParseLang(%q) not ok", n)
		}
		b, _ := l.MarshalText()
		if string(b) != n {
			t.Errorf("Lang %q marshals to %q", n, b)
		}
	}
	if _, ok := ParseLang("zsh"); ok {
		t.Error("ParseLang(zsh) ok")
	}
	for k, want := range map[Kind]string{KindFile: "file", KindTask: "task"} {
		b, _ := k.MarshalText()
		if string(b) != want {
			t.Errorf("Kind marshals to %q, want %q", b, want)
		}
	}
}

func TestJSON(t *testing.T) {
	tg := &Target{Name: "rel", Kind: KindTask, Pos: syntax.Pos{File: "Loomfile", Line: 1, Col: 1},
		Recipe: &Recipe{Lang: LangGo, Body: "return nil\n", Pos: syntax.Pos{File: "Loomfile", Line: 2, Col: 1}}}
	b, err := json.Marshal(tg)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"kind":"task"`, `"lang":"go"`, `"pos":"Loomfile:1:1"`, `"pos":"Loomfile:2:1"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON %s missing %s", b, want)
		}
	}
}

func TestDump(t *testing.T) {
	pos := func(l int) syntax.Pos { return syntax.Pos{File: "Loomfile", Line: l, Col: 1} }
	def := "./..."
	p := &Project{
		Root:      "/r",
		Files:     []string{"/r/Loomfile"},
		Vars:      map[string]string{"b": "2", "a": "1"},
		Overrides: map[string]string{"a": "1"},
		Order:     []string{"x", "t"},
		Targets: map[string]*Target{
			"x": {Name: "x", Kind: KindFile, Pos: pos(1), Doc: "Doc.", Prereqs: []string{"a b", "c"},
				OrderOnly: []string{"d"}, Vars: map[string]string{"V": "0"},
				Recipe: &Recipe{Lang: LangSh, Body: "one\n\ntwo\n", Pos: pos(2), Shell: []string{"bash", "-x"}}},
			"t": {Name: "t", Kind: KindTask, Pos: pos(5), Params: []Param{{Name: "pkg", Default: &def}, {Name: "n"}}},
		},
		Patterns: []*PatternRule{{Pos: pos(7), Targets: []Pattern{{Suffix: ".o"}}, Prereqs: []string{"{stem}.c"}}},
		GoBlocks: []*GoBlock{{Pos: pos(9), Body: "func f() {}\n"}},
	}
	const want = `root /r
file /r/Loomfile
var a = "1"
var b = "2"
override a = "1"
target x @Loomfile:1:1 doc="Doc."
  prereqs: "a b" c
  orderonly: d
  var V = "0"
  recipe sh shell="bash -x" @Loomfile:2:1
  | one
  |
  | two
task t @Loomfile:5:1
  params: pkg="./..." n
pattern @Loomfile:7:1 targets={stem}.o
  prereqs: {stem}.c
go @Loomfile:9:1
  | func f() {}
`
	if got := p.Dump(); got != want {
		t.Errorf("Dump mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
