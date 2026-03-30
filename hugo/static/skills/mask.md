# mask - Secret redactor - entropy + pattern-based, streaming

When to use: redact secrets from text before sending to an LLM or writing to logs. Catches Bearer tokens, passwords, API keys, and high-entropy strings.
Composes with: prompt, emit, kv

| Flag        | Short | Type   | Description                                                       |
|-------------|-------|--------|-------------------------------------------------------------------|
| `--pattern` |       | string | Additional regex pattern (repeatable)                             |
| `--entropy` |       | float  | Shannon entropy threshold (default: 4.0; lower = more aggressive) |
| `--json`    | `-j`  | bool   | Append `{"_vrk":"mask","lines":N,"redacted":N,...}` after output  |

Exit 0: success (text passed through, redacted or not)
Exit 1: I/O error (with --json: error JSON to stdout)
Exit 2: interactive terminal, unknown flag, invalid --pattern regex

Example:

    vrk prompt --system "summarise" < doc.txt | vrk mask | vrk kv set summary

Anti-pattern:
- Don't mask after prompt. Mask before. The data goes to the API in the request, not the output. By the time you mask the response, the secrets are already at the provider.
- Don't rely solely on built-in patterns for internal secrets. Use --pattern with custom regexes for internal ticket IDs, employee numbers, or project codes.
