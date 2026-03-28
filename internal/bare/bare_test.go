package bare

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// testBare creates a temp bin dir with a fake vrk binary, overrides OsExecutable,
// captures stdout/stderr, calls Run, and returns the results.
// The setup function receives (binDir, vrkPath) for pre-test filesystem setup.
func testBare(t *testing.T, args, toolNames []string, setup func(binDir, vrkPath string)) (binDir, stdout, stderr string, code int) {
	t.Helper()

	binDir = t.TempDir()
	// Resolve macOS /var → /private/var so paths match filepath.EvalSymlinks output.
	var err error
	binDir, err = filepath.EvalSymlinks(binDir)
	if err != nil {
		t.Fatalf("resolve binDir: %v", err)
	}

	vrkPath := filepath.Join(binDir, "vrk")
	if err := os.WriteFile(vrkPath, []byte("fake"), 0o755); err != nil {
		t.Fatalf("create fake vrk: %v", err)
	}

	if setup != nil {
		setup(binDir, vrkPath)
	}

	origExec := OsExecutable
	OsExecutable = func() (string, error) { return vrkPath, nil }
	t.Cleanup(func() { OsExecutable = origExec })

	origStdout := os.Stdout
	origStderr := os.Stderr
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	})

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	code = Run(args, toolNames)

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

	return binDir, outBuf.String(), errBuf.String(), code
}

