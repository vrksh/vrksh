# sql - SQL over CSV files - SQLite in memory, CSV or JSONL out

When to use: filter, join or aggregate CSV files without loading them into a database or writing a script. Each file is a table named after its basename; piped CSV is the table `stdin`.
Composes with: jq, prompt, tok, assert

| Flag          | Short | Type   | Description                                                   |
|---------------|-------|--------|---------------------------------------------------------------|
| `--query`     |       | string | SQL query; every positional argument is then a CSV file       |
| `--no-header` |       | bool   | Leave the header row out of CSV output                        |
| `--json`      | `-j`  | bool   | One JSON object per row, then `{"_vrk":"sql","count":N}`      |
| `--quiet`     | `-q`  | bool   | Suppress stderr                                               |

Exit 0: success (including empty stdin when the query reads stdin)
Exit 1: SQL error, file not found, CSV parse error, empty file
Exit 2: no query, a table but no files, duplicate table name, stdin in the query with nothing piped, unknown flag

Example:

    vrk sql --json 'SELECT u.name, sum(o.total) AS spent FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.name' users.csv orders.csv

Anti-pattern:
- Don't put the file name in FROM. `FROM users.csv` fails; the table is `users`, and the file goes after the query.
- Don't expect leading zeros to become numbers. `007` keeps the column TEXT; use `CAST(col AS INTEGER)` if you mean it.
- Don't forget the trailer under --json. Use `select(._vrk | not)` or pick a real column, like `select(.id)`, to skip `{"_vrk":"sql","count":N}`.
