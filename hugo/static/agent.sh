#!/usr/bin/env bash
# agent.sh - install vrk and print agent onboarding block.
# Usage: curl -sSf https://vrk.sh/agent.sh | bash

set -euo pipefail

# --- Install ---
curl -sSf https://vrk.sh/install.sh | bash

# --- Locate binary ---
VRK=""
if command -v vrk &>/dev/null; then
  VRK="vrk"
elif [ -x "$HOME/.local/bin/vrk" ]; then
  VRK="$HOME/.local/bin/vrk"
elif [ -x "/usr/local/bin/vrk" ]; then
  VRK="/usr/local/bin/vrk"
fi

if [ -z "$VRK" ]; then
  echo "error: vrk binary not found after install" >&2
  exit 1
fi

VERSION=$($VRK --version 2>/dev/null || echo "unknown")

# --- Agent onboarding ---
cat <<EOF

# vrksh installed

version: $VERSION
binary:  $(command -v vrk || echo "$VRK")

## Three tools to know first

1. **tok** - count tokens and gate pipelines before LLM calls.
   cat prompt.txt | vrk tok --check 4000 | vrk prompt --system "summarise"

2. **prompt** - send a prompt to an LLM (Anthropic or OpenAI).
   echo "Summarise this." | vrk prompt

3. **grab** - fetch a URL as clean markdown for pipeline input.
   vrk grab https://example.com | vrk tok --check 8000 | vrk prompt --system "summarise"

## Discovery

    vrk --manifest       # JSON list of all 26 tools
    vrk --skills         # full reference: flags, exit codes, compose patterns
    vrk --skills tok     # single-tool reference (lower token cost)

## Full orientation

    https://vrk.sh/agents.md

EOF
