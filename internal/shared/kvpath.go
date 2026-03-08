package shared

import (
	"os"
	"path/filepath"
)

// KVPath returns the path to the SQLite database used by vrk kv.
// Reads VRK_KV_PATH if set; otherwise defaults to ~/.vrk.db.
// Creates the parent directory if it does not exist.
func KVPath() string {
	if p := os.Getenv("VRK_KV_PATH"); p != "" {
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			Die("creating kv directory %s: %v", dir, err)
		}
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		Die("finding home directory: %v", err)
	}
	return filepath.Join(home, ".vrk.db")
}
