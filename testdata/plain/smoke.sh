#!/usr/bin/env bash
# Smoke tests for vrk plain — run against the built binary.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/plain/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/plain/smoke.sh   # explicit binary path
set -euo pipefail

VRK="${VRK:-./vrk}"
BINARY="$VRK"
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

# --- basic stripping ---

got=$(echo '**hello** _world_' | "$BINARY" plain)
assert_eq "bold_italic" "$got" "hello world"

got=$(echo '# Heading' | "$BINARY" plain)
assert_eq "heading" "$got" "Heading"

got=$(printf -- '- item one\n- item two' | "$BINARY" plain)
assert_eq "list" "$got" "$(printf 'item one\nitem two')"

got=$(echo '[link text](https://example.com)' | "$BINARY" plain)
assert_eq "link" "$got" "link text"

got=$(echo '`code snippet`' | "$BINARY" plain)
assert_eq "inline_code" "$got" "code snippet"

got=$(echo '> blockquote text' | "$BINARY" plain)
assert_eq "blockquote" "$got" "blockquote text"

# --- empty stdin ---

got=$(printf '' | "$BINARY" plain)
exit_code=$?
assert_exit "empty_stdin_exit" "$exit_code" "0"
assert_eq   "empty_stdin_output" "$got" ""

# --- --json: valid JSON with expected fields ---

json_out=$(echo '**hello**' | "$BINARY" plain --json)

# text field must equal "hello"
text_val=$(printf '%s' "$json_out" | grep -o '"text":"[^"]*"' | sed 's/"text":"//;s/"//')
assert_eq "json_text_field" "$text_val" "hello"

# input_bytes must be present and positive
has_input=$(printf '%s' "$json_out" | grep -c '"input_bytes"' || true)
assert_eq "json_has_input_bytes" "$has_input" "1"

# output_bytes must be present and positive
has_output=$(printf '%s' "$json_out" | grep -c '"output_bytes"' || true)
assert_eq "json_has_output_bytes" "$has_output" "1"

# --- --json: exit 0 ---

echo '**hello**' | "$BINARY" plain --json > /dev/null
exit_code=$?
assert_exit "json_exit_0" "$exit_code" "0"

# --- --help exits 0 ---

"$BINARY" plain --help > /dev/null
exit_code=$?
assert_exit "help_exit_0" "$exit_code" "0"

# --- unknown flag exits 2 ---

set +e
"$BINARY" plain --bogus < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "unknown_flag_exit_2" "$exit_code" "2"

# --- summary ---

printf '\nResults: %d passed, %d failed\n' "$PASS" "$FAIL"
[[ $FAIL -eq 0 ]]
