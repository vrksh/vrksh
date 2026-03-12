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

// PrintJSONError writes the fields map as compact JSON to stdout and returns
// the exit code. It reads "code" from the map (accepts int or float64); defaults to 1.
// All error output goes to stdout so stderr stays empty when --json is active.
func PrintJSONError(fields map[string]any) int {
	code := 1
	if c, ok := fields["code"]; ok {
		switch v := c.(type) {
		case int:
			code = v
		case float64:
			code = int(v)
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(fields)
	return code
}
