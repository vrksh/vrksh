# Contributing

## Setup

```bash
git clone https://github.com/vrksh/vrksh.git
cd vrksh
make build
./vrk uuid   # verify it works
```

Requires Go 1.25+. No other dependencies.

## Adding a tool

Each tool is a Go package in `cmd/<tool>/` with one exported function: `Run() int`. It returns an exit code. Never call `os.Exit` inside `Run()`.

### Files to create

1. `cmd/<tool>/<tool>_test.go` - write tests first, confirm they fail
2. `cmd/<tool>/<tool>.go` - implement `Run() int` until tests pass
3. `testdata/<tool>/smoke.sh` - end-to-end tests against the built binary
4. `schema/<tool>.yaml` - tool metadata for doc generation

### Files to update

5. `main.go` - add import and registry entry in the `tools` map
6. `manifest.json` - add tool name and description (under 60 chars)
7. `integrations/skills/SKILLS.md` - add flags, exit codes, gotchas section

All seven changes go in one commit: `feat(<tool>): <what it does>`.

### The Run() pattern

Use `cmd/links/links.go` as the reference implementation. Key rules:

- `pflag` for flag parsing, not stdlib `flag`, not cobra
- `fs.SetOutput(io.Discard)` before `fs.Parse` to suppress pflag's own error output
- Check `errors.Is(err, pflag.ErrHelp)` before the generic error path
- TTY guard: exit 2 when stdin is a terminal with no positional args
- Use `shared.Errorf()` (returns 1) and `shared.UsageErrorf()` (returns 2) for errors
- Use `shared.ReadInput()` so both positional args and stdin work

See `AGENTS.md` for the full tool template and shared utility reference.

## Testing

```bash
make test                  # all unit tests
make test-tool TOOL=jwt    # one tool only
make smoke                 # end-to-end smoke tests (builds first)
make check                 # build + test + lint + cross-compile + smoke
```

Always use the Makefile. It sets `CGO_ENABLED=0` consistently.

### Required test coverage

Every tool must test:

1. Happy path with correct output
2. Exit 0 on success
3. Exit 1 on runtime error
4. Exit 2 on usage error (unknown flag, missing input)
5. `--help` exits 0, stdout contains tool name
6. Interactive TTY exits 2
7. TTY + `--json` exits 2, error JSON on stdout, stderr empty
8. `--json` + I/O error: error JSON on stdout, stderr empty, exit 1
9. Empty stdin exits 0 with no output (most tools)

## Exit codes

These are public API. Never change them after a tool ships.

| Code | Meaning | When |
|------|---------|------|
| 0 | Success | Output produced, condition met |
| 1 | Runtime error | Invalid input, API failure, condition not met |
| 2 | Usage error | Unknown flag, missing input, bad argument |

## stdout / stderr contract

Data goes to stdout. Errors and warnings go to stderr. When `--json` is active, errors go to stdout as `{"error":"...","code":N}` and stderr stays empty. Use `shared.PrintJSONError()` for this.

## Before opening a PR

- [ ] `make check` passes (build + test + lint + cross-compile + smoke)
- [ ] New tool has `schema/<tool>.yaml` for doc generation
- [ ] New tool has `testdata/<tool>/smoke.sh`
- [ ] Exit codes match the contract (0/1/2)
- [ ] No `os.Exit` calls inside `Run()`
- [ ] `--help` works with no stdin
- [ ] Both positional args and stdin work for input

For the full codebase reference (shared utilities, flag conventions, architecture decisions), see `AGENTS.md`.
