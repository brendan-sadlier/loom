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

func TestParseIf(t *testing.T) {
	src := "if $os == windows {\n" +
		"    rm := del /q\n" +
		"} else if $(env CI) && !$(exists .nocache) {\n" +
		"    rm := rm -rf --verbose\n" +
		"} else {\n" +
		"    if ($os == linux || $os == darwin) {\n" +
		"        rm := rm -rf\n" +
		"    }\n" +
		"}\n" +
		"go {\n" +
		"    import \"time\"\n" +
		"}\n" +
		"if $x {\n" +
		"    a: {\n" +
		"        echo\n" +
		"    }\n" +
		"} else {\n" +
		"}\n"
	const want = `File t.loom
If 1:1 cond=(== [V:os] [T:"windows"])
  then:
    Assign 2:5 rm := [T:"del /q"]
  else:
    If 3:8 cond=(&& [C:env([T:"CI"])] (! [C:exists([T:".nocache"])]))
      then:
        Assign 4:5 rm := [T:"rm -rf --verbose"]
      else:
        If 6:5 cond=(|| (== [V:os] [T:"linux"]) (== [V:os] [T:"darwin"]))
          then:
            Assign 7:9 rm := [T:"rm -rf"]
Go 10:1 body@11:1
  | import "time"
If 13:1 cond=[V:x]
  then:
    Target 14:5 names=[[T:"a"]]
      body@15:1
      | echo
  else:
`
	f, err := Parse("t.loom", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Dump(); got != want {
		t.Errorf("Dump mismatch\n--- got\n%s--- want\n%s", got, want)
	}
}

func TestMalformedIf(t *testing.T) {
	tests := []struct{ src, want string }{
		{"if $x == y", `Loomfile:1:1: expected "{" at end of if line`},
		{"if {\n}", `Loomfile:1:4: expected expression`},
		{"if $x {\n  y := 1\n  }", `Loomfile:3:3: "}" must be at the indentation of the matching "if"`},
		{"if $x {\n  y := 1", `Loomfile:1:1: unterminated "if" block`},
		{"if ($x {\n}", `Loomfile:1:8: expected ")"`},
		{"if $x ) {\n}", `Loomfile:1:7: unexpected token ")" in expression`},
		{"if $x == {\n}", `Loomfile:1:10: expected expression`},
		{"if $x {\n} els {\n}", `Loomfile:2:1: expected "}" , "} else {" or "} else if ... {"`},
		{"if $x {\n} else {\n} else {\n}", `Loomfile:3:1: expected "}" , "} else {" or "} else if ... {"`},
		{"go", `Loomfile:1:1: expected assignment, target declaration or directive`},
		{"go {\n    x", `Loomfile:1:1: unterminated recipe body`},
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

func TestDocComments(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"two lines", "# A\n# B\ntask t\n", "A\nB"},
		{"blank separates", "# A\n\na: b\n", ""},
		{"directive discards", "# A\ninclude x\ntask t\n", ""},
		{"no space after hash", "#A\na: b\n", "A"},
		{"indented comment", "  # A\na: b\n", "A"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse("t", []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var got string
			switch x := f.Items[len(f.Items)-1].(type) {
			case *TaskDecl:
				got = x.Doc
			case *TargetDecl:
				got = x.Doc
			default:
				t.Fatalf("last item is %T", x)
			}
			if got != tt.want {
				t.Errorf("doc = %q, want %q", got, tt.want)
			}
		})
	}
}
