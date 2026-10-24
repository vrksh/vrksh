// Package sql implements vrk sql - SQL over CSV files.
// Each CSV file becomes a table in an in-memory SQLite database, named after
// the file. Piped CSV is the table "stdin". Results come out as CSV, or as
// JSONL with --json.
//
// Column types are inferred: INTEGER or REAL when every non-empty value fits,
// TEXT otherwise, so "007" stays "007". ATTACH is blocked (limit set to 0),
// which also blocks VACUUM INTO, so a query can't touch the filesystem.
package sql

import (
	"bytes"
	"context"
	dbsql "database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

var (
	stdinRe = regexp.MustCompile(`(?i)\bstdin\b`)
	intRe   = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	realRe  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)\.[0-9]+$`)
	nameRe  = regexp.MustCompile(`[^A-Za-z0-9_]`)
)

// table is one CSV source waiting to be loaded.
type table struct {
	name, source string // source is the file path, or "stdin"
	data         []byte
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "sql",
		Short: "SQL over CSV files - SQLite in memory, CSV or JSONL out",
		Flags: []shared.FlagMeta{
			{Name: "query", Usage: "SQL query; every positional argument is then a CSV file"},
			{Name: "no-header", Usage: "omit the header row from CSV output"},
			{Name: "json", Shorthand: "j", Usage: `one JSON object per row, then {"_vrk":"sql","count":N}`},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk sql. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("sql", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var queryFlag string
	var noHeader, jsonFlag, quietFlag bool
	fs.StringVar(&queryFlag, "query", "", "SQL query; every positional argument is then a CSV file")
	fs.BoolVar(&noHeader, "no-header", false, "omit the header row from CSV output")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `one JSON object per row, then {"_vrk":"sql","count":N}`)
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quietFlag)()

	fail := func(code int, format string, args ...any) int {
		msg := fmt.Sprintf(format, args...)
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": msg, "code": code})
		}
		if code == 2 {
			return shared.UsageErrorf("%s", msg)
		}
		return shared.Errorf("%s", msg)
	}

	query, files := queryFlag, fs.Args()
	if !fs.Changed("query") && len(files) > 0 {
		query, files = files[0], files[1:]
	}
	if strings.TrimSpace(query) == "" {
		return fail(2, `sql: no query: usage: vrk sql 'SELECT ...' file.csv`)
	}
	usesStdin := stdinRe.MatchString(query)

	// Table names first, so a clash fails before anything is read.
	tables := make([]table, 0, len(files)+1)
	taken := map[string]bool{}
	addTable := func(name, source string) bool {
		if taken[strings.ToLower(name)] {
			return false
		}
		taken[strings.ToLower(name)] = true
		tables = append(tables, table{name: name, source: source})
		return true
	}
	for _, f := range files {
		if name := tableName(f); !addTable(name, f) {
			return fail(2, "sql: duplicate table name: %s", name)
		}
	}
	if usesStdin && !addTable("stdin", "stdin") {
		return fail(2, "sql: duplicate table name: stdin")
	}

	// Stdin is only read when the query names it.
	if usesStdin {
		if isTerminal(int(os.Stdin.Fd())) {
			return fail(2, "sql: query references stdin but nothing is piped")
		}
		data, err := readAll(os.Stdin)
		if err != nil {
			return fail(1, "sql: reading stdin: %v", err)
		}
		if len(data) == 0 {
			return 0
		}
		tables[len(tables)-1].data = data
	}

	// ponytail: whole files are read into memory before loading; stream rows
	// from csv.Reader if CSVs bigger than RAM show up.
	for i := range tables {
		if tables[i].source == "stdin" {
			continue
		}
		data, err := os.ReadFile(tables[i].source)
		if errors.Is(err, os.ErrNotExist) {
			return fail(1, "sql: file not found: %s", tables[i].source)
		}
		if err != nil {
			return fail(1, "sql: reading %s: %v", tables[i].source, err)
		}
		tables[i].data = data
	}

	ctx := context.Background()
	db, err := dbsql.Open("sqlite", ":memory:")
	if err != nil {
		return fail(1, "sql: opening database: %v", err)
	}
	defer func() { _ = db.Close() }()
	// Each connection is its own :memory: database, so everything runs on one.
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return fail(1, "sql: opening database: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := sqlite.Limit(conn, sqlite3.SQLITE_LIMIT_ATTACHED, 0); err != nil {
		return fail(1, "sql: opening database: %v", err)
	}

	for _, t := range tables {
		if err := load(ctx, conn, t); err != nil {
			return fail(1, "%v", err)
		}
	}

	out, err := runQuery(ctx, conn, query, jsonFlag, noHeader)
	if err != nil {
		if len(files) == 0 && !usesStdin && strings.Contains(err.Error(), "no such table") {
			return fail(2, "sql: no input files; provide CSV file paths as arguments")
		}
		return fail(1, "sql: %v", err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return shared.Errorf("sql: writing output: %v", err)
	}
	return 0
}

// tableName is the file's basename without its last extension, sanitized.
func tableName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		name = base
	}
	return nameRe.ReplaceAllString(name, "_")
}

// load creates table t from its CSV data in one transaction.
func load(ctx context.Context, conn *dbsql.Conn, t table) error {
	data := bytes.TrimPrefix(t.data, []byte("\xef\xbb\xbf"))
	recs, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return fmt.Errorf("sql: CSV parse error in %s: %v", t.source, err)
	}
	if len(recs) == 0 {
		return fmt.Errorf("sql: CSV parse error in %s: empty file, no header row", t.source)
	}

	cols := sanitizeNames(recs[0])
	rows := recs[1:]
	types := make([]string, len(cols))
	defs := make([]string, len(cols))
	for c := range cols {
		vals := make([]string, len(rows))
		for r := range rows {
			vals[r] = rows[r][c]
		}
		types[c] = inferType(vals)
		defs[c] = quote(cols[c]) + " " + types[c]
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sql: loading %s: %v", t.source, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("CREATE TABLE %s (%s)", quote(t.name), strings.Join(defs, ", "))); err != nil {
		return fmt.Errorf("sql: loading %s: %v", t.source, err)
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	ins, err := tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s VALUES (%s)", quote(t.name), marks))
	if err != nil {
		return fmt.Errorf("sql: loading %s: %v", t.source, err)
	}
	defer func() { _ = ins.Close() }()
	args := make([]any, len(cols))
	for _, row := range rows {
		for c, v := range row {
			// Empty numeric cells are NULL; column affinity converts the rest.
			if v == "" && types[c] != "TEXT" {
				args[c] = nil
			} else {
				args[c] = v
			}
		}
		if _, err := ins.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("sql: loading %s: %v", t.source, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sql: loading %s: %v", t.source, err)
	}
	return nil
}

// runQuery runs query and returns the formatted output. Rows are buffered:
// SQLite can fail partway through a result set, and stdout must stay empty
// on error.
func runQuery(ctx context.Context, conn *dbsql.Conn, query string, jsonOut, noHeader bool) ([]byte, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// Statements like DELETE have no result columns: nothing to print.
	if len(cols) == 0 {
		return nil, rows.Err()
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	keys := make([]string, len(cols))
	if jsonOut {
		for i, k := range dedupe(cols) {
			keys[i] = jsonString(k)
		}
	} else if !noHeader {
		writeCSVRow(w, &buf, cols)
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	cells := make([]string, len(cols))
	count := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		count++
		if jsonOut {
			buf.WriteByte('{')
			for i, v := range vals {
				if i > 0 {
					buf.WriteByte(',')
				}
				buf.WriteString(keys[i])
				buf.WriteByte(':')
				buf.WriteString(jsonValue(v))
			}
			buf.WriteString("}\n")
			continue
		}
		for i, v := range vals {
			cells[i] = csvValue(v)
		}
		writeCSVRow(w, &buf, cells)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if jsonOut {
		fmt.Fprintf(&buf, "{\"_vrk\":\"sql\",\"count\":%d}\n", count)
	}
	return buf.Bytes(), nil
}

// writeCSVRow writes one record. A lone empty field is written as "" so it
// doesn't come out as a blank line, which CSV readers skip.
func writeCSVRow(w *csv.Writer, buf *bytes.Buffer, rec []string) {
	if len(rec) == 1 && rec[0] == "" {
		buf.WriteString("\"\"\n")
		return
	}
	// Writes into a bytes.Buffer can't fail.
	_ = w.Write(rec)
	w.Flush()
}

func csvValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

func jsonValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return "null"
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return jsonString(csvValue(x))
	}
}

// jsonString encodes s without json.Marshal's HTML escaping of <, > and &.
func jsonString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return strings.TrimSuffix(b.String(), "\n")
}

// quote makes name a SQLite identifier.
func quote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// sanitizeNames turns a CSV header into usable column names: characters
// outside [A-Za-z0-9_] become _, empty names become col_N (1-indexed), and
// case-insensitive duplicates get _1, _2 suffixes.
func sanitizeNames(header []string) []string {
	names := make([]string, len(header))
	for i, h := range header {
		names[i] = nameRe.ReplaceAllString(strings.TrimSpace(h), "_")
		if names[i] == "" {
			names[i] = "col_" + strconv.Itoa(i+1)
		}
	}
	return dedupe(names)
}

// dedupe suffixes case-insensitive repeats with _1, _2, ..., skipping any
// suffixed name that is already taken.
func dedupe(names []string) []string {
	out := make([]string, len(names))
	taken := map[string]bool{}
	for i, n := range names {
		cand := n
		for j := 1; taken[strings.ToLower(cand)]; j++ {
			cand = n + "_" + strconv.Itoa(j)
		}
		taken[strings.ToLower(cand)] = true
		out[i] = cand
	}
	return out
}

// inferType picks a SQLite column type from a column's values. Empty values
// are ignored; an all-empty column is TEXT. Leading zeros, exponents and
// integers past int64 stay TEXT so nothing is silently changed.
func inferType(vals []string) string {
	isInt, isReal, seen := true, true, false
	for _, v := range vals {
		if v == "" {
			continue
		}
		seen = true
		if intRe.MatchString(v) {
			if _, err := strconv.ParseInt(v, 10, 64); err == nil {
				continue
			}
		}
		isInt = false
		if !realRe.MatchString(v) {
			isReal = false
			break
		}
	}
	switch {
	case !seen:
		return "TEXT"
	case isInt:
		return "INTEGER"
	case isReal:
		return "REAL"
	default:
		return "TEXT"
	}
}

// Flags returns flag metadata for schema generation.
// This FlagSet is never used for parsing - Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("sql", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("query", "", "SQL query; every positional argument is then a CSV file")
	fs.Bool("no-header", false, "omit the header row from CSV output")
	fs.BoolP("json", "j", false, `one JSON object per row, then {"_vrk":"sql","count":N}`)
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk sql [flags] 'SELECT ...' [file.csv ...]",
		"       vrk sql --query 'SELECT ...' file.csv ...",
		"       cat file.csv | vrk sql 'SELECT * FROM stdin'",
		"",
		"SQL over CSV files, in an in-memory SQLite database. Each file is a table",
		"named after its basename (users.csv -> users); piped CSV is the table stdin.",
		"The first row is the header. Column types are inferred: INTEGER or REAL when",
		"every non-empty value is a number, TEXT otherwise (so 007 stays 007).",
		"Output is CSV with a header row, or JSONL with --json.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("sql: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
