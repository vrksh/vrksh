#!/usr/bin/env bash
# testdata/links/smoke.sh
#
# End-to-end smoke tests for vrk links.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/links/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/links/smoke.sh   # explicit binary path

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
    fail "$desc" "expected '$expected', got '$actual'"
  fi
}

assert_stdout_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    ok "$desc (contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
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

echo "vrk links — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Markdown inline link
# ------------------------------------------------------------
echo "--- 1. markdown inline link ---"

got=$(echo 'See [Homebrew](https://brew.sh) for install.' | "$VRK" links)
assert_stdout_contains "markdown inline: text field"  '"text":"Homebrew"'        "$got"
assert_stdout_contains "markdown inline: url field"   '"url":"https://brew.sh"'  "$got"
assert_stdout_contains "markdown inline: line field"  '"line":1'                 "$got"

# ------------------------------------------------------------
# 2. HTML anchor
# ------------------------------------------------------------
echo ""
echo "--- 2. HTML anchor ---"

got=$(echo '<a href="https://example.com">Example</a>' | "$VRK" links)
assert_stdout_contains "html anchor: url"   '"url":"https://example.com"'  "$got"
assert_stdout_contains "html anchor: text"  '"text":"Example"'             "$got"

# ------------------------------------------------------------
# 3. Bare URL
# ------------------------------------------------------------
echo ""
echo "--- 3. bare URL ---"

got=$(echo 'Visit https://example.com for more.' | "$VRK" links)
assert_stdout_contains "bare URL: url"   '"url":"https://example.com"'   "$got"
assert_stdout_contains "bare URL: text"  '"text":"https://example.com"'  "$got"

# ------------------------------------------------------------
# 4. Markdown reference-style link
# ------------------------------------------------------------
echo ""
echo "--- 4. markdown reference-style link ---"

ref_input=$(printf '[Homebrew][brew]\n\n[brew]: https://brew.sh\n')
got=$(printf '%s' "$ref_input" | "$VRK" links)
assert_stdout_contains "ref link: url"   '"url":"https://brew.sh"'  "$got"
assert_stdout_contains "ref link: text"  '"text":"Homebrew"'        "$got"
assert_stdout_contains "ref link: line"  '"line":1'                 "$got"

# ------------------------------------------------------------
# 5. --bare flag
# ------------------------------------------------------------
echo ""
echo "--- 5. --bare flag ---"

got=$(echo 'See [Homebrew](https://brew.sh) for install.' | "$VRK" links --bare)
assert_stdout_equals "bare: URL only" "https://brew.sh" "$got"

# ------------------------------------------------------------
# 6. --json trailing metadata record
# ------------------------------------------------------------
echo ""
echo "--- 6. --json trailing metadata ---"

json_out=$(echo '[link](https://example.com)' | "$VRK" links --json)
last_line=$(printf '%s' "$json_out" | tail -1)
assert_stdout_contains "json trailing: _vrk field"  '"_vrk":"links"'  "$last_line"
assert_stdout_contains "json trailing: count field"  '"count":1'       "$last_line"

# ------------------------------------------------------------
# 7. --json with no links emits count:0
# ------------------------------------------------------------
echo ""
echo "--- 7. --json no links ---"

json_out=$(echo 'no links here' | "$VRK" links --json)
assert_stdout_contains "json no links: _vrk"    '"_vrk":"links"'  "$json_out"
assert_stdout_contains "json no links: count=0"  '"count":0'       "$json_out"

# ------------------------------------------------------------
# 8. Mixed-format input (markdown, HTML, bare URL)
# ------------------------------------------------------------
echo ""
echo "--- 8. mixed-format input ---"

mixed_input=$(printf '[MD](https://md.example.com)\n<a href="https://html.example.com">HTML</a>\nhttps://bare.example.com\n')
got=$(printf '%s' "$mixed_input" | "$VRK" links --bare)
assert_stdout_contains "mixed: markdown URL"  "https://md.example.com"    "$got"
assert_stdout_contains "mixed: HTML URL"      "https://html.example.com"  "$got"
assert_stdout_contains "mixed: bare URL"      "https://bare.example.com"  "$got"

