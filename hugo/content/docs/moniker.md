---
title: "vrk moniker"
description: "Memorable name generator - run IDs, job labels, temp dirs"
tool: moniker
group: utilities
mcp_callable: true
noindex: false
---

## The problem

Your pipeline needs a name for a run. UUIDs are unique but nobody can say "run 3f7a2b1c-9e2a-4d1b-8c7f-1a2b3c4d5e6f" in a standup. Sequential integers require state. Timestamps collide under parallel execution. You want something human-readable, pronounceable, and collision-resistant enough for a job queue or a temporary directory - like Docker's container names, but available in your pipeline without a daemon running.

`moniker` generates memorable adjective-noun names from a curated word list. Two-word names from a pool of a few thousand words give you millions of combinations. Add a third word and collisions become practically impossible for any workload you'd run on a single machine.

## The fix

```bash
vrk moniker
```

<!-- output: verify against binary -->

```
swift-falcon
```

## Walkthrough

### Generate one name

```bash
vrk moniker
```

<!-- output: verify against binary -->

No stdin required. `moniker` is purely generative - it doesn't read from stdin at all.

### Generate multiple names

```bash
vrk moniker --count 5
```

<!-- output: verify against binary -->

One name per line. Names within a single invocation are unique - no two lines will be the same. Uniqueness is guaranteed within a run using a Fisher-Yates shuffle of the word pool; it's not just random sampling.

### Deterministic output with --seed

```bash
vrk moniker --seed 42
vrk moniker --seed 42
```

<!-- output: verify against binary -->

Both invocations produce the same name. `--seed 0` is valid and deterministic - it's not "unset". If you need reproducible pipeline runs for debugging, seed with a fixed value or with a run number from your CI environment.

### More words for lower collision probability

```bash
vrk moniker --words 3 --count 3
```

<!-- output: verify against binary -->

The minimum is 2 words. Adding a third word multiplies the namespace significantly. Useful when you're generating names at high concurrency and need stronger collision avoidance.

### Custom separator

```bash
vrk moniker --separator _
vrk moniker --separator ""
```

<!-- output: verify against binary -->

Underscores make the name valid as a Python identifier or shell variable name. An empty separator produces a single concatenated word.

### JSON output

```bash
vrk moniker --count 2 --json
```

<!-- output: verify against binary -->

```json
{"name":"swift-falcon","words":["swift","falcon"]}
{"name":"quiet-river","words":["quiet","river"]}
```

The `words` array lets you extract components if you need them separately - for example, using the adjective as a namespace prefix and the noun as a key.

## Pipeline example

Name a pipeline run and store it for later reference:

```bash
vrk moniker --seed "$CI_BUILD_NUMBER" \
  | vrk kv set --ns jobs current_run
```

Or create a uniquely-named temporary directory for a job:

```bash
DIR="/tmp/$(vrk moniker)"
mkdir "$DIR"
# ... do work in $DIR ...
```

Generate a batch of names for pre-allocating job slots in a queue:

```bash
vrk moniker --count 20 --json \
  | jq -r '.name' \
  | while read name; do
      vrk kv set --ns queue "$name" "pending"
    done
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--count` | `-n` | int | `1` | Number of names to generate |
| `--separator` | | string | `"-"` | Word separator |
| `--words` | | int | `2` | Words per name (minimum 2) |
| `--seed` | | int64 | `0` | Random seed for deterministic output (0 is valid) |
| `--json` | `-j` | bool | `false` | Emit JSON per name: `{name, words}` |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (word pool exhausted for requested count) |
| 2 | Usage error - `--count` less than 1, `--words` less than 2 |
