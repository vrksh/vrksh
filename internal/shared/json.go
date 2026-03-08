package shared

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON encodes v as JSON and writes it to stdout with a trailing newline.
// Returns an error if v is nil or encoding fails — the caller decides whether to Die().
func PrintJSON(v any) error {
	if v == nil {
		return fmt.Errorf("cannot marshal nil")
	}
	return json.NewEncoder(os.Stdout).Encode(v)
}

// PrintJSONL writes each item in items as a separate JSON object on its own line.
func PrintJSONL(items []any) error {
	enc := json.NewEncoder(os.Stdout)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
	}
	return nil
}
