package syntax

import "testing"

func TestParseHeaders(t *testing.T) {
	src := "shell bash\n" +
		"CC := gcc\n" +
		"bin/app: CGO_ENABLED := 0\n" +
		"bin/app: main.go \\\n" +
		"    util.go\n" +
		"# First line.\n" +
		"# Second line.\n" +
		"task deploy env target=\"prod\": lint | fmt @sh @sh {\n" +
		"    echo $env $target\n" +
		"}\n" +
		"# dropped by blank\n" +
		"\n" +
		"task clean {\n" +
		"\trm -rf $bin\n" +
		"}\n" +
		"$bin: {\n" +
		"    mkdir -p $out\n" +
		"}\n" +
		"include a.loom \"b c.loom\"\n" +
		"dotenv .env.local\n"
	const want = `File t.loom
Shell 1:1 [[T:"bash"]]
Assign 2:1 CC := [T:"gcc"]
Target 3:1 names=[[T:"bin/app"]] var=(CGO_ENABLED := [T:"0"])
Target 4:1 names=[[T:"bin/app"]] prereqs=[[T:"main.go"] [T:"util.go"]]
Task 8:1 name=deploy params=[env target=Q[T:"prod"]] prereqs=[[T:"lint"]] orderonly=[[T:"fmt"]] attrs=[sh] doc="First line.\nSecond line."
  body@9:1
  | echo $env $target
Task 13:1 name=clean
  body@14:1
  | rm -rf $bin
Target 16:1 names=[[V:bin]]
  body@17:1
  | mkdir -p $out
Include 19:1 [[T:"a.loom"] Q[T:"b c.loom"]]
Dotenv 20:1 [T:".env.local"]
`
	f, err := Parse("t.loom", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Dump(); got != want {
		t.Errorf("Dump mismatch\n--- got\n%s--- want\n%s", got, want)
	}
}

func TestMalformedHeaders(t *testing.T) {
	tests := []struct{ src, want string }{
		{"build: a {\n    echo hi", `Loomfile:1:1: unterminated recipe body`},
		{"build: a {\necho hi\n}", `Loomfile:2:1: recipe body lines must be indented further than the header; close the body with "}" at the header's indentation`},
		{"foo bar", `Loomfile:1:1: expected assignment, target declaration or directive`},
		{"clean { rm -rf bin }", `Loomfile:1:7: recipe body must start on the line after "{"`},
		{"task {", `Loomfile:1:6: expected task name`},
		{"task my-task: x", `Loomfile:1:6: invalid task name "my-task": must match [A-Za-z_][A-Za-z0-9_]*`},
		{"build: @rust {\n    x\n}", `Loomfile:1:8: unknown attribute "@rust" (expected @sh, @go, @bash or @pwsh)`},
		{"x := $(glob a", `Loomfile:1:6: unterminated "$(": missing ")"`},
		{"x := $ y", `Loomfile:1:6: invalid "$" sequence; use $name, ${name}, $(fn ...) or $$`},
		{`x := "abc`, `Loomfile:1:6: unterminated quoted string`},
		{"} else {", `Loomfile:1:1: unexpected "}"`},
		{"include", `Loomfile:1:1: include requires at least one path`},
		{"a: b | c | d", `Loomfile:1:10: only one "|" allowed`},
		// Header errors beyond the §6.3 table.
		{"dotenv a b", `Loomfile:1:1: dotenv takes at most one path`},
		{"shell", `Loomfile:1:1: shell requires a program name`},
		{"include a {", `Loomfile:1:11: directives cannot have a body`},
		{"a: X := 1 {", `Loomfile:1:11: target-specific variables cannot have a recipe body`},
		{"task t: X := 1", `Loomfile:1:9: tasks cannot have target-specific variables`},
		{"task t a a", `Loomfile:1:10: duplicate parameter "a"`},
		{"task t ./x {", `Loomfile:1:8: expected ":" before prerequisites`},
		{": a", `Loomfile:1:1: expected target name before ":"`},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			_, err := Parse("Loomfile", []byte(tt.src))
			if err == nil || err.Error() != tt.want {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}
