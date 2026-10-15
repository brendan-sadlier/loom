package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/eval"
	"github.com/spf13/cobra"
)

var overrideRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// parseOverrides splits VAR=value arguments. Later duplicates win, as in Make.
func parseOverrides(args []string, cmdName string) (map[string]string, error) {
	var out map[string]string
	for _, a := range args {
		if !overrideRE.MatchString(a) {
			return nil, fmt.Errorf("%s takes only VAR=value arguments (got %q)", cmdName, a)
		}
		k, v, _ := strings.Cut(a, "=")
		if out == nil {
			out = map[string]string{}
		}
		out[k] = v
	}
	return out, nil
}

func newDumpCmd(stdio Stdio) *cobra.Command {
	var file, dir string
	var text bool
	cmd := &cobra.Command{
		Use:    "__dump [VAR=value]...",
		Short:  "Print the evaluated project model (debugging)",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			overrides, err := parseOverrides(args, "__dump")
			if err != nil {
				return err
			}
			if dir != "" {
				// The only permitted Chdir (§16): once, before evaluation.
				if err := os.Chdir(dir); err != nil {
					return fmt.Errorf("-C %s: %w", dir, err)
				}
			}
			loomPath, _ := os.Executable()
			proj, err := eval.Evaluate(eval.Options{
				File:      file,
				Overrides: overrides,
				Environ:   os.Environ(),
				LoomPath:  loomPath,
			})
			if err != nil {
				return err
			}
			if text {
				_, err = fmt.Fprint(stdio.Stdout, proj.Dump())
				return err
			}
			b, err := json.MarshalIndent(proj, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdio.Stdout, "%s\n", b)
			return err
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "Loomfile", "build file")
	cmd.Flags().StringVarP(&dir, "directory", "C", "", "change directory first")
	cmd.Flags().BoolVar(&text, "text", false, "print the stable text form instead of JSON")
	return cmd
}
