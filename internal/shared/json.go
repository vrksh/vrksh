package shared

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON marshals v to JSON and writes it to stdout with a trailing newline.
// Returns an error if v is nil or marshaling fails — the caller decides whether to Die().
func PrintJSON(v any) error {
	if v == nil {
		return fmt.Errorf("cannot marshal nil")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	_, err = fmt.Fprintf(os.Stdout, "%s\n", b)
	return err
}

// PrintJSONL writes each item in items as a separate JSON object on its own line.
func PrintJSONL(items []any) error {
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s\n", b); err != nil {
			return err
		}
	}
	return nil
}
