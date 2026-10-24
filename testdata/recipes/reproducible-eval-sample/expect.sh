#!/usr/bin/env bash
# 50 lines out, all from the input, and the same seed gives the same sample.
set -e
[ "$(wc -l < "$OUT" | tr -d ' ')" = "50" ]
[ -z "$(grep -vxFf prompts.jsonl "$OUT")" ]
vrk sip --count 50 --seed 7 < prompts.jsonl | diff -q - "$OUT" >/dev/null
[ "$(vrk sip --count 50 --seed 8 < prompts.jsonl)" != "$(cat "$OUT")" ]
