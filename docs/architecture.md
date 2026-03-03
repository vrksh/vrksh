# vrksh Architecture

Decisions made before implementation. Each one has a reason. If you want to change one, change it here first and document why — do not just change the code.

---

## 1. Single Static Binary — Multicall Dispatch

**Decision:** One binary named `vrk`. All tools dispatched via `os.Args[0]` or `os.Args[1]`.

```
os.Args[0] == "jwt"      →  jwt tool  (via symlink: vrk jwt → vrk)
os.Args[1] == "jwt"      →  jwt tool  (via: vrk jwt)
```

**Why:** Distribution is the hardest problem for CLI tools. A single static binary means `brew install vrk`, `curl vrk.sh/install.sh | sh`, or `COPY vrk /usr/local/bin/` in a Dockerfile. No runtime, no dependencies, no version conflicts. The BusyBox pattern has worked since 1999.

**Consequence:** Every tool lives in `cmd/<tool>/main.go` and exposes a `func <Tool>Main()` that `main.go` calls. No tool has its own `main()`.

**Do not:** create separate binaries per tool. Do not let `goreleaser` produce `vrk-jwt`, `vrk-tok` etc. One binary only.

---

## 2. `pflag` for Flag Parsing — No Cobra, No stdlib `flag`

**Decision:** Use `github.com/spf13/pflag`. No cobra, no urfave/cli, no kong. Not stdlib `flag` either.

**Why not cobra:** adds ~500KB to the binary, introduces its own structural conventions that fight the multicall pattern, generates help text in a format that conflicts with vrksh's spec, and is designed for apps with deep subcommand trees — not a suite of simple filters.

**Why not stdlib `flag`:** no short flag support. `-j` for `--json`, `-q` for `--quiet`, `-n` for `--count` are standard Unix ergonomics. stdlib `flag` requires duplicate registration hacks to achieve this. `pflag` is a drop-in replacement that adds POSIX-compliant short flags cleanly.

**Why `pflag`:** drop-in replacement for `flag` (same API: `BoolVar`, `StringVar`, `Parse`), adds `-x` shorthands via `BoolVarP`/`StringVarP`, ~50KB binary increase (vs ~500KB for cobra), no structural opinions, used by kubectl and Hugo. Fixes the one real stdlib `flag` weakness without adding overhead.

```go
// in internal/shared/flags.go
import "github.com/spf13/pflag"

func (f *StandardFlags) Register(fs *pflag.FlagSet) {
    fs.BoolVarP(&f.JSON,    "json",    "j", false, "emit output as JSON")
    fs.BoolVarP(&f.Text,    "text",    "t", false, "emit plain text, no formatting")
    fs.BoolVarP(&f.Quiet,   "quiet",   "q", false, "suppress stderr")
    fs.BoolVarP(&f.Fail,    "fail",    "f", false, "exit 1 if condition not met")
    fs.StringVarP(&f.Schema,"schema",  "s", "",    "output must match this JSON schema")
    fs.StringVarP(&f.Model, "model",   "m", "",    "override model")
    fs.BoolVar(&f.Explain,  "explain",      false, "print action without executing (no shorthand — too dangerous)")
    fs.BoolVar(&f.DryRun,   "dry-run",      false, "preview mutations without executing (no shorthand)")
}
```

**Standard shorthands — consistent across every tool:**

| Long | Short | Rationale |
|------|-------|-----------|
| `--json` | `-j` | most common output flag |
| `--text` | `-t` | plain text output |
| `--quiet` | `-q` | Unix convention |
| `--fail` | `-f` | condition guard |
| `--schema` | `-s` | structured output |
| `--model` | `-m` | model override |
| `--count` | `-n` | numeric count (like `head -n`) |
| `--explain` | none | dangerous to fat-finger — forces intent |
| `--dry-run` | none | same reason |

**Subcommands (kv, checkpoint):** manual switch on `os.Args[1]`, then a sub-`pflag.FlagSet` per subcommand. pflag handles flag parsing; you handle routing. No framework needed.

