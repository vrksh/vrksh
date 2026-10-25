// Package sql implements vrk sql - SQL over CSV files.
// Each CSV file becomes a table in an in-memory SQLite database, named after
// the file. Piped CSV is the table "stdin". Results come out as CSV, or as
// JSONL with --json.
//
// Column types are inferred: INTEGER or REAL when every non-empty value is a
// number (NULL markers like N/A load as NULL), TEXT otherwise, so "007" stays
// "007". A REAL column whose values all have the same number of decimals is
// declared REAL(n), and its values print with n decimals, so 19.90 stays
// 19.90. ATTACH is blocked (limit set to 0), which also blocks VACUUM INTO, so
// a query can't touch the filesystem.
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
	"unicode/utf8"

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
	nameRe  = regexp.MustCompile(`[^\p{L}\p{N}_]`)
	// looseNumRe matches digit strings intRe and realRe turn down on purpose:
	// leading zeros and integers past int64. Those are codes, not numbers.
	looseNumRe = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
	identRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	noTableRe  = regexp.MustCompile(`no such table: (\S+)`)
	dqRe       = regexp.MustCompile(`"((?:[^"]|"")*)"`)
	asRe       = regexp.MustCompile(`(?i)\bas\s*$`)
	scaleRe    = regexp.MustCompile(`^REAL\(([0-9]+)\)$`)
)

// nullMarkers load as NULL in a numeric column. In a TEXT column they stay text.
var nullMarkers = map[string]bool{"na": true, "n/a": true, "null": true, "none": true, "nan": true, "-": true}

