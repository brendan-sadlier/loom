package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/cli"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"loom": func() {
			os.Exit(cli.Main(os.Args[1:], cli.Stdio{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
		},
	})
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			// Deterministic output in every script.
			env.Setenv("NO_COLOR", "1")
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"exitcode": cmdExitCode,
		},
	})
}

// exitcode N cmd [args...] runs cmd and asserts its exit status is N.
// Stdout/stderr remain available to the following stdout/stderr assertions.
func cmdExitCode(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: exitcode N cmd [args...]")
	}
	want, err := strconv.Atoi(args[0])
	if err != nil {
		ts.Fatalf("exitcode: bad code %q", args[0])
	}
	got := 0
	if err := ts.Exec(args[1], args[2:]...); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			ts.Fatalf("exitcode: %v", err)
		}
		got = ee.ExitCode()
	}
	if (got == want) == neg {
		ts.Fatalf("exit code %d, want %d (neg=%v)", got, want, neg)
	}
}
