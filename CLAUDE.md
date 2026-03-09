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
  prompt/prompt.go               — package prompt, exports Run()
  kv/kv.go                      — package kv, exports Run()
internal/
  shared/
    input.go                    — ReadInput, ReadInputOptional, ReadInputFile
    exit.go                     — Errorf, UsageErrorf, Warn (for Run()); Die, DieUsage (main() only); exit code constants
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
  skills/
    SKILLS.md                   — agent usage reference: flags, exit codes, gotchas, compose patterns
                                  This teaches agents how to USE vrksh. Hand-authored. Ships with v1.
                                  Served at vrk.sh/skills. Accessible via `vrk --skills`.
  prompts/                      — system prompt snippets
  direnv/                       — .envrc template
AGENTS.md                       — quick orientation: one line per tool, 5 key patterns
CLAUDE.md                       — this file (for coding agents building vrksh)
README.md

Two agent-facing files, different purposes:
- AGENTS.md        quick orientation, under 60 lines, agents skim this
- integrations/skills/SKILLS.md   full reference, flags/exit codes/gotchas/compose per tool
```

**Package structure:** each tool is its own Go package (`package jwt`, `package epoch`, etc.)
with a single exported function `Run() int` that returns an exit code. `main.go` maintains a
registry map and calls `os.Exit(tool())`. This is idiomatic Go — tools in their own directories,
own namespaces, independently testable without process exit.

```go
// main.go pattern — registry map, not switch statement
import (
    "github.com/vrksh/vrksh/cmd/jwt"
    "github.com/vrksh/vrksh/cmd/epoch"
    // one import per tool
)

//go:embed integrations/skills/SKILLS.md
var skillsDoc string

//go:embed manifest.json
var manifestJSON string

var tools = map[string]func() int{
    "jwt":    jwt.Run,
    "epoch":  epoch.Run,
    "prompt": prompt.Run,
    // one line per tool — no switch, no manual argument shifting
}

func main() {
    // handle --manifest and --skills before tool dispatch
    if len(os.Args) > 1 && os.Args[1] == "--manifest" {
        fmt.Print(manifestJSON)
        os.Exit(0)
    }
    if len(os.Args) > 1 && os.Args[1] == "--skills" {
        fmt.Print(skillsDoc)
        os.Exit(0)
    }
    // multicall: check argv[0] first, then argv[1]
    name := filepath.Base(os.Args[0])
    if fn, ok := tools[name]; ok {
        os.Exit(fn())
    }
    if len(os.Args) < 2 {
        fmt.Fprintf(os.Stderr, "usage: vrk <tool> [args]\n")
        os.Exit(2)
    }
    name = os.Args[1]
    os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
    fn, ok := tools[name]
    if !ok {
        fmt.Fprintf(os.Stderr, "vrk: unknown tool %q\n", name)
        os.Exit(2)
    }
    os.Exit(fn())
}
```

**Why registry, not switch:** at 30+ tools a switch requires two edits per tool (import + case) and drifts from the manifest. The registry requires one line. `--manifest` is trivially derived by iterating `tools`. Never revert to a switch.

**`Run() int` not `Run()`:** every tool's `Run()` must return an exit code (0/1/2). `main.go` calls `os.Exit(tool())`. This keeps tools usable as libraries, makes tests check return values instead of intercepting `os.Exit`, and is the standard Go pattern. Never call `os.Exit` inside a tool's `Run()`.

Inside `Run()`, use `shared.Errorf()` and `shared.UsageErrorf()` — they print to stderr and return the exit code:
```go
return shared.Errorf("jwt: invalid token: %v", err)   // prints "error: ...", returns 1
return shared.UsageErrorf("missing required input")    // prints "usage error: ...", returns 2
```
`Die()`/`DieUsage()` call `os.Exit` and are for `main()` only. Calling them inside `Run()` terminates the test process.

**`manifest.json`:** lives at repo root, checked in, updated manually when tools are added. Format:
```json
{"version":"0.1.0","tools":[{"name":"jwt","description":"JWT inspector"},{"name":"epoch","description":"Timestamp converter"}]}
```
When you add a tool, add it to `tools` map, `manifest.json`, and `SKILLS.md` in the same commit.

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

**Makefile must have `export CGO_ENABLED=0` at the top** — not repeated per target. If it is not there, add it before anything else. This prevents accidental CGO creep from any target that doesn't explicitly set it.

**Linters enabled in `.golangci.yml`:** `errcheck`, `govet`, `staticcheck`, `gosimple`, `ineffassign`, `unused`, `revive`, `gocritic`. Add `bodyclose` when `grab` is built (catches leaked HTTP response bodies). Do not add linters you are not going to fix — every enabled linter is a rule Claude Code must satisfy.

Two flags are built into the root binary alongside the tool dispatcher:

- `vrk --manifest` — prints embedded JSON tool manifest to stdout. For agent discovery.
- `vrk --skills`   — prints `integrations/skills/SKILLS.md` to stdout. For agent context.

Both use `//go:embed` in main.go. When you add a new tool, update both the manifest JSON and `integrations/skills/SKILLS.md` before committing.

