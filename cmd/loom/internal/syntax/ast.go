package syntax

// File is a parsed Loomfile.
type File struct {
	Name  string
	Items []Item
}

// Item is a top-level or block-level declaration.
type Item interface {
	Pos() Pos
	item()
}

// AssignOp is an assignment operator.
type AssignOp string

const (
	OpSet     AssignOp = ":="
	OpDefault AssignOp = "?="
	OpAppend  AssignOp = "+="
)

// Assign is "name op value".
type Assign struct {
	P     Pos
	Name  string
	Op    AssignOp
	Value Word // raw_text; may be empty
}

// TargetDecl is a file target (or target-specific variable) declaration.
type TargetDecl struct {
	P         Pos
	Doc       string  // joined doc-comment lines, "" if none
	Names     []Word  // one or more; text may contain {stem} or %
	Var       *Assign // target-specific variable form; when set, Prereqs/OrderOnly/Attrs/Body are empty
	Prereqs   []Word
	OrderOnly []Word
	Attrs     []string // without "@": sh, go, bash, pwsh (deduplicated, order kept)
	Body      *Body    // nil = aggregate
}

// TaskDecl is "task name params...: prereqs { body }".
type TaskDecl struct {
	P         Pos
	Doc       string
	Name      string
	Params    []Param
	Prereqs   []Word
	OrderOnly []Word
	Attrs     []string
	Body      *Body
}

// Param is a task parameter.
type Param struct {
	P       Pos
	Name    string
	Default *Word // nil = required
}

// Include is "include path...".
type Include struct {
	P     Pos
	Paths []Word // len >= 1
}

// Dotenv is "dotenv [path]".
type Dotenv struct {
	P    Pos
	Path *Word // nil = default ./.env
}

// ShellDir is "shell program args...".
type ShellDir struct {
	P    Pos
	Args []Word // len >= 1
}

// GoBlock is a top-level "go { ... }" block.
type GoBlock struct {
	P    Pos
	Body Body
}

// If is a conditional block.
type If struct {
	P    Pos
	Cond Expr
	Then []Item
	Else []Item // nil = no else; a single *If = "else if"
}

func (x *Assign) Pos() Pos     { return x.P }
func (x *TargetDecl) Pos() Pos { return x.P }
func (x *TaskDecl) Pos() Pos   { return x.P }
func (x *Include) Pos() Pos    { return x.P }
func (x *Dotenv) Pos() Pos     { return x.P }
func (x *ShellDir) Pos() Pos   { return x.P }
func (x *GoBlock) Pos() Pos    { return x.P }
func (x *If) Pos() Pos         { return x.P }

func (*Assign) item()     {}
func (*TargetDecl) item() {}
func (*TaskDecl) item()   {}
func (*Include) item()    {}
func (*Dotenv) item()     {}
func (*ShellDir) item()   {}
func (*GoBlock) item()    {}
func (*If) item()         {}

// Expr is a conditional expression.
type Expr interface{ expr() }

// Operand is truthy if non-empty after expansion.
type Operand struct{ W Word }

// Compare is "==" or "!=".
type Compare struct {
	Op   string
	L, R Word
}

// Not is "!" X.
type Not struct{ X Expr }

// Binary is "&&" or "||".
type Binary struct {
	Op   string
	L, R Expr
}

func (Operand) expr() {}
func (Compare) expr() {}
func (Not) expr()     {}
func (Binary) expr()  {}
