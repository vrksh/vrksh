#!/usr/bin/env bash
# testdata/sse/smoke.sh
#
# End-to-end smoke tests for vrk sse.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/sse/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/sse/smoke.sh   # explicit binary path
#
# NOTE: TTY detection (exit 2 when run interactively with no pipe) cannot be
# automated here — /dev/null is not a character device. Verify manually:
# run `vrk sse` in a terminal with no pipe and confirm it exits 2.
#
# SSE streams are written by functions, not captured into variables, because
# command substitution strips trailing newlines — which would silently break
# SSE dispatch (dispatch happens only on the blank line separating blocks).
#
# Exit 0 if all pass. Exit 1 on first failure.

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

# ------------------------------------------------------------
# Helpers
# ------------------------------------------------------------

ok() {
  echo "  PASS  $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "  FAIL  $1"
  echo "        $2"
  FAIL=$((FAIL + 1))
}

assert_exit() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" -eq "$expected" ]; then
    ok "$desc (exit $expected)"
  else
    fail "$desc" "expected exit $expected, got exit $actual"
  fi
}

assert_stdout_equals() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" = "$expected" ]; then
    ok "$desc (stdout = '$expected')"
  else
    fail "$desc" "expected '$expected', got '$actual'"
  fi
}

assert_stdout_empty() {
  local desc=$1 actual=$2
  if [ -z "$actual" ]; then
    ok "$desc (stdout empty)"
  else
    fail "$desc" "expected empty stdout, got: $actual"
  fi
}

assert_stderr_empty() {
  local desc=$1 stderr=$2
  if [ -z "$stderr" ]; then
    ok "$desc (stderr empty)"
  else
    fail "$desc" "expected empty stderr, got: $stderr"
  fi
}

assert_stdout_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    ok "$desc (stdout contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
  fi
}

assert_stdout_not_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    fail "$desc" "stdout must NOT contain '$pattern' but did: $actual"
  else
    ok "$desc (stdout does not contain '$pattern')"
  fi
}

assert_line_count() {
  local desc=$1 expected=$2 actual=$3
  # Use echo so we always have at least one newline to count, then subtract 1
  # for the echo-added newline when actual is empty.
  local count
  if [ -z "$actual" ]; then
    count=0
  else
    count=$(echo "$actual" | wc -l | tr -d ' ')
  fi
  if [ "$count" -eq "$expected" ]; then
    ok "$desc ($expected lines)"
  else
    fail "$desc" "expected $expected lines, got $count. stdout: $actual"
  fi
}

assert_valid_jsonl() {
  local desc=$1 actual=$2
  if command -v jq > /dev/null 2>&1; then
    if echo "$actual" | jq -e . > /dev/null 2>&1; then
      ok "$desc (valid JSON)"
    else
      fail "$desc" "output is not valid JSON. got: $actual"
    fi
  else
    echo "  SKIP  $desc (jq not installed)"
    PASS=$((PASS + 1))
  fi
}

# run_sse streams an SSE input through vrk sse with optional flags.
# Usage: stream_fn | run_sse [flags...] — do not call directly; pipe into it.
# All tests use functions to produce SSE streams to avoid $() stripping \n\n.

echo "vrk sse — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# SSE stream fixtures (functions preserve trailing \n\n)
# ------------------------------------------------------------

stream_two_events() {
  printf 'data: {"text":"hello"}\n\ndata: {"text":"world"}\n\n'
}

stream_named_event() {
  printf 'event: content_block_delta\ndata: {"delta":{"text":"hi"}}\n\n'
}

stream_mixed_events() {
  printf 'event: ping\ndata: {"a":1}\n\nevent: delta\ndata: {"text":"hi"}\n\nevent: ping\ndata: {"b":2}\n\n'
}

stream_done_after_event() {
  printf 'data: {"text":"hello"}\n\ndata: [DONE]\n\n'
}

stream_anthropic() {
  printf 'event: message_start\ndata: {"type":"message_start"}\n\n'
  printf 'event: content_block_delta\ndata: {"delta":{"text":"Hello"}}\n\n'
  printf 'event: content_block_delta\ndata: {"delta":{"text":", world"}}\n\n'
  printf 'event: message_stop\ndata: {"type":"message_stop"}\n\n'
  printf 'data: [DONE]\n\n'
}

