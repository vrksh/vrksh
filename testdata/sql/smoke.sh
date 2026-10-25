#!/usr/bin/env bash
# testdata/sql/smoke.sh
#
# End-to-end smoke tests for vrk sql.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/sql/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/sql/smoke.sh   # explicit binary path

set -euo pipefail

VRK="${VRK:-./vrk}"
FIX="$(dirname "$0")/fixtures"
PASS=0
FAIL=0

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

ok() {
  echo "  PASS  $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "  FAIL  $1"
  echo "        $2"
  FAIL=$((FAIL + 1))
}

# run <stdin-file> <args...>: sets $stdout, $stderr, $code.
run() {
  local in=$1
  shift
  code=0
  "$VRK" sql "$@" < "$in" > "$TMP/out" 2> "$TMP/err" || code=$?
  stdout="$(cat "$TMP/out")"
  stderr="$(cat "$TMP/err")"
}

assert_exit() {
  if [ "$code" -eq "$2" ]; then ok "$1 (exit $2)"; else fail "$1" "expected exit $2, got $code. stderr: $stderr"; fi
}

assert_eq() {
  if [ "$3" = "$2" ]; then ok "$1"; else fail "$1" "expected '$2', got '$3'"; fi
}

assert_contains() {
  if printf '%s' "$3" | grep -qF -- "$2"; then ok "$1 (contains '$2')"; else fail "$1" "missing '$2' in: $3"; fi
}

echo "vrk sql smoke tests"
echo ""

U="$FIX/users.csv"
O="$FIX/orders.csv"

# --- happy paths ---

run /dev/null 'SELECT name, city FROM users ORDER BY id' "$U"
assert_exit "query + file" 0
assert_eq "query + file: csv with header" "$(printf 'name,city\nana,Pune\nbo,Delhi\ncy,Pune')" "$stdout"
assert_eq "query + file: stderr empty" "" "$stderr"

run /dev/null --query 'SELECT count(*) AS n FROM users, orders' "$U" "$O"
assert_eq "--query: every positional is a file" "$(printf 'n\n9')" "$stdout"

run /dev/null 'SELECT age FROM users ORDER BY age' "$U"
assert_eq "integer column sorts numerically" "$(printf 'age\n9\n30\n100')" "$stdout"

run /dev/null 'SELECT zip, typeof(pop) AS t FROM zips' "$FIX/zips.csv"
assert_eq "leading zeros kept, empty numeric is NULL" "$(printf 'zip,t\n007,integer\n110001,null')" "$stdout"

run /dev/null 'SELECT first_name, unit_price, col_3, Name, name_1 FROM messy' "$FIX/messy.csv"
assert_exit "messy headers + BOM" 0
assert_eq "messy headers sanitized" "$(printf 'first_name,unit_price,col_3,Name,name_1\nx,1.5,a,b,c')" "$stdout"

run /dev/null 'SELECT count(*) AS n FROM header_only' "$FIX/header_only.csv"
assert_eq "header-only file is an empty table" "$(printf 'n\n0')" "$stdout"

run /dev/null --no-header 'SELECT name FROM users WHERE id = 1' "$U"
assert_eq "--no-header" "ana" "$stdout"

run /dev/null --json 'SELECT name, id, NULL AS gone FROM users WHERE id = 1' "$U"
assert_exit "--json" 0
assert_eq "--json: ordered keys, null" '{"name":"ana","id":1,"gone":null}' "$(head -1 "$TMP/out")"
assert_eq "--json: trailer" '{"_vrk":"sql","count":1}' "$(tail -1 "$TMP/out")"

run "$U" 'SELECT name FROM stdin WHERE city = '"'Pune'"' ORDER BY name'
assert_exit "stdin table" 0
assert_eq "stdin table: rows" "$(printf 'name\nana\ncy')" "$stdout"

run /dev/null 'SELECT 1 + 1 AS two'
assert_eq "no files, no table" "$(printf 'two\n2')" "$stdout"

