# vrksh

Unix tools for AI pipelines. One static Go binary, multicall architecture.

> "Unix tools for building reliable AI pipelines."

## Project structure

```
main.go                         — package main, multicall dispatcher, imports each tool package
cmd/
  jwt/
    jwt.go                      — package jwt, exports Run()
    jwt_test.go                 — tests for jwt
  epoch/
    epoch.go                    — package epoch, exports Run()
    epoch_test.go
  uuid/uuid.go                  — package uuid, exports Run()
  tok/tok.go                    — package tok, exports Run()
  sse/sse.go                    — package sse, exports Run()
  coax/coax.go                  — package coax, exports Run()
  ask/ask.go                    — package ask, exports Run()
  kv/kv.go                      — package kv, exports Run()
internal/
  shared/
    input.go                    — ReadInput, ReadInputOptional, ReadInputFile
    exit.go                     — Die, DieUsage, Warn, exit code constants (0/1/2)
    json.go                     — PrintJSON, PrintJSONL, JSONOutput
    flags.go                    — StandardFlags with pflag shorthands (-j/--json, -q/--quiet, etc.)
    kvpath.go                   — KVPath()
    flag_file.go                — CheckpointDir()
  shared/testutil/
    contract.go                 — RunContractTests
testdata/
  jwt/                          — golden files: *.input, *.expected
  epoch/
  ...
docs/
  flag-conventions.md           — canonical flag semantics, read before adding any flag
  architecture.md               — why decisions were made (read before changing anything fundamental)
scripts/
  install.sh                    — served at vrk.sh/install.sh
integrations/
  mcp/                          — MCP server (Quarter 2)
  github-action/                — setup-vrk action (Quarter 2)
  cursor/                       — .cursorrules / Cursor MCP config
  skills/                       — SKILLS.md templates
  prompts/                      — system prompt snippets
  direnv/                       — .envrc template
AGENTS.md                       — machine-readable tool reference for agents
CLAUDE.md                       — this file
README.md
```

**Package structure:** each tool is its own Go package (`package jwt`, `package epoch`, etc.)
with a single exported function `Run()`. `main.go` imports each tool package and calls `Run()`.
This is idiomatic Go — tools in their own directories, own namespaces, independently testable.

```go
// main.go pattern
import "github.com/vrksh/vrksh/cmd/jwt"
...
case "jwt":
    jwt.Run()
```

**Monorepo.** Everything lives here. Only `homebrew-vrksh` is a separate repo — hard Homebrew naming requirement.

## Commands

Always use the Makefile. Never run raw `go` commands - the Makefile sets CGO_ENABLED=0
and keeps flags consistent between local dev and CI.

```bash
make build                 # build the binary (CGO_ENABLED=0)
make test                  # run all tests
make test-v                # verbose tests - use when debugging failures
make test-tool TOOL=jwt    # run tests for one tool only
make lint                  # golangci-lint
make cross                 # verify cross-compilation for all targets - run after every session
make fuzz                  # fuzz all required tools for 60s each
make check                 # build + test + lint + cross - run before every commit
make clean                 # remove build artifacts
```

## Git Workflow

One tool per commit. Run `make check` before every commit.

```bash
# Before committing
make check                 # must pass - build + test + lint + cross

# Commit format: feat(<tool>): <what it does>
git add .
git commit -m "feat(jwt): JWT decoder - decode, --claim, --expired, --json"
git push origin main
```

Do not commit if `make check` fails. Do not skip `make cross` - it catches CGO
creep that `make test` does not catch.

Tag format for releases: `v0.1.0`, `v0.1.1`, `v0.2.0`. Pushing a tag triggers
the release workflow which builds binaries and opens an auto-PR on the Homebrew tap.

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Architecture rules — never deviate

- **One binary, multicall dispatch.** `main.go` reads `os.Args[0]` or `os.Args[1]` to route. Never create separate binaries per tool. Each tool is `package <toolname>` in `cmd/<tool>/` with a single exported `Run()` function. `main.go` imports them all.
- **`pflag` for flag parsing.** `github.com/spf13/pflag` — not stdlib `flag`, not cobra. Drop-in replacement with POSIX short flags (`-j`/`--json`). For subcommands (`kv set`/`kv get`), use a manual switch on `os.Args[1]` then a sub-`pflag.FlagSet`. No cobra.
- **`modernc.org/sqlite` for SQLite.** Never `mattn/go-sqlite3` — it requires CGO and breaks cross-compilation.
- **No CGO anywhere.** The static binary promise depends on this. `CGO_ENABLED=0` must produce a working binary.
- **Minimal dependencies.** Check `go.mod` after every session. Justify every new import.

