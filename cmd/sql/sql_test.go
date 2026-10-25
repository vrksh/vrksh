package sql

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// runSql replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe - tests share
// global OS state.
func runSql(t *testing.T, args []string, stdin string) (stdout, stderr string, code int) {
	t.Helper()

	origStdin, origStdout, origStderr, origArgs := os.Stdin, os.Stdout, os.Stderr, os.Args
	t.Cleanup(func() {
		os.Stdin, os.Stdout, os.Stderr, os.Args = origStdin, origStdout, origStderr, origArgs
	})

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	// Write in a goroutine so inputs bigger than the pipe buffer don't block.
	go func() {
		_, _ = stdinW.Write([]byte(stdin))
		_ = stdinW.Close()
	}()
	os.Stdin = stdinR

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = stdoutW, stderrW

	var outBuf, errBuf bytes.Buffer
	outDone, errDone := make(chan struct{}), make(chan struct{})
	go func() { _, _ = io.Copy(&outBuf, stdoutR); close(outDone) }()
	go func() { _, _ = io.Copy(&errBuf, stderrR); close(errDone) }()

	os.Args = append([]string{"sql"}, args...)
	code = Run()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	<-outDone
	<-errDone
	_ = stdinR.Close()
	return outBuf.String(), errBuf.String(), code
}

const (
	usersCSV  = "id,name,age,city\n1,ana,30,Pune\n2,bo,100,Delhi\n3,cy,9,Pune\n"
	ordersCSV = "id,user_id,total\n10,1,25.5\n11,1,4.5\n12,3,100\n"
)

