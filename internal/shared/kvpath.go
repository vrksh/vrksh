package shared

import (
	"fmt"
	"os"
	"path/filepath"
)

// KVPath returns the path to the SQLite database used by vrk kv.
// Reads VRK_KV_PATH if set; otherwise defaults to ~/.vrk.db.
// Creates the parent directory if it does not exist.
// Returns an error — the caller decides whether to Die().
func KVPath() (string, error) {
	if p := os.Getenv("VRK_KV_PATH"); p != "" {
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("creating kv directory %s: %w", dir, err)
		}
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".vrk.db"), nil
}
