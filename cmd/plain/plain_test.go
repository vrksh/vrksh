package plain

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// runPlain replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runPlain(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"plain"}, args...)
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

func TestPositionalArg(t *testing.T) {
	// Positional arg bypasses stdin entirely — TTY guard must not fire.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runPlain(t, []string{"**bold**"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "bold" {
		t.Errorf("stdout = %q, want %q", strings.TrimSpace(stdout), "bold")
	}
}

func TestPlainBold(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "**hello**\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "hello" {
		t.Errorf("stdout = %q, want %q", stdout, "hello")
	}
}

func TestPlainItalic(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "_world_\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "world" {
		t.Errorf("stdout = %q, want %q", stdout, "world")
	}
}

func TestPlainHeading(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "# Heading\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "Heading" {
		t.Errorf("stdout = %q, want %q", stdout, "Heading")
	}
}

func TestPlainLink(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "[text](https://example.com)\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "text" {
		t.Errorf("stdout = %q, want %q", stdout, "text")
	}
}

func TestPlainReferenceLink(t *testing.T) {
	input := "[text][ref]\n\n[ref]: https://example.com\n"
	stdout, _, code := runPlain(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "text" {
		t.Errorf("stdout = %q, want %q", stdout, "text")
	}
}

func TestPlainInlineCode(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "`code snippet`\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "code snippet" {
		t.Errorf("stdout = %q, want %q", stdout, "code snippet")
	}
}

func TestPlainFencedCode(t *testing.T) {
	input := "```\nfenced code\n```\n"
	stdout, _, code := runPlain(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "fenced code" {
		t.Errorf("stdout = %q, want %q", stdout, "fenced code")
	}
}

func TestPlainBlockquote(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "> blockquote text\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "blockquote text" {
		t.Errorf("stdout = %q, want %q", stdout, "blockquote text")
	}
}

func TestPlainList(t *testing.T) {
	input := "- item one\n- item two\n"
	stdout, _, code := runPlain(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := strings.TrimSpace(stdout)
	want := "item one\nitem two"
	if got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestPlainNestedEmphasis(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "**bold _nested_**\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := strings.TrimSpace(stdout)
	want := "bold nested"
	if got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestPlainEmpty(t *testing.T) {
	stdout, stderr, code := runPlain(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestPlainNoMarkdown(t *testing.T) {
	stdout, _, code := runPlain(t, nil, "no markdown here\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "no markdown here" {
		t.Errorf("stdout = %q, want %q", stdout, "no markdown here")
	}
}

func TestPlainNoStdinTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runPlain(t, nil, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on usage error", stdout)
	}
	if !strings.Contains(stderr, "no input") {
		t.Errorf("stderr = %q, want 'no input'", stderr)
	}
}

func TestPlainUnknownFlag(t *testing.T) {
	stdout, _, code := runPlain(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on usage error", stdout)
	}
}

func TestPlainHelp(t *testing.T) {
	stdout, _, code := runPlain(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "plain") {
		t.Errorf("--help stdout = %q, want it to contain 'plain'", stdout)
	}
}

func TestPlainJSON(t *testing.T) {
	stdout, stderr, code := runPlain(t, []string{"--json"}, "**hello**\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	for _, field := range []string{"text", "input_bytes", "output_bytes"} {
		if _, ok := obj[field]; !ok {
			t.Errorf("JSON missing field %q", field)
		}
	}
	if obj["text"] != "hello" {
		t.Errorf("text = %q, want %q", obj["text"], "hello")
	}
}

func TestPlainJSONOutputBytes(t *testing.T) {
	stdout, _, code := runPlain(t, []string{"--json"}, "**hello**\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	text, _ := obj["text"].(string)
	outputBytes := int(obj["output_bytes"].(float64))
	if outputBytes != len(text) {
		t.Errorf("output_bytes = %d, want len(%q) = %d", outputBytes, text, len(text))
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runPlain(t, []string{"--json"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json active", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
}
