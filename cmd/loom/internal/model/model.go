package model

import (
	"fmt"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

// Kind distinguishes file targets from tasks.
type Kind int

const (
	KindFile Kind = iota
	KindTask
)

func (k Kind) String() string {
	switch k {
	case KindFile:
		return "file"
	case KindTask:
		return "task"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// MarshalText renders the kind as "file" or "task".
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// Lang is the recipe language selected by an attribute.
type Lang int

const (
	LangSh Lang = iota
	LangGo
	LangBash
	LangPwsh
)

var langNames = [...]string{LangSh: "sh", LangGo: "go", LangBash: "bash", LangPwsh: "pwsh"}

func (l Lang) String() string {
	if l >= 0 && int(l) < len(langNames) {
		return langNames[l]
	}
	return fmt.Sprintf("Lang(%d)", int(l))
}

// MarshalText renders the language name.
func (l Lang) MarshalText() ([]byte, error) { return []byte(l.String()), nil }

// ParseLang maps an attribute name (without "@") to a Lang.
func ParseLang(attr string) (Lang, bool) {
	for i, n := range langNames {
		if n == attr {
			return Lang(i), true
		}
	}
	return 0, false
}

type Project struct {
	Root      string             `json:"root"`                // absolute, forward slashes
	Files     []string           `json:"files"`               // absolute, forward slashes; Loomfile first, then includes in evaluation order
	Vars      map[string]string  `json:"vars"`                // fully expanded; includes built-ins and overrides
	Overrides map[string]string  `json:"overrides,omitempty"` // CLI VAR=val as given
	Order     []string           `json:"order"`               // target names in first-declaration order; never nil
	Targets   map[string]*Target `json:"targets"`             // keyed by Name; never nil
	Patterns  []*PatternRule     `json:"patterns,omitempty"`  // declaration order
	GoBlocks  []*GoBlock         `json:"goBlocks,omitempty"`  // declaration order
}

type Target struct {
	Name      string            `json:"name"` // cleaned (path.Clean) for files; identifier for tasks
	Kind      Kind              `json:"kind"`
	Pos       syntax.Pos        `json:"pos"` // first declaration
	Doc       string            `json:"doc,omitempty"`
	Prereqs   []string          `json:"prereqs,omitempty"` // normal; deduplicated, declaration order
	OrderOnly []string          `json:"orderOnly,omitempty"`
	Params    []Param           `json:"params,omitempty"` // tasks only
	Vars      map[string]string `json:"vars,omitempty"`   // target-specific, expanded
	Recipe    *Recipe           `json:"recipe,omitempty"` // nil = aggregate
}

type Param struct {
	Name    string  `json:"name"`
	Default *string `json:"default"` // nil (JSON null) = required
}

type Recipe struct {
	Lang  Lang       `json:"lang"`
	Body  string     `json:"body"`            // dedented, verbatim, ends with "\n" unless empty
	Pos   syntax.Pos `json:"pos"`             // first body line
	Shell []string   `json:"shell,omitempty"` // from the `shell` directive in effect; nil = embedded interpreter
}

// PatternRule is a declaration whose target names contain {stem} or %.
// Prerequisites are stored with % normalised to {stem}; literal prerequisites are kept as-is.
type PatternRule struct {
	Pos       syntax.Pos        `json:"pos"`
	Doc       string            `json:"doc,omitempty"`
	Targets   []Pattern         `json:"targets"`
	Prereqs   []string          `json:"prereqs,omitempty"`
	OrderOnly []string          `json:"orderOnly,omitempty"`
	Vars      map[string]string `json:"vars,omitempty"`
	Recipe    *Recipe           `json:"recipe,omitempty"`
}

type Pattern struct {
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

type GoBlock struct {
	Pos  syntax.Pos `json:"pos"` // first body line
	Body string     `json:"body"`
}
