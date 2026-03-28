# moniker - Memorable name generator - run IDs, job labels, temp dirs

When to use: generate human-readable adjective-noun names for run IDs or job labels.
Composes with: kv, emit

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--count` | `-n` | int | Number of names (default: 1) |
| `--separator` | | string | Word separator (default: `-`) |
| `--words` | | int | Words per name, minimum 2 (default: 2) |
| `--seed` | | int | Fix random seed for deterministic output |
| `--json` | `-j` | bool | Emit `{"name":"...","words":["w1","w2"]}` per name |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success
Exit 1: count exceeds available unique combinations
Exit 2: --count 0, --words < 2, unknown flag

Example:

    RUN_ID=$(vrk moniker --seed 42)
    vrk kv set "run:$RUN_ID" "active"

Anti-pattern:
- Don't pipe input to moniker -- stdin is silently ignored. It generates from embedded wordlists.
