# plain - Markdown stripper - removes syntax, keeps prose

When to use: strip markdown formatting before sending to a model that expects plain text.
Composes with: grab, tok, prompt

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | `-j` | bool | JSON envelope: `{"text":"...","input_bytes":N,"output_bytes":M}` |

Exit 0: success (including empty input)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no input, unknown flag

Example:

    vrk grab https://example.com | vrk plain | vrk tok

Anti-pattern:
- Don't use plain on HTML-heavy input -- raw HTML tags are dropped silently. Use `grab --text` instead.
