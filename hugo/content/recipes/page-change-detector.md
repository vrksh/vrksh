---
title: "Page change detector"
meta_title: "Page change detector - vrk pipeline recipe"
description: "You get told when a page changes, with no scraper, no database and no diff tool. Hash the page text and compare it with the hash stored on the last ..."
why: "You get told when a page changes, with no scraper, no database and no diff tool."
body: "Hash the page text and compare it with the hash stored on the last run."
slug: "page-change-detector"
steps:
  - |-
    NEW=$(vrk grab --text --allow-internal http://localhost:8099/pricing.html | vrk digest --bare)
    OLD=$(vrk kv get --ns pages pricing 2>/dev/null || true)
    if [ "$NEW" = "$OLD" ]; then echo unchanged; else echo changed; vrk kv set --ns pages pricing "$NEW"; fi
tags:
  - "grab"
  - "digest"
  - "kv"
---

## The problem

You want to know when a pricing page, a changelog or a status page changes. Hashing the raw HTML does not work. Ads, timestamps and CSRF tokens change on every load, so the hash changes every time.

## How the pipeline works

`vrk grab --text` fetches the page and keeps only the readable text. Navigation, scripts and markup are gone, so the noise that breaks an HTML hash goes with them.

`vrk digest --bare` hashes that text and prints the hex, with no `sha256:` prefix.

`vrk kv get --ns pages pricing` reads the hash from the last run. The first time there is none, so the page counts as changed and the new hash is saved. After that you only hear about real changes.

The example uses a page on localhost, so it needs `--allow-internal`. For a public URL, drop that flag. `grab` refuses private addresses unless you pass it.

## Running it on a schedule

Wrap the three lines in a script and run it from cron. Make the script print only when the page changed, and cron will email you.

```bash
0 * * * * /home/me/bin/check-pricing.sh
```

## Seeing what changed

The hash says that something changed, not what. Store the text instead of the hash if you want a diff:

```bash
vrk grab --text "$URL" > new.txt
diff old.txt new.txt && echo unchanged
mv new.txt old.txt
```

Or send both versions to `vrk prompt` and ask what changed.

## Limits

`grab` does not run JavaScript. A page that builds its content in the browser gives you an empty or partial text, and the hash will not move when the real content does.