```go
func kvMain() {
    if len(os.Args) < 2 {
        shared.DieUsage("usage: vrk kv <set|get|del|incr|list>")
    }
    switch os.Args[1] {
    case "set":  kvSet(os.Args[2:])
    case "get":  kvGet(os.Args[2:])
    case "del":  kvDel(os.Args[2:])
    case "incr": kvIncr(os.Args[2:])
    default:
        shared.DieUsage("unknown kv subcommand: %s", os.Args[1])
    }
}

func kvSet(args []string) {
    fs := pflag.NewFlagSet("kv-set", pflag.ExitOnError)
    ttl := fs.Duration("ttl", 0, "expiry duration")
    ns  := fs.String("ns", "default", "namespace")
    fs.Parse(args)
    // remaining positional args: fs.Args()
}
```

**Do not:** add cobra. If you find yourself wanting cobra, the tool is probably too complex for vrksh.

---

## 3. No CGO — `CGO_ENABLED=0` Always

**Decision:** `CGO_ENABLED=0` for all builds. Zero C dependencies.

**Why:** CGO breaks cross-compilation. `GOOS=linux GOARCH=arm64 go build` fails if any dependency uses CGO. The static binary promise depends entirely on this. The most common way this breaks is `mattn/go-sqlite3` — it is the famous SQLite library but it requires CGO. Do not use it.

**SQLite specifically:** use `modernc.org/sqlite` — pure Go, no CGO, ~10MB larger binary, slightly slower for large queries (irrelevant for kv workloads). The tradeoff is correct.

**Verify:** `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /dev/null .` must pass on every commit. This is in the CI matrix.

---

## 4. `//go:embed` for Static Data

**Decision:** All static data (tokenizer vocabulary tables, emoji CLDR data, name wordlists, MIME database) is embedded at compile time via `//go:embed`.

**Why:** Zero runtime dependencies, zero network calls, deterministic behaviour. The binary is self-contained. A tool that needs to download data on first run will fail in air-gapped environments, slow CI, and offline laptops.

**Current embeds:**
- `tok` — BPE vocabulary tables for cl100k_base (~2MB, tiktoken-go)
- `name` — adjective and noun wordlists (~50KB)
- `emoji` — Unicode CLDR emoji data (~200KB)
- `vrk --manifest` — JSON tool manifest (tiny, updated at build time)

**Consequence:** binary is larger than it would be otherwise. Acceptable — the alternative is network calls or missing functionality.

---

## 5. Input: Argument OR Stdin, Never Require Echo

**Decision:** Every tool accepts input as a positional argument or via stdin. Both produce identical output.

```bash
vrk epoch '+3d'          # argument form
echo '+3d' | vrk epoch   # stdin form — also works
printf '+3d' | vrk epoch # no trailing newline — also works
```

**Implementation:** `internal/shared/input.go:ReadInput()`. Every tool uses this. Never reimplement it.

**Trailing newline:** strip exactly one trailing `\n` from stdin — `strings.TrimRight(s, "\n")`. Not `strings.TrimSpace` — that strips meaningful whitespace from content.

**Why:** requiring `echo` is friction. Both forms appear in documentation. Both must work identically or users get confused.

---

## 6. Tokenizer: cl100k_base Approximation

**Decision:** `tok` and all token-counting tools use cl100k_base (tiktoken-go) as the tokenizer for all models.

**Accuracy:**
- GPT-4, GPT-4o: exact
- Claude (all): ~95% accurate — documented limitation
- Local models: rough approximation

**Why not Claude's tokenizer:** it is not publicly available as a standalone library. When Anthropic ships one, add it behind `--model claude-*` detection.

**User guidance:** set `--budget` at 90% of actual model limit. A 5% error on 100k tokens is 5,000 tokens — the 10% margin absorbs this.

**Do not:** silently claim exact counts for Claude. The README and `--help` text must document the approximation.

---

## 7. `ask` Defaults — Determinism First

**Decision:** `ask` defaults to temperature 0, uses `VRK_DEFAULT_MODEL` or built-in default, never infers model from context.

**Why:** `ask` is a pipeline tool, not a chat interface. Pipelines need reproducible outputs. Temperature 0 is the correct default for a tool that is supposed to behave like `jq` or `sed`.

**Temperature escalation on `--retry`:** retrying at temperature 0 produces identical wrong output every time. `--retry N` escalates: attempt 1 at 0.0, attempt 2 at 0.1, attempt 3 at 0.2. First attempt is maximally deterministic; subsequent attempts introduce variance to break failure loops.

**`--schema` is provider-aware:**
- OpenAI: `response_format.json_schema` with `strict: true` — API-level enforcement
- Claude: schema injected into system prompt + post-response validation — exits 1 on mismatch

