## Supported conventions

`vrk recase` converts identifiers between naming conventions such as camelCase, snake_case and kebab-case.

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

recase detects the input convention itself. You only set the target with `--to`.

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

recase converts each line on its own, so you can convert many names at once:

```bash
$ printf 'user_first_name\nuser_last_name\nemail_address\n' | vrk recase --to camel
userFirstName
userLastName
emailAddress
```

### JSON output

```bash
$ echo 'hello_world' | vrk recase --to camel --json
{"from":"snake","input":"hello_world","output":"helloWorld","to":"camel"}
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

If you leave out `--to`, recase exits 2:

```bash
$ echo 'hello' | vrk recase
usage error: --to is required
$ echo $?
2
```

If the convention is unknown, recase exits 2:

```bash
$ echo 'hello' | vrk recase --to unknown
usage error: recase: unknown convention: "unknown"; valid: camel, pascal, snake, kebab, screaming, title, lower, upper
$ echo $?
2
```