## The contract — every tool must follow this

- Read from **stdin** only. Never open files unless explicitly flagged.
- Write data to **stdout** only.
- Write errors and warnings to **stderr** only. Stdout must be empty on error.
- **Exit 0** — success.
- **Exit 1** — runtime error (invalid input, API failure, budget exceeded).
- **Exit 2** — usage error (missing required input, unknown flag, ambiguous argument).
- Never prompt interactively. If stdin is required and missing, exit 2 with a message to stderr.
- `--help` must always work, even with no stdin.
- **Every tool that accepts input must accept it two ways — positional argument OR stdin. Never require `echo`.**

```go
// Use this pattern in every tool. Copy from internal/shared/input.go.
// For flags, use internal/shared/flags.go — pflag.FlagSet, not flag.FlagSet.
func readInput(args []string) (string, error) {
    if len(args) > 0 {
        return strings.Join(args, " "), nil   // vrk epoch '+3d'
    }
    b, err := io.ReadAll(os.Stdin)            // echo '+3d' | vrk epoch
    if err != nil {
        return "", err
    }
    if len(bytes.TrimSpace(b)) == 0 {
        return "", fmt.Errorf("no input: provide as argument or via stdin")
    }
    return strings.TrimRight(string(b), "\n"), nil
}
```

Both of these must always work identically:
```bash
vrk epoch '+3d'
echo '+3d' | vrk epoch
printf '+3d' | vrk epoch   # no trailing newline — must also work
```

## Flag conventions

Read `docs/flag-conventions.md` before adding any flag. Summary:

- `--json` — emit output as JSON object or JSONL. Same meaning on every tool.
- `--text` — plain prose, no formatting. Same meaning on every tool.
- `--fail` — exit 1 if condition not met (budget, schema, expiry).
- `--schema` — output must match this JSON schema or exit 1.
- `--explain` — print what the tool would do without doing it. Never makes network calls.
- `--quiet` — suppress all stderr.
- `--dry-run` — preview side effects without executing.

## Things Claude Code gets wrong on this project

- **Exit codes**: usage errors must be exit 2, not exit 1. Check every error path.
- **Flag library**: use `pflag` (`github.com/spf13/pflag`), not stdlib `flag`, not cobra. `pflag` is already imported via `internal/shared/flags.go` — use `StandardFlags` and extend with `pflag.FlagSet` per tool. Do not add cobra.
- **Stdin-only input**: the most common mistake — implementing a tool that only reads from stdin and requires `echo 'input' | vrk tool`. Every tool must also accept a positional argument: `vrk tool 'input'`. Use `internal/shared/input.go:readInput()`. Do not reinvent this.
- **Trailing newline stripping**: `echo` appends a newline. `printf` does not. `strings.TrimRight(input, "\n")` — strip exactly one trailing newline, not all whitespace. `strings.TrimSpace` is wrong here; it would strip meaningful leading/trailing whitespace from content.
- **Buffered stdout**: streaming tools (`sse`, `ask`) need explicit `bufio.Writer` flushing after every write. Do not rely on default buffering.
- **Stderr contamination**: informational messages, warnings, progress — all go to stderr. If it's not data, it must not touch stdout.
- **`--json` on `ask`**: means "emit response as JSON object with metadata." It does NOT mean "instruct the LLM to respond in JSON." That is `--schema`.
- **Temperature on `ask`**: default is 0. Do not change this without an explicit `--temperature` flag. Determinism is the default behaviour.
- **Secret safety in `ask`**: API keys from env vars must never appear in stdout, stderr, or error messages. Sanitise all error output before writing.

## Testing approach

- Test the contract, not the implementation.
- Golden files in `testdata/<tool>/` for deterministic tools.
- Exit code tests are highest priority — they must never regress.
- For `epoch` tests, always pass `--now 1740000000` to make relative times deterministic.
- Fuzz targets required for: `jwt`, `epoch`, `tok`, `sse`. Contract: never panic, never hang, exit within 1 second.
- Integration tests tagged `//go:build integration` — excluded from default `go test ./...`.

## See also

- `docs/flag-conventions.md` — full flag spec
- `docs/architecture.md` — why decisions were made; read before changing anything fundamental
- `AGENTS.md` — tool reference for agents
- `internal/shared/` — shared contract helpers, use these, don't reinvent
