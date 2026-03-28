#!/bin/sh
# Idempotent CLAUDE.md appender for vrksh.
# Usage: curl -sSL https://vrk.sh/agents.sh | sh
# Or:    sh agents.sh [path/to/CLAUDE.md]
set -e

CLAUDE_MD="${1:-CLAUDE.md}"
BEGIN_MARKER="# BEGIN vrksh"
END_MARKER="# END vrksh"

# Fetch the agents.md content
if command -v curl >/dev/null 2>&1; then
  BLOCK="$(curl -sSL https://vrk.sh/agents.md)" || { printf 'error: failed to fetch agents.md\n' >&2; exit 1; }
elif command -v wget >/dev/null 2>&1; then
  BLOCK="$(wget -q -O - https://vrk.sh/agents.md)" || { printf 'error: failed to fetch agents.md\n' >&2; exit 1; }
else
  printf 'error: neither curl nor wget found\n' >&2
  exit 1
fi

CONTENT="$(printf '%s\n%s\n%s' "$BEGIN_MARKER" "$BLOCK" "$END_MARKER")"

if [ ! -f "$CLAUDE_MD" ]; then
  # Create new file
  printf '%s\n' "$CONTENT" > "$CLAUDE_MD"
  printf 'vrksh block added to %s\n' "$CLAUDE_MD"
  exit 0
fi

if grep -q "$BEGIN_MARKER" "$CLAUDE_MD"; then
  # Replace existing block between markers
  TMPFILE="$(mktemp)"
  trap 'rm -f "$TMPFILE"' EXIT
  awk -v begin="$BEGIN_MARKER" -v end="$END_MARKER" -v content="$CONTENT" '
    $0 == begin { skip=1; printed=1; print content; next }
    $0 == end   { skip=0; next }
    !skip       { print }
  ' "$CLAUDE_MD" > "$TMPFILE"
  mv "$TMPFILE" "$CLAUDE_MD"
  printf 'vrksh block updated in %s\n' "$CLAUDE_MD"
else
  # Append to end
  printf '\n%s\n' "$CONTENT" >> "$CLAUDE_MD"
  printf 'vrksh block added to %s\n' "$CLAUDE_MD"
fi
