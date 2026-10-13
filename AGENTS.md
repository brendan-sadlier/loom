# Loom — Agent Guide

Loom is a Make replacement written in Go: Make's file-target/dependency model with a terse brace syntax and recipes in
shell (embedded interpreter) or Go (compiled, cached).

## Read in this order

1. This file.
2. `ARCHITECTURE.md` — the specification. Section references (`§6.3`) point here.
3. `.docs/IMPLEMENTATION_PLAN.md` — phase index, master task list, session protocol.
4. `.docs/plan/PHASE_NN_*.md` — the phase you are working on.

## Repository shape

- Root module `github.com/brendan-sadlier/loom` — **runtime** library imported by user Go recipes. Standard library
  only. Packages: `loom` (root), `sh`.
- `cmd/loom/` — nested module, the CLI. All third-party deps live here. It does not import the runtime module.
- `go.work` at the root joins both for development. `cmd/loom/go.mod` has no `replace`.
- `cmd/loom/internal/*` — see the dependency table in `ARCHITECTURE.md` §2. A guard test enforces it.

## Commands (run from repo root)

```sh
go build ./... && go vet ./... && go test ./...
go test -tags gotoolchain ./cmd/loom/...      # Go-recipe e2e (needs `go` in PATH)
(cd cmd/loom && golangci-lint run ./...) && golangci-lint run ./...
(cd cmd/loom && GOWORK=off go build ./...)    # CLI must build standalone
```

Linux/Windows are verified only in CI. Do not claim cross-platform behaviour works without a green CI run on the PR.

## Rules (summary of ARCHITECTURE.md §16 — that section is authoritative)

- Recipe bodies are opaque: never interpolate or reformat them.
- Never write outside `./.loom/` except target outputs and `loom init` files.
- No third-party imports in the runtime module.
- No silent fallbacks; missing toolchain/shell/runtime → exit 3 with the fix.
- Behaviour must be identical on Linux, macOS, Windows.
- No new dependencies, flags, directives or functions without a Decision Log entry.
- Nothing from the non-goals list (§1).

## Workflow

- Branch `phase-NN`; one task → verify → one commit `phase N (§x.y): summary`; tick the box in
  `.docs/IMPLEMENTATION_PLAN.md` §5 in the same commit.
- Finish a phase by completing its gate checklist and exit report, then ask before pushing and opening the PR.
- When blocked, write it under "Blockers" in the phase file and ask; do not redesign.

## Style

- `gofmt`, `go vet`, `golangci-lint` clean.
- Errors wrap with `%w`; user-facing errors are `*syntax.Diagnostic` printed as
  `FILE:LINE:COL: message`.
- Comments state what code cannot: invariants, platform quirks, Make-compat rationale.
- Tests: unit tests beside packages; end-to-end as `testscript` txtar files under
  `cmd/loom/testdata/script/`; no `time.Sleep` for mtimes — use `os.Chtimes`.