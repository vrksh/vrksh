# Flag Conventions

Every vrk tool follows these conventions without exception. Consistency is a feature — a developer who learns a flag on one tool knows it on all of them.

When adding a new flag to any tool, check this file first. If the flag concept already exists here, use the canonical name. If it's genuinely new, add it here before implementing it.

---

## Standard Flags

### `--json`

**Meaning:** Emit output as a JSON object (single result) or JSONL stream (multiple results), instead of plain text.

**Applies to:** Every tool that produces output.

**Rules:**
- For tools where JSON/JSONL is already the default output (e.g. `sse`, `md`), `--json` is a no-op. Document this explicitly in `--help`.
- The JSON shape should include the primary data plus any metadata that is cheap to compute.
- Output must be valid JSON parseable by `jq`. Test: `vrk <tool> --json | jq . >/dev/null`

**Examples:**
```bash
echo 'hello world' | vrk tok              # 2
echo 'hello world' | vrk tok --json       # {"tokens": 2}

echo $TOKEN | vrk jwt --json              # {"sub": "user123", "exp": 1740000000, ...}

vrk kv get mykey --json                   # {"key": "mykey", "value": "...", "ns": "default"}

vrk fetch https://example.com --json      # {"url": "...", "title": "...", "content": "...",
                                           #  "fetched_at": 1740000000, "token_estimate": 180}
```

---

### `--text`

**Meaning:** Emit plain prose with no markup, formatting, or structure. The lowest-common-denominator output.

**Use when:** Downstream tools don't understand markdown, or the output needs to be clean for display.

**Examples:**
```bash
vrk fetch https://example.com             # markdown (default)
vrk fetch https://example.com --text      # plain prose, no ## headers or **bold**

cat response.md | vrk strip --text        # redundant here but valid
```

---

### `--fail`

**Meaning:** Exit 1 if a condition is not met. Makes measurement tools into guard tools.

**Rules:**
- Must be pipeline-safe: exit 1 from `--fail` must propagate correctly through `&&` chains.
- Stdout must still be populated on exit 1 from `--fail` (the measurement result is valid even if the condition failed). Only stderr gets the failure reason.
- The condition being checked must be documented in `--help`.

**Examples:**
```bash
cat prompt.txt | vrk tok --budget 4000 --fail    # exit 1 if over 4000 tokens
echo $TOKEN | vrk jwt --expired --fail           # exit 1 if token is expired
cat data.txt | vrk ask --schema s.json --fail    # exit 1 if output doesn't match schema
```

---

### `--schema <file>`

**Meaning:** The output must match the JSON schema in `<file>`, or exit 1.

**Distinct from `--json`:** `--json` is about format. `--schema` is about contract. A tool can produce `--json` output without enforcing any shape. `--schema` enforces a specific shape and exits 1 on mismatch.

**Provider-aware on `ask`:**
- OpenAI: uses `response_format.json_schema` with `strict: true` — API-level guarantee, no validation step needed.
- Anthropic/Claude: injects schema into system prompt, then validates response post-call. Exits 1 on mismatch. Use `--retry N` to retry on validation failure.

**Examples:**
```bash
cat data.txt | vrk ask --schema resume.json           # output must match schema
cat data.txt | vrk ask --schema resume.json --retry 3 # retry up to 3x if Claude misses
cat messy.txt | vrk cast --schema invoice.json        # extract + enforce structure
```

---

### `--explain`

**Meaning:** Print what the tool would do without actually doing it. Exits 0.

**Rules:**
- Must never make network calls.
- Must never write to any file or database.
- Must never read from stdin (or drain it silently).
- Output goes to stdout. Format: the equivalent shell command(s) that would achieve the same result.
- Useful for: debugging pipelines, auditing agent actions, generating reproducible examples.

**Examples:**
```bash
echo 'summarize this' | vrk ask --explain
# prints: curl https://api.anthropic.com/v1/messages -H '...' -d '{...}'

vrk kv set mykey myvalue --explain
# prints: sqlite3 ~/.vrk.db "INSERT OR REPLACE INTO kv (key, value) VALUES ('mykey', 'myvalue')"
```

---

### `--quiet`

**Meaning:** Suppress all stderr output. Stdout is unaffected.

**Use when:** The tool is used in a pipeline where stderr would pollute logs, or the caller manages its own error reporting.

**Rules:**
- Exit codes are unaffected. `--quiet` only suppresses messages, not behaviour.
- `--fail` still exits 1. The caller just won't see the reason.

---

### `--dry-run`

**Meaning:** Preview side effects without executing them. For tools that write files, modify state, or make mutations.

**Distinct from `--explain`:** `--explain` shows the equivalent shell command. `--dry-run` shows what state would change. Use `--dry-run` for stateful tools (`kv`, `vrk --bare`), `--explain` for network/API tools (`ask`, `fetch`).

**Examples:**
```bash
vrk --bare tok jwt --dry-run     # shows what symlinks would be created
vrk kv set mykey val --dry-run   # shows what would be written without writing
```

---

### `--model <name>`

**Meaning:** Override the default model. Applies to tools that make LLM calls (`ask`, `cast`, `slim`, `seek`).

**Format:** Provider-prefixed model string or bare model name. Resolution order:
1. `--model` flag
2. `$VRK_DEFAULT_MODEL` env var
3. `~/.config/vrk/config.toml` → `default_model`
4. Built-in default (`claude-sonnet-4-5`)

**Examples:**
```bash
cat prompt.txt | vrk ask --model gpt-4o
cat prompt.txt | vrk ask --model claude-opus-4-5
cat prompt.txt | vrk ask --model ollama/llama3   # local via --endpoint
```

---

### `--budget <N>`

**Meaning:** Token budget. Behaviour depends on tool:
- On `tok`: warn or fail if stdin exceeds N tokens.
- On `ask`: refuse to send if stdin exceeds N tokens (integrates `tok` internally).

Always used with `--fail` to make it a hard guard, or `--warn` to make it advisory.

```bash
cat prompt.txt | vrk tok --budget 4000 --warn    # warn to stderr, continue
cat prompt.txt | vrk tok --budget 4000 --fail    # exit 1 if over budget
cat prompt.txt | vrk ask --budget 4000 --fail    # refuse to call API if over budget
```

---

### `--retry <N>`

**Meaning:** Retry the operation up to N times on failure. Not the same as `coax` — `--retry` is a flag on a tool for a specific failure mode within that tool (e.g. schema validation failure on `ask`). `coax` wraps any external command.

```bash
cat data.txt | vrk ask --schema s.json --retry 3   # retry if Claude's output fails schema
```

---

## Flags That Do Not Exist in vrksh

These are intentionally absent. Do not add them.

| Flag | Why absent |
|------|-----------|
| `--config` | Config is optional and XDG-located. Never required, never flagged. |
| `--verbose` / `-v` | Debugging output goes to stderr unconditionally or not at all. |
| `--output <file>` | Unix pipes handle redirection. Tools write to stdout, callers redirect. |
| `--interactive` / `-i` | All tools are non-interactive by design. |
| `--format` | Use `--json` or `--text` instead. `--format` is too generic. |

---

## Exit Code Reference

| Code | Meaning | Examples |
|------|---------|---------|
| `0` | Success | Output produced, condition met |
| `1` | Runtime error | Invalid JWT, over budget with `--fail`, schema mismatch, API error |
| `2` | Usage error | No stdin when required, unknown flag, ambiguous argument, missing required flag |

Exit codes must never change for a released tool. They are part of the public contract.
