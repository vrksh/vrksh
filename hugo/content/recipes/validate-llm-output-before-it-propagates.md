---
title: "Validate LLM output before it propagates"
meta_title: "Validate LLM output before it propagates - vrk pipeline recipe"
description: "A bad structured response exits 1 before it reaches downstream systems, so you catch schema drift at the source. Check the LLM response against a ..."
why: "A bad structured response exits 1 before it reaches downstream systems, so you catch schema drift at the source."
body: "Check the LLM response against a schema and exit 1 on a mismatch, so the next stage does not run."
slug: "validate-llm-output-before-it-propagates"
steps:
  - |-
    cat doc.txt \
      | vrk prompt --system 'Extract entities as JSON: {"entities":"array"}' \
      | vrk validate --schema '{"entities":"array"}' --strict \
      | vrk kv set entities
tags:
  - "prompt"
  - "validate"
  - "kv"
---

## The problem

You asked the LLM for structured JSON. It returned something that looks like JSON but has a missing field, an extra key, or a wrong type. Your downstream code parses it, hits an unexpected null, and fails three steps later with an error that traces back to bad LLM output.

Schema validation at the source catches this immediately. The pipeline stops at the point of failure, not somewhere downstream where the root cause is hidden.

## How the pipeline works

`vrk prompt` calls the LLM and prints the raw response. `vrk validate` checks the response against a flat schema. It checks top-level keys and their types, nothing deeper. If the response matches, the data passes through. If not, `--strict` makes `vrk validate` exit 1 with no output, so `vrk kv set` gets nothing to store, exits 2 with `kv set: no value`, and the key is never written. Without `--strict`, a bad line is only warned about on stderr.

A bad response never reaches storage, and no downstream code sees it.

## The schema

The schema is a flat object: each key is a required top-level field, each value is a type (`string`, `number`, `boolean`, `array` or `object`). Extra keys in the response are ignored. Inline JSON works, or point `--schema` at a file. The retry example below keeps it in `entities.json` to avoid shell quoting.

```json
{"entities":"array"}
```

vrk validate does not read JSON Schema (`type`, `properties`, `$schema`). If you pass one it exits 2 and tells you so. It does not look inside arrays, so item shapes are not checked.

## Combining with retries

LLMs sometimes produce valid JSON that doesn't match your schema. Combine with `vrk coax` to retry:

```bash
vrk coax --times 3 --backoff exp:1s --on 1 -- \
  bash -c 'set -o pipefail; cat doc.txt \
      | vrk prompt --system "Extract entities as JSON: {\"entities\":\"array\"}" \
      | vrk validate --schema entities.json --strict'
```

If the API call or the validation fails (exit 1), `vrk coax` retries the entire sub-pipeline up to 3 times with exponential backoff. `set -o pipefail` is what lets a failed API call reach coax: without it the pipeline exits with validate's status, and validate exits 0 on empty input.