# --- killer pipeline: join two CSVs, JSONL into jq ---

top="$("$VRK" sql --json 'SELECT u.name, sum(o.total) AS spent FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.name ORDER BY spent DESC' "$U" "$O" | jq -r 'select(.name) | "\(.name)=\(.spent)"' | tr '\n' ' ')"
assert_eq "pipeline: join | jq" "cy=100 ana=30 " "$top"

piped="$(cat "$O" | "$VRK" sql 'SELECT u.name FROM stdin s JOIN users u ON u.id = s.user_id WHERE s.total > 50' "$U")"
assert_eq "pipeline: stdin joined with a file" "$(printf 'name\ncy')" "$piped"

# --- dirty data, decimals, table names, delimiters ---

I="$FIX/inventory.csv"
run /dev/null 'SELECT sku FROM inventory WHERE qty > 3 ORDER BY sku' "$I"
assert_exit "N/A in a numeric column" 0
assert_eq "N/A loads as NULL, qty compares as a number" "$(printf 'sku\na\nc')" "$stdout"

run /dev/null 'SELECT price FROM inventory WHERE price > 4 ORDER BY price' "$I"
assert_eq "decimals keep their digits" "$(printf 'price\n5.00\n19.90')" "$stdout"

run /dev/null 'SELECT count(*) AS n FROM inv' "inv=$I"
assert_eq "name=path names the table" "$(printf 'n\n4')" "$stdout"

run "$U" 'SELECT count(*) AS n FROM stdin' -
assert_eq "- is stdin" "$(printf 'n\n3')" "$stdout"

run /dev/null "SELECT count(*) AS n FROM users WHERE name <> 'stdin'" "$U"
assert_exit "stdin as a word, empty stdin" 0
assert_eq "stdin as a word: query still runs" "$(printf 'n\n3')" "$stdout"

printf 'a\tb\n1\tx,y\n' > "$TMP/t.tsv"
run /dev/null 'SELECT b FROM t' "$TMP/t.tsv"
assert_eq ".tsv is tab-separated" "$(printf 'b\n"x,y"')" "$stdout"

printf 'a;b\n1;2\n' > "$TMP/semi.csv"
run "$TMP/semi.csv" -d ';' 'SELECT b FROM stdin' -
assert_eq "-d ';'" "$(printf 'b\n2')" "$stdout"

printf 'a,b\n1\n2,"x"y"\n' > "$TMP/loose.csv"
run /dev/null 'SELECT count(*) AS n FROM loose WHERE b IS NULL' "$TMP/loose.csv"
assert_exit "short rows and stray quotes" 0
assert_eq "short row padded with NULL" "$(printf 'n\n1')" "$stdout"

gzip -c "$U" > "$TMP/u.csv.gz"
run /dev/null 'SELECT 1' "$TMP/u.csv.gz"
assert_exit "gzip file" 1
assert_contains "gzip file: message" "is gzip-compressed" "$stderr"

# --- runtime errors (exit 1) ---

run /dev/null 'SELEC 1'
assert_exit "syntax error" 1
assert_contains "syntax error: message" "syntax error" "$stderr"
assert_eq "syntax error: stdout empty" "" "$stdout"

run /dev/null 'SELECT * FROM nope' "$U"
assert_exit "missing table with files" 1
assert_contains "missing table: SQLite message" "no such table: nope" "$stderr"

run /dev/null 'SELECT 1' "$TMP/missing.csv"
assert_exit "missing file" 1
assert_contains "missing file: message" "sql: file not found:" "$stderr"

run /dev/null 'SELECT * FROM ragged' "$FIX/ragged.csv"
assert_exit "ragged CSV" 1
assert_contains "ragged CSV: message" "line 3: 3 fields, header has 2" "$stderr"

run /dev/null 'SELECT * FROM empty' "$FIX/empty.csv"
assert_exit "empty file" 1
assert_contains "empty file: message" "empty file, no header row" "$stderr"

