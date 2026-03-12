package epoch

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// runEpoch replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Restores globals via t.Cleanup.
// Do not call t.Parallel() — tests share global state.
func runEpoch(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
	t.Helper()

	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr
	origArgs := os.Args

	t.Cleanup(func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
		os.Stderr = origStderr
		os.Args = origArgs
	})

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if _, err := io.WriteString(stdinW, stdinContent); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := stdinW.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}
	os.Stdin = stdinR

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = stdoutW

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stderr = stderrW

	os.Args = append([]string{"epoch"}, args...)
	code = Run()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, stdoutR); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.Copy(&errBuf, stderrR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return outBuf.String(), errBuf.String(), code
}

// --- Unix passthrough ---

func TestUnixPassthroughArg(t *testing.T) {
	stdout, _, code := runEpoch(t, []string{"1740009600"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740009600\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740009600\n")
	}
}

func TestUnixPassthroughStdin(t *testing.T) {
	stdout, _, code := runEpoch(t, []string{}, "1740009600")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740009600\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740009600\n")
	}
}

func TestNegativeUnixPassthrough(t *testing.T) {
	// Pre-epoch timestamp via stdin. Negative integers as positional args are
	// treated as flags by pflag; the pipe form is the natural way to pass them.
	stdout, _, code := runEpoch(t, []string{}, "-1000")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "-1000\n" {
		t.Errorf("stdout = %q, want %q", stdout, "-1000\n")
	}
}

// --- ISO date / datetime → unix ---

