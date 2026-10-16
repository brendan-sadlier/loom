package cli

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"

	"github.com/spf13/cobra"
)

// Exit codes per ARCHITECTURE.md §11.
const (
	ExitOK          = 0
	ExitRecipe      = 1
	ExitLoomfile    = 2
	ExitEnvironment = 3
	ExitInterrupted = 130
)

// Stdio lets tests capture output without touching os.Stdout.
type Stdio struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// ExitCoder is implemented by errors that choose the process exit code.
// Errors without it exit with ExitLoomfile.
type ExitCoder interface {
	ExitCode() int
}

// version is set at release time with
// -ldflags "-X github.com/brendan-sadlier/loom/cmd/loom/internal/cli.version=vX.Y.Z".
var version = "dev"

// Version prefers the ldflags value, then module build info, then "dev".
func Version() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		switch v := bi.Main.Version; v {
		case "", "(devel)":
		default:
			return v
		}
	}
	return version
}

// Main runs the CLI and returns the process exit code. It never calls os.Exit.
func Main(args []string, stdio Stdio) int {
	root := newRootCmd(stdio)
	root.SetArgs(args)
	root.SetIn(stdio.Stdin)
	root.SetOut(stdio.Stdout)
	root.SetErr(stdio.Stderr)

	err := root.Execute()
	if err == nil {
		return ExitOK
	}
	_, err = fmt.Fprintf(stdio.Stderr, "loom: %v\n", err)
	if err != nil {
		return 0
	}
	var diag *syntax.Diagnostic
	if errors.As(err, &diag) && diag.Hint != "" {
		_, err2 := fmt.Fprintf(stdio.Stderr, "  hint: %s\n", diag.Hint)
		if err2 != nil {
			return 0
		}
	}
	var ec ExitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return ExitLoomfile
}

func newRootCmd(stdio Stdio) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "loom [flags] [VAR=value]... [TARGET [PARAMS...]]...",
		Short:         "Build targets from a Loomfile using shell or Go recipes",
		Version:       Version(),
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Replaced in Phase 03.
			return errors.New("building targets is not implemented yet (Phase 03)")
		},
	}
	cmd.SetVersionTemplate("loom {{.Version}}\n")
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddCommand(newDumpCmd(stdio))
	return cmd
}
