package completions

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/vrksh/vrksh/internal/shared"
)

func mockTools() []shared.ToolMeta {
	return []shared.ToolMeta{
		{
			Name:  "bar",
			Short: "do bar things",
			Flags: []shared.FlagMeta{
				{Name: "verbose", Shorthand: "", Usage: "increase verbosity"},
				{Name: "quiet", Shorthand: "q", Usage: "suppress output"},
			},
		},
		{
			Name:  "foo",
			Short: "do foo things",
			Flags: []shared.FlagMeta{
				{Name: "json", Shorthand: "j", Usage: "emit JSON"},
				{Name: "count", Shorthand: "n", Usage: "number of items"},
			},
		},
	}
}

func readGolden(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}
	return string(data)
}

// runCompletions replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and
// returns captured stdout, stderr, and the exit code.
func runCompletions(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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
		t.Fatalf("close stdin write end: %v", err)
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

	os.Args = append([]string{"completions"}, args...)
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

func TestBashOutput(t *testing.T) {
	tools := mockTools()
	got := genBash(tools)
	want := readGolden(t, "testdata/bash_golden.txt")
	if got != want {
		t.Errorf("bash output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// Assert tool names appear as subcommands.
	if !strings.Contains(got, "bar foo") {
		t.Error("bash output missing tool names in subcommand list")
	}
	// Assert flags appear for a specific tool.
	if !strings.Contains(got, "--json -j --count -n") {
		t.Error("bash output missing expected flags for foo")
	}
	// Assert registration line.
	if !strings.HasSuffix(got, "complete -F _vrk vrk\n") {
		t.Error("bash output must end with 'complete -F _vrk vrk'")
	}
}

func TestZshOutput(t *testing.T) {
	tools := mockTools()
	got := genZsh(tools)
	want := readGolden(t, "testdata/zsh_golden.txt")
	if got != want {
		t.Errorf("zsh output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// Assert compdef line.
	if !strings.HasPrefix(got, "#compdef vrk\n") {
		t.Error("zsh output must start with '#compdef vrk'")
	}
	// Assert tool descriptions appear.
	if !strings.Contains(got, "'bar:do bar things'") {
		t.Error("zsh output missing bar tool description")
	}
	if !strings.Contains(got, "'foo:do foo things'") {
		t.Error("zsh output missing foo tool description")
	}
	// Assert flag strings appear.
	if !strings.Contains(got, "'--json[emit JSON]'") {
		t.Error("zsh output missing --json flag for foo")
	}
}

func TestFishOutput(t *testing.T) {
	tools := mockTools()
	got := genFish(tools)
	want := readGolden(t, "testdata/fish_golden.txt")
	if got != want {
		t.Errorf("fish output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// Assert disable-file-completions line.
	if !strings.HasPrefix(got, "complete -c vrk -f\n") {
		t.Error("fish output must start with 'complete -c vrk -f'")
	}
	// Assert subcommand lines.
	if !strings.Contains(got, "-a 'bar' -d 'do bar things'") {
		t.Error("fish output missing bar subcommand line")
	}
	// Assert flag lines with tool association.
	if !strings.Contains(got, "__fish_seen_subcommand_from foo' -l json -s j") {
		t.Error("fish output missing json flag for foo")
	}
}

func TestUnknownShellExitsNonZero(t *testing.T) {
	_, stderr, code := runCompletions(t, []string{"powershell"}, "")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown shell") {
		t.Errorf("stderr should mention unknown shell, got: %q", stderr)
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	stdout, stderr, code := runCompletions(t, []string{"--json", "powershell"}, "")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty with --json, got: %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON output missing 'error' field")
	}
	if c, ok := obj["code"]; !ok || c != float64(1) {
		t.Errorf("JSON output code = %v, want 1", c)
	}
}

func TestNoArgExitsNonZero(t *testing.T) {
	_, stderr, code := runCompletions(t, []string{}, "")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "shell argument required") {
		t.Errorf("stderr should mention shell argument required, got: %q", stderr)
	}
}

func TestHelpExitsZero(t *testing.T) {
	stdout, _, code := runCompletions(t, []string{"--help"}, "")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "completions") {
		t.Error("help output should mention 'completions'")
	}
}

func TestNoVrkshInOutput(t *testing.T) {
	tools := mockTools()
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var out string
		switch shell {
		case "bash":
			out = genBash(tools)
		case "zsh":
			out = genZsh(tools)
		case "fish":
			out = genFish(tools)
		}
		if strings.Contains(out, "vrksh") {
			t.Errorf("%s output contains 'vrksh' — must use 'vrk' everywhere", shell)
		}
	}
}

func TestEscapeZshBrackets(t *testing.T) {
	got := escapeZshBrackets("emit [REDACTED] markers")
	want := "emit \\[REDACTED\\] markers"
	if got != want {
		t.Errorf("escapeZshBrackets = %q, want %q", got, want)
	}
}

func TestEscapeFishQuote(t *testing.T) {
	got := escapeFishQuote("it's a test")
	want := "it\\'s a test"
	if got != want {
		t.Errorf("escapeFishQuote = %q, want %q", got, want)
	}
}
