package kv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; no CGO
)

var errNotFound = errors.New("key not found")
var errNotANumber = errors.New("value is not a number")

// Run is the entry point for vrk kv. Returns 0, 1, or 2. Never calls os.Exit.
func Run() int {
	if len(os.Args) < 2 {
		return shared.UsageErrorf("usage: vrk kv <set|get|del|list|incr|decr>")
	}

	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "set":
		return kvSet(args)
	case "get":
		return kvGet(args)
	case "del":
		return kvDel(args)
	case "list":
		return kvList(args)
	case "incr":
		return kvIncrDecr(args, "incr", 1)
	case "decr":
		return kvIncrDecr(args, "decr", -1)
	case "--help", "-h":
		return printUsage()
	default:
		return shared.UsageErrorf("unknown kv subcommand: %s", sub)
	}
}

// openDB opens the SQLite database, enables WAL mode and a 5-second busy
// timeout, creates the schema if needed, and returns a ready *sql.DB.
// SetMaxOpenConns(1) serialises all Go-level access through a single connection
// so goroutines queue at the Go level rather than racing at the SQLite level.
func openDB() (*sql.DB, error) {
	path, err := shared.KVPath()
	if err != nil {
		return nil, fmt.Errorf("kv: cannot determine home directory: %w\nset VRK_KV_PATH to override", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("kv: open database %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	// busy_timeout must be set FIRST so that the WAL journal_mode change
	// (which requires an exclusive lock) retries automatically when concurrent
	// processes are opening the database simultaneously.
	for _, pragma := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA journal_mode=WAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("kv: %s: %w", pragma, err)
		}
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS kv (
			ns         TEXT    NOT NULL,
			key        TEXT    NOT NULL,
			value      TEXT    NOT NULL,
			expires_at INTEGER,
			PRIMARY KEY (ns, key)
		)`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("kv: create table: %w", err)
	}

	return db, nil
}

// getVal fetches the value for (ns, key). Returns errNotFound if the key is
// absent or has expired (and lazily deletes the expired row).
func getVal(db *sql.DB, ns, key string) (string, error) {
	var value string
	var expiresAt sql.NullInt64

	err := db.QueryRow(
		"SELECT value, expires_at FROM kv WHERE ns = ? AND key = ?",
		ns, key,
	).Scan(&value, &expiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return "", errNotFound
	}
	if err != nil {
		return "", err
	}

	if expiresAt.Valid && expiresAt.Int64 <= time.Now().Unix() {
		_, _ = db.Exec("DELETE FROM kv WHERE ns = ? AND key = ?", ns, key)
		return "", errNotFound
	}

	return value, nil
}

// setVal stores (ns, key, value) with an optional TTL. ttl == 0 means no expiry.
func setVal(db *sql.DB, ns, key, value string, ttl time.Duration) error {
	var expiresAt interface{}
	if ttl > 0 {
		expiresAt = time.Now().Unix() + int64(ttl.Seconds())
	}

	_, err := db.Exec(
		`INSERT INTO kv (ns, key, value, expires_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(ns, key) DO UPDATE SET
		     value      = excluded.value,
		     expires_at = excluded.expires_at`,
		ns, key, value, expiresAt,
	)
	return err
}

// delVal removes (ns, key). Silent if the key is absent.
func delVal(db *sql.DB, ns, key string) error {
	_, err := db.Exec("DELETE FROM kv WHERE ns = ? AND key = ?", ns, key)
	return err
}

// listKeys returns all non-expired keys in ns, sorted alphabetically.
func listKeys(db *sql.DB, ns string) ([]string, error) {
	rows, err := db.Query(
		"SELECT key FROM kv WHERE ns = ? AND (expires_at IS NULL OR expires_at > ?)",
		ns, time.Now().Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Strings(keys)
	return keys, nil
}

// incrVal atomically reads and updates (ns, key) by delta.
//
// It uses BEGIN IMMEDIATE to acquire the SQLite write lock before reading, so
// concurrent processes cannot interleave their read-modify-write cycles. Other
// writers receive SQLITE_BUSY and retry for up to busy_timeout milliseconds.
// Missing keys start at 0. Returns errNotANumber if the stored value is not
// a parseable integer.
func incrVal(db *sql.DB, ns, key string, delta int64) (int64, error) {
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()

	// BEGIN IMMEDIATE takes the write lock immediately. Any concurrent process
	// trying BEGIN IMMEDIATE will receive SQLITE_BUSY; we retry with backoff
	// for up to ~5 seconds. This is belt-and-suspenders alongside
	// PRAGMA busy_timeout, which may not fire for BEGIN statements in all
	// SQLite driver implementations.
	if err := beginImmediate(ctx, conn); err != nil {
		return 0, err
	}

	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	var strVal string
	var expiresAt sql.NullInt64
	var cur int64

	err = conn.QueryRowContext(ctx,
		"SELECT value, expires_at FROM kv WHERE ns = ? AND key = ?",
		ns, key,
	).Scan(&strVal, &expiresAt)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		cur = 0
	case err != nil:
		return 0, err
	default:
		if expiresAt.Valid && expiresAt.Int64 <= time.Now().Unix() {
			cur = 0
		} else {
			cur, err = strconv.ParseInt(strings.TrimSpace(strVal), 10, 64)
			if err != nil {
				return 0, errNotANumber
			}
		}
	}

	newVal := cur + delta
	_, err = conn.ExecContext(ctx,
		`INSERT INTO kv (ns, key, value, expires_at) VALUES (?, ?, ?, NULL)
		 ON CONFLICT(ns, key) DO UPDATE SET
		     value      = excluded.value,
		     expires_at = excluded.expires_at`,
		ns, key, strconv.FormatInt(newVal, 10),
	)
	if err != nil {
		return 0, err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, err
	}
	committed = true
	return newVal, nil
}

// kvSet implements vrk kv set.
// beginImmediate executes BEGIN IMMEDIATE on conn, retrying on SQLITE_BUSY for
// up to ~5 seconds. This is needed because PRAGMA busy_timeout may not apply
// to BEGIN statements in all SQLite driver implementations.
func beginImmediate(ctx context.Context, conn *sql.Conn) error {
	const (
		maxAttempts = 100
		retryDelay  = 50 * time.Millisecond
	)
	for i := 0; i < maxAttempts; i++ {
		_, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		if err == nil {
			return nil
		}
		msg := err.Error()
		if !strings.Contains(msg, "SQLITE_BUSY") && !strings.Contains(msg, "database is locked") {
			return err
		}
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("database is locked after %d retries", maxAttempts)
}

func kvSet(args []string) int {
	fs := pflag.NewFlagSet("kv-set", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	ns := fs.String("ns", "default", "namespace")
	ttl := fs.Duration("ttl", 0, "expiry duration (e.g. 1s, 5m, 24h); 0 = no expiry")
	dryRun := fs.Bool("dry-run", false, "print intent without writing to db")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printSubUsage("set", fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	posArgs := fs.Args()
	var key, value string

	switch len(posArgs) {
	case 0:
		return shared.UsageErrorf("kv set: usage: vrk kv set [flags] <key> [value]")
	case 1:
		key = posArgs[0]
		if !*dryRun {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return shared.Errorf("kv set: reading stdin: %v", err)
			}
			if len(raw) == 0 {
				return shared.UsageErrorf("kv set: no value: provide as argument or via stdin")
			}
			value = strings.TrimSuffix(string(raw), "\n")
		}
	case 2:
		key = posArgs[0]
		value = posArgs[1]
	default:
		return shared.UsageErrorf("kv set: too many arguments")
	}

	if *dryRun {
		if _, err := fmt.Fprintf(os.Stdout, "would set %s = %s\n", key, value); err != nil {
			return shared.Errorf("kv set: %v", err)
		}
		return 0
	}

	db, err := openDB()
	if err != nil {
		return shared.Errorf("%v", err)
	}
	defer func() { _ = db.Close() }()

	if err := setVal(db, *ns, key, value, *ttl); err != nil {
		return shared.Errorf("kv set: %v", err)
	}
	return 0
}

// kvGet implements vrk kv get.
func kvGet(args []string) int {
	fs := pflag.NewFlagSet("kv-get", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ns := fs.String("ns", "default", "namespace")
	var jsonFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit errors as JSON")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printSubUsage("get", fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	if len(fs.Args()) != 1 {
		return shared.UsageErrorf("kv get: usage: vrk kv get [--ns NS] <key>")
	}
	key := fs.Args()[0]

	db, err := openDB()
	if err != nil {
		return shared.Errorf("%v", err)
	}
	defer func() { _ = db.Close() }()

	value, err := getVal(db, *ns, key)
	if errors.Is(err, errNotFound) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "key not found",
				"key":   key,
				"code":  1,
			})
		}
		return shared.Errorf("kv get: key not found")
	}
	if err != nil {
		return shared.Errorf("kv get: %v", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, value); err != nil {
		return shared.Errorf("kv get: %v", err)
	}
	return 0
}

// kvDel implements vrk kv del.
func kvDel(args []string) int {
	fs := pflag.NewFlagSet("kv-del", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ns := fs.String("ns", "default", "namespace")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printSubUsage("del", fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	if len(fs.Args()) != 1 {
		return shared.UsageErrorf("kv del: usage: vrk kv del [--ns NS] <key>")
	}
	key := fs.Args()[0]

	db, err := openDB()
	if err != nil {
		return shared.Errorf("%v", err)
	}
	defer func() { _ = db.Close() }()

	if err := delVal(db, *ns, key); err != nil {
		return shared.Errorf("kv del: %v", err)
	}
	return 0
}

// kvList implements vrk kv list.
func kvList(args []string) int {
	fs := pflag.NewFlagSet("kv-list", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ns := fs.String("ns", "default", "namespace")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printSubUsage("list", fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	db, err := openDB()
	if err != nil {
		return shared.Errorf("%v", err)
	}
	defer func() { _ = db.Close() }()

	keys, err := listKeys(db, *ns)
	if err != nil {
		return shared.Errorf("kv list: %v", err)
	}

	for _, k := range keys {
		if _, err := fmt.Fprintln(os.Stdout, k); err != nil {
			return shared.Errorf("kv list: %v", err)
		}
	}
	return 0
}

// kvIncrDecr implements vrk kv incr and vrk kv decr.
// sign is +1 for incr, -1 for decr.
func kvIncrDecr(args []string, name string, sign int64) int {
	fs := pflag.NewFlagSet("kv-"+name, pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ns := fs.String("ns", "default", "namespace")
	by := fs.Int64("by", 1, "delta (must be >= 1)")
	var jsonFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit errors as JSON")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printSubUsage(name, fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	if *by < 1 {
		return shared.UsageErrorf("kv %s: --by must be >= 1, got %d", name, *by)
	}

	if len(fs.Args()) != 1 {
		return shared.UsageErrorf("kv %s: usage: vrk kv %s [--ns NS] [--by N] <key>", name, name)
	}
	key := fs.Args()[0]

	db, err := openDB()
	if err != nil {
		return shared.Errorf("%v", err)
	}
	defer func() { _ = db.Close() }()

	newVal, err := incrVal(db, *ns, key, sign*(*by))
	if errors.Is(err, errNotANumber) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "value is not a number",
				"key":   key,
				"code":  1,
			})
		}
		return shared.Errorf("kv %s: value is not a number", name)
	}
	if err != nil {
		return shared.Errorf("kv %s: %v", name, err)
	}

	if _, err := fmt.Fprintln(os.Stdout, newVal); err != nil {
		return shared.Errorf("kv %s: %v", name, err)
	}
	return 0
}

// printUsage writes top-level kv help to stdout and returns 0.
func printUsage() int {
	lines := []string{
		"usage: vrk kv <subcommand> [flags] [args]",
		"",
		"Persistent key-value store backed by SQLite (~/.vrk.db).",
		"Database path overridden by VRK_KV_PATH.",
		"",
		"subcommands:",
		"  set <key> [value]   Store a value (reads from stdin when value is absent)",
		"  get <key>           Print value; exit 1 if not found or expired",
		"  del <key>           Delete a key (silent if absent)",
		"  list                List all keys in namespace, sorted alphabetically",
		"  incr <key>          Increment integer value by 1 (or --by N); missing key starts at 0",
		"  decr <key>          Decrement integer value by 1 (or --by N); missing key starts at 0",
		"",
		"flags available on all subcommands:",
		"  --ns string   namespace (default \"default\"); namespaces are isolated",
		"",
		"flags for set:",
		"  --ttl duration   expiry duration (e.g. 1s, 5m, 24h); 0 = no expiry",
		"  --dry-run        print intent without writing to db",
		"",
		"flags for incr/decr:",
		"  --by int   delta (default 1; must be >= 1)",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("kv: %v", err)
		}
	}
	return 0
}

// printSubUsage writes subcommand help to stdout and returns 0.
func printSubUsage(sub string, fs *pflag.FlagSet) int {
	var usageLines []string
	switch sub {
	case "set":
		usageLines = []string{
			"usage: vrk kv set [--ns NS] [--ttl D] [--dry-run] <key> [value]",
			"       echo <value> | vrk kv set [--ns NS] [--ttl D] <key>",
		}
	case "get":
		usageLines = []string{"usage: vrk kv get [--ns NS] <key>"}
	case "del":
		usageLines = []string{"usage: vrk kv del [--ns NS] <key>"}
	case "list":
		usageLines = []string{"usage: vrk kv list [--ns NS]"}
	case "incr":
		usageLines = []string{"usage: vrk kv incr [--ns NS] [--by N] <key>"}
	case "decr":
		usageLines = []string{"usage: vrk kv decr [--ns NS] [--by N] <key>"}
	}
	for _, l := range append(usageLines, "", "flags:") {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("kv: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
