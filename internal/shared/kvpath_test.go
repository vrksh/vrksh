package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKVPath(t *testing.T) {
	t.Run("env var overrides default", func(t *testing.T) {
		t.Setenv("VRK_KV_PATH", "/tmp/test-vrk.db")
		got := KVPath()
		if got != "/tmp/test-vrk.db" {
			t.Errorf("got %q, want %q", got, "/tmp/test-vrk.db")
		}
	})

	t.Run("default is inside home directory", func(t *testing.T) {
		if err := os.Unsetenv("VRK_KV_PATH"); err != nil {
			t.Fatal(err)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		got := KVPath()
		if !strings.HasPrefix(got, home) {
			t.Errorf("got %q, does not start with home dir %q", got, home)
		}
	})

	t.Run("default filename is .vrk.db", func(t *testing.T) {
		if err := os.Unsetenv("VRK_KV_PATH"); err != nil {
			t.Fatal(err)
		}
		got := KVPath()
		if filepath.Base(got) != ".vrk.db" {
			t.Errorf("base = %q, want .vrk.db", filepath.Base(got))
		}
	})

	t.Run("creates parent directory when missing", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "subdir", "test.db")
		t.Setenv("VRK_KV_PATH", dbPath)

		got := KVPath()
		if got != dbPath {
			t.Errorf("got %q, want %q", got, dbPath)
		}
		if _, err := os.Stat(filepath.Join(dir, "subdir")); os.IsNotExist(err) {
			t.Error("did not create parent directory")
		}
	})
}
