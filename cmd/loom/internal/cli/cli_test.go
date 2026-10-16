package cli

//
// import (
//	"bytes"
//	"errors"
//	"strings"
//	"testing"
//)
//
// type codedErr struct{ code int }
//
// func (e codedErr) Error() string { return "coded" }
// func (e codedErr) ExitCode() int { return e.code }
//
// func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
//	t.Helper()
//	var out, errb bytes.Buffer
//	code = Main(args, Stdio{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb})
//	return code, out.String(), errb.String()
// }
//
// func TestVersionFlag(t *testing.T) {
//	code, out, errOut := run(t, "--version")
//	if code != ExitOK {
//		t.Fatalf("exit %d, want 0; stderr=%q", code, errOut)
//	}
//	if !strings.HasPrefix(out, "loom ") || !strings.HasSuffix(out, "\n") {
//		t.Fatalf("stdout %q, want \"loom <version>\\n\"", out)
//	}
//	if errOut != "" {
//		t.Fatalf("stderr %q, want empty", errOut)
//	}
// }
//
// func TestUnknownFlagExits2(t *testing.T) {
//	code, _, errOut := run(t, "--bogus")
//	if code != ExitLoomfile {
//		t.Fatalf("exit %d, want %d", code, ExitLoomfile)
//	}
//	if !strings.Contains(errOut, "unknown flag: --bogus") {
//		t.Fatalf("stderr %q", errOut)
//	}
// }
//
// func TestExitCoderIsHonoured(t *testing.T) {
//	var ec ExitCoder
//	err := error(codedErr{code: ExitEnvironment})
//	if !errors.As(err, &ec) || ec.ExitCode() != ExitEnvironment {
//		t.Fatal("ExitCoder not detected through errors.As")
//	}
// }
