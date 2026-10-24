package chunk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

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
			{Name: "model", Shorthand: "m", Usage: "tokenizer: an OpenAI model (gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo) or encoding (default o200k_base)"},
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
	var model string
	var quietFlag bool
	fs.IntVar(&size, "size", 0, "max tokens per chunk (required, >= 1)")
	fs.IntVar(&overlap, "overlap", 0, `token overlap between adjacent chunks (must be < --size)`)
	fs.StringVar(&by, "by", "", `chunking strategy; supported: "paragraph"`)
	fs.StringVarP(&model, "model", "m", "", "tokenizer: an OpenAI model (gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo) or encoding (default o200k_base)")
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
	tk, err := tokcount.ForModel(model)
	if err != nil {
		return shared.UsageErrorf("chunk: %v", err)
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
	ids, err := tk.Encode(input)
	if err != nil {
		return shared.Errorf("chunk: tokenizer error: %v", err)
	}
	if len(ids) == 0 {
		return 0
	}

	// Split into records.
	var records []chunkRecord
	if by == "paragraph" {
		records, err = splitByParagraph(tk, input, size, overlap)
	} else {
		records, err = splitByTokens(tk, ids, size, overlap, 0)
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
//
// A token can hold part of a multi-byte character, so window edges
// only land between tokens that sit on a character boundary. The end moves
// back to the nearest boundary; if that would empty the window it moves
// forward instead (at most 3 tokens past size, the only allowed overflow).
// Tokens reports the real token count of the emitted text.
func splitByTokens(tk *tokcount.Encoding, ids []int, size, overlap, startIndex int) ([]chunkRecord, error) {
	n := len(ids)
	raw := tk.DecodeBytes(ids)

	// offs[k] is the byte offset in raw where token k starts.
	offs := make([]int, n+1)
	for i, id := range ids {
		offs[i+1] = offs[i] + len(tk.DecodeBytes([]int{id}))
	}
	// runeStart[b] is true when byte offset b starts a character (or is the end).
	runeStart := make([]bool, len(raw)+1)
	for b := 0; b < len(raw); {
		runeStart[b] = true
		_, w := utf8.DecodeRune(raw[b:])
		b += w
	}
	runeStart[len(raw)] = true
	isBoundary := func(k int) bool { return runeStart[offs[k]] }

	var records []chunkRecord
	for start := 0; start < n; {
		end := min(start+size, n)
		e := end
		for e > start && !isBoundary(e) {
			e--
		}
		if e == start {
			for e = end; !isBoundary(e); e++ {
			}
		}
		limit := max(size, e-start)

		// Re-encoding a window can give a different count than the window
		// length (BPE merges differ at the edges). Shrink until it fits.
		text := string(raw[offs[start]:offs[e]])
		count, err := tk.Count(text)
		if err != nil {
			return nil, err
		}
		for count > limit {
			e2 := e - 1
			for e2 > start && !isBoundary(e2) {
				e2--
			}
			if e2 == start {
				break
			}
			e = e2
			text = string(raw[offs[start]:offs[e]])
			if count, err = tk.Count(text); err != nil {
				return nil, err
			}
		}

		records = append(records, chunkRecord{
			Index:  startIndex + len(records),
			Text:   text,
			Tokens: count,
		})
		if e >= n {
			break
		}

		// Next window starts overlap tokens back, also on a boundary.
		next := max(e-overlap, start+1)
		for next > start && !isBoundary(next) {
			next--
		}
		if next == start {
			for next = max(e-overlap, start+1); !isBoundary(next); next++ {
			}
		}
		start = next
	}
	return records, nil
}

// splitByParagraph splits text at double-newline boundaries, greedily packing
// paragraphs (joined by a blank line) into chunks of at most size tokens.
// Paragraphs that exceed size tokens fall back to token-level splitting.
// With overlap, a chunk starts with the last overlap tokens of the previous
// chunk, trimmed forward to a character boundary.
func splitByParagraph(tk *tokcount.Encoding, text string, size, overlap int) ([]chunkRecord, error) {
	var records []chunkRecord
	var parts []string // pieces of the chunk being built
	approx := 0        // token estimate for parts: sum of counts + 1 per separator
	prev := ""         // text of the last emitted chunk

	flush := func() error {
		if len(parts) == 0 {
			return nil
		}
		s := strings.Join(parts, "\n\n")
		// ponytail: packing uses the per-paragraph estimate; the real count of
		// the joined text is reported. Joins only merge whitespace, so the
		// real count is not expected to exceed the estimate.
		n, err := tk.Count(s)
		if err != nil {
			return err
		}
		records = append(records, chunkRecord{Index: len(records), Text: s, Tokens: n})
		prev, parts, approx = s, nil, 0
		return nil
	}

	// overlapTail returns the last overlap tokens of prev, starting on a
	// character boundary, and their count.
	overlapTail := func() (string, int, error) {
		if overlap == 0 || prev == "" {
			return "", 0, nil
		}
		ids, err := tk.Encode(prev)
		if err != nil {
			return "", 0, err
		}
		tail := ids[max(len(ids)-overlap, 0):]
		for len(tail) > 0 && !utf8.Valid(tk.DecodeBytes(tail)) {
			tail = tail[1:]
		}
		s := string(tk.DecodeBytes(tail))
		n, err := tk.Count(s)
		return s, n, err
	}

	// startChunk seeds a new chunk with the overlap tail when tail + p fits.
	startChunk := func(pTokens int) error {
		tail, n, err := overlapTail()
		if err != nil {
			return err
		}
		if tail != "" && n+1+pTokens <= size {
			parts, approx = []string{tail}, n
		}
		return nil
	}

	for _, p := range strings.Split(text, "\n\n") {
		if strings.TrimSpace(p) == "" {
			continue
		}
		ids, err := tk.Encode(p)
		if err != nil {
			return nil, fmt.Errorf("tokenizer error: %v", err)
		}

		// Oversized paragraph: fall back to token-level split.
		if len(ids) > size {
			if err := flush(); err != nil {
				return nil, err
			}
			subs, err := splitByTokens(tk, ids, size, overlap, len(records))
			if err != nil {
				return nil, err
			}
			records = append(records, subs...)
			prev = subs[len(subs)-1].Text
			continue
		}

		if len(parts) > 0 && approx+1+len(ids) > size {
			if err := flush(); err != nil {
				return nil, err
			}
		}
		if len(parts) == 0 {
			if err := startChunk(len(ids)); err != nil {
				return nil, err
			}
		}
		if len(parts) > 0 {
			approx++ // separator
		}
		parts = append(parts, p)
		approx += len(ids)
	}
	if err := flush(); err != nil {
		return nil, err
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
	fs.StringP("model", "m", "", "tokenizer: an OpenAI model (gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo) or encoding (default o200k_base)")
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
