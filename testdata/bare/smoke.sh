#!/usr/bin/env bash
# testdata/bare/smoke.sh
#
# End-to-end smoke tests for vrk --bare.
# Run after make build to verify real process behaviour: symlink creation/removal,
# exit codes, collision handling, and safety guarantees.
#
# Usage:
#   ./testdata/bare/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/bare/smoke.sh   # explicit binary path

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

assert_stdout_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    ok "$desc (contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
  fi
}

assert_stderr_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    ok "$desc (stderr contains '$pattern')"
  else
    fail "$desc" "stderr did not contain '$pattern'. got: $actual"
  fi
}

assert_file_exists() {
  local desc=$1 path=$2
  if [ -e "$path" ]; then
    ok "$desc (exists)"
  else
    fail "$desc" "$path does not exist"
  fi
}

assert_file_not_exists() {
  local desc=$1 path=$2
  if [ ! -e "$path" ]; then
    ok "$desc (not exists)"
  else
    fail "$desc" "$path should not exist"
  fi
}

assert_symlink_to() {
  local desc=$1 path=$2 target=$3
  if [ -L "$path" ]; then
    local resolved
    resolved=$(python3 -c "import os,sys; print(os.path.realpath(sys.argv[1]))" "$path")
    local expected
    expected=$(python3 -c "import os,sys; print(os.path.realpath(sys.argv[1]))" "$target")
    if [ "$resolved" = "$expected" ]; then
      ok "$desc (symlink to vrk)"
    else
      fail "$desc" "$path points to $resolved, expected $expected"
    fi
  else
    fail "$desc" "$path is not a symlink"
  fi
}

echo "vrk --bare — smoke tests"
echo "binary: $VRK"
echo ""

# Resolve the real vrk binary path for symlink target checks.
VRK_REAL=$(python3 -c "import os,sys; print(os.path.realpath(sys.argv[1]))" "$VRK")

# Create a temp directory and copy the vrk binary there.
TMPDIR=$(mktemp -d)
trap 'chmod -R u+w "$TMPDIR" 2>/dev/null; rm -rf "$TMPDIR"' EXIT

cp "$VRK_REAL" "$TMPDIR/vrk"
chmod +x "$TMPDIR/vrk"
T="$TMPDIR/vrk"

# ------------------------------------------------------------
# 1. vrk --bare links tools
# ------------------------------------------------------------
echo "--- 1. --bare links tools ---"

stdout=$("$T" --bare 2>/dev/null)
exit_code=$?
assert_exit "bare link: exit 0" 0 "$exit_code"
assert_symlink_to "bare link: epoch symlink" "$TMPDIR/epoch" "$T"
assert_symlink_to "bare link: jwt symlink"   "$TMPDIR/jwt"   "$T"
assert_stdout_contains "bare link: summary" "linked" "$stdout"

# ------------------------------------------------------------
# 2. Linked tool produces identical output to vrk <tool>
# ------------------------------------------------------------
echo ""
echo "--- 2. linked tool output matches vrk <tool> ---"

via_bare=$("$TMPDIR/epoch" 0 2>/dev/null)
via_vrk=$("$T" epoch 0 2>/dev/null)
if [ "$via_bare" = "$via_vrk" ]; then
  ok "epoch via symlink == vrk epoch"
else
  fail "epoch output mismatch" "bare='$via_bare' vrk='$via_vrk'"
fi

# ------------------------------------------------------------
# 3. Idempotent — second run shows "already linked"
# ------------------------------------------------------------
echo ""
echo "--- 3. idempotent ---"

stdout=$("$T" --bare 2>/dev/null)
exit_code=$?
assert_exit "idempotent: exit 0" 0 "$exit_code"
assert_stdout_contains "idempotent: already linked" "already linked" "$stdout"

# ------------------------------------------------------------
# 4. Collision — pre-existing file skipped without --force
# ------------------------------------------------------------
echo ""
echo "--- 4. collision skip ---"

# Clean up and recreate.
rm -f "$TMPDIR"/alpha 2>/dev/null
echo "not-vrk" > "$TMPDIR/alpha"

