package shell

import (
	"context"
	"fmt"
	"io"
)

// Runner executes recipe scripts. Implementations: Embedded (this phase), External (Phase 08).
type Runner interface {
	// Check parses script without running it. Errors are *ParseError.
	Check(name, script string) error
	// Run executes script. A non-zero exit is returned as *ExitError.
	Run(ctx context.Context, script string, opts RunOptions) error
}

type RunOptions struct {
	Name   string // for error messages / parse positions
	Dir    string
	Env    []string // complete environment, KEY=VALUE
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// ExitError is a script that ran and exited non-zero.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// ParseError is a syntax error at a 1-based line/column within the script.
type ParseError struct {
	Line, Col int
	Msg       string
}

func (e *ParseError) Error() string { return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg) }