// symlinksIn returns sorted names of symlinks in dir that point to target.
func symlinksIn(t *testing.T, dir, target string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		fi, err := os.Lstat(path)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			continue
		}
		if resolved == target {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// --- Link tests ---

func TestBareLinkAll(t *testing.T) {
	tools := []string{"alpha", "beta", "gamma"}
	binDir, stdout, _, code := testBare(t, nil, tools, nil)

	if code != 0 {
		t.Fatalf("exit code: got %d, want 0", code)
	}

	linked := symlinksIn(t, binDir, filepath.Join(binDir, "vrk"))
	if len(linked) != 3 {
		t.Fatalf("symlinks: got %v, want [alpha beta gamma]", linked)
	}
	for _, name := range tools {
		if !strings.Contains(stdout, name) {
			t.Errorf("stdout missing tool %q", name)
		}
	}
	if !strings.Contains(stdout, "linked") {
		t.Error("stdout missing summary with 'linked'")
	}
}

func TestBareSkipCollision(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, stdout, _, code := testBare(t, nil, tools, func(binDir, _ string) {
		// Pre-create a regular file at "alpha" — collision.
		if err := os.WriteFile(filepath.Join(binDir, "alpha"), []byte("occupant"), 0o755); err != nil {
			t.Fatalf("create collision file: %v", err)
		}
	})

	if code != 0 {
		t.Fatalf("exit code: got %d, want 0 (skips are not errors)", code)
	}

	// beta should be linked, alpha should not.
	vrkPath := filepath.Join(binDir, "vrk")
	linked := symlinksIn(t, binDir, vrkPath)
	if len(linked) != 1 || linked[0] != "beta" {
		t.Errorf("symlinks: got %v, want [beta]", linked)
	}

	// alpha file should be unchanged.
	data, err := os.ReadFile(filepath.Join(binDir, "alpha"))
	if err != nil || string(data) != "occupant" {
		t.Error("collision file was modified")
	}

	if !strings.Contains(stdout, "skipped") {
		t.Error("stdout should mention skipped collision")
	}
}

func TestBareForceOverwrite(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, stdout, _, code := testBare(t, []string{"--force"}, tools, func(binDir, _ string) {
		if err := os.WriteFile(filepath.Join(binDir, "alpha"), []byte("occupant"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	})

	if code != 0 {
		t.Fatalf("exit code: got %d, want 0", code)
	}

	vrkPath := filepath.Join(binDir, "vrk")
	linked := symlinksIn(t, binDir, vrkPath)
	if len(linked) != 2 {
		t.Fatalf("symlinks: got %v, want [alpha beta]", linked)
	}

	if !strings.Contains(stdout, "overwritten") {
		t.Error("stdout should mention 'overwritten' for forced collision")
	}
}

func TestBareForceSpecificTool(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, _, _, code := testBare(t, []string{"--force", "alpha"}, tools, func(binDir, _ string) {
		if err := os.WriteFile(filepath.Join(binDir, "alpha"), []byte("occupant"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	})

	if code != 0 {
		t.Fatalf("exit code: got %d, want 0", code)
	}

	// Only alpha should be linked (it was the only positional arg).
	vrkPath := filepath.Join(binDir, "vrk")
	linked := symlinksIn(t, binDir, vrkPath)
	if len(linked) != 1 || linked[0] != "alpha" {
		t.Errorf("symlinks: got %v, want [alpha]", linked)
	}
}

func TestBareIdempotent(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, _, _, code := testBare(t, nil, tools, nil)
	if code != 0 {
		t.Fatalf("first run: exit %d, want 0", code)
	}

	// Run again — symlinks already exist.
	vrkPath := filepath.Join(binDir, "vrk")
	origExec := OsExecutable
	OsExecutable = func() (string, error) { return vrkPath, nil }
	defer func() { OsExecutable = origExec }()

	origStdout := os.Stdout
	origStderr := os.Stderr
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	code = Run(nil, tools)

	_ = stdoutW.Close()
	_ = stderrW.Close()
	var outBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, stdoutR); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.Copy(io.Discard, stderrR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	_ = stdoutR.Close()
	_ = stderrR.Close()

	if code != 0 {
		t.Fatalf("second run: exit %d, want 0", code)
	}
	if !strings.Contains(outBuf.String(), "already linked") {
		t.Error("second run should show 'already linked'")
	}
}

// --- Remove tests ---

func TestBareRemoveAll(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, _, _, code := testBare(t, nil, tools, nil)
	if code != 0 {
		t.Fatalf("link: exit %d", code)
	}

	// Now remove all.
	_, stdout, _, code := testBareWithDir(t, binDir, []string{"--remove"}, tools)
	if code != 0 {
		t.Fatalf("remove: exit %d, want 0", code)
	}

	vrkPath := filepath.Join(binDir, "vrk")
	remaining := symlinksIn(t, binDir, vrkPath)
	if len(remaining) != 0 {
		t.Errorf("remaining symlinks: %v, want none", remaining)
	}
	if !strings.Contains(stdout, "removed") {
		t.Error("stdout should mention removal")
	}
}

func TestBareRemoveSpecific(t *testing.T) {
	tools := []string{"alpha", "beta", "gamma"}
	binDir, _, _, _ := testBare(t, nil, tools, nil)

	_, _, _, code := testBareWithDir(t, binDir, []string{"--remove", "alpha"}, tools)
	if code != 0 {
		t.Fatalf("remove: exit %d, want 0", code)
	}

	vrkPath := filepath.Join(binDir, "vrk")
	remaining := symlinksIn(t, binDir, vrkPath)
	expected := []string{"beta", "gamma"}
	if strings.Join(remaining, ",") != strings.Join(expected, ",") {
		t.Errorf("remaining: got %v, want %v", remaining, expected)
	}
}

func TestBareRemoveSafety(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, _, _, _ := testBare(t, nil, tools, nil)

	// Replace alpha symlink with a regular file — simulates a non-vrk binary.
	alphaPath := filepath.Join(binDir, "alpha")
	if err := os.Remove(alphaPath); err != nil {
		t.Fatalf("remove alpha: %v", err)
	}
	if err := os.WriteFile(alphaPath, []byte("not-vrk"), 0o755); err != nil {
		t.Fatalf("write alpha: %v", err)
	}

	// Remove all — should only remove beta (vrk symlink), not alpha (regular file).
	_, _, _, code := testBareWithDir(t, binDir, []string{"--remove"}, tools)
	if code != 0 {
		t.Fatalf("remove: exit %d, want 0", code)
	}

	// alpha must still exist with original content.
	data, err := os.ReadFile(alphaPath)
	if err != nil {
		t.Fatal("alpha was deleted — safety violation")
	}
	if string(data) != "not-vrk" {
		t.Error("alpha content was modified — safety violation")
	}

	// beta should be gone.
	if _, err := os.Lstat(filepath.Join(binDir, "beta")); err == nil {
		t.Error("beta should have been removed")
	}
}

func TestBareRemoveDryRun(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, _, _, _ := testBare(t, nil, tools, nil)

	_, stdout, _, code := testBareWithDir(t, binDir, []string{"--remove", "--dry-run"}, tools)
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}

	// Symlinks should still exist.
	vrkPath := filepath.Join(binDir, "vrk")
	remaining := symlinksIn(t, binDir, vrkPath)
	if len(remaining) != 2 {
		t.Errorf("dry-run removed symlinks: got %d remaining, want 2", len(remaining))
	}
	if !strings.Contains(stdout, "Would remove") {
		t.Error("stdout should contain 'Would remove'")
	}
}

// --- List tests ---

func TestBareListActiveOnly(t *testing.T) {
	tools := []string{"alpha", "beta", "gamma"}
	binDir, _, _, _ := testBare(t, nil, tools, func(binDir, _ string) {
		// Pre-create "gamma" as a regular file — collision, won't be linked.
		if err := os.WriteFile(filepath.Join(binDir, "gamma"), []byte("occupant"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	})

	_, stdout, _, code := testBareWithDir(t, binDir, []string{"--list"}, tools)
	if code != 0 {
		t.Fatalf("list: exit %d, want 0", code)
	}

	if !strings.Contains(stdout, "alpha") {
		t.Error("--list should show alpha (active symlink)")
	}
	if !strings.Contains(stdout, "beta") {
		t.Error("--list should show beta (active symlink)")
	}
	if strings.Contains(stdout, "gamma") {
		t.Error("--list should NOT show gamma (collision, not linked)")
	}
}

func TestBareListEmpty(t *testing.T) {
	tools := []string{"alpha"}
	// Don't link anything — just call --list on an empty dir.
	binDir, stdout, _, code := testBare(t, []string{"--list"}, tools, nil)
	_ = binDir

	if code != 0 {
		t.Fatalf("list: exit %d, want 0", code)
	}
	stdout = strings.TrimSpace(stdout)
	if stdout != "" {
		t.Errorf("--list on empty dir should produce no output, got %q", stdout)
	}
}

// --- Dry-run tests ---

func TestBareDryRun(t *testing.T) {
	tools := []string{"alpha", "beta"}
	binDir, stdout, _, code := testBare(t, []string{"--dry-run"}, tools, nil)

	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}

	// No symlinks should exist.
	vrkPath := filepath.Join(binDir, "vrk")
	linked := symlinksIn(t, binDir, vrkPath)
	if len(linked) != 0 {
		t.Errorf("dry-run created symlinks: %v", linked)
	}
	if !strings.Contains(stdout, "Would link") {
		t.Error("stdout should contain 'Would link'")
	}
}

func TestBareDryRunForce(t *testing.T) {
	tools := []string{"alpha"}
	_, stdout, _, code := testBare(t, []string{"--dry-run", "--force"}, tools, func(binDir, _ string) {
		if err := os.WriteFile(filepath.Join(binDir, "alpha"), []byte("occupant"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	})

	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(stdout, "Would overwrite") {
		t.Error("stdout should contain 'Would overwrite'")
	}
}

// --- Error tests ---

func TestBareUnknownTool(t *testing.T) {
	tools := []string{"alpha", "beta"}
	_, _, stderr, code := testBare(t, []string{"unknowntool"}, tools, nil)

	if code != 1 {
		t.Fatalf("exit code: got %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown tool") {
		t.Errorf("stderr should mention 'unknown tool', got %q", stderr)
	}
}

func TestBareUnknownFlag(t *testing.T) {
	tools := []string{"alpha"}
	_, _, stderr, code := testBare(t, []string{"--json"}, tools, nil)

	if code != 2 {
		t.Fatalf("exit code: got %d, want 2", code)
	}
	if !strings.Contains(stderr, "unknown flag") {
		t.Errorf("stderr should mention unknown flag, got %q", stderr)
	}
}

func TestBareMutuallyExclusive(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"remove+force", []string{"--remove", "--force"}, "--remove and --force"},
		{"list+force", []string{"--list", "--force"}, "--list and --force"},
		{"list+remove", []string{"--list", "--remove"}, "--list and --remove"},
		{"list+dry-run", []string{"--list", "--dry-run"}, "--list and --dry-run"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, stderr, code := testBare(t, tc.args, []string{"alpha"}, nil)
			if code != 2 {
				t.Fatalf("exit code: got %d, want 2", code)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr should contain %q, got %q", tc.want, stderr)
			}
		})
	}
}

func TestBarePermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test as root")
	}

	tools := []string{"alpha"}
	_, _, stderr, code := testBare(t, nil, tools, func(binDir, _ string) {
		// Make dir read-only after vrk binary is created.
		if err := os.Chmod(binDir, 0o555); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(binDir, 0o755); err != nil {
				t.Logf("cleanup chmod: %v", err)
			}
		})
	})

	if code != 1 {
		t.Fatalf("exit code: got %d, want 1", code)
	}
	if !strings.Contains(stderr, "permission denied") {
		t.Errorf("stderr should mention 'permission denied', got %q", stderr)
	}
	if !strings.Contains(stderr, "sudo") {
		t.Errorf("stderr should suggest sudo, got %q", stderr)
	}
}

// --- Self-exclusion ---

func TestBareSelfExclusion(t *testing.T) {
	// Even if "vrk" appears in toolNames, no symlink named "vrk" should be created.
	tools := []string{"alpha", "vrk"}
	binDir, _, _, code := testBare(t, nil, tools, nil)

	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}

	vrkPath := filepath.Join(binDir, "vrk")
	linked := symlinksIn(t, binDir, vrkPath)
	for _, name := range linked {
		if name == "vrk" {
			t.Fatal("created a symlink named 'vrk' — self-exclusion violated")
		}
	}
	if len(linked) != 1 || linked[0] != "alpha" {
		t.Errorf("symlinks: got %v, want [alpha]", linked)
	}
}

