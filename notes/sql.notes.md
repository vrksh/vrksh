## How it works

`vrk sql` runs a SQL query on CSV, TSV or JSONL files with SQLite and prints the result as CSV.

### One file, one table

The table name is the file name: `users.csv` is `users`. The first row is the header.

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

Strings take single quotes. Double quotes are for column and table names, so a string in double quotes, or a misspelled quoted name, is an error rather than a silent match on nothing:

```bash
$ vrk sql 'SELECT name FROM users WHERE city = "Pune"' users.csv
error: sql: no such column: Pune (users: id, name, age, city); strings take single quotes: 'Pune'
```

### Joins

If you pass more files, each file is another table.

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

vrk sql reads stdin only when the query mentions `stdin`, so a query on files never waits on an empty pipe. To read stdin in a different way, pass `-` as a file. Use `name=-` to give the piped CSV its own table name.

```bash
$ cat users.csv | vrk sql 'SELECT count(*) AS n FROM people' people=-
n
3
```

### Naming tables (name=file.csv)

`name=path` sets the table name. Use it when the file name is a poor table name, like `2024-sales.csv`, or when you use process substitution (`<(...)`).

```bash
$ vrk sql 'SELECT name FROM u WHERE id = 2' u=users.csv
name
bo
```

### TSV and other delimiters

vrk sql reads files that end in `.tsv` as tab-separated. For any other delimiter, `--delimiter` (`-d`) takes one character, or `\t` for a tab.

```bash
$ printf 'a;b\n1;2\n' | vrk sql -d ';' 'SELECT b FROM stdin'
b
2
```

### JSONL in

JSONL (one JSON object per line) works as input too, so you can query the output of other vrk tools. Each key becomes a column, in the order the keys first appear. A missing key or `null` is NULL, and `true` and `false` are 1 and 0. Nested objects and arrays stay as JSON text, which you can read with `json_extract`.

vrk sql skips the `{"_vrk":...}` trailer that `--json` adds. If the trailer is the only record, vrk sql treats the input as empty stdin: no output, exit 0.

```bash
$ printf '{"id":1,"ok":true,"tags":["a","b"]}\n{"id":2,"ok":false}\n' | vrk sql "SELECT id, json_extract(tags, '\$[0]') AS tag FROM stdin WHERE ok"
id,tag
1,a
```

```bash
$ vrk pdf --json report.pdf | vrk sql 'SELECT page, length(text) AS chars FROM stdin ORDER BY chars DESC LIMIT 1'
page,chars
2,29
```

### JSONL out (--json)

```bash
$ vrk sql --json 'SELECT id, name, age FROM users WHERE city = '"'"'Pune'"'"'' users.csv
{"id":1,"name":"ana","age":30}
{"id":3,"name":"cy","age":9}
{"_vrk":"sql","count":2}
```

The keys keep the column order. Numbers stay numbers and NULL is `null`. The last record is the trailer.

## Type inference

vrk sql sets the type of each column from its values. A column is INTEGER when every non-empty value is a whole number that fits in 64 bits. It is REAL when the values are whole numbers and decimals like `25.5`. In all other cases it is TEXT. Leading zeros keep a column TEXT, so ZIP codes and IDs like `007` come out unchanged. Empty cells are NULL.

`N/A`, `NA`, `null`, `none`, `nan` and `-` do not stop a column from being a number column. They load as NULL, so one missing value does not turn `qty > 3` into a string comparison:

```bash
$ printf 'sku,qty\na,5\nb,N/A\nc,12\n' | vrk sql 'SELECT sku FROM stdin WHERE qty > 3'
sku
a
c
```

Any other text in a mostly numeric column makes it TEXT, and so does an integer too big for 64 bits. vrk sql then warns on stderr and names the column and the row. With `--json`, stderr stays empty and the warnings go in the trailer as `"warnings":[...]`.

If every value in a column has the same number of decimal places, the values keep them, so `19.90` prints as `19.90`, not `19.9`. Computed values like `sum(price)` print with the fewest digits they need.

```bash
$ printf 'sku,price\na,19.90\nb,5.00\n' | vrk sql 'SELECT price FROM stdin ORDER BY price'
price
5.00
19.90
```

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

If the query names a table and you pass no files, vrk sql exits 2, because the file is almost always missing:

```bash
$ vrk sql 'SELECT * FROM users'
usage error: sql: no input files; provide CSV file paths as arguments
$ echo $?
2
```

If you pass files and the table name is wrong, SQLite gives the error and vrk sql exits 1:

```bash
$ vrk sql 'SELECT * FROM user' users.csv
error: sql: no such table: user (tables: users)
$ echo $?
1
```

vrk sql blocks ATTACH and VACUUM INTO, so a query cannot open or write other database files. It rejects PRAGMA, and temp tables stay in memory:

```bash
$ vrk sql "ATTACH DATABASE 'other.db' AS o"
error: sql: too many attached databases - max 0
$ vrk sql 'PRAGMA temp_store = FILE'
error: sql: PRAGMA is not allowed
```

A query that never ends, like a recursive CTE with no stop condition, stops after 60 seconds. Use `--timeout` to change the limit. Use `--timeout 0` to turn the limit off:

```bash
$ vrk sql --timeout 200ms --query 'WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c) SELECT count(*) FROM c'
error: sql: query timed out after 200ms
```

If a quote is never closed, vrk sql exits 1. It does not fold the rest of the file into one cell:

```bash
$ printf 'id,note\n1,"never closed\n2,b\n' | vrk sql 'SELECT count(*) FROM stdin'
error: sql: CSV parse error in stdin: line 2: stray or unclosed quote in a quoted field; quotes inside one must be doubled ("")
```

vrk sql also has size limits: 1 GiB of input per file, 50 million cells (rows times header columns), 64 MiB for one value and 1 GiB of result. If a limit is hit, vrk sql exits 1 and names the limit.

The output is raw. vrk sql does not escape cells that start with `=`, `+`, `-` or `@` for spreadsheets, and control characters pass through. Use `--json` when the result goes to a terminal or an LLM.
