#!/usr/bin/env bash
# testdata/mask/smoke.sh
#
# End-to-end smoke tests for vrk mask.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/mask/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/mask/smoke.sh   # explicit binary path
#
# NOTE: TTY detection (exit 2 when run interactively with no pipe) cannot be
# automated here — /dev/null is not a character device. Verify manually:
# run `vrk mask` in a terminal with no pipe and confirm it exits 2.
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
    ok "$desc"
  else
    fail "$desc" "expected $(printf '%q' "$expected"), got $(printf '%q' "$actual")"
  fi
}

assert_stdout_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -qF "$pattern"; then
    ok "$desc (contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
  fi
}

assert_stdout_not_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -qF "$pattern"; then
    fail "$desc" "stdout must NOT contain '$pattern' but did: $actual"
  else
    ok "$desc (does not contain '$pattern')"
  fi
}

assert_stderr_empty() {
  local desc=$1 actual=$2
  if [ -z "$actual" ]; then
    ok "$desc (stderr empty)"
  else
    fail "$desc" "expected empty stderr, got: $actual"
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

assert_line_count() {
  local desc=$1 expected=$2 actual=$3
  # Use echo so we always have a trailing newline for wc -l to count correctly;
  # guard the empty case explicitly so echo "" does not contribute a phantom line.
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

echo "vrk mask — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Built-in bearer pattern
# ------------------------------------------------------------
echo "--- built-in patterns ---"

stdout=$(echo 'Authorization: Bearer sk-ant-abc123def456' | "$VRK" mask 2>/dev/null)
stderr=$(set +e; echo 'Authorization: Bearer sk-ant-abc123def456' | "$VRK" mask 2>&1 >/dev/null; true)
exit_code=$(set +e; echo 'Authorization: Bearer sk-ant-abc123def456' | "$VRK" mask >/dev/null 2>&1; echo $?)

assert_exit "bearer: exit 0" 0 "$exit_code"
assert_stderr_empty "bearer: stderr empty" "$stderr"
assert_stdout_equals "bearer: redacted" "Authorization: Bearer [REDACTED]" "$stdout"
assert_stdout_not_contains "bearer: original gone" "sk-ant-abc123def456" "$stdout"

# password pattern
stdout=$(echo 'password=hunter2' | "$VRK" mask 2>/dev/null)
assert_stdout_equals "password: redacted" "password=[REDACTED]" "$stdout"
assert_stdout_not_contains "password: original gone" "hunter2" "$stdout"

# token pattern
stdout=$(echo 'token=supersecretvalue' | "$VRK" mask 2>/dev/null)
assert_stdout_equals "token: redacted" "token=[REDACTED]" "$stdout"

# secret pattern
stdout=$(echo 'secret=supersecretvalue' | "$VRK" mask 2>/dev/null)
assert_stdout_equals "secret: redacted" "secret=[REDACTED]" "$stdout"

# api_key pattern
stdout=$(echo 'api_key=supersecretvalue' | "$VRK" mask 2>/dev/null)
assert_stdout_equals "api_key: redacted" "api_key=[REDACTED]" "$stdout"

# ------------------------------------------------------------
# 2. Clean text passes through unchanged
# ------------------------------------------------------------
echo ""
echo "--- clean text ---"

stdout=$(echo 'no secrets here' | "$VRK" mask 2>/dev/null)
exit_code=$(set +e; echo 'no secrets here' | "$VRK" mask >/dev/null 2>&1; echo $?)
assert_exit "clean: exit 0" 0 "$exit_code"
assert_stdout_equals "clean: unchanged" "no secrets here" "$stdout"

# ------------------------------------------------------------
# 3. Custom --pattern flag
# ------------------------------------------------------------
echo ""
echo "--- custom --pattern ---"

stdout=$(echo 'key: sk-ant-AAAA' | "$VRK" mask --pattern 'sk-ant-[A-Za-z0-9]+' 2>/dev/null)
exit_code=$(set +e; echo 'key: sk-ant-AAAA' | "$VRK" mask --pattern 'sk-ant-[A-Za-z0-9]+' >/dev/null 2>&1; echo $?)
assert_exit "custom pattern: exit 0" 0 "$exit_code"
assert_stdout_equals "custom pattern: redacted" "key: [REDACTED]" "$stdout"

# ------------------------------------------------------------
# 4. --entropy flag
# ------------------------------------------------------------
echo ""
echo "--- --entropy flag ---"

# High threshold: short abc123 (below floor) not redacted
stdout=$(echo 'abc123' | "$VRK" mask --entropy 4.5 2>/dev/null)
exit_code=$(set +e; echo 'abc123' | "$VRK" mask --entropy 4.5 >/dev/null 2>&1; echo $?)
assert_exit "entropy high: exit 0" 0 "$exit_code"
assert_stdout_equals "entropy high: unchanged" "abc123" "$stdout"

# Low threshold: repetitive string redacted at 3.0
stdout=$(echo 'sk-ant-AAABBBCCC111222333444555' | "$VRK" mask --entropy 3.0 2>/dev/null)
exit_code=$(set +e; echo 'sk-ant-AAABBBCCC111222333444555' | "$VRK" mask --entropy 3.0 >/dev/null 2>&1; echo $?)
assert_exit "entropy low: exit 0" 0 "$exit_code"
assert_stdout_contains "entropy low: redacted" "[REDACTED]" "$stdout"

# ------------------------------------------------------------
# 5. Multi-line input
# ------------------------------------------------------------
echo ""
echo "--- multi-line ---"

stdout=$(printf 'line1\nBearer abc123xyz\nline3\n' | "$VRK" mask 2>/dev/null)
exit_code=$(set +e; printf 'line1\nBearer abc123xyz\nline3\n' | "$VRK" mask >/dev/null 2>&1; echo $?)
assert_exit "multiline: exit 0" 0 "$exit_code"
assert_line_count "multiline: three output lines" 3 "$stdout"
assert_stdout_contains "multiline: clean line1 present" "line1" "$stdout"
assert_stdout_contains "multiline: secret redacted" "Bearer [REDACTED]" "$stdout"
assert_stdout_contains "multiline: clean line3 present" "line3" "$stdout"
assert_stdout_not_contains "multiline: original token gone" "abc123xyz" "$stdout"

# ------------------------------------------------------------
# 6. --json metadata shape
# ------------------------------------------------------------
echo ""
echo "--- --json metadata ---"

stdout=$(echo 'token: sk-abc123XYZ' | "$VRK" mask --json 2>/dev/null)
exit_code=$(set +e; echo 'token: sk-abc123XYZ' | "$VRK" mask --json >/dev/null 2>&1; echo $?)
assert_exit "--json: exit 0" 0 "$exit_code"
assert_stdout_contains "--json: text output present" "[REDACTED]" "$stdout"
assert_stdout_contains "--json: _vrk field" '"_vrk":"mask"' "$stdout"
assert_stdout_contains "--json: lines field" '"lines"' "$stdout"
assert_stdout_contains "--json: redacted field" '"redacted"' "$stdout"
assert_stdout_contains "--json: patterns_matched field" '"patterns_matched"' "$stdout"

# --json on clean input: redacted=0, patterns_matched=[]
stdout=$(echo 'no secrets here' | "$VRK" mask --json 2>/dev/null)
assert_stdout_contains "--json clean: metadata present" '"_vrk":"mask"' "$stdout"
assert_stdout_contains "--json clean: redacted=0" '"redacted":0' "$stdout"

# ------------------------------------------------------------
# 7. Empty line
# ------------------------------------------------------------
echo ""
echo "--- empty line ---"

# echo '' outputs a newline; mask must echo it back (one empty line) and exit 0.
# Command substitution $(...) strips all trailing newlines so a file captures
# the real line count.
exit_code=$(set +e; echo '' | "$VRK" mask >/dev/null 2>&1; echo $?)
assert_exit "empty line: exit 0" 0 "$exit_code"
_tmpfile=$(mktemp)
echo '' | "$VRK" mask >"$_tmpfile" 2>/dev/null
_line_count=$(wc -l <"$_tmpfile" | tr -d ' ')
rm -f "$_tmpfile"
if [ "$_line_count" -eq 1 ]; then
  ok "empty line: one output line (1 lines)"
else
  fail "empty line: one output line" "expected 1 lines, got $_line_count"
fi

# ------------------------------------------------------------
# 8. stdout/stderr separation
# ------------------------------------------------------------
echo ""
echo "--- stdout/stderr separation ---"

stderr=$(set +e; echo 'Bearer abc123xyz' | "$VRK" mask 2>&1 >/dev/null; true)
assert_stderr_empty "normal run: stderr empty" "$stderr"

stdout=$(set +e; "$VRK" mask --bogus-flag < /dev/null 2>/dev/null; true)
assert_stdout_empty "unknown flag: stdout empty" "$stdout"

# ------------------------------------------------------------
# 9. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- usage errors ---"

exit_code=$(set +e; "$VRK" mask --bogus-flag < /dev/null >/dev/null 2>&1; echo $?)
assert_exit "unknown flag: exit 2" 2 "$exit_code"

exit_code=$(set +e; echo 'x' | "$VRK" mask --pattern '[bad' >/dev/null 2>&1; echo $?)
assert_exit "invalid pattern: exit 2" 2 "$exit_code"

stdout=$(set +e; echo 'x' | "$VRK" mask --pattern '[bad' 2>/dev/null; true)
assert_stdout_empty "invalid pattern: stdout empty" "$stdout"

# ------------------------------------------------------------
# 10. --help
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" mask --help 2>/dev/null) || true
exit_code=$(set +e; "$VRK" mask --help >/dev/null 2>&1; echo $?)
assert_exit "help: exit 0" 0 "$exit_code"
assert_stdout_contains "help: mentions mask"    "mask"    "$stdout"
assert_stdout_contains "help: mentions pattern" "pattern" "$stdout"
assert_stdout_contains "help: mentions entropy" "entropy" "$stdout"

# ------------------------------------------------------------
# 11. Killer pipeline: scrub logs before storing
# ------------------------------------------------------------
echo ""
echo "--- killer pipeline ---"

log=$(printf 'INFO: starting job\nDEBUG: Authorization: Bearer sk-ant-xyzABC123\nINFO: job complete\n')
stdout=$(printf '%s\n' "$log" | "$VRK" mask 2>/dev/null)
assert_line_count "pipeline: three output lines" 3 "$stdout"
assert_stdout_contains "pipeline: clean lines preserved" "INFO: starting job" "$stdout"
assert_stdout_not_contains "pipeline: secret gone" "sk-ant-xyzABC123" "$stdout"
assert_stdout_contains "pipeline: placeholder present" "[REDACTED]" "$stdout"

# ------------------------------------------------------------
# --quiet flag
# ------------------------------------------------------------
echo ""
echo "--- --quiet ---"

stdout=$(echo "token = abc123xyz" | "$VRK" mask --quiet 2>/dev/null)
stderr=$(echo "token = abc123xyz" | "$VRK" mask --quiet 2>&1 >/dev/null)
exit_code=0; echo "token = abc123xyz" | "$VRK" mask --quiet > /dev/null 2>&1 || exit_code=$?
assert_exit            "--quiet success: exit 0"                   0             "$exit_code"
assert_stdout_contains "--quiet success: stdout has [REDACTED]"    "[REDACTED]"  "$stdout"
assert_stderr_empty    "--quiet success: no stderr"                              "$stderr"

# ------------------------------------------------------------
# Corpus: every secret redacted, every decoy unchanged
# ------------------------------------------------------------
echo ""
echo "--- corpus ---"

DIR="$(cd "$(dirname "$0")" && pwd)"
miss=0
while IFS= read -r l; do
  printf '%s\n' "$l" | "$VRK" mask | grep -qF '[REDACTED]' || { miss=$((miss + 1)); echo "        not redacted: $l"; }
done < "$DIR/secrets.txt"
assert_exit "corpus: all secrets redacted" 0 "$miss"

decoys_out=$("$VRK" mask < "$DIR/decoys.txt")
assert_stdout_equals "corpus: decoys unchanged" "$(cat "$DIR/decoys.txt")" "$decoys_out"

# json.dumps-shaped records: quoted keys, hex tokens, short passwords.
stdout=$(printf '%s\n' '[{"id": 0, "password": "pw0", "token": "f9c0b7fb5a83541b7e4924b2d17670dbdf99c483"}, {"id": 1, "password": "pw1", "token": "813924b2fd8d01583142086d2d275718d5cc72e7"}]' | "$VRK" mask)
assert_stdout_equals "json.dumps records: values gone, JSON intact" '[{"id": 0, "password": "[REDACTED]", "token": "[REDACTED]"}, {"id": 1, "password": "[REDACTED]", "token": "[REDACTED]"}]' "$stdout"

stdout=$(echo 'key sk-proj-FAKE0000example0000notreal0000000, please' | "$VRK" mask)
assert_stdout_equals "punctuation kept after redaction" "key [REDACTED], please" "$stdout"

# Deploy log: URLs and image refs are context, the key is not.
stdout=$(printf 'GET https://payments.internal/v1/charges failed\npulling registry.example.com/api-gateway:2.31.4\nretry with key=sk-proj-FAKE0000example0000notreal0000000\n' | "$VRK" mask)
assert_stdout_contains     "deploy log: URL kept"       "https://payments.internal/v1/charges" "$stdout"
assert_stdout_contains     "deploy log: image ref kept" "registry.example.com/api-gateway:2.31.4" "$stdout"
assert_stdout_not_contains "deploy log: key gone"       "FAKE0000example0000notreal0000000" "$stdout"

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
