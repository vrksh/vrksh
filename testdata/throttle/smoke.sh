#!/usr/bin/env bash
# Smoke tests for vrk throttle — run against the built binary.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/throttle/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/throttle/smoke.sh   # explicit binary path
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

assert_line_count() {
    local label="$1" got="$2" want="$3"
    local count
    if [[ -z "$got" ]]; then
        count=0
    else
        # printf '%s\n' restores the trailing newline stripped by $() so that
        # wc -l counts all lines including the last one correctly.
        count=$(printf '%s\n' "$got" | wc -l | tr -d ' ')
    fi
    if [[ "$count" == "$want" ]]; then
        printf 'PASS: %s (lines=%s)\n' "$label" "$count"
        PASS=$((PASS + 1))
    else
        printf 'FAIL: %s (lines=%s, want %s)\n  got: %s\n' "$label" "$count" "$want" "$(printf '%q' "$got")"
        FAIL=$((FAIL + 1))
    fi
}

# --- Basic pass-through ---

got=$(printf 'a\nb\nc\n' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit "basic_exit_0"       "$exit_code" "0"
assert_line_count "basic_3_lines" "$got" "3"
assert_eq "basic_line1" "$(printf '%s' "$got" | head -1)" "a"
assert_eq "basic_line3" "$(printf '%s' "$got" | tail -1)" "c"

# --- Content unchanged ---

got=$(echo 'hello' | "$VRK" throttle --rate 100/s)
assert_eq "content_unchanged" "$got" "hello"

# --- Empty stdin exits 0 with no output ---

got=$(printf '' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit "empty_stdin_exit"   "$exit_code" "0"
assert_eq   "empty_stdin_output" "$got" ""

# --- Empty line (echo '') exits 0 with no output ---

got=$(echo '' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit "empty_line_exit"   "$exit_code" "0"
assert_eq   "empty_line_output" "$got" ""

# --- Whitespace-only line is content, passes through ---

got=$(printf '   \n' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit "whitespace_line_exit"    "$exit_code" "0"
assert_eq   "whitespace_line_content" "$got" "   "

# --- Missing --rate exits 2 ---

set +e
"$VRK" throttle < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "missing_rate_exit_2" "$exit_code" "2"

# --- --rate 0/s exits 2 ---

set +e
"$VRK" throttle --rate 0/s < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "rate_zero_exit_2" "$exit_code" "2"

# --- --rate abc exits 2 ---

set +e
"$VRK" throttle --rate abc < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "rate_invalid_format_exit_2" "$exit_code" "2"

# --- --rate 0.5/s exits 2 (decimal N rejected) ---

set +e
"$VRK" throttle --rate 0.5/s < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "rate_decimal_exit_2" "$exit_code" "2"

# --- --help exits 0 ---

"$VRK" throttle --help > /dev/null
exit_code=$?
assert_exit "help_exit_0" "$exit_code" "0"

# --- --json trailing metadata record ---

json_out=$(printf 'x\ny\n' | "$VRK" throttle --rate 100/s --json)
exit_code=$?
assert_exit "json_exit_0" "$exit_code" "0"
assert_line_count "json_line_count" "$json_out" "3"  # 2 data + 1 metadata

last_line=$(printf '%s' "$json_out" | tail -1)
assert_contains "json_vrk_field"   "$last_line" '"_vrk":"throttle"'
assert_contains "json_rate_field"  "$last_line" '"rate":"100/s"'
assert_contains "json_lines_field" "$last_line" '"lines":2'

# --- --json with empty stdin emits only metadata ---

json_out=$(printf '' | "$VRK" throttle --rate 10/s --json)
exit_code=$?
assert_exit "json_empty_stdin_exit" "$exit_code" "0"
assert_contains "json_empty_lines_0" "$json_out" '"lines":0'

# --- --tokens-field happy path ---

tf_input='{"prompt":"hi"}
{"prompt":"hello world"}'
got=$(printf '%s\n' "$tf_input" | "$VRK" throttle --rate 100/s --tokens-field prompt)
exit_code=$?
assert_exit "tokens_field_exit_0" "$exit_code" "0"
assert_line_count "tokens_field_2_lines" "$got" "2"
assert_contains "tokens_field_line1" "$got" '"prompt":"hi"'

# --- --tokens-field invalid JSON exits 1 ---

set +e
echo 'not json' | "$VRK" throttle --rate 10/s --tokens-field prompt > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "tokens_field_bad_json_exit_1" "$exit_code" "1"

# --- Pipeline composition ---

count=$(seq 10 | "$VRK" throttle --rate 100/s | wc -l | tr -d ' ')
assert_eq "pipeline_line_count" "$count" "10"

# --- Timing: seq 3 at 2/s takes >= 1s ---
# Allow up to 5s to avoid flakiness on slow CI.

start_ts=$(date +%s)
seq 3 | "$VRK" throttle --rate 2/s > /dev/null
end_ts=$(date +%s)
elapsed=$((end_ts - start_ts))

if [[ "$elapsed" -ge 1 && "$elapsed" -le 5 ]]; then
    printf 'PASS: timing_2s_rate (elapsed=%ds)\n' "$elapsed"
    PASS=$((PASS + 1))
else
    printf 'FAIL: timing_2s_rate (elapsed=%ds, want 1-5s)\n' "$elapsed"
    FAIL=$((FAIL + 1))
fi

# --- TTY guard note ---
# Running vrk throttle with no stdin in an interactive TTY exits 2. Automated
# smoke testing cannot simulate a real TTY; that path is verified by
# TestInteractiveTTY and TestInteractiveTTYWithJSON in throttle_test.go.

# --- summary ---

printf '\nResults: %d passed, %d failed\n' "$PASS" "$FAIL"
[[ $FAIL -eq 0 ]]
