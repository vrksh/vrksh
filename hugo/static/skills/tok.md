# tok - Token Counter - cl100k_base, --check gate, --json

When to use: count tokens or gate a pipeline before sending text to an LLM. `--check N` passes input through if within limit.
Composes with: grab, chunk, prompt, assert

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--check` | | int | Pass input through if within N tokens, exit 1 if over |
| `--model` | `-m` | string | Tokenizer model label (default: cl100k_base) |
| `--json` | `-j` | bool | Emit `{"tokens":N,"model":"cl100k_base"}` |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success, within limit
Exit 1: over limit or I/O error
Exit 2: unknown flag, interactive terminal with no input

Example:

    cat prompt.txt | vrk tok --check 4000 | vrk prompt --system "summarise"

Anti-pattern:
- Don't skip tok before prompt on untrusted input -- a 200k-token page will blow the context window and waste API spend.
