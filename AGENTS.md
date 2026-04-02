# vrksh

Unix tools for AI pipelines. One static Go binary, multicall architecture.

> "Unix tools for building reliable AI pipelines."

## Project structure

```
main.go                         — package main, multicall dispatcher, imports each tool package
manifest.json                   — tool registry, embedded at build time via //go:embed
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
  prompt/prompt.go              — package prompt, exports Run()
  kv/kv.go                      — package kv, exports Run()
  chunk/chunk.go                — package chunk, exports Run()
  grab/grab.go                  — package grab, exports Run()
  links/links.go                — package links, exports Run()  ← most recent tool; reference for patterns
  plain/plain.go                — package plain, exports Run()
  validate/validate.go          — package validate, exports Run()
  mask/mask.go                  — package mask, exports Run()
  emit/emit.go                  — package emit, exports Run()
internal/
  shared/
    input.go                    — ReadInput, ReadInputOptional, ReadInputLines, ScanLines
    exit.go                     — Errorf, UsageErrorf, Warn (for Run()); Die, DieUsage (main() only); exit constants
    json.go                     — PrintJSON, PrintJSONL, PrintJSONError
    flags.go                    — StandardFlags(), SilenceStderr()
    terminal.go                 — IsTerminal (var, overridable in tests)
    kvpath.go                   — KVPath()
    plaintext/
      plaintext.go              — StripMarkdown() — goldmark-based markdown-to-plain-text
    tokcount/
      tokcount.go               — CountTokens(), EncodeTokens(), DecodeTokens() — cl100k_base BPE
    testutil/
      contract.go               — RunContractTests, ContractCase, assertion helpers
testdata/
  <tool>/
    smoke.sh                    — end-to-end smoke tests run against the built binary
  integration/
    smoke.sh                    — cross-tool pipeline tests
docs/
  flag-conventions.md           — canonical flag semantics, read before adding any flag
  architecture.md               — why decisions were made (read before changing anything fundamental)
scripts/
  install.sh                    — served at vrk.sh/install.sh
integrations/
  skills/
    SKILLS.md                   — agent usage reference: flags, exit codes, gotchas, compose patterns
                                  Teaches agents how to USE vrksh. Hand-authored. Embedded in binary.
                                  Served at vrk.sh/skills. Accessible via `vrk --skills`.
  prompts/                      — system prompt snippets
  direnv/                       — .envrc template
AGENTS.md                       — this file (for coding agents building vrksh)
CLAUDE.md                       — includes AGENTS.md via @AGENTS.md
CONTRIBUTING.md                 — human-readable contributing guide
README.md

Two documentation files for different audiences:
- AGENTS.md (this file)            codebase reference for coding agents
- integrations/skills/SKILLS.md    runtime reference for agents using vrksh in pipelines
```

**Package structure:** each tool is its own Go package (`package jwt`, `package epoch`, etc.)
with a single exported function `Run() int` that returns an exit code. `main.go` maintains a
registry map and calls `os.Exit(tool())`. This is idiomatic Go — tools in their own directories,
own namespaces, independently testable without process exit.

**Why registry, not switch:** at 30+ tools a switch requires two edits per tool (import + case) and drifts from the manifest. The registry requires one line. `--manifest` is trivially derived by iterating `tools`. Never revert to a switch.

**`Run() int` not `Run()`:** every tool's `Run()` must return an exit code (0/1/2). `main.go` calls `os.Exit(tool())`. This keeps tools usable as libraries, makes tests check return values instead of intercepting `os.Exit`, and is the standard Go pattern. Never call `os.Exit` inside a tool's `Run()`.

Inside `Run()`, use `shared.Errorf()` (prints "error: ...", returns 1) and `shared.UsageErrorf()` (prints "usage error: ...", returns 2). `Die()`/`DieUsage()` call `os.Exit` — for `main()` only, never inside `Run()`.