func TestISODateToUnix(t *testing.T) {
	// 2025-02-20 at midnight UTC = 1740009600
	stdout, _, code := runEpoch(t, []string{"2025-02-20"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740009600\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740009600\n")
	}
}

func TestISODatetimeToUnix(t *testing.T) {
	// 2025-02-20T10:00:00Z = 1740045600
	stdout, _, code := runEpoch(t, []string{"2025-02-20T10:00:00Z"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740045600\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740045600\n")
	}
}

// --- --now flag (boolean) ---

func TestBareNowFlag(t *testing.T) {
	// --now with no input: prints current unix timestamp, exit 0.
	stdout, _, code := runEpoch(t, []string{"--now"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	ts := strings.TrimSpace(stdout)
	if ts == "" || ts == "0" {
		t.Errorf("stdout = %q, want a non-zero unix timestamp", stdout)
	}
}

// --- --at flag (reference timestamp) ---

func TestAtWithNoInput(t *testing.T) {
	// --at with no input: exit 2 (nothing to calculate).
	stdout, stderr, code := runEpoch(t, []string{"--at", "1740009600"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
	if stderr == "" {
		t.Error("stderr must contain a usage hint")
	}
}

// --- Relative times ---

// anchor: 1740009600 = 2025-02-20T00:00:00Z
const nowAnchor = "1740009600"

func TestRelativePlusDays(t *testing.T) {
	// +3d from 1740009600 = 1740009600 + 3*86400 = 1740268800
	// Also covers PLAN's TestNowOverrideWithRelative — identical case.
	stdout, _, code := runEpoch(t, []string{"+3d", "--at", nowAnchor}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740268800\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740268800\n")
	}
}

func TestRelativeMinusDays(t *testing.T) {
	// -3d from 1740009600 = 1740009600 - 3*86400 = 1739750400
	// Positional arg form — the pre-pass in Run() extracts '-3d' before pflag.
	stdout, _, code := runEpoch(t, []string{"-3d", "--at", nowAnchor}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1739750400\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1739750400\n")
	}
}

func TestRelativeMinusDaysStdin(t *testing.T) {
	// Same result as TestRelativeMinusDays but input arrives via stdin.
	stdout, _, code := runEpoch(t, []string{"--at", nowAnchor}, "-3d")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1739750400\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1739750400\n")
	}
}

func TestRelativePlusHours(t *testing.T) {
	// +6h from 1740009600 = 1740009600 + 6*3600 = 1740031200
	stdout, _, code := runEpoch(t, []string{"+6h", "--at", nowAnchor}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740031200\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740031200\n")
	}
}

func TestRelativeMinusHours(t *testing.T) {
	// -2h from 1740009600 = 1740009600 - 2*3600 = 1740002400
	// Positional arg form — the pre-pass in Run() extracts '-2h' before pflag.
	stdout, _, code := runEpoch(t, []string{"-2h", "--at", nowAnchor}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740002400\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740002400\n")
	}
}

func TestRelativePlusWeeks(t *testing.T) {
	// +1w from 1740009600 = 1740009600 + 7*86400 = 1740614400
	stdout, _, code := runEpoch(t, []string{"+1w", "--at", nowAnchor}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740614400\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740614400\n")
	}
}

func TestRelativePlusSeconds(t *testing.T) {
	// +30s from 1740009600 = 1740009600 + 30 = 1740009630
	stdout, _, code := runEpoch(t, []string{"+30s", "--at", nowAnchor}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740009630\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740009630\n")
	}
}

func TestRelativePlusDaysStdin(t *testing.T) {
	// Same as TestRelativePlusDays but input via stdin.
	stdout, _, code := runEpoch(t, []string{"--at", nowAnchor}, "+3d")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "1740268800\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1740268800\n")
	}
}

func TestRelativeNoSignRequired(t *testing.T) {
	// Bare '3d' without sign prefix must exit 2.
	stdout, stderr, code := runEpoch(t, []string{"3d"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "sign required") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "sign required")
	}
}

// --- --iso output ---

func TestUnixToISO(t *testing.T) {
	// 1740009600 in UTC = 2025-02-20T00:00:00Z
	stdout, _, code := runEpoch(t, []string{"1740009600", "--iso"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "2025-02-20T00:00:00Z\n" {
		t.Errorf("stdout = %q, want %q", stdout, "2025-02-20T00:00:00Z\n")
	}
}

func TestRelativePlusDaysISO(t *testing.T) {
	// +3d from anchor in ISO: 1740268800 = 2025-02-23T00:00:00Z
	stdout, _, code := runEpoch(t, []string{"+3d", "--iso", "--at", nowAnchor}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "2025-02-23T00:00:00Z\n" {
		t.Errorf("stdout = %q, want %q", stdout, "2025-02-23T00:00:00Z\n")
	}
}

// --- --tz ---

func TestUnixToISOTZNumericOffset(t *testing.T) {
	// 1740009600 UTC is 05:30 local in +05:30 zone.
	stdout, _, code := runEpoch(t, []string{"1740009600", "--iso", "--tz", "+05:30"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "2025-02-20T05:30:00+05:30\n" {
		t.Errorf("stdout = %q, want %q", stdout, "2025-02-20T05:30:00+05:30\n")
	}
}

func TestUnixToISOTZIANA(t *testing.T) {
	// 1740009600 UTC = 2025-02-19T19:00:00-05:00 in America/New_York (EST, winter).
	stdout, _, code := runEpoch(t, []string{"1740009600", "--iso", "--tz", "America/New_York"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "2025-02-19T19:00:00-05:00\n" {
		t.Errorf("stdout = %q, want %q", stdout, "2025-02-19T19:00:00-05:00\n")
	}
}

func TestAmbiguousTZAbbrev(t *testing.T) {
	// IST is ambiguous (India/Ireland/Israel) — must exit 2.
	stdout, stderr, code := runEpoch(t, []string{"1740009600", "--iso", "--tz", "IST"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "IST is ambiguous") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "IST is ambiguous")
	}
}

func TestTZWithoutISO(t *testing.T) {
	// --tz without --iso: usage error.
	stdout, stderr, code := runEpoch(t, []string{"1740009600", "--tz", "America/New_York"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "--tz requires --iso") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "--tz requires --iso")
	}
}

// --- Unsupported formats ---

func TestNaturalLanguageRejected(t *testing.T) {
	stdout, stderr, code := runEpoch(t, []string{"next tuesday"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "natural language not supported") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "natural language not supported")
	}
}

func TestTooManyArguments(t *testing.T) {
	stdout, stderr, code := runEpoch(t, []string{"+3d", "2025-03-08"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
	if !strings.Contains(stderr, "too many arguments") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "too many arguments")
	}
}

// --- No input ---

func TestNoInput(t *testing.T) {
	// No args, empty stdin (simulates closed pipe / no TTY input).
	stdout, stderr, code := runEpoch(t, []string{}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty, got %q", stdout)
	}
	if stderr == "" {
		t.Error("stderr must contain a usage hint")
	}
}

// --- Flag errors ---

func TestUnknownFlag(t *testing.T) {
	_, stderr, code := runEpoch(t, []string{"--not-a-flag"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr must contain an error message")
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runEpoch(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Error("stdout must contain usage text")
	}
}

// --- Property test ---

// TestPropertyExitCodeIsValid verifies that for any input, Run() returns only
// 0, 1, or 2 — never any other value.
func TestPropertyExitCodeIsValid(t *testing.T) {
	cases := []struct {
		args  []string
		stdin string
	}{
		{[]string{"1740009600"}, ""},
		{[]string{"+3d", "--at", nowAnchor}, ""},
		{[]string{"-3d", "--at", nowAnchor}, ""},
		{[]string{"3d"}, ""},
		{[]string{"2025-02-20"}, ""},
		{[]string{"next tuesday"}, ""},
		{[]string{"garbage!!!"}, ""},
		{[]string{""}, ""},
		{[]string{"--now"}, ""},
		{[]string{"1740009600", "--iso"}, ""},
		{[]string{"1740009600", "--iso", "--tz", "IST"}, ""},
		{[]string{"1740009600", "--iso", "--tz", "America/New_York"}, ""},
		{[]string{"1740009600", "--iso", "--tz", "+05:30"}, ""},
		{[]string{}, "1740009600"},
		{[]string{}, "+3d"},
		{[]string{}, ""},
		{[]string{}, "-1000"},
	}
	for _, tc := range cases {
		_, _, code := runEpoch(t, tc.args, tc.stdin)
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("args=%v stdin=%q: exit code = %d, want 0/1/2", tc.args, tc.stdin, code)
		}
	}
}

// --- --json output ---

func TestJSONUnixInput(t *testing.T) {
	// 1740009600 --json → {input:"1740009600", unix:1740009600, iso:"2025-02-20T00:00:00Z"}, exit 0.
	stdout, stderr, code := runEpoch(t, []string{"1740009600", "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if input, _ := obj["input"].(string); input != "1740009600" {
		t.Errorf("input = %q, want %q", input, "1740009600")
	}
	if unix, _ := obj["unix"].(float64); int64(unix) != 1740009600 {
		t.Errorf("unix = %v, want 1740009600", obj["unix"])
	}
	if iso, _ := obj["iso"].(string); iso != "2025-02-20T00:00:00Z" {
		t.Errorf("iso = %q, want %q", iso, "2025-02-20T00:00:00Z")
	}
	// ref and tz must not appear when --at and --tz were not used.
	if _, ok := obj["ref"]; ok {
		t.Error("ref must not appear when --at was not used")
	}
	if _, ok := obj["tz"]; ok {
		t.Error("tz must not appear when --tz was not used")
	}
}

func TestJSONISODateInput(t *testing.T) {
	// 2025-02-20 --json: input preserved as given; unix matches midnight UTC.
	stdout, _, code := runEpoch(t, []string{"2025-02-20", "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if input, _ := obj["input"].(string); input != "2025-02-20" {
		t.Errorf("input = %q, want %q", input, "2025-02-20")
	}
	if unix, _ := obj["unix"].(float64); int64(unix) != 1740009600 {
		t.Errorf("unix = %v, want 1740009600", obj["unix"])
	}
}

func TestJSONRelativeWithAt(t *testing.T) {
	// +3d --at 1740009600 --json: ref field present because --at was used.
	stdout, _, code := runEpoch(t, []string{"+3d", "--at", nowAnchor, "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if unix, _ := obj["unix"].(float64); int64(unix) != 1740268800 {
		t.Errorf("unix = %v, want 1740268800", obj["unix"])
	}
	if ref, _ := obj["ref"].(string); ref != nowAnchor {
		t.Errorf("ref = %q, want %q", ref, nowAnchor)
	}
}

func TestJSONNow(t *testing.T) {
	// --now --json: no input field; unix and iso present.
	stdout, stderr, code := runEpoch(t, []string{"--now", "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	// input must NOT appear for --now.
	if _, ok := obj["input"]; ok {
		t.Error("input must not appear for --now --json")
	}
	if _, ok := obj["unix"]; !ok {
		t.Error("unix must be present")
	}
	if _, ok := obj["iso"]; !ok {
		t.Error("iso must be present")
	}
}

func TestJSONWithTZ(t *testing.T) {
	// --json --tz +05:30: tz field present; iso uses the offset.
	stdout, _, code := runEpoch(t, []string{"1740009600", "--json", "--tz", "+05:30"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if tz, _ := obj["tz"].(string); tz != "+05:30" {
		t.Errorf("tz = %q, want %q", tz, "+05:30")
	}
	if iso, _ := obj["iso"].(string); iso != "2025-02-20T05:30:00+05:30" {
		t.Errorf("iso = %q, want %q", iso, "2025-02-20T05:30:00+05:30")
	}
}

func TestJSONTZWithoutISOAllowed(t *testing.T) {
	// --json --tz without --iso must NOT be a usage error.
	// The old rule was "--tz requires --iso"; --json relaxes it because json always includes iso.
	_, _, code := runEpoch(t, []string{"1740009600", "--json", "--tz", "America/New_York"}, "")
	if code == 2 {
		t.Fatalf("exit code = 2 (usage error) — --tz must be allowed without --iso when --json is active")
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestJSONError(t *testing.T) {
	// Usage error + --json → {"error":"...","code":2} on stdout, exit 2. Stderr empty.
	stdout, stderr, code := runEpoch(t, []string{"3d", "--json"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing key \"error\"")
	}
	if c, _ := obj["code"].(float64); int(c) != 2 {
		t.Errorf("code = %v, want 2", obj["code"])
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on a usage
// error. Exit code is unaffected.
func TestQuietSuppressesStderr(t *testing.T) {
	// "3d" without a sign is a usage error (exit 2).
	stdout, stderr, code := runEpoch(t, []string{"--quiet", "3d"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, _, code := runEpoch(t, []string{"--quiet", "1740009600"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "1740009600" {
		t.Errorf("stdout = %q, want %q", strings.TrimSpace(stdout), "1740009600")
	}
}

// --- Fuzz target ---

func FuzzEpoch(f *testing.F) {
	f.Add("1740009600", false, "", "")
	f.Add("2025-02-20", false, "", "")
	f.Add("+3d", false, nowAnchor, "")
	f.Add("-3d", false, nowAnchor, "")
	f.Add("3d", false, "", "")
	f.Add("", false, "", "")
	f.Add("next tuesday", false, "", "")
	f.Add("garbage", false, "", "")
	f.Add("-1000", false, "", "")
	f.Add("1740009600", true, "", "UTC")
	f.Add("1740009600", true, "", "IST")
	f.Add("1740009600", true, "", "+05:30")

	f.Fuzz(func(t *testing.T, input string, iso bool, atVal string, tz string) {
		args := []string{}
		if atVal != "" {
			args = append(args, "--at", atVal)
		}
		if iso {
			args = append(args, "--iso")
		}
		if tz != "" {
			args = append(args, "--tz", tz)
		}
		if input != "" {
			args = append(args, input)
		}

		_, _, code := runEpoch(t, args, "")
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("exit code = %d, want 0/1/2", code)
		}
	})
}

func TestJSONErrorToStdout(t *testing.T) {
	// "next tuesday" is an unsupported format → exit 2.
	// With --json, the error must go to stdout as JSON; stderr must be empty.
	stdout, stderr, code := runEpoch(t, []string{"next tuesday", "--json"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing key \"error\"")
	}
	if c, _ := obj["code"].(float64); int(c) != 2 {
		t.Errorf("code = %v, want 2", obj["code"])
	}
}
