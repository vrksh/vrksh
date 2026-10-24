## How it works

```bash
$ echo 'Hello World' | vrk slug
hello-world

$ echo 'My Blog Post: A Deep Dive into LLM Pipelines!' | vrk slug
my-blog-post-a-deep-dive-into-llm-pipelines
```

### Custom separator

```bash
$ echo 'Hello World' | vrk slug --separator _
hello_world
```

### Length limit

Truncates at word boundaries so slugs don't end mid-word:

```bash
$ echo 'My Very Long Blog Post Title That Goes On Forever' | vrk slug --max 30
my-very-long-blog-post-title
```

### Batch processing

One slug per input line:

```bash
$ printf 'First Post\nSecond Post\nThird Post\n' | vrk slug
first-post
second-post
third-post
```

### Non-Latin text

Latin letters fold to ASCII. Other scripts are kept as they are, so a Hindi or Japanese title still gets a slug:

```bash
$ printf 'Straße\nनमस्ते दुनिया\n東京 2026\n' | vrk slug
strasse
नमस्ते-दुनिया
東京-2026
```

Every input line produces exactly one output line. If a line has nothing to keep, you get an empty line and a warning on stderr, so batch output stays aligned with the input. Use `--ascii` when the target needs strict ASCII:

```bash
$ echo '東京 2026' | vrk slug --ascii
2026
```

### JSON output

```bash
$ echo 'Hello World' | vrk slug --json
{"input":"Hello World","output":"hello-world"}
```

## Pipeline integration

### Generate cache keys from URLs

```bash
# Use slugified URLs as kv keys
for url in $(cat urls.txt); do
  KEY=$(echo "$url" | vrk slug)
  vrk kv set --ns cache "$KEY" "$(vrk grab "$url")" --ttl 24h
done
```

### Create filenames from document titles

```bash
# Generate safe filenames for downloaded articles
while IFS= read -r url; do
  TITLE=$(vrk grab --json "$url" | jq -r '.title')
  FILENAME=$(echo "$TITLE" | vrk slug --max 50).md
  vrk grab "$url" > "$FILENAME"
done < urls.txt
```

## When it fails

No input:

```bash
$ vrk slug
usage error: slug: no input
$ echo $?
2
```
