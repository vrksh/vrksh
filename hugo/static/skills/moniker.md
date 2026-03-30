# moniker - Memorable name generator - run IDs, job labels, temp dirs

When to use: generate memorable adjective-noun identifiers for run IDs, batch labels, or temp directories. Use --seed for reproducible names.
Composes with: kv, emit

| Flag          | Short | Type   | Description                                        |
|---------------|-------|--------|----------------------------------------------------|
| `--count`     | `-n`  | int    | Number of names (default: 1)                       |
| `--separator` |       | string | Word separator (default: `-`)                      |
| `--words`     |       | int    | Words per name, minimum 2 (default: 2)             |
| `--seed`      |       | int    | Fix random seed for deterministic output           |
| `--json`      | `-j`  | bool   | Emit `{"name":"...","words":["w1","w2"]}` per name |
| `--quiet`     | `-q`  | bool   | Suppress stderr                                    |

Exit 0: success
Exit 1: count exceeds available unique combinations
Exit 2: --count 0, --words < 2, unknown flag

Example:

    RUN_ID=$(vrk moniker --seed 42)
    vrk kv set "run:$RUN_ID" "active"

Anti-pattern:
- Don't use moniker for security-sensitive identifiers. The wordlist is small and names are guessable. Use vrk uuid for cryptographic randomness.
