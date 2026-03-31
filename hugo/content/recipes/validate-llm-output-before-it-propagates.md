---
title: "Validate LLM output before it propagates"
meta_title: "Validate LLM output before it propagates - vrk pipeline recipe"
description: "Bad structured output exits 1 before reaching downstream systems - you catch schema drift at the source, not in production. Gate the pipeline on ..."
why: "Bad structured output exits 1 before reaching downstream systems - you catch schema drift at the source, not in production."
body: "Gate the pipeline on schema correctness. Exit 1 on mismatch stops the next stage from running."
slug: "validate-llm-output-before-it-propagates"
steps:
  - |-
    cat doc.txt \
      | vrk prompt --system "Extract entities as JSON" --json \
      | vrk validate --schema entities.json \
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

`vrk prompt` calls the LLM and gets a response. `vrk validate` checks the response against `entities.json` (a JSON Schema file). If it matches, the data passes through. If not, `vrk validate` exits 1 and `vrk kv set` never runs.

No bad data reaches storage. No downstream code processes an invalid response.

## The schema file

A simple JSON Schema that defines what you expect:

```json
{
  "type": "object",
  "required": ["entities"],
  "properties": {
    "entities": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name", "type"],
        "properties": {
          "name": { "type": "string" },
          "type": { "type": "string" }
        }
      }
    }
  }
}
```

## Combining with retries

LLMs sometimes produce valid JSON that doesn't match your schema. Combine with `vrk coax` to retry:

```bash
vrk coax --times 3 --backoff exp:1s --on 1 -- \
  sh -c 'cat doc.txt \
      | vrk prompt --system "Extract entities as JSON" \
      | vrk validate --schema entities.json'
```

If validation fails (exit 1), `vrk coax` retries the entire sub-pipeline up to 3 times with exponential backoff.

