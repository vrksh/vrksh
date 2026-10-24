#!/usr/bin/env bash
# The biggest file comes first, and test files are left out.
set -e
head -1 "$OUT" | grep -q 'src/big.go'
grep -q 'src/small.go' "$OUT"
! grep -q '_test.go' "$OUT"
