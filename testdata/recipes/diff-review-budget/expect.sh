#!/usr/bin/env bash
# The diff reached the model, and a tiny budget stops the pipeline before any request.
set -e
[ "$(wc -l < "$MOCK_LOG" | tr -d ' ')" = "1" ]
grep -q 'two changed' "$MOCK_LOG"
if git diff main | vrk tok --check 5 >/dev/null 2>&1; then exit 1; fi
[ "$(wc -l < "$MOCK_LOG" | tr -d ' ')" = "1" ]