**Two agent-facing files, different purposes — do not confuse them:**
- `CLAUDE.md` (this file) — for coding agents *building* vrksh. Covers codebase patterns.
- `integrations/skills/SKILLS.md` — for AI agents *using* vrksh in their pipelines. Covers flags, exit codes, gotchas, and compose patterns per tool. Hand-authored. Ships with v1.

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

- **One binary, multicall dispatch.** `main.go` reads `os.Args[0]` or `os.Args[1]` to route. Never create separate binaries per tool. Each tool is `package <toolname>` in `cmd/<tool>/` with a single exported `Run() int` function. `main.go` imports them all via the registry map.
- **`Run() int` returns exit code.** Never call `os.Exit` inside a tool's `Run()`. Return 0, 1, or 2. `main.go` calls `os.Exit(fn())`. This keeps tools testable as libraries and removes the need for panic/recover in tests.
- **Library-first shared utilities.** Functions in `internal/shared/` must return `error`, not call `Die()` directly. `KVPath() (string, error)`, `PrintJSON(v any) error`. The tool's `Run()` receives the error and calls `shared.Errorf()` or `shared.UsageErrorf()` — never `Die()`. This makes shared utilities usable in tests and future library consumers without process exit.
- **`pflag` for flag parsing.** `github.com/spf13/pflag` — not stdlib `flag`, not cobra. Drop-in replacement with POSIX short flags (`-j`/`--json`). For subcommands (`kv set`/`kv get`), use a manual switch on `os.Args[1]` then a sub-`pflag.FlagSet`. No cobra.
- **Always check `fs.Parse(args)` error.** When using `pflag.ContinueOnError`, you must check the return value of `fs.Parse(args)` and `return shared.UsageErrorf(err.Error())` if it is non-nil. Never let pflag print to stderr and continue with invalid flag state.
- **`modernc.org/sqlite` for SQLite.** Never `mattn/go-sqlite3` — it requires CGO and breaks cross-compilation. Verify `modernc.org/sqlite` is in `go.mod` before building any tool that touches `kv`.
- **No CGO anywhere.** The static binary promise depends on this. `CGO_ENABLED=0` must produce a working binary.
- **Streaming input for record-processing tools.** Tools that operate on JSONL record-by-record (`valve`, `fuse`, `each`, `dedup`, `recase`, `agg`, `emit`) must use `bufio.Scanner` — not `io.ReadAll`. `io.ReadAll` is only appropriate for tools where the full input is semantically required (`prompt`, `tok`, `chunk`). Use `internal/shared/input.go:ScanLines()` for the streaming path.
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
- **Buffered stdout**: streaming tools (`sse`, `prompt`) need explicit `bufio.Writer` flushing after every write. Do not rely on default buffering.
- **Stderr contamination**: informational messages, warnings, progress — all go to stderr. If it's not data, it must not touch stdout.
- **`--json` on `prompt`**: means "emit response as JSON object with metadata." It does NOT mean "instruct the LLM to respond in JSON." That is `--schema`.
- **Temperature on `prompt`**: default is 0. Do not change this without an explicit `--temperature` flag. Determinism is the default behaviour.
- **Secret safety in `prompt`**: API keys from env vars must never appear in stdout, stderr, or error messages. Sanitise all error output before writing.
- **`Run()` must return int, not call `os.Exit`.** A tool's `Run()` returns 0, 1, or 2. `os.Exit` lives only in `main()`. Inside `Run()`, use `return shared.Errorf(...)` (returns 1) and `return shared.UsageErrorf(...)` (returns 2). If you call `os.Exit` or `Die()`/`DieUsage()` inside `Run()`, it terminates the test process.
- **Shared utilities return error, not die.** `KVPath()`, `PrintJSON()`, and other `internal/shared` functions must return `error`. The tool's `Run()` handles the error with `shared.Errorf()`. Never call `Die()` inside a shared utility.
- **`fs.Parse` error must be checked.** After `fs.Parse(args)`, always check the error. If non-nil, `return shared.UsageErrorf(err.Error())`. Never continue with unparsed flags.
- **`io.ReadAll` is wrong for record-processing tools.** `valve`, `fuse`, `each`, `dedup`, `recase`, `agg`, `emit` process JSONL line-by-line. Use `bufio.Scanner` / `ScanLines()`. `io.ReadAll` on a 10GB log file is an OOM crash. Only use `io.ReadAll` for tools that need the full input (`prompt`, `tok`, `chunk`).
- **`KVPath()` failure message must be actionable.** If `os.UserHomeDir()` fails, die with: `kv: cannot determine home directory: <err>\nset VRK_KV_PATH to override`. Do not silently fall back to `/tmp/vrk.db` — silent fallback creates two databases and confuses users.
- **`modernc.org/sqlite` must be in `go.mod`** before building any tool that touches `kv`. If it is missing, add it explicitly. Do not substitute `mattn/go-sqlite3`.
- **JSON numbers: use `json.NewDecoder` with `UseNumber()`, never `json.Unmarshal` into `interface{}`** when the JSON may contain large integers (timestamps, IDs, token counts). `json.Unmarshal` into `interface{}` converts all numbers to `float64`, which silently loses precision above 2^53. JWT `exp`/`iat` claims, Unix timestamps, and database IDs all fall in this range. Always:
  ```go
  var v interface{}
  d := json.NewDecoder(strings.NewReader(input))
  d.UseNumber()
  if err := d.Decode(&v); err != nil { ... }
  ```
  This applies to `jwt`, `epoch`, `kv`, `prompt --json`, and any tool that unmarshals JSONL records.

