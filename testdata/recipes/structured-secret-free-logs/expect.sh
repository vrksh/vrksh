#!/usr/bin/env bash
# Every line is JSON with a level and tag=deploy, and no secret survives.
set -e
[ "$(wc -l < "$OUT" | tr -d ' ')" = "4" ]
! grep -q 'notreal0000' "$OUT"
grep -q 'REDACTED' "$OUT"
grep -q '"level":"warn"' "$OUT"
grep -q '"level":"error"' "$OUT"
[ "$(grep -c '"tag":"deploy"' "$OUT")" = "4" ]
