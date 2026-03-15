package validate

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

var allowedTypes = map[string]bool{
	"string":  true,
	"number":  true,
	"boolean": true,
	"array":   true,
	"object":  true,
}

// fixFn is the function called to repair an invalid line when --fix is active.
// A package-level var so tests inject a stub without shelling out to exec.
var fixFn = execPromptFix

// newStdinReader returns the reader used for scanning stdin. A package-level var
// so tests inject an error-producing reader to cover the scanner error path.
var newStdinReader = func() io.Reader { return os.Stdin }

// metaRecord is the trailing --json record emitted after all data lines.
type metaRecord struct {
	VRK    string `json:"_vrk"`
	Total  int    `json:"total"`
	Passed int    `json:"passed"`
	Failed int    `json:"failed"`
}

// Run is the entry point for vrk validate. Returns 0, 1, or 2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("validate", pflag.ContinueOnError)
	var schemaFlag string
	var strict, fix, jsonOut bool

	fs.StringVarP(&schemaFlag, "schema", "s", "", "JSON schema or file path (required)")
	// --strict has no shorthand — -s is reserved for --schema.
	fs.BoolVar(&strict, "strict", false, "exit 1 on first invalid line")
	fs.BoolVar(&fix, "fix", false, "attempt to repair invalid lines via prompt")
	fs.BoolVarP(&jsonOut, "json", "j", false, "append metadata record to stdout at end")

	fs.SetOutput(io.Discard)
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	if schemaFlag == "" {
		return shared.UsageErrorf("validate: --schema is required")
	}

	schema, err := loadSchema(schemaFlag)
	if err != nil {
		return shared.UsageErrorf("validate: %s", err.Error())
	}

	// Marshal schema to JSON once for passing to fixFn's system prompt.
	schemaJSON, _ := json.Marshal(schema)

	bw := bufio.NewWriter(os.Stdout)
	defer func() { _ = bw.Flush() }()

	scanner := shared.ScanLines(newStdinReader())
	var total, passed, failed int

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		total++
		failures := validateRecord(line, schema)

		if len(failures) == 0 {
			if _, werr := fmt.Fprintln(bw, line); werr != nil {
				return shared.Errorf("validate: writing output: %v", werr)
			}
			passed++
		} else if fix {
			if repaired, ok := fixFn(line, string(schemaJSON)); ok {
				if _, werr := fmt.Fprintln(bw, repaired); werr != nil {
					return shared.Errorf("validate: writing output: %v", werr)
				}
				passed++
			} else {
				for _, f := range failures {
					shared.Warn("validation failed: %s", f)
				}
				failed++
				if strict {
					return strictReturn(bw, jsonOut, total, passed, failed)
				}
			}
		} else {
			for _, f := range failures {
				shared.Warn("validation failed: %s", f)
			}
			failed++
			if strict {
				return strictReturn(bw, jsonOut, total, passed, failed)
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		_ = bw.Flush()
		if jsonOut {
			// Universal contract: --json routes errors to stdout so stderr stays clean.
			return shared.PrintJSONError(map[string]any{"error": scanErr.Error(), "code": 1})
		}
		return shared.Errorf("validate: reading stdin: %v", scanErr)
	}

	if jsonOut {
		meta := metaRecord{VRK: "validate", Total: total, Passed: passed, Failed: failed}
		b, _ := json.Marshal(meta)
		if _, werr := fmt.Fprintln(bw, string(b)); werr != nil {
			return shared.Errorf("validate: writing output: %v", werr)
		}
	}

	return shared.ExitOK
}

// strictReturn emits the --json metadata record when active, flushes the writer,
// and returns ExitError. Called whenever --strict fires on an invalid line so
// that downstream consumers always receive a closing metadata record even on
// non-zero exits — consistent with how every other tool in the suite behaves.
func strictReturn(bw *bufio.Writer, jsonOut bool, total, passed, failed int) int {
	if jsonOut {
		meta := metaRecord{VRK: "validate", Total: total, Passed: passed, Failed: failed}
		b, _ := json.Marshal(meta)
		_, _ = fmt.Fprintln(bw, string(b))
	}
	return shared.ExitError
}

