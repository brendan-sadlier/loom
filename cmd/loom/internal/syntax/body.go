package syntax

import "strings"

// Body is the verbatim text of a recipe or go block after common-indent removal.
type Body struct {
	Pos  Pos    // position (Col 1) of the first physical line after the header
	Text string // dedented; always ends with "\n" unless empty
}

const msgBodyIndent = `recipe body lines must be indented further than the header; close the body with "}" at the header's indentation`

// readBody consumes physical lines starting at index start until the terminator
// indent+"}" and returns the dedented body and the index of the line after the
// terminator. header is the position of the header line, used for the
// unterminated-body diagnostic.
func readBody(file string, lines []string, start int, indent string, header Pos) (Body, int, error) {
	body := Body{Pos: Pos{File: file, Line: start + 1, Col: 1}}
	var collected []string
	for i := start; i < len(lines); i++ {
		ln := lines[i]
		if strings.TrimSpace(ln) == "" {
			collected = append(collected, "")
			continue
		}
		if ln == indent+"}" {
			body.Text = dedent(collected)
			return body, i + 1, nil
		}
		if !strings.HasPrefix(ln, indent) || len(ln) == len(indent) || !isSpace(ln[len(indent)]) {
			return Body{}, 0, Errorf(Pos{File: file, Line: i + 1, Col: 1}, "%s", msgBodyIndent)
		}
		collected = append(collected, ln)
	}
	return Body{}, 0, Errorf(header, "unterminated recipe body")
}

// dedent removes the longest whitespace prefix common to all non-blank lines.
// Mixed tabs/spaces are compared byte-wise, so a tab-indented body under a
// space-indented header dedents by the common bytes only.
func dedent(lines []string) string {
	common := ""
	first := true
	for _, ln := range lines {
		if ln == "" {
			continue
		}
		ws := ln[:leadingSpace(ln)]
		if first {
			common, first = ws, false
			continue
		}
		common = commonPrefix(common, ws)
	}
	// Trailing blank lines are not part of the body.
	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}
	var b strings.Builder
	for _, ln := range lines[:end] {
		if ln != "" {
			ln = ln[len(common):]
		}
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	return b.String()
}

func leadingSpace(s string) int {
	i := 0
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i
}

func commonPrefix(a, b string) string {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' }
