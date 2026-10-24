---
title: "vrk sql"
description: "Run SQL over CSV files from the command line. Joins, GROUP BY and type inference in an in-memory SQLite database. CSV or JSONL out, no database to set up."
meta_lead: "vrk sql loads CSV files into an in-memory SQLite database, one table per file, runs your query and writes the result as CSV or JSONL."
og_title: "vrk sql - SQL over CSV files for AI pipelines"
tool: sql
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

vrk sql loads CSV files into an in-memory SQLite database, one table per file, runs your query and writes the result as CSV or JSONL.

## The problem

Joining two CSVs or filtering one by a numeric column means a Python script, a pandas install, or importing into a database by hand. sqlite3's `.import` treats every column as text, so `age > 25` compares strings, and it needs a database file and several dot-commands before the query runs.

## The solution

`vrk sql` runs a SQL query over CSV files. Each file becomes a table named after its basename (`users.csv` is `users`), and piped CSV is the table `stdin`. Column types are inferred: INTEGER or REAL when every non-empty value is a number, TEXT otherwise, so `007` stays `007`. Output is CSV with a header row, or one JSON object per row with `--json`. ATTACH is blocked, so a query can't read or write other files.

## Before and after

**Before**

```bash
sqlite3 :memory: -cmd '.mode csv' -cmd '.import users.csv users' \
  -cmd '.import orders.csv orders' -cmd '.headers on' \
  'SELECT u.name, sum(o.total) FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.name'
# every column is TEXT, so numeric comparisons and sorts are wrong
```

**After**

```bash
vrk sql 'SELECT u.name, sum(o.total) AS spent FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.name' users.csv orders.csv
```

## Example

```bash
vrk sql --json 'SELECT * FROM users WHERE age > 25' users.csv | jq -c 'select(.id)'
```

## Exit codes

| Code | Meaning                                                                                                      |
|------|--------------------------------------------------------------------------------------------------------------|
| 0    | Success, including empty stdin when the query reads stdin                                                    |
| 1    | SQL error, file not found, CSV parse error, or empty file                                                    |
| 2    | No query, a table but no files, duplicate table name, stdin in the query with nothing piped, or unknown flag |

## Flags

| Flag          | Short | Type   | Description                                             |
|---------------|-------|--------|---------------------------------------------------------|
| `--query`     |       | string | SQL query; every positional argument is then a CSV file |
| `--no-header` |       | bool   | Leave the header row out of CSV output                  |
| `--json`      | -j    | bool   | One JSON object per row, then a metadata trailer        |
| `--quiet`     | -q    | bool   | Suppress stderr output                                  |


<!-- notes - edit in notes/sql.notes.md -->

## How it works

### One file, one table

The table is named after the file: `users.csv` is `users`. The first row is the header.

```bash
$ vrk sql 'SELECT name, city FROM users WHERE age > 25' users.csv
name,city
ana,Pune
bo,Delhi
```

`age` is an INTEGER column because every value is a whole number, so `age > 25` compares numbers, and `ORDER BY age` puts 9 before 30 before 100.

```bash
$ vrk sql --no-header 'SELECT age FROM users ORDER BY age' users.csv
9
30
100
```

### Joins

Pass more files and each one is another table.

```bash
$ vrk sql 'SELECT u.name, sum(o.total) AS spent FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.name ORDER BY spent DESC' users.csv orders.csv
name,spent
cy,100
ana,30
```

### Piped CSV is the table stdin

```bash
$ cat users.csv | vrk sql 'SELECT city, count(*) AS n FROM stdin GROUP BY city'
city,n
Delhi,1
Pune,2
```

Stdin is only read when the query mentions `stdin`, so a file query inside a script never waits on an empty pipe.

### JSONL out (--json)

```bash
$ vrk sql --json 'SELECT id, name, age FROM users WHERE city = '"'"'Pune'"'"'' users.csv
{"id":1,"name":"ana","age":30}
{"id":3,"name":"cy","age":9}
{"_vrk":"sql","count":2}
```

Keys keep the column order. Numbers stay numbers and NULL is `null`. The last record is the trailer.

## Type inference

A column is INTEGER when every non-empty value is a whole number that fits in 64 bits, REAL when the values are whole numbers and decimals like `25.5`, and TEXT otherwise. Leading zeros keep a column TEXT, so ZIP codes and IDs like `007` come out unchanged. Empty cells in numeric columns are NULL.

## Pipeline integration

### Send matching rows to an LLM

```bash
vrk sql --json 'SELECT id, feedback FROM survey WHERE rating < 3' survey.csv \
  | jq -c 'select(.id)' \
  | vrk prompt --field feedback --system 'Classify this complaint in one word.'
```

### Gate on a count

```bash
$ vrk sql --no-header 'SELECT count(*) FROM orders WHERE total > 50' orders.csv
1
```

## When it fails

A table with no files is a usage error, since it almost always means the file was left off:

```bash
$ vrk sql 'SELECT * FROM users'
usage error: sql: no input files; provide CSV file paths as arguments
$ echo $?
2
```

With files given, a wrong table name is SQLite's own error and exits 1:

```bash
$ vrk sql 'SELECT * FROM user' users.csv
error: sql: SQL logic error: no such table: user (1)
$ echo $?
1
```

ATTACH is blocked, so a query can't open other database files:

```bash
$ vrk sql "ATTACH DATABASE 'other.db' AS o"
error: sql: SQL logic error: too many attached databases - max 0 (1)
```