**`manifest.json`:** lives at repo root, checked in, updated manually when tools are added. Format:
```json
{
  "version": "0.1.0",
  "tools": [
    {"name": "jwt",   "description": "JWT inspector — decode, --claim, --expired"},
    {"name": "links", "description": "Hyperlink extractor — markdown, HTML, bare URLs to JSONL"}
  ]
}
```
Keep descriptions under 60 characters. Format: `<what it does> — <key flags or output format>`.
When you add a tool, update `manifest.json`, the `tools` map in `main.go`, and `integrations/skills/SKILLS.md` in the same commit.

**`integrations/skills/SKILLS.md`:** when adding a tool, follow the existing section format in the file. Each tool gets flags, exit codes, examples, and gotchas sections.

**Monorepo.** Everything lives here. Only `homebrew-vrksh` is a separate repo — hard Homebrew naming requirement.

## Commands

Always use the Makefile. Never run raw `go` commands - the Makefile sets CGO_ENABLED=0
and keeps flags consistent between local dev and CI.

```bash
make build                 # build the binary (CGO_ENABLED=0)
make test                  # run all unit tests
make test-v                # verbose tests - use when debugging failures
make test-tool TOOL=jwt    # run tests for one tool only
make lint                  # golangci-lint
make cross                 # verify cross-compilation for all targets - run after every session
make fuzz                  # fuzz all required tools for 60s each
make smoke                 # end-to-end smoke tests against the real binary (runs make build first)
make check                 # build + test + lint + cross + smoke - run before every commit
make clean                 # remove build artifacts
```

**Makefile must have `export CGO_ENABLED=0` at the top** — not repeated per target. If it is not there, add it before anything else. This prevents accidental CGO creep from any target that doesn't explicitly set it.

**Linters enabled in `.golangci.yml`:** `errcheck`, `govet`, `staticcheck`, `gosimple`, `ineffassign`, `unused`, `revive`, `gocritic`. Do not add linters you are not going to fix — every enabled linter is a rule Claude Code must satisfy.

Two flags are built into the root binary alongside the tool dispatcher:

- `vrk --manifest` — prints embedded JSON tool manifest to stdout. For agent discovery.
- `vrk --skills`          — prints full `integrations/skills/SKILLS.md` to stdout. For agent context.
- `vrk --skills <tool>`   — prints skill documentation for a single tool only. Lower token cost
                            when an agent only needs one tool's flags and gotchas.
                            Implemented in main.go alongside `--skills` (same embed, filtered output).
                            Do NOT add `--skills` as a flag to individual tool functions —
                            it is a meta-operation on the binary, not on the tool.

Both use `//go:embed` in main.go. When you add a new tool, update both the manifest JSON and `integrations/skills/SKILLS.md` before committing.

**Two agent-facing files, different purposes — do not confuse them:**
- `AGENTS.md` (this file) — for coding agents *building* vrksh. Covers codebase patterns.
- `integrations/skills/SKILLS.md` — for AI agents *using* vrksh in their pipelines. Covers flags, exit codes, gotchas, and compose patterns per tool. Hand-authored. Ships with v1.

## Adding a new tool — checklist

All nine steps must happen in the same commit:

1. Create `cmd/<tool>/<tool>_test.go` — write tests FIRST, confirm they fail (red)
2. Create `cmd/<tool>/<tool>.go` — implement `Run() int` until tests pass (green)
3. Create `testdata/<tool>/smoke.sh` — end-to-end tests against the built binary
4. Add import and registry entry in `main.go`
5. Add entry to `manifest.json`
6. Add `## <tool>` section to `integrations/skills/SKILLS.md`
7. Run `make check` — must pass clean
8. Run `make build && bash testdata/<tool>/smoke.sh` — must pass
9. Commit with `feat(<tool>): <what it does>`

## Tool file template

Every tool follows this exact structure. Copy it, do not invent variations.

