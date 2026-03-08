package shared

import (
	"testing"
)

func TestStandardFlags(t *testing.T) {
	t.Run("all flags default to false", func(t *testing.T) {
		fs := StandardFlags()
		if err := fs.Parse([]string{}); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		for _, name := range []string{"json", "quiet", "dry-run", "explain"} {
			got, err := fs.GetBool(name)
			if err != nil {
				t.Fatalf("GetBool(%q): %v", name, err)
			}
			if got {
				t.Errorf("--%s default: want false, got true", name)
			}
		}
	})

	t.Run("long flags set correctly", func(t *testing.T) {
		fs := StandardFlags()
		if err := fs.Parse([]string{"--json", "--quiet", "--dry-run", "--explain"}); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		for _, name := range []string{"json", "quiet", "dry-run", "explain"} {
			got, err := fs.GetBool(name)
			if err != nil {
				t.Fatalf("GetBool(%q): %v", name, err)
			}
			if !got {
				t.Errorf("--%s: want true, got false", name)
			}
		}
	})

	t.Run("-j sets json", func(t *testing.T) {
		fs := StandardFlags()
		if err := fs.Parse([]string{"-j"}); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		got, _ := fs.GetBool("json")
		if !got {
			t.Error("-j: want json=true, got false")
		}
	})

	t.Run("-q sets quiet", func(t *testing.T) {
		fs := StandardFlags()
		if err := fs.Parse([]string{"-q"}); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		got, _ := fs.GetBool("quiet")
		if !got {
			t.Error("-q: want quiet=true, got false")
		}
	})

	t.Run("unset flags stay false", func(t *testing.T) {
		fs := StandardFlags()
		if err := fs.Parse([]string{"-j"}); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		quiet, _ := fs.GetBool("quiet")
		if quiet {
			t.Error("quiet should be false when only -j is set")
		}
	})

	t.Run("explain has no shorthand", func(t *testing.T) {
		fs := StandardFlags()
		if err := fs.Parse([]string{"-e"}); err == nil {
			t.Error("expected error for unknown flag -e")
		}
	})

	t.Run("dry-run has no shorthand", func(t *testing.T) {
		fs := StandardFlags()
		if err := fs.Parse([]string{"-d"}); err == nil {
			t.Error("expected error for unknown flag -d")
		}
	})
}