# ------------------------------------------------------------
# 1. Basic pipeline: two events → two JSONL records
# ------------------------------------------------------------
echo "--- basic pipeline ---"

stdout=$(stream_two_events | "$VRK" sse 2>/dev/null)
stderr=$(set +e; stream_two_events | "$VRK" sse 2>&1 >/dev/null; true)
exit_code=$(set +e; stream_two_events | "$VRK" sse >/dev/null 2>&1; echo $?)

assert_exit        "two events: exit 0"             0  "$exit_code"
assert_stderr_empty "two events: stderr empty"         "$stderr"
assert_line_count  "two events: two records"        2  "$stdout"
assert_stdout_contains "two events: event=message"  '"event":"message"'  "$stdout"
assert_stdout_contains "two events: data.text"      '"text":"hello"'     "$stdout"
assert_valid_jsonl "two events: each line valid JSON" "$(stream_two_events | "$VRK" sse 2>/dev/null)"

# ------------------------------------------------------------
# 2. Named event type
# ------------------------------------------------------------
echo ""
echo "--- named events ---"

stdout=$(stream_named_event | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; stream_named_event | "$VRK" sse >/dev/null 2>&1; echo $?)

assert_exit "named event: exit 0" 0 "$exit_code"
assert_line_count "named event: one record" 1 "$stdout"
assert_stdout_contains "named event: event name" '"content_block_delta"' "$stdout"
assert_valid_jsonl "named event: valid JSON" "$stdout"

# ------------------------------------------------------------
# 3. --event filter
# ------------------------------------------------------------
echo ""
echo "--- --event filter ---"

stdout=$(stream_mixed_events | "$VRK" sse --event delta 2>/dev/null)
exit_code=$(set +e; stream_mixed_events | "$VRK" sse --event delta >/dev/null 2>&1; echo $?)

assert_exit        "filter: exit 0"          0  "$exit_code"
assert_line_count  "filter: one record"      1  "$stdout"
assert_stdout_contains     "filter: delta passes"  '"delta"'  "$stdout"
assert_stdout_not_contains "filter: ping blocked"  '"ping"'   "$stdout"

# No matches → empty output, exit 0
stdout=$(stream_mixed_events | "$VRK" sse --event nonexistent 2>/dev/null)
exit_code=$(set +e; stream_mixed_events | "$VRK" sse --event nonexistent >/dev/null 2>&1; echo $?)
assert_exit        "filter no match: exit 0"        0  "$exit_code"
assert_stdout_empty "filter no match: stdout empty"    "$stdout"

# ------------------------------------------------------------
# 4. [DONE] termination
# ------------------------------------------------------------
echo ""
echo "--- [DONE] termination ---"

stdout=$(stream_done_after_event | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; stream_done_after_event | "$VRK" sse >/dev/null 2>&1; echo $?)

assert_exit        "done: exit 0"                          0  "$exit_code"
assert_line_count  "done: one record (before [DONE])"      1  "$stdout"
assert_stdout_not_contains "done: [DONE] not emitted"  'DONE'  "$stdout"

