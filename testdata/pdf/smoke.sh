#!/usr/bin/env bash
# testdata/pdf/smoke.sh
#
# End-to-end smoke tests for vrk pdf.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/pdf/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/pdf/smoke.sh   # explicit binary path

set -euo pipefail

VRK="${VRK:-./vrk}"
FIX="$(dirname "$0")/fixtures"
PASS=0
FAIL=0

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

ok() {
  echo "  PASS  $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "  FAIL  $1"
  echo "        $2"
  FAIL=$((FAIL + 1))
}

# run <stdin-file> <args...>: sets $stdout, $stderr, $code.
run() {
  local in=$1
  shift
  code=0
  "$VRK" pdf "$@" < "$in" > "$TMP/out" 2> "$TMP/err" || code=$?
  stdout="$(cat "$TMP/out")"
  stderr="$(cat "$TMP/err")"
}

assert_exit() {
  if [ "$code" -eq "$2" ]; then ok "$1 (exit $2)"; else fail "$1" "expected exit $2, got $code. stderr: $stderr"; fi
}

assert_eq() {
  if [ "$3" = "$2" ]; then ok "$1"; else fail "$1" "expected '$2', got '$3'"; fi
}

assert_contains() {
  if printf '%s' "$3" | grep -qF -- "$2"; then ok "$1 (contains '$2')"; else fail "$1" "missing '$2' in: $3"; fi
}

echo "vrk pdf smoke tests"
echo ""

# --- happy paths ---

run /dev/null "$FIX/text.pdf"
assert_exit "file arg" 0
assert_contains "file arg: page 1 text" "Quarterly report, page one." "$stdout"
assert_eq "file arg: stderr empty" "" "$stderr"
assert_eq "file arg: two form feeds for three pages" "2" "$(tr -cd '\f' < "$TMP/out" | wc -c | tr -d ' ')"
assert_eq "file arg: ends in newline" "0a" "$(tail -c 1 "$TMP/out" | od -An -tx1 | tr -d ' ')"
cp "$TMP/out" "$TMP/from-file"

run "$FIX/text.pdf"
assert_exit "stdin" 0
if cmp -s "$TMP/out" "$TMP/from-file"; then ok "stdin output matches file arg"; else fail "stdin output matches file arg" "outputs differ"; fi

run /dev/null --pages 2 "$FIX/text.pdf"
assert_exit "--pages 2" 0
assert_eq "--pages 2: only page 2" "Revenue grew in every region." "$stdout"

run /dev/null -p 2- "$FIX/text.pdf"
assert_exit "-p 2-" 0
assert_eq "-p 2-: one form feed" "1" "$(tr -cd '\f' < "$TMP/out" | wc -c | tr -d ' ')"

run /dev/null --json "$FIX/text.pdf"
assert_exit "--json" 0
assert_eq "--json: page numbers" "1 2 3" "$(jq -r 'select(.page) | .page' "$TMP/out" | tr '\n' ' ' | sed 's/ $//')"
assert_eq "--json: trailer" '{"_vrk":"pdf","count":3}' "$(tail -1 "$TMP/out")"

run /dev/null --json --pages 2-3 "$FIX/text.pdf"
assert_eq "--json --pages 2-3: real page numbers" "2 3" "$(jq -r 'select(.page) | .page' "$TMP/out" | tr '\n' ' ' | sed 's/ $//')"

run /dev/null --text --pages 3,1 "$FIX/text.pdf"
assert_exit "--pages 3,1" 0
assert_eq "--pages 3,1: given order" "Outlook for next year.|Quarterly report, page one." "$(tr '\f\n' '|' < "$TMP/out" | sed 's/||/|/; s/|$//')"

run /dev/null "$FIX/text.pdf"
assert_eq "markdown: blank line before each form feed" "2" "$(grep -c '^\f$' "$TMP/out" | tr -d ' ')"

run /dev/null -t "$FIX/text.pdf"
assert_exit "-t" 0
assert_eq "-t: form feed straight after the page text" "Quarterly report, page one." "$(head -1 "$TMP/out")"
assert_eq "-t: no blank lines" "0" "$(grep -c '^$' "$TMP/out" | tr -d ' ')"

run /dev/null "$FIX/kerned.pdf"
assert_exit "kerned (TJ spacing) PDF" 0
assert_contains "kerned: spaces rebuilt from TJ offsets" "Attention Is All" "$stdout"

run /dev/null "$FIX/quartz.pdf"
assert_exit "quartz (cupsfilter) PDF" 0
assert_contains "quartz: source text" "The quick brown fox jumps over the lazy dog." "$stdout"

# --- killer pipeline: per-page text into another vrk tool ---

