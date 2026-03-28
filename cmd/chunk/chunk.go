package chunk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/tokcount"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// chunkRecord is the JSONL shape emitted for each chunk.
type chunkRecord struct {
	Index  int    `json:"index"`
	Text   string `json:"text"`
	Tokens int    `json:"tokens"`
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "chunk",
		Short: "Token-aware text splitter — splits stdin into chunks within a token budget",
		Flags: []shared.FlagMeta{
			{Name: "size", Usage: "max tokens per chunk (required, >= 1)"},
			{Name: "overlap", Usage: "token overlap between adjacent chunks (must be < --size)"},
			{Name: "by", Usage: `chunking strategy; supported: "paragraph"`},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk chunk. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("chunk", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var size int
	var overlap int
	var by string
	var quietFlag bool
	fs.IntVar(&size, "size", 0, "max tokens per chunk (required, >= 1)")
	fs.IntVar(&overlap, "overlap", 0, `token overlap between adjacent chunks (must be < --size)`)
	fs.StringVar(&by, "by", "", `chunking strategy; supported: "paragraph"`)
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// --size is required and must be >= 1.
	if !fs.Changed("size") {
		return shared.UsageErrorf("chunk: --size is required")
	}
	if size < 1 {
		return shared.UsageErrorf("chunk: --size must be >= 1")
	}
	if overlap < 0 {
		return shared.UsageErrorf("chunk: --overlap must be >= 0")
	}
	if overlap >= size {
		return shared.UsageErrorf("chunk: --overlap must be less than --size")
	}
	if by != "" && by != "paragraph" {
		return shared.UsageErrorf("chunk: unknown --by mode: %q; supported: paragraph", by)
	}

	// TTY guard: an interactive terminal with no args is a usage error. An empty
	// stdin pipe is intentional (0 chunks) and must pass through normally.
	// ReadInputOptional can't distinguish the two — so we check explicitly first.
	if len(fs.Args()) == 0 && isTerminal(int(os.Stdin.Fd())) {
		return shared.UsageErrorf("chunk: no input: pipe text to stdin or pass as argument")
	}

	// Read input: positional args joined with a space, or full stdin.
	// chunk needs the full text before splitting — ReadInputOptional handles the
	// one-trailing-newline strip and returns "" for an empty pipe (→ 0 chunks).
	input, err := shared.ReadInputOptional(fs.Args())
	if err != nil {
		return shared.Errorf("chunk: %v", err)
	}

	// Empty input → no chunks, exit 0.
	if input == "" {
		return 0
	}

	// Encode full text to token IDs.
	ids, err := tokcount.EncodeTokens(input)
	if err != nil {
		return shared.Errorf("chunk: tokenizer error: %v", err)
	}
	if len(ids) == 0 {
		return 0
	}

	// Split into records.
	var records []chunkRecord
	if by == "paragraph" {
		records, err = splitByParagraph(input, size, overlap)
	} else {
		records, err = splitByTokens(ids, size, overlap, 0)
	}
	if err != nil {
		return shared.Errorf("chunk: %v", err)
	}

	// Emit JSONL with an explicit flush after every record (streaming-friendly).
	w := bufio.NewWriter(os.Stdout)
	defer func() { _ = w.Flush() }()
	enc := json.NewEncoder(w)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return shared.Errorf("chunk: writing output: %v", err)
		}
		if err := w.Flush(); err != nil {
			return shared.Errorf("chunk: flushing output: %v", err)
		}
	}
	return 0
}

// splitByTokens splits ids into chunks of at most size tokens with overlap
// tokens shared between adjacent chunks. startIndex offsets the Index field
// (used when called from splitByParagraph to continue a sequence).
func splitByTokens(ids []int, size, overlap, startIndex int) ([]chunkRecord, error) {
	step := size - overlap
	if step < 1 {
		step = 1
	}
	var records []chunkRecord
	for i := 0; i < len(ids); i += step {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		window := ids[i:end]
		records = append(records, chunkRecord{
			Index:  startIndex + len(records),
			Text:   tokcount.DecodeTokens(window),
			Tokens: len(window),
		})
		if end >= len(ids) {
			break
		}
	}
	return records, nil
}

