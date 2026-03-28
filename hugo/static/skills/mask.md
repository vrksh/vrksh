# mask - Secret redactor - entropy + pattern-based, streaming

When to use: scrub secrets from text before logging, storing, or sending downstream.
Composes with: prompt, emit, kv

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--pattern` | | string | Additional regex pattern (repeatable) |
| `--entropy` | | float | Shannon entropy threshold (default: 4.0; lower = more aggressive) |
| `--json` | `-j` | bool | Append `{"_vrk":"mask","lines":N,"redacted":N,...}` after output |

Exit 0: success (text passed through, redacted or not)
Exit 1: I/O error (with --json: error JSON to stdout)
Exit 2: interactive terminal, unknown flag, invalid --pattern regex

Example:

    vrk prompt "summarise" < doc.txt | vrk mask | vrk kv set summary

Anti-pattern:
- Don't rely on mask as a security boundary -- it is best-effort. UUIDs and SHA hashes trigger false positives; short passwords trigger false negatives.
