package tokcount

import (
	"bufio"
	"encoding/base64"
	"strconv"
	"strings"

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

// getEncoder returns a cl100k_base encoder backed by the embedded vocabulary.
// tiktoken.SetBpeLoader is package-global state; calling it here is safe
// because this is the only package that manages the BPE loader.
func getEncoder() (*tiktoken.Tiktoken, error) {
	tiktoken.SetBpeLoader(embeddedBpeLoader{})
	return tiktoken.GetEncoding("cl100k_base")
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
