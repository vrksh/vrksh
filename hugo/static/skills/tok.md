# tok - Token counter - cl100k_base, --budget guard, --json

When to use: count tokens or enforce a budget gate before sending text to an LLM.
Composes with: grab, chunk, prompt, assert

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--budget` | | int | Exit 1 if token count exceeds N |
| `--model` | `-m` | string | Tokenizer model label (default: cl100k_base) |
| `--json` | `-j` | bool | Emit `{"tokens":N,"model":"cl100k_base"}` |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success, within budget
Exit 1: over budget or I/O error
Exit 2: unknown flag, interactive terminal with no input

Example:

    cat prompt.txt | vrk tok --budget 4000 && cat prompt.txt | vrk prompt --system "summarise"

Anti-pattern:
- Don't skip tok before prompt on untrusted input -- a 200k-token page will blow the context window and waste API spend.
