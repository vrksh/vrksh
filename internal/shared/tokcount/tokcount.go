package tokcount

import (
	"bufio"
	"encoding/base64"
	"strconv"
	"strings"
	"sync"

	_ "embed"

	"github.com/pkoukk/tiktoken-go"
)

//go:embed data/cl100k_base.tiktoken
var cl100kData []byte

// embeddedBpeLoader satisfies tiktoken.BpeLoader by reading from the embedded
// cl100k_base vocab instead of downloading from a URL. The URL argument is
// intentionally ignored — the embedded data is always cl100k_base.
type embeddedBpeLoader struct{}

func (embeddedBpeLoader) LoadTiktokenBpe(_ string) (map[string]int, error) {
	ranks := make(map[string]int, 100256)
	sc := bufio.NewScanner(strings.NewReader(string(cl100kData)))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			continue
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		ranks[string(token)] = rank
	}
	return ranks, sc.Err()
}

var (
	encOnce   sync.Once
	cachedEnc *tiktoken.Tiktoken
	encErr    error
)

// getEncoder returns the cl100k_base encoder backed by the embedded vocabulary.
// It is built once per process: building it means parsing 100k vocab entries,
// sorting them and compiling the split regex, which is far too slow to repeat
// per call. tiktoken.SetBpeLoader is package-global state; this is the only
// package that sets it.
func getEncoder() (*tiktoken.Tiktoken, error) {
	encOnce.Do(func() {
		tiktoken.SetBpeLoader(embeddedBpeLoader{})
		cachedEnc, encErr = tiktoken.GetEncoding("cl100k_base")
	})
	return cachedEnc, encErr
}

// CountTokens returns the number of cl100k_base tokens in text. Empty text
// returns 0 without touching the encoder. Returns an error if the embedded
// vocabulary cannot be loaded.
func CountTokens(text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	enc, err := getEncoder()
	if err != nil {
		return 0, err
	}
	return len(enc.Encode(text, nil, nil)), nil
}

// EncodeTokens returns the cl100k_base token ID slice for text.
// Returns nil for empty text without touching the encoder.
func EncodeTokens(text string) ([]int, error) {
	if text == "" {
		return nil, nil
	}
	enc, err := getEncoder()
	if err != nil {
		return nil, err
	}
	return enc.Encode(text, nil, nil), nil
}

// DecodeTokens converts a slice of cl100k_base token IDs back to text.
// Returns "" for an empty slice. A window of tokens can end inside a
// multi-byte character; use DecodeBytes and utf8.Valid when that matters.
func DecodeTokens(ids []int) string {
	return string(DecodeBytes(ids))
}

// DecodeBytes returns the raw bytes for a slice of cl100k_base token IDs.
// The bytes are not always valid UTF-8: one token can hold part of a
// character. Returns nil for an empty slice.
func DecodeBytes(ids []int) []byte {
	if len(ids) == 0 {
		return nil
	}
	e, err := getEncoder()
	if err != nil {
		return nil
	}
	return []byte(e.Decode(ids))
}
