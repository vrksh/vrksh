---
title: "vrk plain"
description: "Markdown stripper - removes syntax, keeps prose"
tool: plain
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You have a markdown file - a README, a docs page, a scraped article - and you want to feed it to something that expects plain text. A token counter, a search indexer, a text-to-speech engine, or an LLM that works better without formatting noise. Raw markdown sent to any of these carries syntax that adds tokens without adding meaning: asterisks, backticks, bracket-URL pairs, heading hashes, horizontal rules.

`vrk plain` removes the syntax and keeps the content. Link text is preserved, URLs are dropped. Code content is kept, fences and backticks are stripped. Heading text becomes plain paragraphs. The result reads the same to a human and costs far fewer tokens downstream.

## The fix

```bash
cat README.md | vrk plain
```

Or pass the content as a positional argument:

```bash
vrk plain '## Installation\n\nRun `make build`.'
```

## Walkthrough

**Stripping a README before token counting**

The common case: you want to know how many tokens a document contains, but markdown syntax inflates the count. Strip it first.

```bash
cat README.md | vrk plain | vrk tok --budget 4000
```

**Getting byte counts alongside the text**

When you need the input and output sizes - to compare compression, to charge for bytes, to log the reduction - use `--json`. The output is a single JSON object with the stripped text and both byte counts.

```bash
$ echo '## Hello\n\nSome **bold** text.' | vrk plain --json
{"text":"Hello\n\nSome bold text.","input_bytes":30,"output_bytes":22}
```

**Feeding a scraped page to a prompt**

`vrk grab` returns markdown by default. Pipe it through `vrk plain` before sending it to an LLM to keep the prompt lean.

```bash
vrk grab https://example.com/article | vrk plain | vrk prompt --system "Summarize this in three bullets"
```

**Inline markdown in a shell one-liner**

Both forms work identically:

```bash
$ echo '**bold** and _italic_ and [link](https://example.com)' | vrk plain
bold and italic and link
```

## Pipeline example

```bash
vrk grab https://example.com | vrk plain | vrk tok --budget 4000
```

Fetch a page, strip markdown syntax, then assert the result fits in a 4k token budget before passing it to a model. If the token count exceeds 4000, `vrk tok` exits 1 and the pipeline stops.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--json` | `-j` | bool | false | Emit a JSON object with `text`, `input_bytes`, and `output_bytes` |
| `--quiet` | `-q` | bool | false | Suppress stderr output; exit codes are unaffected |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success - plain text written to stdout, or `--json` object emitted |
| 1 | Runtime error - could not read stdin or write stdout |
| 2 | Usage error - interactive TTY with no piped input and no positional arg |
