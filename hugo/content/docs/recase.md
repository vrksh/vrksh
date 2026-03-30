---
title: "vrk recase"
description: "Convert between naming conventions. snake_case, camelCase, kebab-case, PascalCase, and Title Case."
og_title: "vrk recase - convert between snake, camel, kebab, and more"
tool: recase
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Your API returns `snake_case` field names but your frontend expects `camelCase`. You write a sed command to convert and it breaks on acronyms - `http_api_url` becomes `httpApiUrl` instead of handling the abbreviation correctly.

`vrk recase` converts between naming conventions: snake_case, camelCase, PascalCase, kebab-case, SCREAMING_SNAKE, Title Case, and more. Auto-detects the input convention. Handles acronyms correctly. Processes line by line so you can convert a list of names in one pass.

## The problem

You're migrating an API and need to convert 200 field names from snake_case to camelCase. You write a Python script. It takes 30 lines to handle edge cases: leading underscores, consecutive capitals, numeric suffixes. You test it on http_api_url and get httpApiUrl. Close enough? Not for a public API.

## Before and after

**Before**

```bash
echo 'user_first_name' | sed 's/_\(.\)/\U\1/g'
# Breaks on acronyms, doesn't handle PascalCase or kebab-case
```

**After**

```bash
echo 'user_first_name' | vrk recase --to camel
```

## Example

```bash
echo 'user_first_name' | vrk recase --to camel
```

## Exit codes

| Code | Meaning                                                      |
|------|--------------------------------------------------------------|
| 0    | Success                                                      |
| 1    | Runtime error (I/O failure)                                  |
| 2    | --to missing or invalid value, interactive TTY with no stdin |

## Flags

| Flag      | Short | Type   | Description                                                                    |
|-----------|-------|--------|--------------------------------------------------------------------------------|
| `--to`    |       | string | Target convention: camel, pascal, snake, kebab, screaming, title, lower, upper |
| `--json`  | -j    | bool   | Emit JSON per line: {input, output, from, to}                                  |
| `--quiet` | -q    | bool   | Suppress stderr output                                                         |


<!-- notes - edit in notes/recase.notes.md -->

## Supported conventions

| Convention | Example | Flag value |
|-----------|---------|------------|
| camelCase | `helloWorld` | `camel` |
| PascalCase | `HelloWorld` | `pascal` |
| snake_case | `hello_world` | `snake` |
| kebab-case | `hello-world` | `kebab` |
| SCREAMING_SNAKE | `HELLO_WORLD` | `screaming` |
| Title Case | `Hello World` | `title` |
| lowercase | `hello world` | `lower` |
| UPPERCASE | `HELLO WORLD` | `upper` |

Input convention is auto-detected. You only need to specify the target.

## How it works

```bash
$ echo 'hello_world' | vrk recase --to camel
helloWorld

$ echo 'helloWorld' | vrk recase --to snake
hello_world

$ echo 'user_first_name' | vrk recase --to pascal
UserFirstName

$ echo 'UserFirstName' | vrk recase --to kebab
user-first-name
```

### Batch conversion

Processes line by line, so you can convert multiple names at once:

```bash
$ printf 'user_first_name\nuser_last_name\nemail_address\n' | vrk recase --to camel
userFirstName
userLastName
emailAddress
```

### JSON output

```bash
$ echo 'hello_world' | vrk recase --to camel --json
{"input":"hello_world","output":"helloWorld","from":"snake","to":"camel"}
```

## Pipeline integration

### Convert API field names

```bash
# Extract keys from a JSON object and convert to camelCase
echo '{"user_name":"alice","email_addr":"a@b.com"}' | \
  jq -r 'keys[]' | vrk recase --to camel
```

### Rename files in a directory

```bash
# Convert kebab-case filenames to snake_case
for f in *.md; do
  NEWNAME=$(echo "${f%.md}" | vrk recase --to snake).md
  mv "$f" "$NEWNAME"
done
```

## When it fails

Missing --to flag:

```bash
$ echo 'hello' | vrk recase
usage error: recase: --to is required
$ echo $?
2
```

Invalid convention:

```bash
$ echo 'hello' | vrk recase --to unknown
usage error: recase: unsupported convention "unknown"
$ echo $?
2
```
