#!/usr/bin/env bash
# The counter holds today's tokens, one request went out, and a spent budget fails the gate.
set -e
DAY=$(date +%F)
N=$(vrk tok < prompt.txt)
[ "$(vrk kv get "budget:$DAY")" = "$N" ]
[ "$(wc -l < "$MOCK_LOG" | tr -d ' ')" = "1" ]
echo 200000 | vrk assert '. <= 100000' 2>/dev/null && exit 1
exit 0