# We can't create a tool named "alpha" (not in registry), so test with a real tool.
# Remove the jwt symlink and replace with a regular file.
rm -f "$TMPDIR/jwt"
echo "not-vrk" > "$TMPDIR/jwt"

stdout=$("$T" --bare 2>/dev/null)
exit_code=$?
assert_exit "collision: exit 0 (skips are not errors)" 0 "$exit_code"
assert_stdout_contains "collision: jwt skipped" "skipped" "$stdout"

# jwt file should be unchanged.
content=$(cat "$TMPDIR/jwt")
if [ "$content" = "not-vrk" ]; then
  ok "collision: jwt file unchanged"
else
  fail "collision: jwt file changed" "got '$content'"
fi

# ------------------------------------------------------------
# 5. --force overwrites collision
# ------------------------------------------------------------
echo ""
echo "--- 5. --force overwrites ---"

stdout=$("$T" --bare --force jwt 2>/dev/null)
exit_code=$?
assert_exit "force: exit 0" 0 "$exit_code"
assert_symlink_to "force: jwt now symlink" "$TMPDIR/jwt" "$T"
assert_stdout_contains "force: overwritten" "overwritten" "$stdout"

# ------------------------------------------------------------
# 6. --list shows active symlinks only (two-column format)
# ------------------------------------------------------------
echo ""
echo "--- 6. --list ---"

stdout=$("$T" --bare --list 2>/dev/null)
exit_code=$?
assert_exit "list: exit 0" 0 "$exit_code"
assert_stdout_contains "list: contains epoch" "epoch" "$stdout"
assert_stdout_contains "list: contains jwt"   "jwt"   "$stdout"

# The non-vrk file "alpha" should not appear.
if echo "$stdout" | grep -q "alpha"; then
  fail "list: alpha should not appear" "alpha found in list output"
else
  ok "list: alpha not shown (not a vrk symlink)"
fi

# ------------------------------------------------------------
# 7. --remove <tool> removes only that tool
# ------------------------------------------------------------
echo ""
echo "--- 7. --remove specific ---"

"$T" --bare --remove jwt >/dev/null 2>&1
exit_code=$?
assert_exit "remove jwt: exit 0" 0 "$exit_code"
assert_file_not_exists "remove jwt: jwt gone" "$TMPDIR/jwt"
assert_file_exists "remove jwt: epoch still there" "$TMPDIR/epoch"

# ------------------------------------------------------------
# 8. --remove (no args) removes all vrk symlinks
# ------------------------------------------------------------
echo ""
echo "--- 8. --remove all ---"

# First re-link everything.
"$T" --bare >/dev/null 2>&1

stdout=$("$T" --bare --remove 2>/dev/null)
exit_code=$?
assert_exit "remove all: exit 0" 0 "$exit_code"
assert_stdout_contains "remove all: removed" "removed" "$stdout"

# No vrk symlinks should remain (but the "alpha" regular file should).
remaining=$(find "$TMPDIR" -maxdepth 1 -type l 2>/dev/null | wc -l | tr -d ' ')
if [ "$remaining" -eq 0 ]; then
  ok "remove all: no symlinks remain"
else
  fail "remove all: symlinks remain" "found $remaining symlinks"
fi

# ------------------------------------------------------------
# 9. --remove does NOT delete a non-vrk file (safety guarantee)
# ------------------------------------------------------------
echo ""
echo "--- 9. --remove safety ---"

# "alpha" is a regular file, not a vrk symlink.
"$T" --bare --remove >/dev/null 2>&1
assert_file_exists "remove safety: alpha preserved" "$TMPDIR/alpha"
content=$(cat "$TMPDIR/alpha")
if [ "$content" = "not-vrk" ]; then
  ok "remove safety: alpha content unchanged"
else
  fail "remove safety: alpha content changed" "got '$content'"
fi

# ------------------------------------------------------------
# 10. --dry-run makes no changes
# ------------------------------------------------------------
echo ""
echo "--- 10. --dry-run ---"

