package cli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// resolveColor decides whether to emit ANSI colour on stderr-adjacent output.
func resolveColor(mode string, stderr io.Writer) (bool, error) {
	switch mode {
	case "never":
		return false, nil
	case "always":
		return true, nil
	case "auto":
		if _, set := os.LookupEnv("NO_COLOR"); set {
			return false, nil
		}
		f, ok := stderr.(*os.File)
		return ok && term.IsTerminal(int(f.Fd())), nil
	}
	return false, fmt.Errorf("invalid --color %q: want auto, always or never", mode)
}
