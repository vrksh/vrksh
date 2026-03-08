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
		got, err := KVPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
		got, err := KVPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(got, home) {
			t.Errorf("got %q, does not start with home dir %q", got, home)
		}
	})

	t.Run("default filename is .vrk.db", func(t *testing.T) {
		if err := os.Unsetenv("VRK_KV_PATH"); err != nil {
			t.Fatal(err)
		}
		got, err := KVPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != ".vrk.db" {
			t.Errorf("base = %q, want .vrk.db", filepath.Base(got))
		}
	})

	t.Run("creates parent directory when missing", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "subdir", "test.db")
		t.Setenv("VRK_KV_PATH", dbPath)

		got, err := KVPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != dbPath {
			t.Errorf("got %q, want %q", got, dbPath)
		}
		if _, err := os.Stat(filepath.Join(dir, "subdir")); os.IsNotExist(err) {
			t.Error("did not create parent directory")
		}
	})

	t.Run("returns error on mkdir failure", func(t *testing.T) {
		// Point VRK_KV_PATH inside a file (not a dir), so MkdirAll fails.
		f, err := os.CreateTemp(t.TempDir(), "not-a-dir")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		t.Setenv("VRK_KV_PATH", filepath.Join(f.Name(), "sub", "vrk.db"))

		_, err = KVPath()
		if err == nil {
			t.Error("expected error when parent dir cannot be created, got nil")
		}
	})
}