```go
// Package links implements vrk links — a hyperlink extractor.
// One-line description of what it does and its output format.
package links

import (
    "errors"
    "fmt"
    "io"
    "os"

    "github.com/spf13/pflag"
    "github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
// Only include this for tools that call io.ReadAll on stdin directly.
var readAll = io.ReadAll

// Run is the entry point for vrk <tool>. Returns 0/1/2. Never calls os.Exit.
func Run() int {
    fs := pflag.NewFlagSet("<tool>", pflag.ContinueOnError)
    fs.SetOutput(io.Discard) // suppress pflag's own error output; we handle it below

    var jsonFlag bool
    fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON envelope")
    // ... register other flags ...

    if err := fs.Parse(os.Args[1:]); err != nil {
        if errors.Is(err, pflag.ErrHelp) {
            return printUsage(fs)
        }
        return shared.UsageErrorf("%s", err.Error())
    }

    // TTY guard: interactive terminal with no piped input → usage error.
    if isTerminal(int(os.Stdin.Fd())) {
        if jsonFlag {
            return shared.PrintJSONError(map[string]any{
                "error": "<tool>: no input: pipe text to stdin",
                "code":  2,
            })
        }
        return shared.UsageErrorf("<tool>: no input: pipe text to stdin")
    }

    // ... tool logic ...

    return 0
}

func printUsage(fs *pflag.FlagSet) int {
    lines := []string{
        "usage: vrk <tool> [flags]",
        "       echo 'input' | vrk <tool>",
        "",
        "One-sentence description.",
        "",
        "flags:",
    }
    for _, l := range lines {
        if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
            return shared.Errorf("<tool>: writing usage: %v", err)
        }
    }
    fs.SetOutput(os.Stdout)
    fs.PrintDefaults()
    return 0
}
```

Key things the template enforces:
- `fs.SetOutput(io.Discard)` before `fs.Parse` — pflag must not write to stderr on error
- `errors.Is(err, pflag.ErrHelp)` before the generic error path — `--help` must exit 0
- TTY guard with dual path: plain stderr message or JSON to stdout depending on `--json`
- Error messages prefixed with the tool name: `<tool>: ...`

## Shared utilities reference

Read the source in `internal/shared/` for full signatures. Key functions and when to use them:

| Function | File | When to use |
|----------|------|-------------|
| `ReadInput(args)` | input.go | Positional args OR stdin. Use for most tools. |
| `ReadInputOptional(args)` | input.go | Like ReadInput but returns ("", nil) for empty input. |
| `ReadInputLines(args)` | input.go | Args as lines or all stdin lines. |
| `ScanLines(r)` | input.go | Streaming line-by-line. Use for JSONL tools, never io.ReadAll. |
| `Errorf(fmt, ...)` | exit.go | Inside Run(). Prints "error: ...", returns 1. |
| `UsageErrorf(fmt, ...)` | exit.go | Inside Run(). Prints "usage error: ...", returns 2. |
| `Warn(fmt, ...)` | exit.go | Warning to stderr, no exit. |
| `Die(fmt, ...)` | exit.go | main() only. Prints error, calls os.Exit(1). |
| `PrintJSON(v)` | json.go | Encode single value to stdout. |
| `PrintJSONL(slice)` | json.go | Encode slice, one item per line. |
| `PrintJSONError(map)` | json.go | Error JSON to stdout when --json active. Returns exit code. |
| `StandardFlags()` | flags.go | Pre-loaded FlagSet with --json, --quiet, --fail, --dry-run, --explain. |
| `SilenceStderr(quiet)` | flags.go | `defer shared.SilenceStderr(quietFlag)()` |
| `IsTerminal` | terminal.go | Var, override in tests: `var isTerminal = shared.IsTerminal` |
| `plaintext.StripMarkdown(s)` | plaintext/ | Markdown to plain text. Uses goldmark. |
| `tokcount.CountTokens(s)` | tokcount/ | cl100k_base token count. Embedded vocab, no network. |

## JSON output conventions

### JSONL tools (one record per input item)

```json
{"text":"Homebrew","url":"https://brew.sh","line":1}
{"text":"Node","url":"https://nodejs.org","line":4}
```

### `--json` metadata trailer (JSONL tools)

The last record emitted when `--json` is active:
```json
{"_vrk":"<tool>","count":N}
```

### Single-result tools with `--json`

A single JSON object with primary data and metadata:
```json
{"tokens":42,"model":"cl100k_base","budget":4000,"over_budget":false}
```

### `--json` error envelope

When `--json` is active and any error occurs, write this to **stdout** (not stderr) and return the exit code. Stderr must be empty:
```json
{"error":"<tool>: <message>","code":1}
```

## Git Workflow