**`request_hash` in `--json` output:** SHA256 of (model + temperature + prompt). Same inputs → same hash → usable as a cache key with `kv`.

---

## 8. SQLite via `modernc.org/sqlite`

**Decision:** `kv` and `checkpoint` use `modernc.org/sqlite`. WAL mode enabled by default.

**Why:** See §3. CGO-free, cross-compiles, static binary preserved. WAL mode handles concurrent readers without blocking. Concurrent writers serialise at file-lock level — acceptable for most agent workloads.

**Concurrency guidance:** in high-parallelism environments (10+ concurrent writers), namespace by agent ID: `vrk kv set --ns "agent-$ID" key val`. Reduces lock contention.

**Do not:** use `mattn/go-sqlite3`. It will appear in every Go SQLite tutorial and Stack Overflow answer. It requires CGO. It is wrong for this project.

**Do not:** store secrets in `kv`. The database is plaintext SQLite at `~/.vrk.db`. For credentials use env vars or the system keychain.

---

## 9. `checkpoint` — `io.TeeReader`, Not a Cache

**Decision:** `checkpoint` is a `tee`-style pipeline primitive. It passes stdin to stdout unchanged while writing a named snapshot as a side effect.

```bash
cat data.jsonl | vrk ask | vrk checkpoint step-2 | vrk validate
#                                    ↑
#              data flows through — pipeline never breaks
#              snapshot written to ~/.vrk/checkpoints/step-2
```

**Why not a cache:** caching is about avoiding recomputation. Checkpointing is about crash recovery and auditability. Different problems, different designs. `kv` is the explicit state store. `checkpoint` is the transparent snapshot.

**Implementation:** `io.TeeReader` reads from stdin and simultaneously writes to both stdout and the snapshot file. Resume is `cat ~/.vrk/checkpoints/<name>`. No daemon, no database — flat files.

**Scope constraint:** if `checkpoint` exceeds ~300 LOC, scope has drifted.

---

## 10. `internal/shared/` — Build First

**Decision:** `internal/shared/` is built before any tool. No tool reimplements anything in this package.

**Files and what they own:**

| File | Owns |
|------|------|
| `input.go` | `ReadInput`, `ReadInputOptional`, `ReadInputFile` |
| `exit.go` | `Die`, `DieUsage`, `Warn`, exit constants 0/1/2 |
| `json.go` | `PrintJSON`, `PrintJSONL`, `JSONOutput` |
| `flags.go` | `StandardFlags` — embed in every tool's flag struct |
| `kvpath.go` | `KVPath()` — respects `VRK_KV_PATH` |
| `flag_file.go` | `CheckpointDir()` — respects `VRK_CHECKPOINT_DIR` |
| `testutil/contract.go` | `RunContractTests` — imported by every tool's `_test.go` |

**Why:** inconsistency across tools is the main way a suite degrades over time. Exit code 2 on one tool, exit code 1 on another for the same class of error. `--json` producing slightly different envelopes. Trailing newlines handled differently. Centralising this makes consistency structurally enforced, not a convention people remember to follow.

---

## 11. Exit Codes — Strict, Never Change

**Decision:** three exit codes, fixed semantics, never change after a tool ships.

| Code | Meaning | Examples |
|------|---------|---------|
| 0 | Success | output produced, condition met |
| 1 | Runtime error | invalid JWT, over budget with `--fail`, schema mismatch, API error |
| 2 | Usage error | no stdin when required, unknown flag, ambiguous argument |

**Why strict:** exit codes are the API that shell scripts and agents use to make decisions. A change from exit 1 to exit 2 (or vice versa) silently breaks every pipeline that relies on that tool. Treat exit codes as a public API — version them accordingly.

**Do not:** return exit 1 for a usage error (missing flag, no input). That is exit 2.

---

## 12. Streaming — Explicit Flush, No Buffering

**Decision:** streaming tools (`sse`, `ask` with streaming) flush stdout explicitly after every write.

```go
w := bufio.NewWriter(os.Stdout)
defer w.Flush()
// after each record:
w.WriteString(line + "\n")
w.Flush()  // explicit — do not rely on defer alone
```

**Why:** Go buffers stdout by default. A streaming tool that doesn't flush produces output in bursts — arriving all at once when the buffer fills, not as records arrive. This defeats the purpose of streaming and breaks agent pipelines that react to individual events.