// loadSchema parses the schema value as inline JSON (if it starts with '{')
// or reads it as a file path. Returns a validated map of field→type.
func loadSchema(value string) (map[string]string, error) {
	if strings.HasPrefix(strings.TrimSpace(value), "{") {
		var schema map[string]string
		if err := json.Unmarshal([]byte(value), &schema); err != nil {
			return nil, fmt.Errorf("invalid schema JSON: %w", err)
		}
		return validateSchemaTypes(schema)
	}

	// File path.
	data, err := os.ReadFile(value)
	if err != nil {
		return nil, fmt.Errorf("cannot read schema file: %w", err)
	}
	var schema map[string]string
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("invalid schema JSON: %w", err)
	}
	return validateSchemaTypes(schema)
}

// validateSchemaTypes checks that all type values are in the allowed set.
func validateSchemaTypes(schema map[string]string) (map[string]string, error) {
	for key, typ := range schema {
		if !allowedTypes[typ] {
			return nil, fmt.Errorf("invalid schema type %q for key %q (must be one of: string number boolean array object)", typ, key)
		}
	}
	return schema, nil
}

// validateRecord decodes line as JSON and checks each schema key exists with
// the correct type. Returns a slice of human-readable failure messages;
// empty means the record is valid.
func validateRecord(line string, schema map[string]string) []string {
	var m map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return []string{"line is not valid JSON"}
	}

	var failures []string
	for key, typ := range schema {
		val, ok := m[key]
		if !ok {
			failures = append(failures, fmt.Sprintf("%s is missing", key))
			continue
		}
		if !typeMatches(val, typ) {
			failures = append(failures, fmt.Sprintf("%s expected %s, got %s", key, typ, jsonTypeName(val)))
		}
	}
	return failures
}

// typeMatches reports whether val has the JSON type named by typ.
func typeMatches(val interface{}, typ string) bool {
	switch typ {
	case "string":
		_, ok := val.(string)
		return ok
	case "number":
		_, ok := val.(json.Number)
		return ok
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "array":
		_, ok := val.([]interface{})
		return ok
	case "object":
		_, ok := val.(map[string]interface{})
		return ok
	}
	return false
}

// jsonTypeName returns a human-readable name for the JSON type of val.
func jsonTypeName(val interface{}) string {
	switch val.(type) {
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	}
	return "unknown"
}

// execPromptFix shells out to `vrk prompt <systemPrompt>` with the invalid line
// on stdin. The system prompt is the first positional argument to vrk prompt (not
// a flag), so it is plain text; schemaJSON embedded inside it contains literal
// double-quotes that are harmless — the prompt tool receives them verbatim, not
// as shell metacharacters. The result is re-validated before being returned so a
// hallucinating model that returns garbage degrades gracefully.
func execPromptFix(line, schemaJSON string) (string, bool) {
	execPath, err := os.Executable()
	if err != nil {
		shared.Warn("validate: --fix: cannot locate binary: %v", err)
		return "", false
	}

	systemPrompt := fmt.Sprintf(
		"Fix the following JSON record so every field matches the schema. "+
			"Output only the corrected JSON object on a single line, nothing else. "+
			"Schema: %s",
		schemaJSON,
	)

	cmd := exec.Command(execPath, "prompt", systemPrompt)
	cmd.Stdin = strings.NewReader(line)
	out, err := cmd.Output()
	if err != nil {
		shared.Warn("validate: --fix: prompt failed: %v", err)
		return "", false
	}

	fixed := strings.TrimRight(string(out), "\n")

	// Re-validate the repaired line. We unmarshal schemaJSON back into a map
	// here rather than threading the original map[string]string through fixFn,
	// because fixFn's interface takes (line, schemaJSON string) for testability
	// — stubs can supply any schema JSON without constructing a Go map.
	// The round-trip is safe: schemaJSON was marshaled from a validated
	// map[string]string immediately before the scan loop.
	var schema map[string]string
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		shared.Warn("validate: --fix: cannot re-parse schema: %v", err)
		return "", false
	}
	if len(validateRecord(fixed, schema)) > 0 {
		shared.Warn("validate: --fix: repaired line still does not match schema")
		return "", false
	}
	return fixed, true
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: validate --schema <spec> [flags]",
		"       <stream> | validate --schema <spec> [flags]",
		"",
		"JSONL schema validator — validates records against a simplified type schema.",
		"Valid lines pass through to stdout. Invalid lines are warned to stderr.",
		"",
		"Schema format: {\"key\":\"type\"} where type is one of:",
		"  string | number | boolean | array | object",
		"Schema keys are required fields; extra keys in records are ignored.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("validate: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