// splitByParagraph splits text at double-newline boundaries, greedily packing
// paragraphs into chunks of at most size tokens. Paragraphs that exceed size
// tokens fall back to token-level splitting. Overlap is applied by prepending
// the last overlap token IDs from the previous chunk at the start of the next.
func splitByParagraph(text string, size, overlap int) ([]chunkRecord, error) {
	// Parse and encode each paragraph.
	type paraInfo struct {
		ids []int
	}
	var paras []paraInfo
	for _, p := range strings.Split(text, "\n\n") {
		if strings.TrimSpace(p) == "" {
			continue
		}
		ids, err := tokcount.EncodeTokens(p)
		if err != nil {
			return nil, fmt.Errorf("tokenizer error: %v", err)
		}
		paras = append(paras, paraInfo{ids})
	}

	var records []chunkRecord
	var prevIDs []int // token IDs of the most recently emitted chunk

	// overlapPrefix returns the last overlap token IDs from prevIDs (copy).
	overlapPrefix := func() []int {
		if overlap == 0 || len(prevIDs) == 0 {
			return nil
		}
		start := len(prevIDs) - overlap
		if start < 0 {
			start = 0
		}
		cp := make([]int, len(prevIDs)-start)
		copy(cp, prevIDs[start:])
		return cp
	}

	// emit appends a record for the given IDs and updates prevIDs.
	emit := func(ids []int) {
		cp := make([]int, len(ids))
		copy(cp, ids)
		records = append(records, chunkRecord{
			Index:  len(records),
			Text:   tokcount.DecodeTokens(cp),
			Tokens: len(cp),
		})
		prevIDs = cp
	}

	i := 0
	for i < len(paras) {
		p := paras[i]

		// Oversized paragraph: fall back to token-level split.
		if len(p.ids) > size {
			subs, err := splitByTokens(p.ids, size, overlap, len(records))
			if err != nil {
				return nil, err
			}
			records = append(records, subs...)
			if len(subs) > 0 {
				last := subs[len(subs)-1]
				lastIDs, err := tokcount.EncodeTokens(last.Text)
				if err != nil {
					return nil, err
				}
				prevIDs = lastIDs
			}
			i++
			continue
		}

		// Start a new chunk with the overlap prefix.
		prefix := overlapPrefix()
		// If prefix + this paragraph would exceed size, drop the prefix.
		// The paragraph alone is guaranteed to fit (len(p.ids) <= size).
		if len(prefix)+len(p.ids) > size {
			prefix = nil
		}

		chunkIDs := append([]int(nil), prefix...)

		// Greedily pack paragraphs until the next one would overflow.
		for i < len(paras) {
			p = paras[i]
			if len(p.ids) > size {
				// Oversized — leave for the next outer iteration.
				break
			}
			if len(chunkIDs)+len(p.ids) > size {
				break
			}
			chunkIDs = append(chunkIDs, p.ids...)
			i++
		}

		if len(chunkIDs) > 0 {
			emit(chunkIDs)
		}
	}

	return records, nil
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("chunk", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Int("size", 0, "max tokens per chunk (required, >= 1)")
	fs.Int("overlap", 0, `token overlap between adjacent chunks (must be < --size)`)
	fs.String("by", "", `chunking strategy; supported: "paragraph"`)
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: chunk [flags] [text]",
		"       echo <text> | chunk [flags]",
		"",
		"Token-aware text splitter — splits stdin into chunks within a token budget.",
		"Emits one JSONL record per chunk: {\"index\":0,\"text\":\"...\",\"tokens\":N}",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("chunk: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
