"""Verify every tool in a tools/list MCP response has flags beyond 'input'.

Reads one JSON line from stdin (the JSON-RPC response to tools/list).
On success: prints nothing, exits 0.
On failure: prints tool name and reason to stderr, exits 1.
"""
import json
import sys

line = sys.stdin.readline()
if not line.strip():
    print("check_flags: empty input", file=sys.stderr)
    sys.exit(1)

resp = json.loads(line)
tools = resp.get("result", {}).get("tools", [])

if not tools:
    print("check_flags: no tools in response", file=sys.stderr)
    sys.exit(1)

failed = False
for t in tools:
    name = t.get("name", "<unknown>")
    props = t.get("inputSchema", {}).get("properties", {})
    desc = t.get("description", "")

    non_input = [k for k in props if k != "input"]
    if len(non_input) == 0:
        print(f"check_flags: {name}: no properties beyond 'input' (empty Flags()?)", file=sys.stderr)
        failed = True

    if not desc:
        print(f"check_flags: {name}: empty description", file=sys.stderr)
        failed = True

if failed:
    sys.exit(1)
