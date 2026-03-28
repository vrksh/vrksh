#!/usr/bin/env bash
# validate-manifest.sh — verify all generated surfaces stay in sync with manifest.json.
# Exits 0 if every tool in the manifest has entries in all surfaces.
# Exits 1 and prints every gap if any surface is missing a tool.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MANIFEST="$ROOT/manifest.json"

# Extract tool names from manifest.json (requires jq or fallback to grep/sed).
if command -v jq &>/dev/null; then
  tools=$(jq -r '.tools[].name' "$MANIFEST")
else
  tools=$(grep '"name"' "$MANIFEST" | sed 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
fi

missing=0

for tool in $tools; do
  # 1. hugo/content/docs/<tool>.md
  if [ ! -f "$ROOT/hugo/content/docs/${tool}.md" ]; then
    echo "MISSING  hugo/content/docs/${tool}.md"
    missing=$((missing + 1))
  fi

  # 2. Section in hugo/static/skills.md (## <tool> — or ## <tool> -)
  if ! grep -qE "^## ${tool}( |-)" "$ROOT/hugo/static/skills.md"; then
    echo "MISSING  hugo/static/skills.md  section for '${tool}'"
    missing=$((missing + 1))
  fi

  # 3. Tool name in hugo/static/agents.md
  if ! grep -q "| \`${tool}\`" "$ROOT/hugo/static/agents.md"; then
    echo "MISSING  hugo/static/agents.md  entry for '${tool}'"
    missing=$((missing + 1))
  fi

  # 4. Tool name in hugo/static/llms.txt
  if ! grep -q "vrk ${tool}" "$ROOT/hugo/static/llms.txt"; then
    echo "MISSING  hugo/static/llms.txt   entry for '${tool}'"
    missing=$((missing + 1))
  fi
done

if [ "$missing" -gt 0 ]; then
  echo ""
  echo "validate-manifest: ${missing} gaps found across $(echo "$tools" | wc -w | tr -d ' ') tools"
  exit 1
fi

echo "validate-manifest: all tools present in all surfaces"
exit 0
