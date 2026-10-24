#!/usr/bin/env bash
# --explain prints the request with secrets masked, and sends nothing.
set -e
grep -q 'migration 0042 failed' "$OUT"
grep -q 'What failed' "$OUT"
grep -q 'REDACTED' "$OUT"
! grep -q 'notreal0000' "$OUT"
[ ! -s "$MOCK_LOG" ]