// writeCSV writes content to dir/name and returns the path.
func writeCSV(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func usersFile(t *testing.T) string {
	t.Helper()
	return writeCSV(t, t.TempDir(), "users.csv", usersCSV)
}

func parseJSONL(t *testing.T, s string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if line == "" {
			continue
		}
		d := json.NewDecoder(strings.NewReader(line))
		d.UseNumber()
		var m map[string]any
		if err := d.Decode(&m); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func assertJSONError(t *testing.T, stdout, stderr string, code, want int) {
	t.Helper()
	if code != want {
		t.Errorf("exit = %d, want %d", code, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty under --json", stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("stdout is not JSON: %q", stdout)
	}
	if !strings.HasPrefix(fmt.Sprint(m["error"]), "sql: ") {
		t.Errorf("error = %v, want sql: prefix", m["error"])
	}
	if fmt.Sprint(m["code"]) != fmt.Sprint(want) {
		t.Errorf("code = %v, want %d", m["code"], want)
	}
}

func TestSanitizeNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"first name,last-name,a.b,f(x),pct%", "first_name,last_name,a_b,f_x_,pct_"},
		{"a,,c", "a,col_2,c"},
		{"a, ,c", "a,col_2,c"},
		{"name,name", "name,name_1"},
		{"Name,name", "Name,name_1"},
		{"name,name,name_1", "name,name_1,name_1_1"},
		{"x,x,x", "x,x_1,x_2"},
		{"2024_total", "2024_total"},
	}
	for _, c := range cases {
		got := strings.Join(sanitizeNames(strings.Split(c.in, ",")), ",")
		if got != c.want {
			t.Errorf("sanitizeNames(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInferType(t *testing.T) {
	cases := []struct {
		vals []string
		want string
		junk int
	}{
		{[]string{"1", "2", "-3"}, "INTEGER", -1},
		{[]string{"0", "-0"}, "INTEGER", -1},
		{[]string{"1.5", "2"}, "REAL", -1},
		{[]string{"1.5", "2.25"}, "REAL", -1},
		{[]string{"-0.25"}, "REAL(2)", -1},
		{[]string{"19.90", "5.00", ""}, "REAL(2)", -1},
		{[]string{"007", "8"}, "TEXT", -1},
		{[]string{"007.5"}, "TEXT", -1},
		{[]string{"1e3"}, "TEXT", -1},
		{[]string{"1", "", "3"}, "INTEGER", -1},
		{[]string{"", ""}, "TEXT", -1},
		{[]string{}, "TEXT", -1},
		{[]string{"99999999999999999999"}, "TEXT", -1},
		{[]string{"1.5", "99999999999999999999"}, "TEXT", -1},
		{[]string{" 30", "7 "}, "INTEGER", -1},
		{[]string{"1.", ".5"}, "TEXT", -1},
		{[]string{"5", "N/A", "2", "null", "-", "NA", "none"}, "INTEGER", -1},
		{[]string{"n/a", "-"}, "TEXT", -1},
		{[]string{"5", "abc", "7"}, "TEXT", 1},
		{[]string{"abc", "def", "5"}, "TEXT", -1},
	}
	for _, c := range cases {
		if got, junk := inferType(c.vals); got != c.want || junk != c.junk {
			t.Errorf("inferType(%q) = %s, %d, want %s, %d", c.vals, got, junk, c.want, c.junk)
		}
	}
}

func TestPositionalQuery(t *testing.T) {
	f := usersFile(t)
	out, errOut, code := runSql(t, []string{"SELECT name, city FROM users ORDER BY id", f}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	want := "name,city\nana,Pune\nbo,Delhi\ncy,Pune\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

func TestQueryFlagMakesEveryPositionalAFile(t *testing.T) {
	dir := t.TempDir()
	u := writeCSV(t, dir, "users.csv", usersCSV)
	o := writeCSV(t, dir, "orders.csv", ordersCSV)
	out, errOut, code := runSql(t, []string{"--query", "SELECT count(*) AS n FROM users, orders", u, o}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if out != "n\n9\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestJoin(t *testing.T) {
	dir := t.TempDir()
	u := writeCSV(t, dir, "users.csv", usersCSV)
	o := writeCSV(t, dir, "orders.csv", ordersCSV)
	q := "SELECT u.name, sum(o.total) AS spent FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.name ORDER BY spent DESC"
	out, errOut, code := runSql(t, []string{q, u, o}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if out != "name,spent\ncy,100\nana,30\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestTableNames(t *testing.T) {
	dir := t.TempDir()
	a := writeCSV(t, dir, "sales-2024.csv", "x\n1\n")
	b := writeCSV(t, dir, "my data.v2.csv", "y\n2\n")
	out, errOut, code := runSql(t, []string{"SELECT x, y FROM sales_2024, my_data_v2", a, b}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if out != "x,y\n1,2\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestDuplicateTableName(t *testing.T) {
	dir := t.TempDir()
	a := writeCSV(t, dir, "a/data.csv", "x\n1\n")
	b := writeCSV(t, dir, "b/Data.csv", "x\n2\n")
	out, errOut, code := runSql(t, []string{"SELECT * FROM data", a, b}, "")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "sql: duplicate table name: Data") {
		t.Errorf("stderr = %q", errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

func TestNumericColumns(t *testing.T) {
	f := usersFile(t)
	cases := []struct{ q, want string }{
		{"SELECT age FROM users WHERE age > 25 ORDER BY age", "age\n30\n100\n"},
		{"SELECT age FROM users ORDER BY age", "age\n9\n30\n100\n"},
		{"SELECT max(age) AS m FROM users", "m\n100\n"},
	}
	for _, c := range cases {
		out, errOut, code := runSql(t, []string{c.q, f}, "")
		if code != 0 || out != c.want {
			t.Errorf("%s: exit %d stdout %q stderr %q, want %q", c.q, code, out, errOut, c.want)
		}
	}
}

func TestLeadingZerosAndNulls(t *testing.T) {
	dir := t.TempDir()
	f := writeCSV(t, dir, "zips.csv", "zip,pop\n007,10\n110001,\n")
	out, errOut, code := runSql(t, []string{"SELECT zip, pop IS NULL AS missing, typeof(pop) AS t FROM zips", f}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if out != "zip,missing,t\n007,0,integer\n110001,1,null\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestNoHeaderNullAndFloat(t *testing.T) {
	out, errOut, code := runSql(t, []string{"--no-header", "SELECT NULL, 2.5, 'a,b'"}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if out != ",2.5,\"a,b\"\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestEmptyResult(t *testing.T) {
	f := usersFile(t)
	q := "SELECT name FROM users WHERE age > 1000"

	out, _, code := runSql(t, []string{q, f}, "")
	if code != 0 || out != "name\n" {
		t.Errorf("csv: exit %d stdout %q, want header only", code, out)
	}
	out, _, code = runSql(t, []string{"--no-header", q, f}, "")
	if code != 0 || out != "" {
		t.Errorf("--no-header: exit %d stdout %q, want empty", code, out)
	}
	out, _, code = runSql(t, []string{"--json", q, f}, "")
	if code != 0 || out != "{\"_vrk\":\"sql\",\"count\":0}\n" {
		t.Errorf("--json: exit %d stdout %q, want trailer only", code, out)
	}
}

func TestStatementWithNoColumns(t *testing.T) {
	f := usersFile(t)
	out, errOut, code := runSql(t, []string{"DELETE FROM users", f}, "")
	if code != 0 || out != "" {
		t.Errorf("exit %d stdout %q stderr %q, want exit 0 and no output", code, out, errOut)
	}
}

func TestJSONOutput(t *testing.T) {
	f := usersFile(t)
	out, errOut, code := runSql(t, []string{"--json", "SELECT name, id, age * 0.5 AS half, NULL AS gone FROM users WHERE id = 1", f}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	want := "{\"name\":\"ana\",\"id\":1,\"half\":15,\"gone\":null}\n{\"_vrk\":\"sql\",\"count\":1}\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestJSONKeepsColumnOrder(t *testing.T) {
	out, _, code := runSql(t, []string{"-j", "SELECT 1 AS zeta, 2 AS alpha, 3 AS mid"}, "")
	if code != 0 || !strings.HasPrefix(out, "{\"zeta\":1,\"alpha\":2,\"mid\":3}\n") {
		t.Errorf("exit %d stdout %q", code, out)
	}
}

func TestJSONDuplicateColumns(t *testing.T) {
	f := usersFile(t)
	out, _, code := runSql(t, []string{"--json", "SELECT id, id, name AS ID FROM users WHERE id = 2", f}, "")
	if code != 0 || !strings.HasPrefix(out, "{\"id\":2,\"id_1\":2,\"ID_2\":\"bo\"}\n") {
		t.Errorf("exit %d stdout %q", code, out)
	}
	// The CSV header uses the same names, so both outputs agree.
	out, _, _ = runSql(t, []string{"SELECT id, id FROM users WHERE id = 2", f}, "")
	if out != "id,id_1\n2,2\n" {
		t.Errorf("csv stdout %q", out)
	}
}

func TestJSONNonFiniteAndBlob(t *testing.T) {
	out, _, code := runSql(t, []string{"--json", "SELECT 1e999 AS inf, x'4142' AS b"}, "")
	if code != 0 || !strings.HasPrefix(out, "{\"inf\":null,\"b\":\"AB\"}\n") {
		t.Errorf("exit %d stdout %q", code, out)
	}
}

func TestNoFileQueries(t *testing.T) {
	cases := []struct{ q, want string }{
		{"SELECT 1", "1\n1\n"},
		{"SELECT date('now') IS NOT NULL AS ok", "ok\n1\n"},
		{"SELECT * FROM (SELECT 1 AS x)", "x\n1\n"},
		{"WITH c AS (SELECT 2 AS y) SELECT y FROM c", "y\n2\n"},
	}
	for _, c := range cases {
		out, errOut, code := runSql(t, []string{c.q}, "")
		if code != 0 || out != c.want {
			t.Errorf("%s: exit %d stdout %q stderr %q", c.q, code, out, errOut)
		}
	}
}

func TestNoSuchTableWithoutFiles(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT * FROM t"}, "")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "sql: no input files; provide CSV file paths as arguments") {
		t.Errorf("stderr = %q", errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q", out)
	}
}

func TestNoSuchTableWithFiles(t *testing.T) {
	dir := t.TempDir()
	u := writeCSV(t, dir, "u.csv", "a\n1\n")
	_, errOut, code := runSql(t, []string{"SELECT * FROM t", u}, "")
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "no such table: t") || strings.Contains(errOut, "no input files") {
		t.Errorf("stderr = %q, want SQLite's message unchanged", errOut)
	}
}

func TestStdinTable(t *testing.T) {
	for _, q := range []string{"SELECT name FROM stdin WHERE age < 50 ORDER BY age", "SELECT name FROM STDIN WHERE age < 50 ORDER BY age"} {
		out, errOut, code := runSql(t, []string{q}, usersCSV)
		if code != 0 || out != "name\ncy\nana\n" {
			t.Errorf("%s: exit %d stdout %q stderr %q", q, code, out, errOut)
		}
	}
}

func TestStdinJoinedWithFile(t *testing.T) {
	dir := t.TempDir()
	o := writeCSV(t, dir, "orders.csv", ordersCSV)
	q := "SELECT s.name, count(*) AS n FROM stdin s JOIN orders o ON o.user_id = s.id GROUP BY s.name ORDER BY s.name"
	out, errOut, code := runSql(t, []string{q, o}, usersCSV)
	if code != 0 || out != "name,n\nana,2\ncy,1\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestStdinReferencedOnTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	out, errOut, code := runSql(t, []string{"SELECT * FROM stdin"}, "")
	if code != 2 || out != "" || !strings.Contains(errOut, "sql: query references stdin but nothing is piped") {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, errOut, code = runSql(t, []string{"--json", "SELECT * FROM stdin"}, "")
	assertJSONError(t, out, errOut, code, 2)
}

func TestTTYWithoutStdinReferenceRuns(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	out, _, code := runSql(t, []string{"SELECT name FROM users WHERE id = 1", usersFile(t)}, "")
	if code != 0 || out != "name\nana\n" {
		t.Errorf("exit %d stdout %q", code, out)
	}
}

func TestEmptyStdinReferenced(t *testing.T) {
	for _, args := range [][]string{{"SELECT * FROM stdin"}, {"--json", "SELECT * FROM stdin"}} {
		out, errOut, code := runSql(t, args, "")
		if code != 0 || out != "" || errOut != "" {
			t.Errorf("%v: exit %d stdout %q stderr %q, want exit 0 and no output", args, code, out, errOut)
		}
	}
}

func TestStdinNotReferencedIsNotRead(t *testing.T) {
	orig := readAll
	readAll = func(io.Reader) ([]byte, error) {
		t.Error("stdin was read but the query doesn't reference it")
		return nil, nil
	}
	t.Cleanup(func() { readAll = orig })

	out, errOut, code := runSql(t, []string{"SELECT count(*) AS n FROM users", usersFile(t)}, "junk,that\nis,not used\n")
	if code != 0 || out != "n\n3\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestStdinCSVFileCollides(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "stdin.csv", "a\n1\n")
	_, errOut, code := runSql(t, []string{"SELECT * FROM stdin", f}, "a\n2\n")
	if code != 2 || !strings.Contains(errOut, "sql: duplicate table name: stdin") {
		t.Errorf("exit %d stderr %q", code, errOut)
	}
}

func TestBOMStripped(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "bom.csv", "\ufeffname,n\nx,1\n")
	out, errOut, code := runSql(t, []string{"SELECT name FROM bom", f}, "")
	if code != 0 || out != "name\nx\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestMessyHeaders(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "messy.csv", "first name,unit-price,v1.2,,Name,name\na,1,2,3,4,5\n")
	out, errOut, code := runSql(t, []string{"SELECT first_name, unit_price, v1_2, col_4, Name, name_1 FROM messy", f}, "")
	if code != 0 || out != "first_name,unit_price,v1_2,col_4,Name,name_1\na,1,2,3,4,5\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestReservedWordColumn(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "t.csv", "order,group\n1,a\n")
	out, errOut, code := runSql(t, []string{`SELECT "order", "group" FROM t`, f}, "")
	if code != 0 || out != "order,group\n1,a\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestHeaderOnly(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "h.csv", "a,b\n")
	out, errOut, code := runSql(t, []string{"SELECT count(*) AS n FROM h", f}, "")
	if code != 0 || out != "n\n0\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestMultipleStatements(t *testing.T) {
	// modernc runs every statement and returns the rows of the last one.
	out, _, code := runSql(t, []string{"SELECT 1; SELECT 2"}, "")
	if code != 0 || out != "2\n2\n" {
		t.Errorf("exit %d stdout %q", code, out)
	}
}

func TestAttachAndVacuumBlocked(t *testing.T) {
	dir := t.TempDir()
	for _, q := range []string{
		fmt.Sprintf("ATTACH DATABASE '%s' AS x", filepath.Join(dir, "x.db")),
		"ATTACH ':memory:' AS m",
		fmt.Sprintf("VACUUM INTO '%s'", filepath.Join(dir, "v.db")),
	} {
		out, errOut, code := runSql(t, []string{q}, "")
		if code != 1 || out != "" || !strings.Contains(errOut, "too many attached databases - max 0") {
			t.Errorf("%s: exit %d stdout %q stderr %q", q, code, out, errOut)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("files created in %s: %v", dir, entries)
	}
}

func TestVacuumColumnName(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "t.csv", "vacuum,attach\n1,2\n")
	out, errOut, code := runSql(t, []string{"SELECT vacuum, attach FROM t", f}, "")
	if code != 0 || out != "vacuum,attach\n1,2\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

// errorCases are runtime (1) and usage (2) errors. Each runs plain and with
// --json.
func errorCases(t *testing.T) []struct {
	name   string
	args   []string
	stdin  string
	code   int
	stderr string
} {
	dir := t.TempDir()
	users := writeCSV(t, dir, "users.csv", usersCSV)
	ragged := writeCSV(t, dir, "ragged.csv", "a,b\n1,2\n3,4,5\n")
	empty := writeCSV(t, dir, "empty.csv", "")
	a := writeCSV(t, dir, "a/data.csv", "x\n1\n")
	b := writeCSV(t, dir, "b/data.csv", "x\n1\n")
	missing := filepath.Join(dir, "nope.csv")
	gz := writeCSV(t, dir, "p.csv.gz", "\x1f\x8b\x08\x00junk")
	return []struct {
		name   string
		args   []string
		stdin  string
		code   int
		stderr string
	}{
		{"syntax error", []string{"SELEC 1"}, "", 1, `near "SELEC": syntax error`},
		{"missing file", []string{"SELECT 1", missing}, "", 1, "sql: file not found: " + missing},
		{"long row", []string{"SELECT * FROM ragged", ragged}, "", 1, "sql: CSV parse error in " + ragged + ": line 3: 3 fields, header has 2"},
		{"long row on stdin", []string{"SELECT * FROM stdin"}, "a,b\n1,2,3\n", 1, "sql: CSV parse error in stdin: line 2: 3 fields, header has 2"},
		{"gzip file", []string{"SELECT * FROM p", gz}, "", 1, "sql: " + gz + " is gzip-compressed"},
		{"bad table name", []string{"SELECT 1", "2x=" + users}, "", 2, "sql: invalid table name: 2x"},
		{"bad delimiter", []string{"--delimiter", "ab", "SELECT 1"}, "", 2, "sql: --delimiter must be one character"},
		{"empty file", []string{"SELECT * FROM empty", empty}, "", 1, "sql: CSV parse error in " + empty + ": empty file, no header row"},
		{"no such table with files", []string{"SELECT * FROM t", users}, "", 1, "no such table: t (tables: users)"},
		{"overflow mid-query", []string{"SELECT CASE WHEN id = 2 THEN abs(-9223372036854775808) ELSE id END FROM users", users}, "", 1, "integer overflow"},
		{"no such table, no files", []string{"SELECT * FROM t"}, "", 2, "sql: no input files"},
		{"duplicate table", []string{"SELECT 1", a, b}, "", 2, "sql: duplicate table name: data"},
		{"no args", nil, "", 2, "sql: no query"},
		{"empty --query", []string{"--query", "", users}, "", 2, "sql: no query"},
		{"unknown flag", []string{"--bogus", "SELECT 1"}, "", 2, "unknown flag"},
		{"--format dropped", []string{"--format", "json", "SELECT 1"}, "", 2, "unknown flag"},
	}
}

func TestErrors(t *testing.T) {
	for _, c := range errorCases(t) {
		out, errOut, code := runSql(t, c.args, c.stdin)
		if code != c.code {
			t.Errorf("%s: exit = %d, want %d (stderr %q)", c.name, code, c.code, errOut)
		}
		if out != "" {
			t.Errorf("%s: stdout = %q, want empty", c.name, out)
		}
		if !strings.Contains(errOut, c.stderr) {
			t.Errorf("%s: stderr = %q, want it to contain %q", c.name, errOut, c.stderr)
		}
	}
}

func TestErrorsJSON(t *testing.T) {
	for _, c := range errorCases(t) {
		if strings.Contains(c.stderr, "unknown flag") {
			continue // flag parsing fails before --json is known
		}
		out, errOut, code := runSql(t, append([]string{"--json"}, c.args...), c.stdin)
		t.Run(c.name, func(t *testing.T) { assertJSONError(t, out, errOut, code, c.code) })
	}
}

func TestReadErrorJSON(t *testing.T) {
	orig := readAll
	readAll = func(io.Reader) ([]byte, error) { return nil, errors.New("simulated") }
	t.Cleanup(func() { readAll = orig })

	out, errOut, code := runSql(t, []string{"--json", "SELECT * FROM stdin"}, "a\n1\n")
	assertJSONError(t, out, errOut, code, 1)
	if !strings.Contains(out, "sql: reading stdin: simulated") {
		t.Errorf("stdout = %q", out)
	}
}

func TestHelp(t *testing.T) {
	out, _, code := runSql(t, []string{"--help"}, "")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	for _, want := range []string{"vrk sql", "inferred", "--query", "--no-header", "--json"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}

func TestQuiet(t *testing.T) {
	out, errOut, code := runSql(t, []string{"--quiet", "SELEC 1"}, "")
	if code != 1 || errOut != "" || out != "" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

// TestPropertyRoundTrip: any table of text cells comes back unchanged through
// SELECT *, the --json count matches the row count, and sanitized headers are
// unique (case-insensitively) and plain identifiers.
func TestPropertyRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	headerChars := []rune("abcXYZ_ -.()%#é")
	cellChars := []rune("abc xyz,\"'")
	word := func(chars []rune, max int) string {
		n := rng.Intn(max + 1)
		r := make([]rune, n)
		for i := range r {
			r[i] = chars[rng.Intn(len(chars))]
		}
		return string(r)
	}
	ident := regexp.MustCompile(`^[\p{L}\p{N}_]+$`)

	for iter := 0; iter < 50; iter++ {
		cols := 1 + rng.Intn(5)
		rows := rng.Intn(6)
		header := make([]string, cols)
		for i := range header {
			header[i] = word(headerChars, 6)
		}

		names := sanitizeNames(header)
		seen := map[string]bool{}
		for _, n := range names {
			if !ident.MatchString(n) {
				t.Fatalf("header %q: sanitized name %q is not an identifier", header, n)
			}
			if seen[strings.ToLower(n)] {
				t.Fatalf("header %q: duplicate sanitized name %q in %q", header, n, names)
			}
			seen[strings.ToLower(n)] = true
		}

		var in bytes.Buffer
		writeRow := func(b *bytes.Buffer, cells []string) {
			for i, c := range cells {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(`"` + strings.ReplaceAll(c, `"`, `""`) + `"`)
			}
			b.WriteByte('\n')
		}
		writeRow(&in, header)
		data := make([][]string, rows)
		for r := range data {
			data[r] = make([]string, cols)
			for c := range data[r] {
				data[r][c] = word(cellChars, 5)
			}
			writeRow(&in, data[r])
		}
		f := writeCSV(t, t.TempDir(), "t.csv", in.String())

		out, errOut, code := runSql(t, []string{"--no-header", "SELECT * FROM t", f}, "")
		if code != 0 {
			t.Fatalf("iter %d: exit %d stderr %q input %q", iter, code, errOut, in.String())
		}
		got := parseCSV(t, out)
		if fmt.Sprint(got) != fmt.Sprint(data) {
			t.Fatalf("iter %d: rows = %q, want %q", iter, got, data)
		}

		out, _, _ = runSql(t, []string{"--json", "SELECT * FROM t", f}, "")
		recs := parseJSONL(t, out)
		trailer := recs[len(recs)-1]
		if fmt.Sprint(trailer["count"]) != fmt.Sprint(rows) || len(recs) != rows+1 {
			t.Fatalf("iter %d: %d records, trailer %v, want %d rows", iter, len(recs)-1, trailer, rows)
		}
	}
}

func parseCSV(t *testing.T, s string) [][]string {
	t.Helper()
	if s == "" {
		return nil
	}
	recs, err := csv.NewReader(strings.NewReader(s)).ReadAll()
	if err != nil {
		t.Fatalf("output is not CSV: %v\n%s", err, s)
	}
	return recs
}

// TestNullMarkersInNumericColumn: N/A and friends load as NULL, so the column
// stays numeric and qty > 3 compares numbers, not strings.
func TestNullMarkersInNumericColumn(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "inv.csv", "sku,qty\na,5\nb,N/A\nc,12\nd,2\ne,\n")
	out, errOut, code := runSql(t, []string{"SELECT sku FROM inv WHERE qty > 3 ORDER BY sku", f}, "")
	if code != 0 || out != "sku\na\nc\n" || errOut != "" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, _, _ = runSql(t, []string{"SELECT count(*) AS n FROM inv WHERE qty IS NULL", f}, "")
	if out != "n\n2\n" {
		t.Errorf("null count stdout %q", out)
	}
}

func TestJunkInNumericColumnWarns(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "inv.csv", "sku,qty\na,5\nb,lots\nc,12\n")
	out, errOut, code := runSql(t, []string{"SELECT sku, typeof(qty) AS t FROM inv WHERE sku = 'a'", f}, "")
	if code != 0 || out != "sku,t\na,text\n" {
		t.Errorf("exit %d stdout %q", code, out)
	}
	if !strings.Contains(errOut, `column qty is TEXT: row 2 has "lots"`) {
		t.Errorf("stderr = %q, want a warning naming the column and value", errOut)
	}
	// --json keeps stderr empty.
	_, errOut, _ = runSql(t, []string{"--json", "SELECT 1", f}, "")
	if errOut != "" {
		t.Errorf("--json stderr = %q, want empty", errOut)
	}
}

func TestDecimalsKeepTheirDigits(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "p.csv", "sku,price\na,19.90\nb,5.00\nc,0.10\n")
	out, errOut, code := runSql(t, []string{"SELECT sku, price FROM p WHERE price > 1 ORDER BY price", f}, "")
	if code != 0 || out != "sku,price\nb,5.00\na,19.90\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, _, _ = runSql(t, []string{"--json", "SELECT price, sum(price) AS total FROM p WHERE sku = 'a'", f}, "")
	if !strings.HasPrefix(out, `{"price":19.90,"total":19.9}`) {
		t.Errorf("--json stdout %q", out)
	}
	out, _, _ = runSql(t, []string{"SELECT price / 2 AS half FROM p WHERE sku = 'b'", f}, "")
	if out != "half\n2.5\n" {
		t.Errorf("arithmetic stdout %q", out)
	}
}

func TestEmptyTextCellIsNull(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "u.csv", "id,name\n1,\n2,bo\n")
	out, _, code := runSql(t, []string{"SELECT id FROM u WHERE name IS NULL", f}, "")
	if code != 0 || out != "id\n1\n" {
		t.Errorf("exit %d stdout %q", code, out)
	}
}

// TestStdinWordInLiteral: "stdin" in a string literal must not stop the query
// when stdin is empty or a terminal.
func TestStdinWordInLiteral(t *testing.T) {
	f := usersFile(t)
	q := "SELECT name FROM users WHERE name <> 'stdin' AND id = 1"
	out, errOut, code := runSql(t, []string{q, f}, "")
	if code != 0 || out != "name\nana\n" {
		t.Errorf("empty stdin: exit %d stdout %q stderr %q", code, out, errOut)
	}

	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })
	out, errOut, code = runSql(t, []string{q, f}, "")
	if code != 0 || out != "name\nana\n" {
		t.Errorf("tty: exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestEmptyStdinWithFiles(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT * FROM stdin JOIN users USING (id)", usersFile(t)}, "")
	if code != 0 || out != "" || errOut != "" {
		t.Errorf("exit %d stdout %q stderr %q, want exit 0 and no output", code, out, errOut)
	}
}

func TestDashIsStdin(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT name FROM stdin WHERE id = 2", "-"}, usersCSV)
	if code != 0 || out != "name\nbo\n" {
		t.Errorf("-: exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, errOut, code = runSql(t, []string{"SELECT name FROM people WHERE id = 2", "people=-"}, usersCSV)
	if code != 0 || out != "name\nbo\n" {
		t.Errorf("people=-: exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestNamedTable(t *testing.T) {
	dir := t.TempDir()
	f := writeCSV(t, dir, "2024-sales.csv", "x\n1\n")
	out, errOut, code := runSql(t, []string{"SELECT x FROM sales", "sales=" + f}, "")
	if code != 0 || out != "x\n1\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	// A real file whose name has = in it is still a file.
	g := writeCSV(t, dir, "a=b.csv", "y\n2\n")
	out, errOut, code = runSql(t, []string{"SELECT y FROM a_b", g}, "")
	if code != 0 || out != "y\n2\n" {
		t.Errorf("a=b.csv: exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestLazyQuotes(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "q.csv", "id,note\n1,5\" screen\n2,ok\n")
	out, errOut, code := runSql(t, []string{"SELECT note FROM q WHERE id = 1", f}, "")
	if code != 0 || out != "note\n\"5\"\" screen\"\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestShortRowsPadded(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT a, b IS NULL AS gone FROM stdin", "-"}, "a,b\n1\n2,3\n")
	if code != 0 || out != "a,gone\n1,1\n2,0\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestDelimiters(t *testing.T) {
	dir := t.TempDir()
	tsv := writeCSV(t, dir, "t.tsv", "a\tb\n1\tx,y\n")
	out, errOut, code := runSql(t, []string{"SELECT b FROM t", tsv}, "")
	if code != 0 || out != "b\n\"x,y\"\n" {
		t.Errorf(".tsv: exit %d stdout %q stderr %q", code, out, errOut)
	}
	for _, d := range []string{`\t`, "\t", "tab"} {
		out, errOut, code = runSql(t, []string{"-d", d, "SELECT b FROM stdin", "-"}, "a\tb\n1\t2\n")
		if code != 0 || out != "b\n2\n" {
			t.Errorf("-d %q: exit %d stdout %q stderr %q", d, code, out, errOut)
		}
	}
	out, errOut, code = runSql(t, []string{"--delimiter", ";", "SELECT b FROM stdin", "-"}, "a;b\n1;2,5\n")
	if code != 0 || out != "b\n\"2,5\"\n" {
		t.Errorf("; : exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestFormatFloat(t *testing.T) {
	cases := []struct {
		x    float64
		want string
	}{
		{1234567.5, "1234567.5"},
		{83393201.94, "83393201.94"},
		{0, "0"},
		{0.000001, "0.000001"},
		{1e-9, "1e-09"},
		{1e300, "1e+300"},
	}
	for _, c := range cases {
		if got := formatFloat(c.x, -1); got != c.want {
			t.Errorf("formatFloat(%v) = %q, want %q", c.x, got, c.want)
		}
	}
}

func TestTTYStdinWithFiles(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	out, errOut, code := runSql(t, []string{"SELECT * FROM stdin JOIN users USING (id)", usersFile(t)}, "")
	if code != 2 || out != "" || !strings.Contains(errOut, "nothing is piped") {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestUnicodeHeaders(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "p.csv", "Prénom,First Name\nAnaïs,ana\n")
	out, errOut, code := runSql(t, []string{"SELECT Prénom, First_Name FROM p", f}, "")
	if code != 0 || out != "Prénom,First_Name\nAnaïs,ana\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, errOut, code = runSql(t, []string{`SELECT "First Name" FROM p`, f}, "")
	if code != 1 || out != "" || !strings.Contains(errOut, "use First_Name") {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, _, code = runSql(t, []string{`SELECT First_Name AS "First Name" FROM p`, f}, "")
	if code != 0 || out != "First Name\nana\n" {
		t.Errorf("alias: exit %d stdout %q", code, out)
	}
}

func TestCROnlyLineEndings(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT sum(a) FROM stdin"}, "a,b\r1,x\r2,y\r")
	if code != 0 || out != "sum(a)\n3\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestInvalidUTF8(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT * FROM stdin"}, "name\nana\nRen\xe9\n")
	if code != 1 || out != "" || !strings.Contains(errOut, "line 3 is not valid UTF-8") {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestPaddedNumbers(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT a + 1 AS b FROM stdin"}, "a\n 41\n41 \n")
	if code != 0 || out != "b\n42\n42\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestJSONLInput(t *testing.T) {
	in := `{"id":1,"name":"ana","ok":true,"tags":["a","b"],"n":null}
{"id":2,"name":"bo","ok":false,"extra":"x"}
{"_vrk":"emit","count":2}
`
	out, errOut, code := runSql(t, []string{"SELECT * FROM stdin"}, in)
	want := "id,name,ok,tags,n,extra\n1,ana,1,\"[\"\"a\"\",\"\"b\"\"]\",,\n2,bo,0,,,x\n"
	if code != 0 || out != want {
		t.Errorf("exit %d stdout %q stderr %q, want %q", code, out, errOut, want)
	}
	out, _, code = runSql(t, []string{"SELECT sum(id) FROM stdin WHERE ok"}, in)
	if code != 0 || out != "sum(id)\n1\n" {
		t.Errorf("bool filter: exit %d stdout %q", code, out)
	}
	out, _, code = runSql(t, []string{"SELECT json_extract(tags, '$[1]') AS t FROM stdin WHERE id = 1"}, in)
	if code != 0 || out != "t\nb\n" {
		t.Errorf("nested: exit %d stdout %q", code, out)
	}
}

func TestJSONArrayAndFile(t *testing.T) {
	f := writeCSV(t, t.TempDir(), "events.jsonl", `[{"a":1.5},{"a":2}]`)
	out, errOut, code := runSql(t, []string{"SELECT sum(a) FROM events", f}, "")
	if code != 0 || out != "sum(a)\n3.5\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestJSONLErrors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"{\"a\":1}\n{\"a\":\n", "line 2: record cut off"},
		{"{\"a\":1}\n42\n", "line 2: want an object"},
	}
	for _, c := range cases {
		out, errOut, code := runSql(t, []string{"SELECT * FROM stdin"}, c.in)
		if code != 1 || out != "" || !strings.Contains(errOut, c.want) {
			t.Errorf("%q: exit %d stdout %q stderr %q, want %q", c.in, code, out, errOut, c.want)
		}
	}
	// Only a trailer means nothing matched upstream: like empty stdin, exit 0.
	for _, args := range [][]string{{"SELECT count(*) FROM stdin"}, {"--json", "SELECT * FROM stdin"}} {
		out, errOut, code := runSql(t, args, "{\"_vrk\":\"emit\",\"count\":0}\n")
		if code != 0 || out != "" || errOut != "" {
			t.Errorf("%v trailer only: exit %d stdout %q stderr %q", args, code, out, errOut)
		}
	}
	f := writeCSV(t, t.TempDir(), "none.jsonl", "{\"_vrk\":\"emit\",\"count\":0}\n")
	out, errOut, code := runSql(t, []string{"SELECT * FROM none JOIN users USING (id)", f, usersFile(t)}, "")
	if code != 0 || out != "" || errOut != "" {
		t.Errorf("trailer-only file: exit %d stdout %q stderr %q", code, out, errOut)
	}
	// A CSV header that starts with a bracket is still CSV.
	out, _, code = runSql(t, []string{"SELECT count(*) FROM stdin"}, "[id],x\n1,2\n")
	if code != 0 || out != "count(*)\n1\n" {
		t.Errorf("bracket header: exit %d stdout %q", code, out)
	}
}

func TestStrayQuoteFailsInsteadOfSwallowing(t *testing.T) {
	out, errOut, code := runSql(t, []string{"SELECT count(*) AS n FROM stdin", "-"}, "id,name\n1,\"x\"y\n2,Acme\n3,Foo\n4,Bar\n")
	if code != 1 || out != "" || !strings.Contains(errOut, "line 2") {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, _, code = runSql(t, []string{"--json", "SELECT 1 FROM stdin", "-"}, "id,name\n1,\"x\"y\n2,a\n")
	if code != 1 || !strings.Contains(out, `"code":1`) {
		t.Errorf("--json: exit %d stdout %q", code, out)
	}
	// Legit multi-line quoted fields still parse.
	out, errOut, code = runSql(t, []string{"SELECT count(*) AS n FROM stdin", "-"}, "id,note\n1,\"two\nlines\"\n2,ok\n")
	if code != 0 || out != "n\n2\n" {
		t.Errorf("multi-line: exit %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestTimeout(t *testing.T) {
	const runaway = "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c) SELECT count(*) FROM c"
	done := make(chan struct{})
	var out, errOut string
	var code int
	go func() {
		out, errOut, code = runSql(t, []string{"--timeout", "200ms", "--query", runaway}, "")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("--timeout 200ms did not stop a runaway query")
	}
	if code != 1 || out != "" || !strings.Contains(errOut, "sql: query timed out after 200ms") {
		t.Errorf("exit %d stdout %q stderr %q", code, out, errOut)
	}
	out, _, code = runSql(t, []string{"--json", "--timeout", "200ms", "--query", runaway}, "")
	if code != 1 || !strings.Contains(out, "timed out") {
		t.Errorf("--json: exit %d stdout %q", code, out)
	}
	for _, bad := range []string{"soon", "-1s"} {
		_, _, code = runSql(t, []string{"--timeout", bad, "--query", "SELECT 1"}, "")
		if code != 2 {
			t.Errorf("--timeout %s: exit %d, want 2", bad, code)
		}
	}
}
