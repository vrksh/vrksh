package uuid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	googleuuid "github.com/google/uuid"
	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

type uuidOutput struct {
	UUID        string `json:"uuid"`
	Version     int    `json:"version"`
	GeneratedAt int64  `json:"generated_at"`
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "uuid",
		Short: "UUID generator — v4 random, v7 time-ordered",
		Flags: []shared.FlagMeta{
			{Name: "v7", Usage: "generate a v7 (time-ordered) UUID instead of v4"},
			{Name: "count", Shorthand: "n", Usage: "number of UUIDs to generate (>= 1)"},
			{Name: "json", Shorthand: "j", Usage: "emit output as JSON (JSONL when --count > 1)"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk uuid. Returns 0 (success) or 2 (usage error).
// Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("uuid", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var v7Flag bool
	var count int
	var jsonFlag bool
	var quietFlag bool

	fs.BoolVar(&v7Flag, "v7", false, "generate a v7 (time-ordered) UUID instead of v4")
	fs.IntVarP(&count, "count", "n", 1, "number of UUIDs to generate (>= 1)")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit output as JSON (JSONL when --count > 1)")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("uuid: %s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	if count < 1 {
		return shared.UsageErrorf("uuid: count must be >= 1")
	}

	version := 4
	if v7Flag {
		version = 7
	}

	// Compute generated_at once so all UUIDs in a batch share the same timestamp.
	generatedAt := time.Now().Unix()

	enc := json.NewEncoder(os.Stdout)
	for i := 0; i < count; i++ {
		id, err := generate(v7Flag)
		if err != nil {
			return shared.Errorf("uuid: %v", err)
		}

		if jsonFlag {
			if err := enc.Encode(uuidOutput{
				UUID:        id,
				Version:     version,
				GeneratedAt: generatedAt,
			}); err != nil {
				return shared.Errorf("uuid: %v", err)
			}
		} else {
			if _, err := fmt.Fprintln(os.Stdout, id); err != nil {
				return shared.Errorf("uuid: %v", err)
			}
		}
	}

	return 0
}

// generate returns a new UUID string. v7=true produces a time-ordered v7 UUID;
// v7=false produces a random v4 UUID.
func generate(v7 bool) (string, error) {
	if v7 {
		u, err := googleuuid.NewV7()
		if err != nil {
			return "", err
		}
		return u.String(), nil
	}
	return googleuuid.New().String(), nil
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("uuid", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Bool("v7", false, "generate a v7 (time-ordered) UUID instead of v4")
	fs.IntP("count", "n", 1, "number of UUIDs to generate (>= 1)")
	fs.BoolP("json", "j", false, "emit output as JSON (JSONL when --count > 1)")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

// printUsage writes usage to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: uuid [flags]",
		"",
		"UUID generator. Generates v4 (random) UUIDs by default.",
		"Use --v7 for time-ordered UUIDs suitable for use as database primary keys.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("uuid: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