tokens="$("$VRK" pdf --json "$FIX/text.pdf" | jq -r 'select(.page) | .text' | "$VRK" tok)"
if [ "$tokens" -gt 0 ] 2>/dev/null; then ok "pipeline: pdf --json | jq | tok counts $tokens tokens"; else fail "pipeline: pdf | jq | tok" "got '$tokens'"; fi

# --- runtime errors (exit 1) ---

run /dev/null "$FIX/scanned.pdf"
assert_exit "scanned PDF" 1
assert_contains "scanned: OCR message" "OCR required" "$stderr"
assert_eq "scanned: stdout empty" "" "$stdout"

run /dev/null "$FIX/blank.pdf"
assert_exit "blank PDF" 1

run /dev/null "$FIX/encrypted.pdf"
assert_exit "encrypted PDF" 1
assert_contains "encrypted: message" "pdf: PDF is encrypted: cannot extract text" "$stderr"

run /dev/null --json "$FIX/encrypted.pdf"
assert_exit "encrypted --json" 1
assert_eq "encrypted --json: stderr empty" "" "$stderr"
assert_eq "encrypted --json: error JSON" "1" "$(jq -r .code "$TMP/out")"

run /dev/null --pages 9 "$FIX/text.pdf"
assert_exit "--pages 9 on 3 pages" 1
assert_contains "--pages 9: message" "pdf: page 9 does not exist (PDF has 3 pages)" "$stderr"

run /dev/null "$TMP/does-not-exist.pdf"
assert_exit "missing file" 1
assert_contains "missing file: message" "pdf: file not found:" "$stderr"

printf 'not a pdf' > "$TMP/fake.pdf"
run "$TMP/fake.pdf"
assert_exit "not a PDF on stdin" 1
assert_contains "not a PDF: message" "pdf: not a valid PDF:" "$stderr"

# --- usage errors (exit 2) ---

run /dev/null --pages 0 "$FIX/text.pdf"
assert_exit "--pages 0" 2
assert_contains "--pages 0: message" "pdf: invalid page range: 0" "$stderr"
assert_eq "--pages 0: stdout empty" "" "$stdout"

run /dev/null --pages 5-2 "$FIX/text.pdf"
assert_exit "--pages 5-2" 2

run /dev/null "$FIX/text.pdf" "$FIX/blank.pdf"
assert_exit "two file args" 2

run /dev/null --bogus
assert_exit "unknown flag" 2

# --- hostile input (each of these used to hang or eat gigabytes) ---

run /dev/null --text "$FIX/prev_loop.pdf"
assert_exit "/Prev loop" 0
assert_eq "/Prev loop: text" "Prev loop" "$stdout"

run /dev/null --text "$FIX/count_huge.pdf"
assert_exit "huge /Count" 0
assert_eq "huge /Count: real page" "Hello count" "$stdout"

run /dev/null --text "$FIX/form_bomb.pdf"
assert_exit "form bomb" 0
assert_contains "form bomb: budget warning" "exceeds the per-page" "$stderr"

# --- empty input and help ---

run /dev/null
assert_exit "empty stdin" 0
assert_eq "empty stdin: no output" "" "$stdout"
assert_eq "empty stdin: no stderr" "" "$stderr"

run /dev/null --help
assert_exit "--help" 0
assert_contains "--help: names the tool" "vrk pdf" "$stdout"
assert_contains "--help: lists --pages" "--pages" "$stdout"

run /dev/null --quiet "$FIX/encrypted.pdf"
assert_exit "--quiet error" 1
assert_eq "--quiet: stderr empty" "" "$stderr"

echo ""
# --- cleanup and --as-printed ---

run /dev/null --text "$FIX/running.pdf"
assert_exit "running headers" 0
assert_eq "default: header kept once" "1" "$(grep -c 'Journal of Things' "$TMP/out")"
assert_contains "default: hyphen joined" "the existing" "$stdout"

run /dev/null --text --as-printed "$FIX/running.pdf"
assert_exit "--as-printed" 0
assert_eq "--as-printed: header on every page" "3" "$(grep -c 'Journal of Things' "$TMP/out")"
assert_contains "--as-printed: hyphen kept" "the exist-" "$stdout"
assert_eq "--as-printed: page numbers kept" "3" "$(grep -cE '^[0-9]+$' "$TMP/out")"

run /dev/null --json --as-printed "$FIX/running.pdf"
assert_contains "--as-printed --json: page number in text" 'end one\n\n1"' "$stdout"

echo "---"
echo "Results: $PASS passed, $FAIL failed"
echo ""

[ "$FAIL" -eq 0 ]
