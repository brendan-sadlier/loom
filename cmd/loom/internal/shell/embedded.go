package shell

import (
	"context"
	"errors"
	"io"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// Embedded runs scripts in-process. It is safe for concurrent use; every Run creates
// its own interpreter, so "cd" inside one recipe never leaks into another.
type Embedded struct{}

func NewEmbedded() *Embedded { return &Embedded{} }

// Recipes are parsed as bash: pipefail, [[ ]], arrays and ${x//a/b} are expected by users.
var parser = syntax.NewParser(syntax.Variant(syntax.LangBash))

func (e *Embedded) parse(name, script string) (*syntax.File, error) {
	f, err := parser.Parse(strings.NewReader(script), name)
	if err != nil {
		if pe, ok := errors.AsType[syntax.ParseError](err); ok {
			return nil, &ParseError{Line: int(pe.Pos.Line()), Col: int(pe.Pos.Col()), Msg: pe.Text}
		}
		return nil, err
	}
	return f, nil
}

func (e *Embedded) Check(name, script string) error {
	_, err := e.parse(name, script)
	return err
}

func (e *Embedded) Run(ctx context.Context, script string, o RunOptions) error {
	f, err := e.parse(o.Name, script)
	if err != nil {
		return err
	}
	if o.Stdin == nil {
		o.Stdin = strings.NewReader("")
	}
	if o.Stdout == nil {
		o.Stdout = io.Discard
	}
	if o.Stderr == nil {
		o.Stderr = io.Discard
	}
	r, err := interp.New(
		interp.Dir(o.Dir),
		interp.Env(expand.ListEnviron(o.Env...)),
		interp.StdIO(o.Stdin, o.Stdout, o.Stderr),
		interp.Params("-e", "-u", "-o", "pipefail"),
	)
	if err != nil {
		return err
	}
	err = r.Run(ctx, f)
	if code, ok := errors.AsType[interp.ExitStatus](err); ok {
		return &ExitError{Code: int(code)}
	}
	return err
}