// --- Symlink resolution ---

func TestBareSymlinkResolution(t *testing.T) {
	// Create a vrk binary, then a symlink to it via a relative path.
	// isVrkSymlink must resolve both sides and recognise it.
	binDir := t.TempDir()
	binDir, err := filepath.EvalSymlinks(binDir)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	vrkPath := filepath.Join(binDir, "vrk")
	if err := os.WriteFile(vrkPath, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write vrk: %v", err)
	}

	// Create symlink alpha → vrk (relative, not absolute).
	alphaPath := filepath.Join(binDir, "alpha")
	if err := os.Symlink("vrk", alphaPath); err != nil {
		t.Fatalf("symlink alpha: %v", err)
	}

	if !isVrkSymlink(alphaPath, vrkPath) {
		t.Fatal("isVrkSymlink should recognise relative symlink to vrk")
	}

	// Create a symlink that does NOT point to vrk.
	betaPath := filepath.Join(binDir, "beta")
	if err := os.WriteFile(filepath.Join(binDir, "other"), []byte("x"), 0o755); err != nil {
		t.Fatalf("write other: %v", err)
	}
	if err := os.Symlink("other", betaPath); err != nil {
		t.Fatalf("symlink beta: %v", err)
	}

	if isVrkSymlink(betaPath, vrkPath) {
		t.Fatal("isVrkSymlink should reject symlink to different target")
	}

	// Regular file is not a vrk symlink.
	if isVrkSymlink(vrkPath, vrkPath) {
		t.Fatal("isVrkSymlink should reject regular file")
	}

	// Broken symlink (target deleted) should still be recognised via Readlink fallback.
	ghostPath := filepath.Join(binDir, "ghost")
	if err := os.Symlink(vrkPath, ghostPath); err != nil {
		t.Fatalf("symlink ghost: %v", err)
	}
	// Delete the vrk binary — ghost symlink is now dangling.
	if err := os.Remove(vrkPath); err != nil {
		t.Fatalf("remove vrk: %v", err)
	}
	if !isVrkSymlink(ghostPath, vrkPath) {
		t.Fatal("isVrkSymlink should recognise broken symlink to vrk via Readlink fallback")
	}
}

// testBareWithDir reuses an existing binDir (for tests that need link-then-operate).
// It overrides OsExecutable to point at binDir/vrk and captures stdout/stderr.
func testBareWithDir(t *testing.T, binDir string, args, toolNames []string) (_, stdout, stderr string, code int) {
	t.Helper()

	vrkPath := filepath.Join(binDir, "vrk")

	origExec := OsExecutable
	OsExecutable = func() (string, error) { return vrkPath, nil }
	t.Cleanup(func() { OsExecutable = origExec })

	origStdout := os.Stdout
	origStderr := os.Stderr
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	})

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	code = Run(args, toolNames)

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

	return binDir, outBuf.String(), errBuf.String(), code
}
