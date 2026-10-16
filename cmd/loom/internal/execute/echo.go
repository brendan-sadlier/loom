package execute

import (
	"fmt"
	"io"
	"strings"
)

const (
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"
	ansiReset = "\x1b[0m"
)

// echo prints "→ NAME" then each body line prefixed with two spaces.
// With colour: header bold, body dim (ANSI 1 / 2, reset 0).
func echo(w io.Writer, name, body string, color bool) {
	var b strings.Builder
	if color {
		_, err := fmt.Fprintf(&b, "%s→ %s%s\n", ansiBold, name, ansiReset)
		if err != nil {
			return
		}
	} else {
		_, err := fmt.Fprintf(&b, "→ %s\n", name)
		if err != nil {
			return
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		if body == "" {
			break
		}
		if color {
			_, err := fmt.Fprintf(&b, "%s  %s%s\n", ansiDim, line, ansiReset)
			if err != nil {
				return
			}
		} else {
			_, err := fmt.Fprintf(&b, "  %s\n", line)
			if err != nil {
				return
			}
		}
	}
	_, _ = io.WriteString(w, b.String()) // one write: keeps the block contiguous
}
