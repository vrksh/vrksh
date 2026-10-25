// Package tokcount counts, encodes and decodes OpenAI BPE tokens with
// embedded vocabularies: o200k_base (GPT-4o and later, the default) and
// cl100k_base (GPT-4 and GPT-3.5). No network access.
//
// The encoder follows github.com/pkoukk/tiktoken-go (MIT), a Go port of
// OpenAI's tiktoken: the same split regexes and the same byte pair merge
// order, so token IDs are identical (tokcount_test.go checks this against
// tiktoken-go). It is built here because tiktoken-go's constructor also
// builds a reverse map and sorts the whole vocab, which was more than half of
// `vrk tok` startup time.
package tokcount

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/dlclark/regexp2"
)

//go:generate go run gen_vocab.go

// The vocabs are in rank order: one length byte, then the raw token bytes.
// Generated from data/<name>.tiktoken by gen_vocab.go, so startup skips
// base64 decoding and number parsing.
var (
	//go:embed data/cl100k_base.bin
	cl100kBin []byte
	//go:embed data/o200k_base.bin
	o200kBin []byte
)

// Encoding is one BPE vocabulary plus its pre-tokenizer regex. The encoder
// is built on first use and cached for the life of the process.
type Encoding struct {
	Name    string
	bin     []byte
	size    int // number of mergeable tokens
	pattern string
	once    sync.Once
	enc     *encoder
	err     error
}

var (
	// O200k is the vocab of GPT-4o, GPT-4.1, GPT-5 and the o-series.
	O200k = &Encoding{
		Name: "o200k_base", bin: o200kBin, size: 199998,
		pattern: strings.Join([]string{
			`[^\r\n\p{L}\p{N}]?[\p{Lu}\p{Lt}\p{Lm}\p{Lo}\p{M}]*[\p{Ll}\p{Lm}\p{Lo}\p{M}]+(?i:'s|'t|'re|'ve|'m|'ll|'d)?`,
			`[^\r\n\p{L}\p{N}]?[\p{Lu}\p{Lt}\p{Lm}\p{Lo}\p{M}]+[\p{Ll}\p{Lm}\p{Lo}\p{M}]*(?i:'s|'t|'re|'ve|'m|'ll|'d)?`,
			`\p{N}{1,3}`,
			` ?[^\s\p{L}\p{N}]+[\r\n/]*`,
			`\s*[\r\n]+`,
			`\s+(?!\S)`,
			`\s+`,
		}, "|"),
	}
	// Cl100k is the vocab of GPT-4, GPT-3.5 and text-embedding-3.
	Cl100k = &Encoding{
		Name: "cl100k_base", bin: cl100kBin, size: 100256,
		pattern: `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`,
	}
	// Default is what tools count with when no --model is given: the vocab
	// of OpenAI's current models.
	Default = O200k
)

// ForModel returns the encoding for a model or encoding name. Empty means
// Default. Matching follows tiktoken's model table: gpt-4, gpt-3.5 and the
// older embedding models use cl100k_base; gpt-4o, gpt-4.1, gpt-5 and later,
// and the o-series use o200k_base.
func ForModel(model string) (*Encoding, error) {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case m == "":
		return Default, nil
	case m == "cl100k_base", m == "gpt-4", strings.HasPrefix(m, "gpt-4-"),
		strings.HasPrefix(m, "gpt-3.5"), strings.HasPrefix(m, "gpt-35"),
		strings.HasPrefix(m, "text-embedding-"):
		return Cl100k, nil
	case m == "o200k_base", strings.HasPrefix(m, "gpt-"), strings.HasPrefix(m, "chatgpt-"),
		len(m) >= 2 && m[0] == 'o' && m[1] >= '1' && m[1] <= '9':
		return O200k, nil
	}
	return nil, fmt.Errorf("unknown model %q: use an OpenAI model name (gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo) or o200k_base / cl100k_base", model)
}

// loadBinaryVocab builds the token-to-rank map and the rank-indexed token
// table from the binary vocab. Table entries are slices of data, not copies.
func loadBinaryVocab(data []byte, size int) (map[string]int, [][]byte, error) {
	ranks := make(map[string]int, size)
	tokens := make([][]byte, 0, size)
	for i := 0; i < len(data); {
		n := int(data[i])
		i++
		if i+n > len(data) {
			return nil, nil, errors.New("tokcount: embedded vocab is truncated")
		}
		tok := data[i : i+n]
		ranks[string(tok)] = len(tokens)
		tokens = append(tokens, tok)
		i += n
	}
	return ranks, tokens, nil
}

