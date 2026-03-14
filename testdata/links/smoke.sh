#!/usr/bin/env bash
# Smoke tests for vrk links — run against the built binary.
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

assert_eq() {
    local label="$1" got="$2" want="$3"
    if [[ "$got" == "$want" ]]; then
        printf 'PASS: %s\n' "$label"
        PASS=$((PASS + 1))
    else
        printf 'FAIL: %s\n  got:  %s\n  want: %s\n' "$label" "$(printf '%q' "$got")" "$(printf '%q' "$want")"
        FAIL=$((FAIL + 1))
    fi
}

assert_exit() {
    local label="$1" got="$2" want="$3"
    if [[ "$got" == "$want" ]]; then
        printf 'PASS: %s (exit %s)\n' "$label" "$got"
        PASS=$((PASS + 1))
    else
        printf 'FAIL: %s (exit %s, want %s)\n' "$label" "$got" "$want"
        FAIL=$((FAIL + 1))
    fi
}

assert_contains() {
    local label="$1" haystack="$2" needle="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        printf 'PASS: %s\n' "$label"
        PASS=$((PASS + 1))
    else
        printf 'FAIL: %s\n  output does not contain: %s\n  got: %s\n' "$label" "$(printf '%q' "$needle")" "$(printf '%q' "$haystack")"
        FAIL=$((FAIL + 1))
    fi
}

# --- Markdown inline link ---

got=$(echo 'See [Homebrew](https://brew.sh) for install.' | "$VRK" links)
assert_contains "markdown_inline_text"   "$got" '"text":"Homebrew"'
assert_contains "markdown_inline_url"    "$got" '"url":"https://brew.sh"'
assert_contains "markdown_inline_line"   "$got" '"line":1'

# --- HTML anchor ---

got=$(echo '<a href="https://example.com">Example</a>' | "$VRK" links)
assert_contains "html_anchor_url"  "$got" '"url":"https://example.com"'
assert_contains "html_anchor_text" "$got" '"text":"Example"'

# --- Bare URL ---

got=$(echo 'Visit https://example.com for more.' | "$VRK" links)
assert_contains "bare_url_url"  "$got" '"url":"https://example.com"'
assert_contains "bare_url_text" "$got" '"text":"https://example.com"'

# --- Markdown reference-style link ---

ref_input=$(printf '[Homebrew][brew]\n\n[brew]: https://brew.sh\n')
got=$(printf '%s' "$ref_input" | "$VRK" links)
assert_contains "ref_link_url"  "$got" '"url":"https://brew.sh"'
assert_contains "ref_link_text" "$got" '"text":"Homebrew"'
assert_contains "ref_link_line" "$got" '"line":1'

# --- --bare flag ---

got=$(echo 'See [Homebrew](https://brew.sh) for install.' | "$VRK" links --bare)
assert_eq "bare_flag_output" "$got" "https://brew.sh"

# --- --json trailing metadata record ---

json_out=$(echo '[link](https://example.com)' | "$VRK" links --json)
last_line=$(printf '%s' "$json_out" | tail -1)
assert_contains "json_trailing_vrk"   "$last_line" '"_vrk":"links"'
assert_contains "json_trailing_count" "$last_line" '"count":1'

# --- --json with no links emits count:0 ---

json_out=$(echo 'no links here' | "$VRK" links --json)
assert_contains "json_no_links_vrk"   "$json_out" '"_vrk":"links"'
assert_contains "json_no_links_count" "$json_out" '"count":0'

# --- mixed-format input (all three formats on same stream) ---

mixed_input=$(printf '[MD](https://md.example.com)\n<a href="https://html.example.com">HTML</a>\nhttps://bare.example.com\n')
got=$(printf '%s' "$mixed_input" | "$VRK" links --bare)
assert_contains "mixed_markdown" "$got" "https://md.example.com"
assert_contains "mixed_html"     "$got" "https://html.example.com"
assert_contains "mixed_bare"     "$got" "https://bare.example.com"

# --- empty stdin exits 0 with no output ---

got=$(printf '' | "$VRK" links)
exit_code=$?
assert_exit "empty_stdin_exit"   "$exit_code" "0"
assert_eq   "empty_stdin_output" "$got" ""

# --- --bare with empty stdin exits 0 with no output ---

got=$(printf '' | "$VRK" links --bare)
exit_code=$?
assert_exit "bare_empty_stdin_exit"   "$exit_code" "0"
assert_eq   "bare_empty_stdin_output" "$got" ""

# --- --bare --json with empty stdin exits 0, emits only metadata count:0 ---

got=$(printf '' | "$VRK" links --bare --json)
exit_code=$?
assert_exit "bare_json_empty_stdin_exit" "$exit_code" "0"
assert_contains "bare_json_empty_stdin_vrk"   "$got" '"_vrk":"links"'
assert_contains "bare_json_empty_stdin_count" "$got" '"count":0'

# --- no-stdin interactive guard (covered by unit tests) ---
# Running vrk links with no stdin in an interactive TTY exits 2, and when
# --json is also active the error record goes to stdout (not stderr).
# Automated smoke testing cannot simulate a real TTY; both paths are verified
# by TestInteractiveTTYNoArg and TestInteractiveTTYWithJSONFlag in links_test.go.

# --- --help exits 0 ---

"$VRK" links --help > /dev/null
exit_code=$?
assert_exit "help_exit_0" "$exit_code" "0"

# --- unknown flag exits 2 ---

set +e
"$VRK" links --bogus < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "unknown_flag_exit_2" "$exit_code" "2"

# --- summary ---

printf '\nResults: %d passed, %d failed\n' "$PASS" "$FAIL"
[[ $FAIL -eq 0 ]]
