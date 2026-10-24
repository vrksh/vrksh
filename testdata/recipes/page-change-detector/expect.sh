#!/usr/bin/env bash
# First run saves a baseline. A second run on the same page says unchanged.
# Changing the page flips it back to changed.
trap 'kill "$(cat server.pid)" 2>/dev/null' EXIT
set -e
check() {
  NEW=$(vrk grab --text --allow-internal http://localhost:8099/pricing.html | vrk digest --bare)
  OLD=$(vrk kv get --ns pages pricing 2>/dev/null || true)
  if [ "$NEW" = "$OLD" ]; then echo unchanged; else echo changed; vrk kv set --ns pages pricing "$NEW"; fi
}
grep -qx 'changed' "$OUT"
[ "$(check)" = "unchanged" ]
cp pricing-v2.html site/pricing.html
[ "$(check)" = "changed" ]
