package syntax

import "testing"

func TestDump(t *testing.T) {
	p := func(l, c int) Pos { return Pos{File: "t.loom", Line: l, Col: c} }
	txt := func(s string) Word { return Word{Segs: []Segment{Text{S: s}}} }
	def := Word{Quoted: true, Segs: []Segment{Text{S: "prod"}}}
	f := &File{Name: "t.loom", Items: []Item{
		&Dotenv{P: p(1, 1)},
		&If{P: p(2, 1), Cond: Compare{Op: "==", L: Word{Segs: []Segment{VarRef{Name: "os"}}}, R: txt("linux")},
			Then: []Item{&Assign{P: p(3, 5), Name: "x", Op: OpSet}},
			Else: []Item{&Assign{P: p(5, 5), Name: "x", Op: OpAppend, Value: txt("y")}}},
		&GoBlock{P: p(7, 1), Body: Body{Pos: p(8, 1), Text: "a\n\nb\n"}},
		&TaskDecl{P: p(11, 1), Name: "deploy", Doc: "One.\nTwo.",
			Params:    []Param{{Name: "env"}, {Name: "target", Default: &def}},
			OrderOnly: []Word{txt("fmt")}, Attrs: []string{"sh"},
			Body: &Body{Pos: p(12, 1), Text: "echo\n"}},
	}}
	const want = `File t.loom
Dotenv 1:1
If 2:1 cond=(== [V:os] [T:"linux"])
  then:
    Assign 3:5 x := []
  else:
    Assign 5:5 x += [T:"y"]
Go 7:1 body@8:1
  | a
  |
  | b
Task 11:1 name=deploy params=[env target=Q[T:"prod"]] orderonly=[[T:"fmt"]] attrs=[sh] doc="One.\nTwo."
  body@12:1
  | echo
`
	if got := f.Dump(); got != want {
		t.Errorf("Dump mismatch\n--- got\n%s--- want\n%s", got, want)
	}
}
