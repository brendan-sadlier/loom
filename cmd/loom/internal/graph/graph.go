package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"
)

// Node is a target or a source file in the build graph.
type Node struct {
	Name      string
	Target    *model.Target // nil for a source file (exists on disk, no rule)
	Prereqs   []*Node       // normal prerequisites, declaration order
	OrderOnly []*Node
	Stem      string // set by pattern instantiation (Phase 06)
}

// IsSource reports whether n is a plain input file with no rule.
func (n *Node) IsSource() bool { return n.Target == nil }

// Graph is the closure of the requested targets.
type Graph struct {
	Nodes     map[string]*Node
	Requested []*Node // CLI order
	Order     []*Node // topological: every node after all its prerequisites
}

type Options struct {
	Root   string                 // OS path of the project root
	Exists func(name string) bool // nil = os.Stat under Root
}

// UnknownTargetError is returned for a requested name with no rule.
type UnknownTargetError struct{ Name string }

func (e *UnknownTargetError) Error() string { return fmt.Sprintf("unknown target %q", e.Name) }

type builder struct {
	p        *model.Project
	opts     Options
	g        *Graph
	visiting map[string]bool
	stack    []string
}

// Build resolves requested against p. Names are matched as given (already cleaned by eval).
func Build(p *model.Project, requested []string, opts Options) (*Graph, error) {
	if opts.Exists == nil {
		opts.Exists = func(name string) bool {
			_, err := os.Stat(filepath.Join(opts.Root, filepath.FromSlash(name)))
			return err == nil
		}
	}
	b := &builder{p: p, opts: opts, g: &Graph{Nodes: map[string]*Node{}}, visiting: map[string]bool{}}
	for _, name := range requested {
		n, err := b.resolve(name, nil)
		if err != nil {
			return nil, err
		}
		if n == nil {
			return nil, &UnknownTargetError{Name: name}
		}
		b.g.Requested = append(b.g.Requested, n)
	}
	return b.g, nil
}

// resolve returns the node for name, creating it and its closure on first sight.
// neededBy is nil for requested names. A nil node with nil error means "no rule and
// not a file" for requested names; for prerequisites that is an error here.
func (b *builder) resolve(name string, neededBy *Node) (*Node, error) {
	if n, ok := b.g.Nodes[name]; ok {
		return n, nil
	}
	if b.visiting[name] {
		return nil, b.cycleError(name)
	}
	t, ok := b.p.Targets[name]
	if !ok {
		if inst, matched, err := b.instantiatePattern(name); err != nil {
			return nil, err
		} else if matched {
			t = inst
		}
	}
	if t == nil {
		if neededBy != nil && b.opts.Exists(name) {
			n := &Node{Name: name}
			b.g.Nodes[name] = n
			b.g.Order = append(b.g.Order, n)
			return n, nil
		}
		if neededBy == nil {
			return nil, nil
		}
		return nil, syntax.Errorf(neededBy.Target.Pos, "no rule to make %q, needed by %q", name, neededBy.Name)
	}

	n := &Node{Name: name, Target: t}
	b.visiting[name] = true
	b.stack = append(b.stack, name)
	for _, dep := range t.Prereqs {
		d, err := b.resolve(dep, n)
		if err != nil {
			return nil, err
		}
		n.Prereqs = append(n.Prereqs, d)
	}
	for _, dep := range t.OrderOnly {
		d, err := b.resolve(dep, n)
		if err != nil {
			return nil, err
		}
		n.OrderOnly = append(n.OrderOnly, d)
	}
	b.stack = b.stack[:len(b.stack)-1]
	delete(b.visiting, name)
	b.g.Nodes[name] = n
	b.g.Order = append(b.g.Order, n) // post-order == topological
	return n, nil
}

func (b *builder) cycleError(name string) error {
	i := 0
	for i < len(b.stack) && b.stack[i] != name {
		i++
	}
	path := append(append([]string{}, b.stack[i:]...), name)
	return syntax.Errorf(b.p.Targets[name].Pos, "dependency cycle: %s", strings.Join(path, " -> "))
}

// instantiatePattern is implemented in Phase 06. Until then nothing matches.
func (b *builder) instantiatePattern(name string) (*model.Target, bool, error) {
	return nil, false, nil
}
