package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary builds the vrk binary into a temp dir and returns its path.
// The binary is built once per test binary run; the temp dir is cleaned up
// when the test process exits.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "vrk")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

func TestManifest(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "--manifest")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("vrk --manifest: %v", err)
	}

	var manifest struct {
		Version string `json:"version"`
		Tools   []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(out, &manifest); err != nil {
		t.Fatalf("--manifest output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(manifest.Tools) == 0 {
		t.Error("--manifest: tools array is empty")
	}
	if manifest.Version == "" {
		t.Error("--manifest: version is empty")
	}
}

func TestSkills(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "--skills")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("vrk --skills: %v", err)
	}
	if len(out) == 0 {
		t.Error("--skills: output is empty")
	}
}

func TestUnknownTool(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "doesnotexist")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown tool, got 0")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
}

func TestNoArgs(t *testing.T) {
	bin := buildBinary(t)
	out, _, code := runVrk(t, bin)
	if code != 0 || !strings.Contains(out, "tools:") {
		t.Errorf("no args: code=%d, want 0 with tool list; out=%q", code, out)
	}
}

func TestStdinRequiredCoversAllTools(t *testing.T) {
	_, flagsFn, stdinRequired := mcpMaps()
	for name := range flagsFn {
		if _, ok := stdinRequired[name]; !ok {
			t.Errorf("tool %q is in flagsFn but missing from stdinRequired — add an explicit true/false entry", name)
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func runVrk(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return o.String(), e.String(), code
}

func TestVersion(t *testing.T) {
	bin := buildBinary(t)
	for _, a := range []string{"--version", "version"} {
		out, _, code := runVrk(t, bin, a)
		if code != 0 || !strings.HasPrefix(out, "vrk ") {
			t.Errorf("%s: code=%d out=%q", a, code, out)
		}
	}
}

func TestHelpListsTools(t *testing.T) {
	bin := buildBinary(t)
	for _, a := range [][]string{{}, {"-h"}, {"--help"}, {"help"}} {
		out, errOut, code := runVrk(t, bin, a...)
		if code != 0 || errOut != "" {
			t.Errorf("%v: code=%d stderr=%q", a, code, errOut)
		}
		for _, want := range []string{"tools:", "\n  tok ", "\n  assert ", "meta:", "vrk completions"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: stdout lacks %q", a, want)
			}
		}
	}
}

func TestUnknownToolHint(t *testing.T) {
	bin := buildBinary(t)
	_, errOut, code := runVrk(t, bin, "nope")
	if code != 2 || !strings.Contains(errOut, `unknown tool "nope"`) || !strings.Contains(errOut, "run 'vrk help' for the tool list") {
		t.Errorf("code=%d stderr=%q", code, errOut)
	}
}

func TestBareAlias(t *testing.T) {
	bin := buildBinary(t)
	a, _, ca := runVrk(t, bin, "bare", "--list")
	b, _, cb := runVrk(t, bin, "--bare", "--list")
	if ca != 0 || ca != cb || a != b {
		t.Errorf("bare vs --bare differ: %d/%d\n%q\n%q", ca, cb, a, b)
	}
}