// encoder is a byte pair encoder for one vocab.
type encoder struct {
	ranks  map[string]int
	tokens [][]byte // tokens[rank] is the raw bytes of that token
	split  *regexp2.Regexp
}

func (e *encoder) encode(text string) ([]int, error) {
	var out, spaceIDs []int
	var spaceSeg string
	for len(text) > 0 {
		n := segmentLen(text)
		seg := text[:n]
		text = text[n:]
		// A cut segment of only spaces is a single piece to the regex, and a
		// long run is the same 4 KB segment over and over: encode it once.
		if strings.Trim(seg, " ") == "" {
			if seg != spaceSeg {
				spaceSeg, spaceIDs = seg, encodePiece([]byte(seg), e.ranks)
			}
			out = append(out, spaceIDs...)
			continue
		}
		ids, err := e.encodeSegment(seg)
		if err != nil {
			return nil, err
		}
		out = append(out, ids...)
	}
	return out, nil
}

// segmentLen returns how much of text to hand the split regex at once: all
// of it, or up to a cut once a run of spaces reaches maxPieceBytes. On a long
// run the regex keeps a backtrack entry per rune, so 10 MB of spaces cost
// 1.3 GB. The cut lands where encodePiece would chunk the run anyway, and
// only where another space follows, so the run's last space still joins the
// next word as it does in one pass.
// ponytail: only U+0020 runs are cut. A long run of tabs or other Unicode
// spaces still goes to the regex whole; cut those too if they show up. When a
// tab or newline sits right before a 4 KB+ space run the count can move by
// one token per 4 KB of run (TestSegmentedMatchesSinglePass).
func segmentLen(text string) int {
	run := 0
	for i := 0; i < len(text); i++ {
		if text[i] != ' ' {
			run = 0
			continue
		}
		run++
		if run >= maxPieceBytes && i+1 < len(text) && text[i+1] == ' ' {
			return i + 1
		}
	}
	return len(text)
}

func (e *encoder) encodeSegment(text string) ([]int, error) {
	var out []int
	m, err := e.split.FindStringMatch(text)
	for m != nil && err == nil {
		// m.String() is built from runes, so invalid UTF-8 in text becomes
		// U+FFFD here, the same as tiktoken-go.
		piece := m.String()
		if r, ok := e.ranks[piece]; ok {
			out = append(out, r)
		} else {
			out = append(out, encodePiece([]byte(piece), e.ranks)...)
		}
		m, err = e.split.FindNextMatch(m)
	}
	return out, err
}

func (e *encoder) decode(ids []int) []byte {
	var out []byte
	for _, id := range ids {
		if id >= 0 && id < len(e.tokens) {
			out = append(out, e.tokens[id]...)
		}
	}
	return out
}

// get returns the encoder, built once per process.
func (e *Encoding) get() (*encoder, error) {
	e.once.Do(func() {
		ranks, tokens, err := loadBinaryVocab(e.bin, e.size)
		if err != nil {
			e.err = err
			return
		}
		split, err := regexp2.Compile(e.pattern, regexp2.None)
		if err != nil {
			e.err = err
			return
		}
		e.enc = &encoder{ranks: ranks, tokens: tokens, split: split}
	})
	return e.enc, e.err
}

// Count returns the number of tokens in text. Empty text returns 0 without
// touching the encoder. Returns an error if the embedded vocabulary cannot be
// loaded.
func (e *Encoding) Count(text string) (int, error) {
	ids, err := e.Encode(text)
	return len(ids), err
}

// Encode returns the token IDs for text. Returns nil for empty text without
// touching the encoder. Special tokens such as <|endoftext|> are encoded as
// ordinary text.
func (e *Encoding) Encode(text string) ([]int, error) {
	if text == "" {
		return nil, nil
	}
	enc, err := e.get()
	if err != nil {
		return nil, err
	}
	return enc.encode(text)
}

// Decode converts token IDs back to text. A window of tokens can end inside a
// multi-byte character; use DecodeBytes and utf8.Valid when that matters.
func (e *Encoding) Decode(ids []int) string {
	return string(e.DecodeBytes(ids))
}

// DecodeBytes returns the raw bytes for token IDs. The bytes are not always
// valid UTF-8: one token can hold part of a character. Unknown IDs are
// skipped. Returns nil for an empty slice.
func (e *Encoding) DecodeBytes(ids []int) []byte {
	if len(ids) == 0 {
		return nil
	}
	enc, err := e.get()
	if err != nil {
		return nil
	}
	return enc.decode(ids)
}

// CountTokens counts with the Default encoding.
func CountTokens(text string) (int, error) { return Default.Count(text) }
