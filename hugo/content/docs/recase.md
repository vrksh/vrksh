---
title: "vrk recase"
description: "Naming convention converter - snake, camel, kebab, pascal, title"
tool: recase
group: utilities
mcp_callable: true
noindex: false
---

## The problem

Every language has a different casing rule and they never agree. Python wants `snake_case`, JavaScript wants `camelCase`, CSS wants `kebab-case`, and your database schema wants `PascalCase`. When you're generating code from a JSON schema, converting API field names, or renaming variables across a codebase, you need something that converts reliably - not a regex you'll tweak forever, and not a one-liner that breaks on acronyms.

`recase` reads one name per line, detects the input convention automatically, and emits it in whatever convention you ask for. Acronyms are handled: `HTMLParser` becomes `html_parser`, not `h_t_m_l_parser`.

## The fix

```bash
$ echo "getUserName" | vrk recase --to snake
get_user_name
```

## Walkthrough

### Basic conversion

```bash
echo "getUserName" | vrk recase --to snake
echo "get_user_name" | vrk recase --to camel
echo "MyComponent" | vrk recase --to kebab
echo "http_response_code" | vrk recase --to pascal
```

<!-- output: verify against binary -->

`recase` auto-detects the input convention from the word boundaries it finds - underscores for snake, hyphens for kebab, capital letters for camel and pascal. You never specify the source; only the target matters.

### Acronym handling

```bash
echo "HTMLParser" | vrk recase --to snake
echo "parseHTTPSRequest" | vrk recase --to snake
```

<!-- output: verify against binary -->

Consecutive capitals are treated as a single word. `HTMLParser` becomes `html_parser`, not `h_t_m_l_parser`. `parseHTTPSRequest` becomes `parse_https_request`.

### Batch processing a file

```bash
cat fields.txt | vrk recase --to kebab
```

Each line is converted independently. Blank lines pass through as blank lines. Lines that cannot be parsed as any known convention are passed through unchanged with a warning on stderr.

### JSON output

```bash
$ echo "myFieldName" | vrk recase --to snake --json
{"input":"myFieldName","output":"my_field_name","from":"camel","to":"snake"}
```

The `from` field shows the detected input convention. Useful when you need a record of what was changed, or when you're piping into another tool that needs structured data.

### Available conventions

| Value | Example |
|-------|---------|
| `camel` | `myVariableName` |
| `pascal` | `MyVariableName` |
| `snake` | `my_variable_name` |
| `kebab` | `my-variable-name` |
| `screaming` | `MY_VARIABLE_NAME` |
| `title` | `My Variable Name` |
| `lower` | `my variable name` |
| `upper` | `MY VARIABLE NAME` |

## Pipeline example

Generate a Go struct from a JSON schema that uses camelCase field names:

```bash
cat schema.json \
  | jq -r '.properties | keys[]' \
  | vrk recase --to pascal \
  | awk '{print $0 " string"}'
```

Or rename API response fields before storing them in a kv namespace:

```bash
cat api-response.json \
  | jq -r 'keys[]' \
  | vrk recase --to snake --json \
  | jq -r '"\(.input)=\(.output)"'
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--to` | | string | `""` | Target convention (required): `camel`, `pascal`, `snake`, `kebab`, `screaming`, `title`, `lower`, `upper` |
| `--json` | `-j` | bool | `false` | Emit JSON per line: `{input, output, from, to}` |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (I/O failure) |
| 2 | Usage error - `--to` missing or invalid value, interactive TTY with no stdin |
