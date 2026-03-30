# plain - Markdown stripper - removes syntax, keeps prose

When to use: strip markdown formatting to reduce token count before LLM processing. Removes headers, links, fences, bullets - keeps only prose content.
Composes with: grab, tok, prompt

| Flag     | Short | Type | Description                                                      |
|----------|-------|------|------------------------------------------------------------------|
| `--json` | `-j`  | bool | JSON envelope: `{"text":"...","input_bytes":N,"output_bytes":M}` |

Exit 0: success (including empty input)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no input, unknown flag

Example:

    vrk grab https://example.com | vrk plain | vrk tok

Anti-pattern:
- Don't strip markdown if the formatting carries meaning for your task. If you're asking an LLM to "fix the markdown", it needs to see the markdown.
