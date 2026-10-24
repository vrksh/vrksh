// Package tokcount counts, encodes and decodes cl100k_base tokens with an
// embedded vocabulary. No network access.
//
// The encoder follows github.com/pkoukk/tiktoken-go (MIT), a Go port of
// OpenAI's tiktoken: the same split regex and the same byte pair merge order,
// so token IDs are identical (tokcount_test.go checks this against
// tiktoken-go). It is built here because tiktoken-go's constructor also
// builds a reverse map and sorts all 100k tokens, which was more than half of
// `vrk tok` startup time.
package tokcount

import (
	_ "embed"
	"errors"
	"sync"

	"github.com/dlclark/regexp2"
)

//go:generate go run gen_vocab.go

// cl100kBin is the cl100k_base vocab in rank order: one length byte, then the
// raw token bytes. Generated from data/cl100k_base.tiktoken by gen_vocab.go,
// so startup skips base64 decoding and number parsing.
//
//go:embed data/cl100k_base.bin
var cl100kBin []byte

// cl100kSize is the number of mergeable tokens in cl100k_base.
const cl100kSize = 100256

// cl100kPattern is the cl100k_base pre-tokenizer regex.
const cl100kPattern = `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`

// loadBinaryVocab builds the token-to-rank map and the rank-indexed token
// table from the binary vocab. Table entries are slices of data, not copies.
func loadBinaryVocab(data []byte) (map[string]int, [][]byte, error) {
	ranks := make(map[string]int, cl100kSize)
	tokens := make([][]byte, 0, cl100kSize)
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

// encoder is a cl100k_base byte pair encoder.
type encoder struct {
	ranks  map[string]int
	tokens [][]byte // tokens[rank] is the raw bytes of that token
	split  *regexp2.Regexp
}

func (e *encoder) encode(text string) ([]int, error) {
	var out []int
	m, err := e.split.FindStringMatch(text)
	for m != nil && err == nil {
		// m.String() is built from runes, so invalid UTF-8 in text becomes
		// U+FFFD here, the same as tiktoken-go.
		piece := m.String()
		if r, ok := e.ranks[piece]; ok {
			out = append(out, r)
		} else {
			out = append(out, bytePairEncode([]byte(piece), e.ranks)...)
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

var (
	encOnce   sync.Once
	cachedEnc *encoder
	encErr    error
)

// getEncoder returns the cl100k_base encoder, built once per process.
func getEncoder() (*encoder, error) {
	encOnce.Do(func() {
		ranks, tokens, err := loadBinaryVocab(cl100kBin)
		if err != nil {
			encErr = err
			return
		}
		split, err := regexp2.Compile(cl100kPattern, regexp2.None)
		if err != nil {
			encErr = err
			return
		}
		cachedEnc = &encoder{ranks: ranks, tokens: tokens, split: split}
	})
	return cachedEnc, encErr
}

// CountTokens returns the number of cl100k_base tokens in text. Empty text
// returns 0 without touching the encoder. Returns an error if the embedded
// vocabulary cannot be loaded.
func CountTokens(text string) (int, error) {
	ids, err := EncodeTokens(text)
	return len(ids), err
}

// EncodeTokens returns the cl100k_base token ID slice for text.
// Returns nil for empty text without touching the encoder. Special tokens
// such as <|endoftext|> are encoded as ordinary text.
func EncodeTokens(text string) ([]int, error) {
	if text == "" {
		return nil, nil
	}
	e, err := getEncoder()
	if err != nil {
		return nil, err
	}
	return e.encode(text)
}

// DecodeTokens converts a slice of cl100k_base token IDs back to text.
// Returns "" for an empty slice. A window of tokens can end inside a
// multi-byte character; use DecodeBytes and utf8.Valid when that matters.
func DecodeTokens(ids []int) string {
	return string(DecodeBytes(ids))
}

// DecodeBytes returns the raw bytes for a slice of cl100k_base token IDs.
// The bytes are not always valid UTF-8: one token can hold part of a
// character. Unknown IDs are skipped. Returns nil for an empty slice.
func DecodeBytes(ids []int) []byte {
	if len(ids) == 0 {
		return nil
	}
	e, err := getEncoder()
	if err != nil {
		return nil
	}
	return e.decode(ids)
}