## Build order — tests before implementation

When building any tool, always follow this order:

1. Read the correctness spec provided in the session prompt
2. Write `cmd/<tool>/<tool>_test.go` first — all cases from the spec
3. Run `go test ./cmd/<tool>/...` — all tests must fail (red)
4. Write the implementation in `cmd/<tool>/main.go`
5. Run `go test ./cmd/<tool>/...` again — all tests must pass (green)
6. Run `make check` — cross-compilation and linting must pass
7. Run `testdata/<tool>/smoke.sh` against the built binary — confirms end-to-end behaviour that unit tests cannot catch (real process exit codes, stdout/stderr separation, pipeline composition)

The smoke script must be committed in the same commit as the tool.

Do not write any implementation code before the tests exist.
If no correctness spec was provided in the session prompt, ask for one before proceeding.

## Testing approach

- Test the contract, not the implementation.
- **Check return value, not `os.Exit`.** Because `Run() int` returns the exit code, tests simply call `Run()` and check the int. No panic/recover, no `exitSentinel`. If you find yourself intercepting `os.Exit` in a test, `Run()` is calling `os.Exit` internally — that is a bug, fix it.
- Golden files in `testdata/<tool>/` for deterministic tools.
- Exit code tests are highest priority — they must never regress.
- For `epoch` tests, always pass `--at 1740009600` to make relative times deterministic. Use `1740009600` (`2025-02-20T00:00:00Z`) as the anchor — not `1740000000` (that is `2025-02-19T21:20:00Z`, not a clean boundary).
- Fuzz targets required for: `jwt`, `epoch`, `tok`, `sse`. Contract: never panic, never hang, exit within 1 second.
- Integration tests tagged `//go:build integration` — excluded from default `go test ./...`.
- Property tests required for every tool — at least one invariant that must hold for any input, not just the example cases.

## See also

- `docs/flag-conventions.md` — full flag spec
- `docs/architecture.md` — why decisions were made; read before changing anything fundamental
- `AGENTS.md` — tool reference for agents
- `internal/shared/` — shared contract helpers, use these, don't reinvent