# Stream is only [DONE] → no output, exit 0
stdout=$(printf 'data: [DONE]\n\n' | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; printf 'data: [DONE]\n\n' | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit        "done only: exit 0"        0  "$exit_code"
assert_stdout_empty "done only: stdout empty"    "$stdout"

# ------------------------------------------------------------
# 5. --field extraction
# ------------------------------------------------------------
echo ""
echo "--- --field extraction ---"

# String value — two events, one value each
stream_two_delta() {
  printf 'event: delta\ndata: {"delta":{"text":"hi"}}\n\nevent: delta\ndata: {"delta":{"text":"there"}}\n\n'
}

stdout=$(stream_two_delta | "$VRK" sse --field data.delta.text 2>/dev/null)
exit_code=$(set +e; stream_two_delta | "$VRK" sse --field data.delta.text >/dev/null 2>&1; echo $?)
assert_exit          "field string: exit 0"   0                      "$exit_code"
assert_line_count    "field string: two lines" 2                     "$stdout"
assert_stdout_equals "field string: values"   "$(printf 'hi\nthere')" "$stdout"

# Number value
stdout=$(printf 'data: {"count":42}\n\n' | "$VRK" sse --field data.count 2>/dev/null)
exit_code=$(set +e; printf 'data: {"count":42}\n\n' | "$VRK" sse --field data.count >/dev/null 2>&1; echo $?)
assert_exit        "field number: exit 0"   0    "$exit_code"
assert_stdout_equals "field number: value"  "42" "$stdout"

# Boolean value
stdout=$(printf 'data: {"ok":true}\n\n' | "$VRK" sse --field data.ok 2>/dev/null)
exit_code=$(set +e; printf 'data: {"ok":true}\n\n' | "$VRK" sse --field data.ok >/dev/null 2>&1; echo $?)
assert_exit        "field bool: exit 0"    0       "$exit_code"
assert_stdout_equals "field bool: value"   "true"  "$stdout"

# Path not found → empty output, exit 0
stdout=$(printf 'data: {"a":1}\n\n' | "$VRK" sse --field data.missing 2>/dev/null)
exit_code=$(set +e; printf 'data: {"a":1}\n\n' | "$VRK" sse --field data.missing >/dev/null 2>&1; echo $?)
assert_exit        "field missing: exit 0"        0  "$exit_code"
assert_stdout_empty "field missing: stdout empty"    "$stdout"

# --field combined with --event
stream_ping_delta() {
  printf 'event: ping\ndata: {"text":"ignored"}\n\nevent: delta\ndata: {"text":"hi"}\n\n'
}
stdout=$(stream_ping_delta | "$VRK" sse --event delta --field data.text 2>/dev/null)
exit_code=$(set +e; stream_ping_delta | "$VRK" sse --event delta --field data.text >/dev/null 2>&1; echo $?)
assert_exit        "field+event: exit 0"   0    "$exit_code"
assert_stdout_equals "field+event: value"  "hi" "$stdout"

# ------------------------------------------------------------
# 6. Skipping: comment lines, malformed lines, missing trailing blank line
# ------------------------------------------------------------
echo ""
echo "--- skipping ---"

stdout=$(printf ': comment\ngarbage\ndata: {"a":1}\n\n' | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; printf ': comment\ngarbage\ndata: {"a":1}\n\n' | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit       "skip garbage: exit 0"     0  "$exit_code"
assert_line_count "skip garbage: one record"  1  "$stdout"

# Empty stream → exit 0, no output
stdout=$(printf '' | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; printf '' | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit        "empty stream: exit 0"        0  "$exit_code"
assert_stdout_empty "empty stream: stdout empty"    "$stdout"

# EOF without trailing blank line → pending block dropped, exit 0
stdout=$(printf 'data: {"a":1}' | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; printf 'data: {"a":1}' | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit        "no trailing blank: exit 0"        0  "$exit_code"
assert_stdout_empty "no trailing blank: stdout empty"    "$stdout"

# ------------------------------------------------------------
# 7. Non-JSON data emitted as string
# ------------------------------------------------------------
echo ""
echo "--- non-JSON data ---"

stdout=$(printf 'data: not-json\n\n' | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; printf 'data: not-json\n\n' | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit "non-json: exit 0" 0 "$exit_code"
assert_stdout_contains "non-json: data is string" '"not-json"' "$stdout"
assert_valid_jsonl     "non-json: still valid JSON record" "$stdout"

# --field on non-JSON data → skip silently
stdout=$(printf 'data: not-json\n\n' | "$VRK" sse --field data.text 2>/dev/null)
exit_code=$(set +e; printf 'data: not-json\n\n' | "$VRK" sse --field data.text >/dev/null 2>&1; echo $?)
assert_exit        "non-json field: exit 0"        0  "$exit_code"
assert_stdout_empty "non-json field: stdout empty"    "$stdout"

# ------------------------------------------------------------
# 8. stdout/stderr separation
# ------------------------------------------------------------
echo ""
echo "--- stdout/stderr separation ---"

# Normal run: stderr must be empty
stderr=$(set +e; printf 'data: {"text":"hello"}\n\n' | "$VRK" sse 2>&1 >/dev/null; true)
assert_stderr_empty "normal run: stderr empty" "$stderr"

# Unknown flag: stdout must be empty, error goes to stderr only
stdout=$(set +e; "$VRK" sse --bogus < /dev/null 2>/dev/null; true)
assert_stdout_empty "unknown flag: stdout empty" "$stdout"

# ------------------------------------------------------------
# 9. --help
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" sse --help 2>/dev/null) || true
exit_code=$(set +e; "$VRK" sse --help >/dev/null 2>&1; echo $?)
assert_exit "help: exit 0" 0 "$exit_code"
assert_stdout_contains "help: mentions sse"     "sse"     "$stdout"
assert_stdout_contains "help: mentions --event" "event"   "$stdout"
assert_stdout_contains "help: mentions --field" "field"   "$stdout"

# ------------------------------------------------------------
# 10. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- usage errors ---"

exit_code=$(set +e; "$VRK" sse --bogus-flag < /dev/null >/dev/null 2>&1; echo $?)
assert_exit "unknown flag: exit 2" 2 "$exit_code"

stdout=$(set +e; "$VRK" sse --bogus-flag < /dev/null 2>/dev/null; true)
assert_stdout_empty "unknown flag: stdout empty" "$stdout"

# ------------------------------------------------------------
# 11. Anthropic-style killer pipeline
# ------------------------------------------------------------
echo ""
echo "--- killer pipeline ---"

# Full JSONL output: four data events before [DONE]
stdout=$(stream_anthropic | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; stream_anthropic | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit       "pipeline full: exit 0"          0  "$exit_code"
assert_line_count "pipeline full: four records"    4  "$stdout"

# Filter + extract text tokens: only content_block_delta events, only text field
stdout=$(stream_anthropic | "$VRK" sse --event content_block_delta --field data.delta.text 2>/dev/null)
exit_code=$(set +e; stream_anthropic | "$VRK" sse --event content_block_delta --field data.delta.text >/dev/null 2>&1; echo $?)
assert_exit          "pipeline tokens: exit 0"    0                         "$exit_code"
assert_line_count    "pipeline tokens: two lines" 2                         "$stdout"
assert_stdout_equals "pipeline tokens: values"    "$(printf 'Hello\n, world')" "$stdout"

# ------------------------------------------------------------
# 12. SSE spec: id/retry fields and multiple event: fields
# ------------------------------------------------------------
echo ""
echo "--- SSE spec edge cases ---"

# id: and retry: fields are silently skipped; data still emits correctly.
stdout=$(printf 'event: message\nid: abc123\nretry: 3000\ndata: {"text":"hello"}\n\n' | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; printf 'event: message\nid: abc123\nretry: 3000\ndata: {"text":"hello"}\n\n' | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit "id+retry skipped: exit 0" 0 "$exit_code"
assert_line_count "id+retry skipped: one record" 1 "$stdout"
assert_stdout_contains "id+retry skipped: event=message" '"event":"message"' "$stdout"
assert_stdout_contains "id+retry skipped: data preserved" '"text":"hello"' "$stdout"

# Last event: field in a block wins.
stdout=$(printf 'event: first\nevent: last\ndata: {"a":1}\n\n' | "$VRK" sse 2>/dev/null)
exit_code=$(set +e; printf 'event: first\nevent: last\ndata: {"a":1}\n\n' | "$VRK" sse >/dev/null 2>&1; echo $?)
assert_exit "last event wins: exit 0" 0 "$exit_code"
assert_stdout_contains     "last event wins: last emitted" '"last"'  "$stdout"
assert_stdout_not_contains "last event wins: first dropped" '"first"' "$stdout"

# ------------------------------------------------------------
# Summary
# ------------------------------------------------------------
echo ""
echo "---"
echo "Results: $PASS passed, $FAIL failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
