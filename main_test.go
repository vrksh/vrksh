package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

	// Run with argv[0] == "vrk" and no additional args.
	cmd := exec.Command(bin)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit when called with no args, got 0")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
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