run /dev/null "ATTACH DATABASE '$TMP/x.db' AS x"
assert_exit "ATTACH blocked" 1
assert_contains "ATTACH: message" "too many attached databases - max 0" "$stderr"
if [ ! -e "$TMP/x.db" ]; then ok "ATTACH: no file created"; else fail "ATTACH: no file created" "$TMP/x.db exists"; fi

run /dev/null "VACUUM INTO '$TMP/v.db'"
assert_exit "VACUUM INTO blocked" 1
if [ ! -e "$TMP/v.db" ]; then ok "VACUUM INTO: no file created"; else fail "VACUUM INTO: no file created" "$TMP/v.db exists"; fi

run /dev/null --json 'SELEC 1'
assert_exit "--json error" 1
assert_eq "--json error: stderr empty" "" "$stderr"
assert_eq "--json error: code" "1" "$(jq -r .code "$TMP/out")"

# --- usage errors (exit 2) ---

run /dev/null 'SELECT * FROM t'
assert_exit "table but no files" 2
assert_contains "table but no files: message" "sql: no input files" "$stderr"

run /dev/null
assert_exit "no query" 2

run /dev/null 'SELECT 1' "$U" "$FIX/../fixtures/users.csv"
assert_exit "duplicate table name" 2
assert_contains "duplicate: message" "sql: duplicate table name: users" "$stderr"

run /dev/null --format json 'SELECT 1'
assert_exit "--format is not a flag" 2

# --- empty input, help, quiet ---

run /dev/null 'SELECT * FROM stdin'
assert_exit "empty stdin" 0
assert_eq "empty stdin: no output" "" "$stdout"
assert_eq "empty stdin: no stderr" "" "$stderr"

printf '{"id":1,"ok":true}\n{"id":2,"ok":false}\n{"_vrk":"emit","count":2}\n' > "$TMP/in.jsonl"
run "$TMP/in.jsonl" 'SELECT id FROM stdin WHERE ok'
assert_exit "jsonl stdin" 0
assert_eq "jsonl stdin: bool filter, trailer skipped" "id
1" "$stdout"

printf '{"a":1}\n{"a":\n' > "$TMP/bad.jsonl"
run "$TMP/bad.jsonl" 'SELECT * FROM stdin'
assert_exit "jsonl cut off" 1
assert_contains "jsonl cut off: names the line" "line 2" "$stderr"

run /dev/null --help
assert_exit "--help" 0
assert_contains "--help: names the tool" "vrk sql" "$stdout"
assert_contains "--help: type inference" "inferred" "$stdout"

run /dev/null --quiet 'SELEC 1'
assert_exit "--quiet error" 1
assert_eq "--quiet: stderr empty" "" "$stderr"

echo ""
# --- stray quotes and runaway queries ---

printf 'id,name\n1,"x"y\n2,Acme\n3,Foo\n' > "$TMP/stray.csv"
run "$TMP/stray.csv" 'SELECT count(*) FROM stdin' -
assert_exit "stray quote in a quoted field" 1
assert_contains "stray quote: names the line" "line 2" "$stderr"
assert_eq "stray quote: stdout empty" "" "$stdout"

run /dev/null --timeout 200ms --query 'WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c) SELECT count(*) FROM c'
assert_exit "--timeout stops a runaway query" 1
assert_contains "--timeout: message" "timed out after 200ms" "$stderr"

got="$("$VRK" sql --query 'WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c) SELECT x FROM c' < /dev/null | head -n 3 | tr '\n' ' ' || true)"
assert_eq "runaway query streams into head" "x 1 2 " "$got"

# modernc only checks the context on the first step; this hangs on the second.
run /dev/null --timeout 300ms --query 'SELECT 1 UNION ALL SELECT (WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c) SELECT count(*) FROM c)'
assert_exit "--timeout stops a query after its first row" 1
assert_contains "--timeout after first row: message" "timed out after 300ms" "$stderr"

run /dev/null --query 'SELECT length(zeroblob(100000000))'
assert_exit "100 MB value refused" 1
assert_contains "100 MB value: message" "too big" "$stderr"

