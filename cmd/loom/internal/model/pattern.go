package model

import (
	"fmt"
	"strings"
)

const stemToken = "{stem}"

// ParsePattern splits s around its single wildcard ({stem} or %).
// ok is false when s has no wildcard. More than one wildcard is an error.
// There is no escape: a literal % in a name is not supported in v1.
func ParsePattern(s string) (p Pattern, ok bool, err error) {
	n := strings.Count(s, "%") + strings.Count(s, stemToken)
	switch n {
	case 0:
		return Pattern{}, false, nil
	case 1:
	default:
		return Pattern{}, false, fmt.Errorf("pattern %q has more than one wildcard", s)
	}
	i := strings.Index(s, stemToken)
	w := len(stemToken)
	if i < 0 {
		i = strings.Index(s, "%")
		w = 1
	}
	return Pattern{Prefix: s[:i], Suffix: s[i+w:]}, true, nil
}

// String renders the pattern in canonical {stem} form.
func (p Pattern) String() string { return p.Prefix + stemToken + p.Suffix }
