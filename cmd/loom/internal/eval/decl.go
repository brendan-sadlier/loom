package eval

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brendan-sadlier/loom/cmd/loom/internal/model"
	"github.com/brendan-sadlier/loom/cmd/loom/internal/syntax"

	doublestar "github.com/bmatcuk/doublestar/v4"
)

// declState tracks positions needed for merge diagnostics.
type declState struct {
	first     syntax.Pos
	recipePos syntax.Pos // zero until a recipe is seen
	paramsPos syntax.Pos // tasks: zero until params are seen
}

// declParts is what one declaration contributes to a target or pattern rule.
type declParts struct {
	pos    syntax.Pos
	doc    string
	normal []string
	order  []string
	rec    *model.Recipe
}

func (ev *evaluator) evalTarget(d *syntax.TargetDecl, fs *fileState) error {
	var lits []string
	var pats []model.Pattern
	for _, w := range d.Names {
		items, err := ev.expandList(w)
		if err != nil {
			return err
		}
		for _, item := range items {
			_, isPat, err := model.ParsePattern(item)
			if err != nil {
				return syntax.Errorf(w.Pos, "%v", err)
			}
			if isPat {
				p, _, _ := model.ParsePattern(path.Clean(item))
				pats = append(pats, p)
				continue
			}
			name, err := cleanName(w.Pos, item)
			if err != nil {
				return err
			}
			lits = append(lits, name)
		}
	}
	if len(lits) > 0 && len(pats) > 0 {
		return syntax.Errorf(d.P, "cannot mix pattern and literal target names in one declaration")
	}
	if len(lits) == 0 && len(pats) == 0 {
		return syntax.Errorf(d.P, "empty target name")
	}
	isPattern := len(pats) > 0

	if d.Var != nil {
		value, err := ev.expand(d.Var.Value)
		if err != nil {
			return err
		}
		if isPattern {
			r, _ := ev.patternFor(pats, d.P)
			if r.Vars == nil {
				r.Vars = map[string]string{}
			}
			return ev.scope.assignTarget(r.Vars, d.Var.P, d.Var.Name, d.Var.Op, value)
		}
		for _, name := range lits {
			t, _, err := ev.targetFor(name, model.KindFile, d.P)
			if err != nil {
				return err
			}
			if t.Vars == nil {
				t.Vars = map[string]string{}
			}
			if err := ev.scope.assignTarget(t.Vars, d.Var.P, d.Var.Name, d.Var.Op, value); err != nil {
				return err
			}
		}
		return nil
	}

	normal, order, err := ev.prereqLists(d.Prereqs, d.OrderOnly, isPattern)
	if err != nil {
		return err
	}
	rec, err := ev.recipe(d.Body, d.Attrs, d.P, fs)
	if err != nil {
		return err
	}
	parts := declParts{pos: d.P, doc: d.Doc, normal: normal, order: order, rec: rec}

	if isPattern {
		r, st := ev.patternFor(pats, d.P)
		key := patternKey(pats)
		return mergeDecl(key, st, parts, &r.Prereqs, &r.OrderOnly, &r.Doc, &r.Recipe)
	}
	for _, name := range lits {
		t, st, err := ev.targetFor(name, model.KindFile, d.P)
		if err != nil {
			return err
		}
		if err := mergeDecl(name, st, parts, &t.Prereqs, &t.OrderOnly, &t.Doc, &t.Recipe); err != nil {
			return err
		}
	}
	return nil
}

func (ev *evaluator) evalTask(d *syntax.TaskDecl, fs *fileState) error {
	normal, order, err := ev.prereqLists(d.Prereqs, d.OrderOnly, false)
	if err != nil {
		return err
	}
	rec, err := ev.recipe(d.Body, d.Attrs, d.P, fs)
	if err != nil {
		return err
	}
	t, st, err := ev.targetFor(d.Name, model.KindTask, d.P)
	if err != nil {
		return err
	}
	if len(d.Params) > 0 {
		if st.paramsPos.IsValid() {
			return syntax.Errorf(d.P, "parameters for task %q already declared at %s", d.Name, st.paramsPos)
		}
		params := make([]model.Param, len(d.Params))
		for i, p := range d.Params {
			params[i].Name = p.Name
			if p.Default != nil {
				v, err := ev.expand(*p.Default)
				if err != nil {
					return err
				}
				params[i].Default = &v
			}
		}
		t.Params = params
		st.paramsPos = d.P
	}
	parts := declParts{pos: d.P, doc: d.Doc, normal: normal, order: order, rec: rec}
	return mergeDecl(d.Name, st, parts, &t.Prereqs, &t.OrderOnly, &t.Doc, &t.Recipe)
}

// targetFor returns the target named name, creating it (and appending to Order) on first sight.
func (ev *evaluator) targetFor(name string, kind model.Kind, pos syntax.Pos) (*model.Target, *declState, error) {
	if t, ok := ev.proj.Targets[name]; ok {
		st := ev.decls[name]
		if t.Kind != kind {
			return nil, nil, syntax.Errorf(pos, "%q is declared as both a task and a file target (see %s)", name, st.first)
		}
		return t, st, nil
	}
	t := &model.Target{Name: name, Kind: kind, Pos: pos}
	st := &declState{first: pos}
	ev.proj.Targets[name] = t
	ev.proj.Order = append(ev.proj.Order, name)
	ev.decls[name] = st
	return t, st, nil
}

