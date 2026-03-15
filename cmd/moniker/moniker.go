// Package moniker implements vrk moniker — a memorable name generator.
// Produces human-readable adjective-noun names for run IDs, job labels, and
// temporary directory names. Like Docker container names and Heroku dyno names.
package moniker

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

//go:embed data/adjectives.txt
var adjectivesData string

//go:embed data/nouns.txt
var nounsData string

var (
	adjectives []string
	nouns      []string
)

func init() {
	adjectives = loadWords(adjectivesData)
	nouns = loadWords(nounsData)
}

func loadWords(data string) []string {
	lines := strings.Split(strings.TrimRight(data, "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// nameRecord is the JSON shape for all generated names.
// The words array always has numWords elements in order.
// Consistent regardless of --words value so agent code can rely on a single shape.
type nameRecord struct {
	Name  string   `json:"name"`
	Words []string `json:"words"`
}

// entry holds one generated name and its component words.
type entry struct {
	name  string
	parts []string
}

// Run is the entry point for vrk moniker. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("moniker", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		count     int
		separator string
		numWords  int
		seed      int64
		jsonFlag  bool
		quietFlag bool
	)
	fs.IntVarP(&count, "count", "n", 1, "number of names to generate (default 1)")
	fs.StringVar(&separator, "separator", "-", "word separator (default \"-\")")
	fs.IntVar(&numWords, "words", 2, "number of words per name, minimum 2 (default 2)")
	fs.Int64Var(&seed, "seed", 0, "fix random seed for deterministic output")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `emit {"name":"...","adjective":"...","noun":"..."} per name`)
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr error messages; exit codes unchanged")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quietFlag)()

	if count <= 0 {
		return shared.UsageErrorf("moniker: --count must be >= 1, got %d", count)
	}
	if numWords < 2 {
		return shared.UsageErrorf("moniker: --words must be >= 2, got %d", numWords)
	}

	var rng *rand.Rand
	if fs.Changed("seed") {
		rng = rand.New(rand.NewSource(seed)) //nolint:gosec — intentionally non-crypto; seed is a user-facing feature
	} else {
		rng = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	}

	entries, err := generate(rng, count, numWords, separator)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": err.Error(),
				"code":  1,
			})
		}
		return shared.Errorf("%s", err.Error())
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		for _, e := range entries {
			rec := nameRecord{Name: e.name, Words: e.parts}
			if err := enc.Encode(rec); err != nil {
				return shared.Errorf("moniker: writing output: %v", err)
			}
		}
	} else {
		for _, e := range entries {
			if _, err := fmt.Fprintln(os.Stdout, e.name); err != nil {
				return shared.Errorf("moniker: writing output: %v", err)
			}
		}
	}

	return 0
}

// generate produces count unique names with numWords words joined by sep.
// For 2-word names it uses a partial Fisher-Yates shuffle over the full index pool,
// guaranteeing uniqueness and checking pool capacity.
// For 3+ word names it uses generate-and-deduplicate (pool too large to enumerate).
func generate(rng *rand.Rand, count, numWords int, sep string) ([]entry, error) {
	if numWords == 2 {
		return generateTwo(rng, count, sep)
	}
	return generateN(rng, count, numWords, sep)
}

// generateTwo builds a flat pool of all adj×noun index pairs, partially shuffles it
// in O(count) time, then decodes the first count entries. Guarantees unique strings
// as long as the wordlists themselves have no duplicates.
func generateTwo(rng *rand.Rand, count int, sep string) ([]entry, error) {
	na := len(adjectives)
	nn := len(nouns)
	total := na * nn
	if count > total {
		return nil, fmt.Errorf("moniker: requested %d names but only %d unique 2-word combinations exist", count, total)
	}

	// Build a flat pool encoding every (adj_idx, noun_idx) pair as a single int.
	pool := make([]int, total)
	for i := range pool {
		pool[i] = i
	}

	// Partial Fisher-Yates: shuffle only the first count positions.
	// Each pool[i] ends up with a uniformly random unchosen index. O(count) time.
	for i := 0; i < count; i++ {
		j := i + rng.Intn(total-i)
		pool[i], pool[j] = pool[j], pool[i]
	}

	result := make([]entry, count)
	for i := 0; i < count; i++ {
		adjIdx := pool[i] / nn
		nounIdx := pool[i] % nn
		parts := []string{adjectives[adjIdx], nouns[nounIdx]}
		result[i] = entry{
			name:  strings.Join(parts, sep),
			parts: parts,
		}
	}
	return result, nil
}

// generateN produces count unique N-word names via generate-and-deduplicate.
// Pool sizes for N≥3 are adj^(N-1)*nouns (≥87M for N=3), making exhaustion
// practically impossible for any reasonable count.
func generateN(rng *rand.Rand, count, numWords int, sep string) ([]entry, error) {
	seen := make(map[string]struct{}, count)
	result := make([]entry, 0, count)
	maxAttempts := count * 100
	for attempts := 0; len(result) < count; attempts++ {
		if attempts >= maxAttempts {
			return nil, fmt.Errorf("moniker: unable to generate %d unique %d-word names after %d attempts",
				count, numWords, maxAttempts)
		}
		parts := make([]string, numWords)
		for i := 0; i < numWords-1; i++ {
			parts[i] = adjectives[rng.Intn(len(adjectives))]
		}
		parts[numWords-1] = nouns[rng.Intn(len(nouns))]
		name := strings.Join(parts, sep)
		if _, dup := seen[name]; !dup {
			seen[name] = struct{}{}
			result = append(result, entry{name: name, parts: parts})
		}
	}
	return result, nil
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk moniker [flags]",
		"",
		"Generates memorable adjective-noun names for run IDs, job labels,",
		"and temporary directory names. Like Docker container names.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("moniker: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