---

## 13. Release Pipeline — goreleaser

**Decision:** releases are automated via goreleaser triggered by git tags. No manual binary building.

**Produces per release:**
- `vrk-linux-amd64`
- `vrk-linux-arm64`
- `vrk-darwin-amd64`
- `vrk-darwin-arm64`
- `vrk-linux-amd64.deb`
- SHA256 checksums for all of the above

**Homebrew:** goreleaser opens an automated PR on `homebrew-vrksh` with the updated formula on each release. The tap is the only separate repo — Homebrew naming requires it.

**Install script:** `scripts/install.sh` detects platform, downloads the right binary, verifies SHA256, moves atomically. Idempotent — running it twice at the same version does nothing.

---

## 14. Website and Documentation — Hugo + Cloudflare Pages

**Decision:** Hugo static site in `hugo/` subdirectory of the monorepo. Deployed to Cloudflare Pages. MCP server on Fly.io at `mcp.vrk.sh`.

**Why Hugo:** written in Go — no Ruby, no Node required. Single binary install. Fast builds. Monorepo-friendly. Do not use Jekyll (Ruby dependency) or cookie (wrong structure for documentation).

**URL structure:**
```
vrk.sh/                 →  Cloudflare Pages (Hugo)
vrk.sh/install.sh       →  static file in hugo/static/
vrk.sh/llms.txt         →  static file in hugo/static/
mcp.vrk.sh              →  Fly.io Docker container
```

**Content consistency rule:** tool reference pages are generated from `cmd/<tool>/doc.go` at build time. The same source produces `--help` output and the website reference page. Never write docs twice — they will diverge.

**`docs.yml` GitHub Actions workflow:**
```yaml
name: docs
on:
  push:
    branches: [main]
    paths: ['hugo/**', 'cmd/**/doc.go']
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: true  # for Doks theme
      - name: Generate tool data
        run: go run ./cmd/docgen/ --output hugo/data/tools/
      - name: Build Hugo
        uses: peaceiris/actions-hugo@v3
        with:
          hugo-version: latest
      - run: hugo --minify --source hugo/
      - name: Deploy to Cloudflare Pages
        uses: cloudflare/pages-action@v1
        with:
          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          accountId: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
          projectName: vrksh
          directory: hugo/public/
```

**Do not:** create a separate repo for the website. Docs live in the monorepo, versioned alongside the code.

---

## 15. What Belongs in This Doc vs Elsewhere

| Question | Where it lives |
|----------|---------------|
| What flags exist and what do they mean? | `docs/flag-conventions.md` |
| How does each tool work? | `cmd/<tool>/doc.go` → auto-generated to `--help` and `vrk.sh/docs/tools/<tool>/` |
| What should a coding agent know? | `CLAUDE.md` |
| What should an AI agent know at runtime? | `AGENTS.md` + `vrk --manifest` |
| Why was X decided this way? | **This file** |
| What tools are planned but not built? | `vrk-research.md` |
| Website content | `hugo/content/` in monorepo |
| Install script | `hugo/static/install.sh` — served at `vrk.sh/install.sh` |
| LLM training corpus | `hugo/static/llms.txt` — served at `vrk.sh/llms.txt` |
| MCP server | `integrations/mcp/` — deployed to `mcp.vrk.sh` on Fly.io |

If you are adding a new architectural decision, add it here before writing code.

---

## 16. GitHub Organisation — `github.com/vrksh`

**Decision:** All repos live under the `vrksh` GitHub org, not a personal account.

**Repos:**
```
github.com/vrksh/vrksh             ← monorepo (main)
github.com/vrksh/homebrew-vrksh    ← Homebrew tap (separate repo — Homebrew requirement)
```

**Why org over personal account:**
- `github.com/vrksh/vrksh` reads as a project; `github.com/yourname/vrksh` reads as a side project
- Go import paths are permanent — `github.com/vrksh/vrksh/internal/shared` baked into every tool. Moving from personal to org after external links exist breaks every import path, every blog post, every `go get`
- Homebrew install reads better: `brew tap vrksh/vrksh` vs `brew tap yourname/vrksh` — your name should not be part of every user's install command
- Org membership shows on your personal profile — you get credit either way

**Go module path:** `github.com/vrksh/vrksh` — set in `go.mod` on day one, never change it.