// table is one CSV source waiting to be loaded.
type table struct {
	name, source string // source is the file path, or "stdin"
	stdin        bool
	comma        rune
	data         []byte
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "sql",
		Short: "SQL over CSV files - SQLite in memory, CSV or JSONL out",
		Flags: []shared.FlagMeta{
			{Name: "query", Usage: "SQL query; every positional argument is then a CSV file"},
			{Name: "no-header", Usage: "omit the header row from CSV output"},
			{Name: "delimiter", Shorthand: "d", Usage: `input field separator, one character or \t (default "," or tab for .tsv)`},
			{Name: "json", Shorthand: "j", Usage: `one JSON object per row, then {"_vrk":"sql","count":N}`},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk sql. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("sql", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var queryFlag, delimFlag string
	var noHeader, jsonFlag, quietFlag bool
	fs.StringVar(&queryFlag, "query", "", "SQL query; every positional argument is then a CSV file")
	fs.BoolVar(&noHeader, "no-header", false, "omit the header row from CSV output")
	fs.StringVarP(&delimFlag, "delimiter", "d", "", `input field separator, one character or \t (default "," or tab for .tsv)`)
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
	var comma rune
	if delimFlag != "" {
		var ok bool
		if comma, ok = parseDelimiter(delimFlag); !ok {
			return fail(2, "sql: --delimiter must be one character other than a quote or newline, got %q", delimFlag)
		}
	}
	delimFor := func(path string) rune {
		if comma != 0 {
			return comma
		}
		if strings.EqualFold(filepath.Ext(path), ".tsv") {
			return '\t'
		}
		return ','
	}

	// Table names first, so a clash fails before anything is read.
	tables := make([]table, 0, len(files)+1)
	taken := map[string]bool{}
	stdinAt := -1
	addTable := func(t table) bool {
		if taken[strings.ToLower(t.name)] {
			return false
		}
		taken[strings.ToLower(t.name)] = true
		if t.stdin {
			stdinAt = len(tables)
		}
		tables = append(tables, t)
		return true
	}
	for _, f := range files {
		name, path := "", f
		// name=path, unless the whole argument is a real file.
		if i := strings.IndexByte(f, '='); i > 0 && !strings.ContainsAny(f[:i], `/\.`) {
			if _, err := os.Stat(f); err != nil {
				name, path = f[:i], f[i+1:]
				if !identRe.MatchString(name) {
					return fail(2, "sql: invalid table name: %s (letters, digits and _, not starting with a digit)", name)
				}
			}
		}
		t := table{name: name, source: path, comma: delimFor(path)}
		if path == "-" {
			if stdinAt >= 0 {
				return fail(2, "sql: stdin can only be one table")
			}
			t.stdin, t.source = true, "stdin"
			if t.name == "" {
				t.name = "stdin"
			}
		} else if t.name == "" {
			t.name = tableName(path)
		}
		if !addTable(t) {
			return fail(2, "sql: duplicate table name: %s", t.name)
		}
	}
	// Without -, stdin is the table "stdin" when the query mentions it.
	implicit := stdinAt < 0 && stdinRe.MatchString(query)
	if implicit && !addTable(table{name: "stdin", source: "stdin", stdin: true, comma: delimFor("")}) {
		return fail(2, "sql: duplicate table name: stdin")
	}

	// Empty stdin drops its table. If the query then needs it, there is
	// nothing to report: exit 0 with no output.
	emptyStdin, ttyStdin := "", false
	if stdinAt >= 0 {
		var data []byte
		if isTerminal(int(os.Stdin.Fd())) {
			switch {
			case !implicit:
				return fail(2, "sql: - reads stdin but nothing is piped")
			case len(tables) == 1:
				return fail(2, "sql: query references stdin but nothing is piped")
			}
			// "stdin" may only be a word in the query, so run it on the files
			// and complain only if it really needed the stdin table.
			ttyStdin = true
		} else {
			var err error
			if data, err = readAll(os.Stdin); err != nil {
				return fail(1, "sql: reading stdin: %v", err)
			}
		}
		if len(data) == 0 && !ttyStdin {
			emptyStdin = tables[stdinAt].name
		}
		if len(data) == 0 {
			tables = append(tables[:stdinAt], tables[stdinAt+1:]...)
		} else {
			tables[stdinAt].data = data
		}
	}

	// ponytail: whole files are read into memory before loading; stream rows
	// from csv.Reader if CSVs bigger than RAM show up.
	for i := range tables {
		if tables[i].stdin {
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

	renamed := map[string]string{} // original header -> column name, where they differ
	for _, t := range tables {
		if err := load(ctx, conn, t, !jsonFlag, renamed); err != nil {
			return fail(1, "%v", err)
		}
	}
	// SQLite reads a double-quoted name that isn't a column as a string, so
	// "First Name" would quietly select the text First Name on every row.
	// An AS "First Name" alias is fine.
	for _, m := range dqRe.FindAllStringSubmatchIndex(query, -1) {
		name := strings.ReplaceAll(query[m[2]:m[3]], `""`, `"`)
		if col, ok := renamed[name]; ok && !asRe.MatchString(query[:m[0]]) {
			return fail(1, "sql: no column %s: headers are sanitized, use %s", query[m[0]:m[1]], col)
		}
	}

	out, err := runQuery(ctx, conn, query, jsonFlag, noHeader)
	if err != nil {
		if m := noTableRe.FindStringSubmatch(err.Error()); m != nil {
			switch {
			case ttyStdin && strings.EqualFold(m[1], "stdin"):
				return fail(2, "sql: query references stdin but nothing is piped")
			case emptyStdin != "" && strings.EqualFold(m[1], emptyStdin):
				return 0
			case len(tables) == 0 && emptyStdin == "":
				return fail(2, "sql: no input files; provide CSV file paths as arguments")
			}
			names := make([]string, len(tables))
			for i, t := range tables {
				names[i] = t.name
			}
			if len(names) == 0 {
				names = []string{"none loaded"}
			}
			return fail(1, "sql: no such table: %s (tables: %s)", m[1], strings.Join(names, ", "))
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

// load creates table t from its CSV data in one transaction. warn reports
// mostly-numeric columns that had to be TEXT. Headers that sanitizing changed
// are added to renamed.
func load(ctx context.Context, conn *dbsql.Conn, t table, warn bool, renamed map[string]string) error {
	data := bytes.TrimPrefix(t.data, []byte("\xef\xbb\xbf"))
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		return fmt.Errorf("sql: %s is gzip-compressed; pipe it in: gunzip -c %s | vrk sql 'SELECT ... FROM stdin' -", t.source, t.source)
	}
	if !utf8.Valid(data) {
		i := 0
		for i < len(data) {
			r, n := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && n <= 1 {
				break
			}
			i += n
		}
		return fmt.Errorf("sql: %s: line %d is not valid UTF-8; convert it first, e.g. iconv -f latin1 -t utf-8", t.source, bytes.Count(data[:i], []byte("\n"))+1)
	}
	// Old Mac line endings: encoding/csv only splits on \n.
	if bytes.IndexByte(data, '\n') < 0 {
		data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = t.comma
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	parseErr := func(format string, args ...any) error {
		return fmt.Errorf("sql: CSV parse error in %s: %s", t.source, fmt.Sprintf(format, args...))
	}
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return parseErr("empty file, no header row")
	}
	if err != nil {
		return parseErr("%v", err)
	}

	cols := sanitizeNames(header)
	for i, h := range header {
		if h != cols[i] {
			renamed[h] = cols[i]
		}
	}
	var rows [][]string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return parseErr("%v", err)
		}
		// Short rows are padded with NULLs; a long row is only an error when
		// the extra fields hold something (a trailing comma is fine).
		for len(rec) > len(cols) && rec[len(rec)-1] == "" {
			rec = rec[:len(rec)-1]
		}
		if len(rec) > len(cols) {
			line, _ := r.FieldPos(0)
			return parseErr("line %d: %d fields, header has %d", line, len(rec), len(cols))
		}
		for len(rec) < len(cols) {
			rec = append(rec, "")
		}
		rows = append(rows, rec)
	}

	types := make([]string, len(cols))
	defs := make([]string, len(cols))
	for c := range cols {
		vals := make([]string, len(rows))
		for r := range rows {
			vals[r] = rows[r][c]
		}
		var junk int
		types[c], junk = inferType(vals)
		if warn && junk >= 0 {
			shared.Warn("sql: %s: column %s is TEXT: row %d has %q; use CAST(%s AS REAL) to compare it as a number", t.source, cols[c], junk+1, vals[junk], cols[c])
		}
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
			// Empty cells are NULL, and so are N/A and friends in numeric
			// columns. Column affinity converts the rest.
			if types[c] != "TEXT" {
				v = strings.TrimSpace(v)
			}
			if v == "" || types[c] != "TEXT" && nullMarkers[strings.ToLower(v)] {
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
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	// A column read straight from a REAL(n) table column prints n decimals.
	scales := make([]int, len(cols))
	for i, ct := range colTypes {
		scales[i] = -1
		if m := scaleRe.FindStringSubmatch(ct.DatabaseTypeName()); m != nil {
			scales[i], _ = strconv.Atoi(m[1])
		}
	}
	// Statements like DELETE have no result columns: nothing to print.
	if len(cols) == 0 {
		return nil, rows.Err()
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	// Repeated names get _1, _2 in both formats, so they agree.
	names := dedupe(cols)
	keys := make([]string, len(cols))
	if jsonOut {
		for i, k := range names {
			keys[i] = jsonString(k)
		}
	} else if !noHeader {
		writeCSVRow(w, &buf, names)
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
				buf.WriteString(jsonValue(v, scales[i]))
			}
			buf.WriteString("}\n")
			continue
		}
		for i, v := range vals {
			cells[i] = csvValue(v, scales[i])
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

// formatFloat prints x with scale decimals, or as short as it round-trips
// when scale is -1. Like JSON, exponents are only for very big or very small
// numbers.
func formatFloat(x float64, scale int) string {
	if scale >= 0 {
		return strconv.FormatFloat(x, 'f', scale, 64)
	}
	if a := math.Abs(x); a == 0 || a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return strconv.FormatFloat(x, 'g', -1, 64)
}

func csvValue(v any, scale int) string {
	switch x := v.(type) {
	case nil:
		return ""
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return formatFloat(x, scale)
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

func jsonValue(v any, scale int) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return "null"
		}
		return formatFloat(x, scale)
	default:
		return jsonString(csvValue(x, -1))
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
// other than letters, digits and _ become _, empty names become col_N (1-indexed), and
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
// and NULL markers are ignored; a column with no numbers is TEXT. Leading
// zeros, exponents and integers past int64 make it TEXT so nothing is
// silently changed. Decimals that all have n places give REAL(n).
//
// junk is the index of the first value that pushed a mostly-numeric column to
// TEXT, worth a warning, or -1.
func inferType(vals []string) (typ string, junk int) {
	nums, others, junk := 0, 0, -1
	isInt, scale := true, -2 // scale -2: nothing seen yet, -1: mixed
	for i, v := range vals {
		// Padded exports (" 42") are still numbers.
		v = strings.TrimSpace(v)
		if v == "" || nullMarkers[strings.ToLower(v)] {
			continue
		}
		places := 0
		switch {
		case intRe.MatchString(v) && fitsInt64(v):
		case realRe.MatchString(v):
			isInt = false
			places = len(v) - strings.IndexByte(v, '.') - 1
		default:
			others++
			if junk < 0 && !looseNumRe.MatchString(v) {
				junk = i
			}
			continue
		}
		nums++
		if scale == -2 {
			scale = places
		} else if scale != places {
			scale = -1
		}
	}
	switch {
	case others > 0 || nums == 0:
		if nums <= others {
			junk = -1
		}
		return "TEXT", junk
	case isInt:
		return "INTEGER", -1
	case scale > 0:
		return "REAL(" + strconv.Itoa(scale) + ")", -1
	default:
		return "REAL", -1
	}
}

func fitsInt64(v string) bool {
	_, err := strconv.ParseInt(v, 10, 64)
	return err == nil
}

// parseDelimiter accepts one character, or \t or "tab" for a tab.
func parseDelimiter(s string) (rune, bool) {
	if s == `\t` || strings.EqualFold(s, "tab") {
		return '\t', true
	}
	r := []rune(s)
	if len(r) != 1 || r[0] == '"' || r[0] == '\n' || r[0] == '\r' || r[0] == utf8.RuneError {
		return 0, false
	}
	return r[0], true
}

// Flags returns flag metadata for schema generation.
// This FlagSet is never used for parsing - Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("sql", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("query", "", "SQL query; every positional argument is then a CSV file")
	fs.Bool("no-header", false, "omit the header row from CSV output")
	fs.StringP("delimiter", "d", "", `input field separator, one character or \t (default "," or tab for .tsv)`)
	fs.BoolP("json", "j", false, `one JSON object per row, then {"_vrk":"sql","count":N}`)
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk sql [flags] 'SELECT ...' [file.csv | name=file.csv | - ...]",
		"       vrk sql --query 'SELECT ...' file.csv ...",
		"       cat file.csv | vrk sql 'SELECT * FROM stdin'",
		"",
		"SQL over CSV files, in an in-memory SQLite database. Each file is a table",
		"named after its basename (users.csv -> users), or name=file.csv names it.",
		"- is stdin, as the table stdin (or name=-). Without -, piped CSV is read",
		"when the query mentions stdin. .tsv files are tab-separated.",
		"The first row is the header. Column types are inferred: INTEGER or REAL when",
		"every non-empty value is a number (N/A, null, - load as NULL), TEXT otherwise",
		"(so 007 stays 007). Empty cells are NULL.",
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
