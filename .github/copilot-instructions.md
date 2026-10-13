# Copilot instructions for Loom

Read `AGENTS.md` first; it links to `ARCHITECTURE.md` (specification) and
`.docs/IMPLEMENTATION_PLAN.md` (tasks and session protocol). Those files are authoritative; this file only summarises.

- Two Go modules: root = stdlib-only runtime (`loom`, `sh`); `cmd/loom` = CLI with all third-party deps. Never import
  the runtime from the CLI. Never add deps to the runtime.
- Work one task at a time from `.docs/IMPLEMENTATION_PLAN.md` §5 on branch `phase-NN`. One commit per task:
  `phase N (§x.y): summary`. Tick the checkbox in the same commit.
- Recipe bodies in `.loom` files are opaque text — do not interpolate or reformat.
- Every behaviour change needs a txtar test in `cmd/loom/testdata/script/` or a unit test beside the package. Use
  `os.Chtimes`, never `time.Sleep`, for mtime tests.
- Cross-platform: Linux/Windows are verified in CI only. Say so rather than assuming.
- Ask before `git push`, opening PRs, or deleting files you did not create.