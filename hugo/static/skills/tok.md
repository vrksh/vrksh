# tok - Token Counter - cl100k_base, --check gate, --json

When to use: measure token count or gate a pipeline to prevent silent truncation. Without --check: prints the count. With --check N: passes input through if within limit, exits 1 if over.
Composes with: grab, chunk, prompt, assert

| Flag      | Short | Type   | Description                                           |
|-----------|-------|--------|-------------------------------------------------------|
| `--check` |       | int    | Pass input through if within N tokens, exit 1 if over |
| `--model` | `-m`  | string | Tokenizer model label (default: cl100k_base)          |
| `--json`  | `-j`  | bool   | Emit `{"tokens":N,"model":"cl100k_base"}`             |
| `--quiet` | `-q`  | bool   | Suppress stderr                                       |

Exit 0: success, within limit
Exit 1: over limit or I/O error
Exit 2: unknown flag, interactive terminal with no input

Example:

    cat prompt.txt | vrk tok --check 4000 | vrk prompt --system "summarise"

Anti-pattern:
- Don't set --check to the model's maximum context limit. Leave room for the system prompt and response. If the model is 8,192 tokens and your system prompt is 1,000, set --check to 7,000 or less.
- Don't skip tok before prompt on untrusted input. A 200k-token page will blow the context window, waste API spend, and produce silently truncated output.