# ------------------------------------------------------------
# 9. Empty stdin
# ------------------------------------------------------------
echo ""
echo "--- 9. empty stdin ---"

got=$(printf '' | "$VRK" links)
exit_code=$?
assert_exit        "empty stdin: exit 0"    0  "$exit_code"
assert_stdout_empty "empty stdin: no output"   "$got"

got=$(printf '' | "$VRK" links --bare)
exit_code=$?
assert_exit        "bare empty stdin: exit 0"    0  "$exit_code"
assert_stdout_empty "bare empty stdin: no output"   "$got"

got=$(printf '' | "$VRK" links --bare --json)
exit_code=$?
assert_exit        "bare+json empty stdin: exit 0"  0  "$exit_code"
assert_stdout_contains "bare+json empty stdin: _vrk"    '"_vrk":"links"'  "$got"
assert_stdout_contains "bare+json empty stdin: count=0"  '"count":0'       "$got"

# ------------------------------------------------------------
# 10. --help exits 0
# ------------------------------------------------------------
echo ""
echo "--- 10. --help ---"

"$VRK" links --help > /dev/null
exit_code=$?
assert_exit "--help: exit 0" 0 "$exit_code"

# ------------------------------------------------------------
# 11. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- 11. usage errors ---"

# TTY guard is covered by TestInteractiveTTYNoArg / TestInteractiveTTYWithJSONFlag
# in links_test.go — a real TTY cannot be simulated in automated smoke tests.

set +e
"$VRK" links --bogus < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "unknown flag: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 12. Positional argument
# ------------------------------------------------------------
echo ""
echo "--- 12. positional argument ---"

# Basic: positional arg exits 0 and contains the expected url field.
got=$("$VRK" links '[Homebrew](https://brew.sh)')
exit_code=$?
assert_exit        "positional: exit 0"         0                      "$exit_code"
assert_stdout_contains "positional: url field"  '"url":"https://brew.sh"' "$got"
assert_stdout_contains "positional: text field" '"text":"Homebrew"'    "$got"

# Positional output == stdin output (byte-for-byte; no network involved).
got_stdin=$(echo '[Homebrew](https://brew.sh)' | "$VRK" links)
assert_stdout_equals "positional == stdin" "$got_stdin" "$got"

# --bare combined with positional arg.
got=$("$VRK" links '[Homebrew](https://brew.sh)' --bare)
assert_stdout_equals "positional --bare: URL only" "https://brew.sh" "$got"

# Two links in one positional string → 2 records.
got=$("$VRK" links '[A](https://a.example.com) [B](https://b.example.com)')
line_count=$(printf '%s\n' "$got" | wc -l | tr -d ' ')
if [ "$line_count" -eq 2 ]; then
  ok "positional two links: 2 records"
else
  fail "positional two links" "expected 2 records, got $line_count"
fi

# --json combined with positional arg: trailing metadata record present with count:1.
got=$("$VRK" links '[Homebrew](https://brew.sh)' --json)
last_line=$(printf '%s' "$got" | tail -1)
assert_stdout_contains "positional --json: _vrk field"  '"_vrk":"links"' "$last_line"
assert_stdout_contains "positional --json: count field" '"count":1'      "$last_line"

# ------------------------------------------------------------
# --quiet flag
# ------------------------------------------------------------
echo ""
echo "--- --quiet ---"

stdout=$(echo "[a](https://example.com)" | "$VRK" links --quiet 2>/dev/null)
stderr=$(echo "[a](https://example.com)" | "$VRK" links --quiet 2>&1 >/dev/null)
exit_code=0; echo "[a](https://example.com)" | "$VRK" links --quiet > /dev/null 2>&1 || exit_code=$?
assert_exit            "--quiet success: exit 0"              0               "$exit_code"
assert_stdout_contains "--quiet success: stdout has url"      "example.com"  "$stdout"
assert_stderr_empty    "--quiet success: no stderr"                           "$stderr"

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