# --- limits and blocked statements ---

run /dev/null --query 'PRAGMA temp_store = FILE'
assert_exit "PRAGMA blocked" 1
assert_contains "PRAGMA: message" "PRAGMA is not allowed" "$stderr"

run /dev/null "SELECT name FROM pragma_table_info('users') LIMIT 1" "$U"
assert_exit "pragma_table_info still works" 0
assert_eq "pragma_table_info: first column" "name
id" "$stdout"

run /dev/null 'SELECT 1 FROM zero' /dev/zero
assert_exit "/dev/zero refused" 1
assert_contains "/dev/zero: message" "not a regular file" "$stderr"

printf 'id,note\n1,"never closed\n2,b\n3,c\n' > "$TMP/unterm.csv"
run "$TMP/unterm.csv" 'SELECT count(*) FROM stdin' -
assert_exit "unclosed quote" 1
assert_eq "unclosed quote: stdout empty" "" "$stdout"

printf 'amt\n12345678901234567890.5\n0.1\n' > "$TMP/big.csv"
run "$TMP/big.csv" 'SELECT amt, typeof(amt) AS t FROM stdin' -
assert_exit "big decimal" 0
assert_eq "big decimal stays TEXT, digits intact" "amt,t
12345678901234567890.5,text
0.1,text" "$stdout"

# A 2000-column header over 20000 one-field rows used to pad every row to
# 2000 cells and take minutes. Short rows now stay short.
{
  seq -f 'c%g' 1 2000 | paste -sd, -
  seq 20000 | sed 's/.*/1/'
} > "$TMP/wide.csv"
code=0
timeout 10 "$VRK" sql 'SELECT count(*), count(c2000) FROM wide' "$TMP/wide.csv" > "$TMP/out" 2> "$TMP/err" || code=$?
stderr="$(cat "$TMP/err")"
assert_exit "wide header, short rows, under 10s" 0
assert_eq "wide header: missing cells are NULL" "count(*),count(c2000)
20000,0" "$(cat "$TMP/out")"

# Double quotes are names only: a typo or a "string" fails instead of
# printing the literal on every row.
run /dev/null 'SELECT "nme" FROM users' "$U"
assert_exit "double-quoted typo" 1
assert_eq "double-quoted typo: stdout empty" "" "$stdout"
assert_contains "double-quoted typo lists columns" "no such column: nme (users: id, name" "$stderr"
run /dev/null 'SELECT count(*) FROM users WHERE city = "Pune"' "$U"
assert_exit "string in double quotes" 1
assert_contains "string in double quotes: hint" "strings take single quotes: 'Pune'" "$stderr"

run /dev/null "$U"
assert_exit "file in place of the query" 2
assert_contains "file in place of the query: hint" "the query comes first" "$stderr"

printf 'id\n5\n50\n99999999999999999999\n' > "$TMP/bigid.csv"
run /dev/null --json 'SELECT count(*) AS n FROM bigid' "$TMP/bigid.csv"
assert_exit "int past int64 --json" 0
assert_eq "int past int64 --json: stderr empty" "" "$stderr"
assert_contains "int past int64 --json: warning in trailer" '"warnings":["sql: ' "$stdout"

printf '\n' > "$TMP/nl"
run "$TMP/nl" 'SELECT count(*) FROM stdin'
assert_exit "lone newline on stdin is empty" 0
assert_eq "lone newline: no output" "" "$stdout$stderr"

# A typo'd column feeding a gate must fail the pipeline, not count literals.
code=0
"$VRK" sql 'SELECT id, "nme" FROM users' "$U" 2>/dev/null | "$VRK" sql --no-header 'SELECT count(*) FROM stdin' > "$TMP/out" 2> "$TMP/err" || code=$?
assert_eq "typo upstream: no rows downstream" "" "$(cat "$TMP/out")"

echo "---"
echo "Results: $PASS passed, $FAIL failed"
echo ""

[ "$FAIL" -eq 0 ]