# Start clean — remove all symlinks.
"$T" --bare --remove >/dev/null 2>&1

stdout=$("$T" --bare --dry-run 2>/dev/null)
exit_code=$?
assert_exit "dry-run: exit 0" 0 "$exit_code"
assert_stdout_contains "dry-run: Would link" "Would link" "$stdout"

# No symlinks should have been created.
symlinks=$(find "$TMPDIR" -maxdepth 1 -type l 2>/dev/null | wc -l | tr -d ' ')
if [ "$symlinks" -eq 0 ]; then
  ok "dry-run: no symlinks created"
else
  fail "dry-run: symlinks created" "found $symlinks"
fi

# ------------------------------------------------------------
# 11. Unknown tool exits 1
# ------------------------------------------------------------
echo ""
echo "--- 11. unknown tool ---"

set +e
stderr=$("$T" --bare unknowntool 2>&1 >/dev/null)
exit_code=$?
set -e
assert_exit "unknown tool: exit 1" 1 "$exit_code"
assert_stderr_contains "unknown tool: message" "unknown tool" "$stderr"

# ------------------------------------------------------------
# 12. --dry-run --force shows what force would do
# ------------------------------------------------------------
echo ""
echo "--- 12. --dry-run --force ---"

# Create a collision file for jwt.
rm -f "$TMPDIR/jwt"
echo "blocker" > "$TMPDIR/jwt"

stdout=$("$T" --bare --dry-run --force 2>/dev/null)
exit_code=$?
assert_exit "dry-run force: exit 0" 0 "$exit_code"
assert_stdout_contains "dry-run force: Would overwrite" "Would overwrite" "$stdout"

# jwt should still be the regular file.
content=$(cat "$TMPDIR/jwt")
if [ "$content" = "blocker" ]; then
  ok "dry-run force: jwt unchanged"
else
  fail "dry-run force: jwt modified" "got '$content'"
fi

# ------------------------------------------------------------
# 13. --json flag exits 2
# ------------------------------------------------------------
echo ""
echo "--- 13. --json unknown flag ---"

set +e
stderr=$("$T" --bare --json 2>&1 >/dev/null)
exit_code=$?
set -e
assert_exit "unknown flag --json: exit 2" 2 "$exit_code"
assert_stderr_contains "unknown flag: message" "unknown flag" "$stderr"

# ------------------------------------------------------------
# 14. --remove + --force exits 2
# ------------------------------------------------------------
echo ""
echo "--- 14. --remove + --force mutex ---"

set +e
stderr=$("$T" --bare --remove --force 2>&1 >/dev/null)
exit_code=$?
set -e
assert_exit "remove+force: exit 2" 2 "$exit_code"
assert_stderr_contains "remove+force: message" "cannot be combined" "$stderr"

# ------------------------------------------------------------
# 15. --list + --remove exits 2
# ------------------------------------------------------------
echo ""
echo "--- 15. --list + --remove mutex ---"

set +e
stderr=$("$T" --bare --list --remove 2>&1 >/dev/null)
exit_code=$?
set -e
assert_exit "list+remove: exit 2" 2 "$exit_code"
assert_stderr_contains "list+remove: message" "cannot be combined" "$stderr"

# ------------------------------------------------------------
# 16. --list + --force exits 2
# ------------------------------------------------------------
echo ""
echo "--- 16. --list + --force mutex ---"

set +e
stderr=$("$T" --bare --list --force 2>&1 >/dev/null)
exit_code=$?
set -e
assert_exit "list+force: exit 2" 2 "$exit_code"
assert_stderr_contains "list+force: message" "cannot be combined" "$stderr"

# ------------------------------------------------------------
# 17. --list + --dry-run exits 2
# ------------------------------------------------------------
echo ""
echo "--- 17. --list + --dry-run mutex ---"

set +e
stderr=$("$T" --bare --list --dry-run 2>&1 >/dev/null)
exit_code=$?
set -e
assert_exit "list+dry-run: exit 2" 2 "$exit_code"
assert_stderr_contains "list+dry-run: message" "cannot be combined" "$stderr"

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
