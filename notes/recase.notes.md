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