One tool per commit. Run `make check` before every commit.

```bash
# Before committing
make check                 # must pass - build + test + lint + cross + smoke

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
- **Always check `fs.Parse(args)` error.** When using `pflag.ContinueOnError`, you must check the return value of `fs.Parse(args)`. If non-nil and not `pflag.ErrHelp`, `return shared.UsageErrorf(err.Error())`. Never let pflag print to stderr and continue with invalid flag state.
- **`modernc.org/sqlite` for SQLite.** Never `mattn/go-sqlite3` — it requires CGO and breaks cross-compilation. Verify `modernc.org/sqlite` is in `go.mod` before building any tool that touches `kv`.
- **No CGO anywhere.** The static binary promise depends on this. `CGO_ENABLED=0` must produce a working binary.
- **Streaming input for record-processing tools.** Tools that operate on JSONL record-by-record (`mask`, `emit`, `validate`) must use `bufio.Scanner` — not `io.ReadAll`. `io.ReadAll` is only appropriate for tools where the full input is semantically required (`prompt`, `tok`, `chunk`, `links`). Use `internal/shared/input.go:ScanLines()` for the streaming path.
- **Minimal dependencies.** Check `go.mod` after every session. Justify every new import.

## The contract — every tool must follow this

- Read from **stdin** only. Never open files unless explicitly flagged.
- Write data to **stdout** only.
- Write errors and warnings to **stderr** only. Stdout must be empty on error (unless `--json` is active — then errors go to stdout as JSON, stderr empty).
- **Exit 0** — success. This includes empty input for most tools (no output, exit 0).
- **Exit 1** — runtime error (invalid input, API failure, budget exceeded).
- **Exit 2** — usage error (missing required input, unknown flag, ambiguous argument).
- Never prompt interactively. If stdin is required and missing, exit 2 with a message to stderr.
- `--help` must always work, even with no stdin.
- **Every tool that accepts input must accept it two ways — positional argument OR stdin. Never require `echo`.**

Both of these must always work identically:
```bash
vrk epoch '+3d'
echo '+3d' | vrk epoch
printf '+3d' | vrk epoch   # no trailing newline — must also work
```

Empty stdin (non-TTY pipe with no bytes) is **not** a usage error for most tools — it is valid input that produces no output and exits 0.

## Flag conventions

Read `docs/flag-conventions.md` before adding any flag. Key shorthands:

| Long | Short | Notes |
|------|-------|-------|
| `--json` | `-j` | Emit as JSON/JSONL + metadata trailer |
| `--quiet` | `-q` | Suppress stderr; exit codes unaffected |
| `--fail` | `-f` | Exit 1 if condition not met |
| `--text` | `-t` | Plain text output |
| `--schema` | `-s` | Enforce JSON schema on output |
| `--model` | `-m` | Override LLM model |
| `--count` | `-n` | Numeric count (like `head -n`) |
| `--bare` | `-b` | Raw/minimal output (e.g. URLs only) |
| `--explain` | none | Print what would happen, no action |
| `--dry-run` | none | Preview mutations without executing |

Reserved shorthands — never use for anything else: `-v` (verbose), `-i` (interactive), `-f` (fail only), `-F` (field on sse specifically).

Intentionally absent: `--config`, `--verbose`, `--output <file>`, `--interactive`, `--format`.

## Common mistakes coding agents make on this project

- **Exit codes**: usage errors must be exit 2, not exit 1. Check every error path.
- **Flag library**: use `pflag` (`github.com/spf13/pflag`), not stdlib `flag`, not cobra. `pflag` is already imported via `internal/shared/flags.go` — use `StandardFlags` and extend with `pflag.FlagSet` per tool. Do not add cobra.
- **Stdin-only input**: the most common mistake — implementing a tool that only reads from stdin and requires `echo 'input' | vrk tool`. Every tool must also accept a positional argument: `vrk tool 'input'`. Use `internal/shared/input.go:ReadInput()`. Do not reinvent this. **Exception**: pure stream filters that operate line-by-line on an unbounded stream (`sip`, `throttle`) may be stdin-only — the positional argument form is meaningless for a stream that can be millions of lines. Document this explicitly in the tool's SKILLS.md section with "Input: stdin only".
- **Trailing newline stripping**: `echo` appends a newline. `printf` does not. Use `strings.TrimSuffix(input, "\n")` — strip exactly one trailing newline, not all whitespace. `strings.TrimSpace` is wrong here; it strips meaningful leading/trailing whitespace.
- **`fs.SetOutput(io.Discard)` is mandatory**: call this before `fs.Parse`. Without it, pflag writes its own error message to stderr, giving duplicate error output. Always silence pflag's own output.
- **`errors.Is(err, pflag.ErrHelp)` check**: after `fs.Parse` returns an error, check for `pflag.ErrHelp` first and call `printUsage` which returns 0. The generic error path returns 2. Skipping this check makes `--help` exit 2 instead of 0.
- **Buffered stdout**: streaming tools (`sse`, `prompt`) need explicit `bufio.Writer` flushing after every write. Do not rely on default buffering.
- **Stderr contamination**: informational messages, warnings, progress — all go to stderr. If it's not data, it must not touch stdout.
- **`--json` active means stderr must be empty**: when `--json` is set, any error must go to stdout as `{"error":"...","code":N}`. Never write to stderr when `--json` is active. Use `shared.PrintJSONError()`.
- **Empty stdin is not a usage error**: for most tools, `printf '' | vrk <tool>` should exit 0 with no output — not exit 2. Only exit 2 when stdin is an interactive TTY with no positional args.
- **`--json` on `prompt`**: means "emit response as JSON object with metadata." It does NOT mean "instruct the LLM to respond in JSON." That is `--schema`.
- **Temperature on `prompt`**: default is 0. Do not change this without an explicit `--temperature` flag. Determinism is the default behaviour.
- **Secret safety in `prompt`**: API keys from env vars must never appear in stdout, stderr, or error messages. Sanitise all error output before writing.
- **`Run()` must return int, not call `os.Exit`.** A tool's `Run()` returns 0, 1, or 2. `os.Exit` lives only in `main()`. Inside `Run()`, use `return shared.Errorf(...)` (returns 1) and `return shared.UsageErrorf(...)` (returns 2). If you call `os.Exit` or `Die()`/`DieUsage()` inside `Run()`, it terminates the test process.
- **Shared utilities return error, not die.** `KVPath()`, `PrintJSON()`, and other `internal/shared` functions must return `error`. The tool's `Run()` handles the error with `shared.Errorf()`. Never call `Die()` inside a shared utility.
- **`fs.Parse` error must be checked.** After `fs.Parse(args)`, always check the error. If non-nil (and not `pflag.ErrHelp`), `return shared.UsageErrorf(err.Error())`. Never continue with unparsed flags.
- **`io.ReadAll` is wrong for record-processing tools.** Tools that process JSONL line-by-line (`mask`, `emit`, `validate`) must use `bufio.Scanner` / `ScanLines()`. `io.ReadAll` on a 10GB log file is an OOM crash. Only use `io.ReadAll` for tools that need the full input semantically (`prompt`, `tok`, `chunk`, `links`).
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
- **SSE streams in smoke tests**: never store SSE streams in bash variables (`$()` strips trailing `\n\n` which breaks SSE dispatch). Use bash functions that `printf` the stream and pipe directly into the binary.
- **Two `grab` calls in count-comparison smoke tests**: cache the shared input to a `mktemp` file and pipe both invocations from it. Two separate fetches of the same URL can disagree if anything changes between them.

## Build order — tests before implementation

When building any tool, always follow this order:

1. Read the correctness spec provided in the session prompt
2. Write `cmd/<tool>/<tool>_test.go` first — all cases from the spec (confirm they fail, red)
3. Write the implementation in `cmd/<tool>/<tool>.go` until tests pass (green)
4. Create `testdata/<tool>/smoke.sh` — end-to-end tests against the built binary
5. Run `make check` — cross-compilation and linting must pass
6. Run `make build && bash testdata/<tool>/smoke.sh` — confirms end-to-end behaviour that unit tests cannot catch (real process exit codes, stdout/stderr separation, pipeline composition)

The smoke script must be committed in the same commit as the tool.

Do not write any implementation code before the tests exist.
If no correctness spec was provided in the session prompt, ask for one before proceeding.

**Verification before declaring done** — `make check` passing is necessary but not
sufficient. Before saying the tool is complete, verify:
- The tool does what the spec says, not just what the tests check
- Edge cases from the spec that were not turned into tests still work manually
- The smoke test covers the "killer pipeline" example from the spec, not just isolated flags
- `vrk <tool> --help` output is accurate — flags listed match flags implemented

If any of these fail, the tool is not done. Do not commit.

## Required test coverage — every tool

Every tool's test file must cover all of these:

1. Happy path — main functionality with correct output
2. Exit code 0 on success
3. Exit code 1 on runtime error (invalid input, API error)
4. Exit code 2 on usage error (unknown flag, missing required input)
5. `--help` → exit 0, stdout contains tool name
6. Interactive TTY → exit 2
7. Interactive TTY + `--json` → exit 2, error JSON on stdout, stderr empty
8. `--json` active + I/O error → error JSON on stdout, stderr empty, exit 1
9. Empty stdin → exit 0 with no output (for most tools)
10. Property test — invariant that holds for any valid input (e.g. every JSONL record has non-empty required fields, `line >= 1`)

## Common root causes (check these first when debugging)
- Exit code wrong: the error path calls `Die()` inside `Run()` instead of `return shared.Errorf()`
- Stdin not read: positional arg path works, pipe path doesn't — `ReadInput` not called for both
- Test passes but smoke fails: test mocks something `Run()` does, real binary does not
- Flaky test: timing or randomness not seeded — add `--seed` or `--at` to make deterministic

## Testing approach

- Test the contract, not the implementation.
- **Check return value, not `os.Exit`.** Because `Run() int` returns the exit code, tests simply call `Run()` and check the int. No panic/recover, no `exitSentinel`. If you find yourself intercepting `os.Exit` in a test, `Run()` is calling `os.Exit` internally — that is a bug, fix it.
- **Standard test helper**: each tool has a `run<Tool>` function that replaces OS globals, calls `Run()`, and captures stdout/stderr. See `cmd/links/links_test.go` for the canonical pattern.
- **TTY simulation**: `isTerminal = func(int) bool { return true }` — not a real pipe redirect.
- **I/O error injection**: `readAll = func(r io.Reader) ([]byte, error) { return nil, errors.New("simulated") }`.
- Golden files in `testdata/<tool>/` for deterministic tools.
- Exit code tests are highest priority — they must never regress.
- For `epoch` tests, always pass `--at 1740009600` to make relative times deterministic. Use `1740009600` (`2025-02-20T00:00:00Z`) as the anchor — not `1740000000` (that is `2025-02-19T21:20:00Z`, not a clean boundary).
- Fuzz targets required for: `jwt`, `epoch`, `tok`, `sse`. Contract: never panic, never hang, exit within 1 second.
- Integration tests tagged `//go:build integration` — excluded from default `go test ./...`.
- Property tests required for every tool — at least one invariant that must hold for any input, not just the example cases.

## Reference tool — `links`

`cmd/links/` is the most recently built tool and shows every pattern in use:
- `var isTerminal` and `var readAll` package-level vars for test injection
- `fs.SetOutput(io.Discard)` + `errors.Is(err, pflag.ErrHelp)` in flag parsing
- TTY guard with dual error path (plain stderr vs JSON to stdout)
- Two-pass parsing: pass 1 collects `[label]: url` Markdown ref definitions, pass 2 emits links
- `--bare` and `--json` flags including the metadata trailer `{"_vrk":"links","count":N}`
- 26 unit tests covering all 10 required coverage items
- 27 smoke assertions covering all formats end-to-end

When in doubt about how to implement something, read `cmd/links/links.go` and `cmd/links/links_test.go` first.

## See also

- `docs/flag-conventions.md` — full flag spec
- `docs/architecture.md` — why decisions were made; read before changing anything fundamental
- `CONTRIBUTING.md` — human-readable contributing guide
- `internal/shared/` — shared contract helpers, use these, don't reinvent
- `cmd/links/` — canonical reference implementation for all patterns