// patternFor returns the rule for the exact set of patterns, creating it on first sight.
func (ev *evaluator) patternFor(pats []model.Pattern, pos syntax.Pos) (*model.PatternRule, *declState) {
	key := patternKey(pats)
	if r, ok := ev.pats[key]; ok {
		return r, ev.decls[key]
	}
	r := &model.PatternRule{Pos: pos, Targets: slices.Clone(pats)}
	st := &declState{first: pos}
	ev.pats[key] = r
	ev.decls[key] = st
	ev.proj.Patterns = append(ev.proj.Patterns, r)
	return r, st
}

func patternKey(pats []model.Pattern) string {
	s := make([]string, len(pats))
	for i, p := range pats {
		s[i] = p.String()
	}
	return strings.Join(s, " ")
}

// mergeDecl folds one declaration into an existing target or rule.
func mergeDecl(name string, st *declState, p declParts, prereqs, orderOnly *[]string, doc *string, recipe **model.Recipe) error {
	*prereqs = appendUnique(*prereqs, p.normal...)
	*orderOnly = appendUnique(*orderOnly, p.order...)
	if *doc == "" && p.doc != "" {
		*doc = p.doc
	}
	if p.rec != nil {
		if *recipe != nil {
			return syntax.Errorf(p.pos, "target %q already has a recipe (declared at %s)", name, st.recipePos)
		}
		r := *p.rec // each target of a multi-name declaration owns its Recipe
		*recipe = &r
		st.recipePos = p.pos
	}
	return nil
}

func (ev *evaluator) recipe(body *syntax.Body, attrs []string, declPos syntax.Pos, fs *fileState) (*model.Recipe, error) {
	if body == nil {
		if len(attrs) > 0 {
			return nil, syntax.Errorf(declPos, "attribute \"@%s\" requires a recipe body on the same declaration", attrs[0])
		}
		return nil, nil
	}
	if len(attrs) > 1 {
		return nil, syntax.Errorf(declPos, "conflicting attributes \"@%s\" and \"@%s\"", attrs[0], attrs[1])
	}
	lang := model.LangSh
	if len(attrs) == 1 {
		l, ok := model.ParseLang(attrs[0])
		if !ok {
			return nil, syntax.Errorf(declPos, "unknown attribute \"@%s\"", attrs[0])
		}
		lang = l
	}
	r := &model.Recipe{Lang: lang, Body: body.Text, Pos: body.Pos}
	if lang == model.LangSh && fs != nil {
		r.Shell = slices.Clone(fs.shell)
	}
	return r, nil
}

func (ev *evaluator) prereqLists(prereqs, orderOnly []syntax.Word, pattern bool) (normal, order []string, err error) {
	if normal, err = ev.prereqItems(prereqs, pattern); err != nil {
		return nil, nil, err
	}
	if order, err = ev.prereqItems(orderOnly, pattern); err != nil {
		return nil, nil, err
	}
	return normal, order, nil
}

// prereqItems expands, globs and cleans prerequisite words.
// Items in pattern rules are never globbed (they may contain {stem}).
func (ev *evaluator) prereqItems(words []syntax.Word, pattern bool) ([]string, error) {
	var out []string
	for _, w := range words {
		items, err := ev.expandList(w)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !pattern && isGlob(item) {
				matches, err := ev.globPrereq(w.Pos, item)
				if err != nil {
					return nil, err
				}
				out = append(out, matches...)
				continue
			}
			_, isPat, err := model.ParsePattern(item)
			if err != nil {
				return nil, syntax.Errorf(w.Pos, "%v", err)
			}
			if isPat {
				if !pattern {
					return nil, syntax.Errorf(w.Pos, `"{stem}" is only allowed in pattern rules`)
				}
				p, _, _ := model.ParsePattern(path.Clean(item))
				out = append(out, p.String())
				continue
			}
			name, err := cleanName(w.Pos, item)
			if err != nil {
				return nil, err
			}
			out = append(out, name)
		}
	}
	return out, nil
}

// globPrerequisite expands item against the filesystem relative to the project root.
// Results are root-relative slash paths (absolute patterns yield absolute paths), sorted.
// Zero matches is an error: a silently empty prerequisite list hides rebuild bugs (§3.5).
func (ev *evaluator) globPrereq(pos syntax.Pos, item string) ([]string, error) {
	pat := filepath.FromSlash(item)
	abs := filepath.IsAbs(pat)
	if !abs {
		pat = filepath.Join(ev.rootOS, pat)
	}
	matches, err := doublestar.FilepathGlob(pat)
	if err != nil {
		return nil, syntax.Errorf(pos, "invalid glob pattern %q: %v", item, err)
	}
	if len(matches) == 0 {
		return nil, &syntax.Diagnostic{
			Pos:  pos,
			Msg:  fmt.Sprintf("prerequisite pattern %q matched no files", item),
			Hint: "use $(glob ...) for a list that may be empty",
		}
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if !abs {
			rel, err := filepath.Rel(ev.rootOS, m)
			if err != nil {
				return nil, fmt.Errorf("glob %q: %w", item, err)
			}
			m = rel
		}
		out = append(out, filepath.ToSlash(m))
	}
	slices.Sort(out)
	return out, nil
}

// appendUnique appends items not already in dst, preserving order.
func appendUnique(dst []string, items ...string) []string {
	for _, s := range items {
		if !slices.Contains(dst, s) {
			dst = append(dst, s)
		}
	}
	return dst
}
