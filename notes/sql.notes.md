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
