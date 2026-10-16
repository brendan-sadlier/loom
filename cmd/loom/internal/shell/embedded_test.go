package shell

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, ctx context.Context, script string, o RunOptions) (string, error) {
	t.Helper()
	var out bytes.Buffer
	o.Stdout, o.Stderr = &out, &out
	if o.Dir == "" {
		o.Dir = t.TempDir()
	}
	err := NewEmbedded().Run(ctx, script, o)
	return out.String(), err
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v (%T), want *ExitError", err, err)
	}
	return ee.Code
}

func TestPipefail(t *testing.T) {
	_, err := run(t, context.Background(), "false | true\n", RunOptions{})
	if c := exitCode(t, err); c != 1 {
		t.Fatalf("code = %d, want 1", c)
	}
}

func TestNounset(t *testing.T) {
	if _, err := run(t, context.Background(), "echo \"$NOPE\"\n", RunOptions{}); err == nil {
		t.Fatal("undefined variable must fail under -u")
	}
}

func TestErrexit(t *testing.T) {
	out, err := run(t, context.Background(), "false\necho reached\n", RunOptions{})
	if c := exitCode(t, err); c != 1 {
		t.Fatalf("code = %d, want 1", c)
	}
	if strings.Contains(out, "reached") {
		t.Fatal("-e did not stop the script")
	}
}

func TestExitCode(t *testing.T) {
	_, err := run(t, context.Background(), "exit 7\n", RunOptions{})
	if c := exitCode(t, err); c != 7 {
		t.Fatalf("code = %d, want 7", c)
	}
}

func TestCwdIsolation(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, context.Background(), "cd sub && pwd\n", RunOptions{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, context.Background(), "pwd\n", RunOptions{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out); got != dir {
		t.Fatalf("second run pwd = %q, want %q", got, dir)
	}
}

func TestEnv(t *testing.T) {
	out, err := run(t, context.Background(), "echo \"$X\"\n", RunOptions{Env: []string{"X=1"}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "1\n" {
		t.Fatalf("out = %q, want %q", out, "1\n")
	}
}

func TestParseErrorPosition(t *testing.T) {
	err := NewEmbedded().Check("recipe", "echo \"unterminated\n")
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v (%T), want *ParseError", err, err)
	}
	if pe.Line != 1 {
		t.Fatalf("Line = %d, want 1", pe.Line)
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := run(t, ctx, "echo hi\n", RunOptions{})
	if err == nil {
		t.Fatal("want error for cancelled context")
	}
	if out != "" {
		t.Fatalf("printed %q after cancel", out)
	}
}
