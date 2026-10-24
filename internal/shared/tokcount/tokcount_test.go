package tokcount

import (
	"encoding/base64"
	"math/rand"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/pkoukk/tiktoken-go"
)

// Known cl100k_base counts verified against the tiktoken reference implementation.
// These are the ground-truth values the package must never regress on.
var countTests = []struct {
	input string
	want  int
}{
	{"", 0},
	{"hello world", 2},
	{"Hello, world!", 4},
	// Single token — a common English word.
	{"the", 1},
	// Whitespace-only: spaces tokenize as their own tokens.
	{" ", 1},
	// Longer prose: token count grows sub-linearly with word count.
	{"The quick brown fox jumps over the lazy dog", 9},
}

func TestCountTokens(t *testing.T) {
	for _, tc := range countTests {
		got, err := Cl100k.Count(tc.input)
		if err != nil {
			t.Errorf("Cl100k.Count(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Cl100k.Count(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// TestCountTokensEmpty ensures the fast path (empty string) returns 0
// without invoking the encoder at all.
func TestCountTokensEmpty(t *testing.T) {
	got, err := Cl100k.Count("")
	if err != nil {
		t.Fatalf("Cl100k.Count(%q): unexpected error: %v", "", err)
	}
	if got != 0 {
		t.Errorf("Cl100k.Count(%q) = %d, want 0", "", got)
	}
}

// TestCountTokensMonotone is a property test: more text never produces
// fewer tokens than a strict prefix of that text.
func TestCountTokensMonotone(t *testing.T) {
	base := "hello"
	extended := "hello world"
	a, err := Cl100k.Count(base)
	if err != nil {
		t.Fatalf("Cl100k.Count(%q): %v", base, err)
	}
	b, err := Cl100k.Count(extended)
	if err != nil {
		t.Fatalf("Cl100k.Count(%q): %v", extended, err)
	}
	if b < a {
		t.Errorf("Cl100k.Count(%q)=%d < Cl100k.Count(%q)=%d: more text produced fewer tokens",
			extended, b, base, a)
	}
}

// TestCountTokensIdempotent verifies that calling CountTokens twice on the
// same input returns the same result (the global BPE loader must be stable).
func TestCountTokensIdempotent(t *testing.T) {
	input := "idempotent check"
	a, err := Cl100k.Count(input)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	b, err := Cl100k.Count(input)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if a != b {
		t.Errorf("Cl100k.Count(%q) not idempotent: first=%d second=%d", input, a, b)
	}
}

// pinnedIDs are cl100k_base token IDs captured before the encoder was cached
// and before the vocab format changed. Any change to loading or caching must
// leave these exactly as they are.
var pinnedIDs = []struct {
	input string
	want  []int
}{
	{"hello world", []int{15339, 1917}},
	{"Hello, world!", []int{9906, 11, 1917, 0}},
	{"यह एक साधारण वाक्य है।", []int{5619, 107, 95048, 15272, 237, 65804, 69258, 32511, 100, 32511, 108, 5619, 96, 15272, 113, 32511, 243, 31584, 107, 85410, 12906, 230, 12906, 97}},
	{"Emoji party 🎉🚀🔥 time.", []int{93831, 4717, 11410, 236, 231, 9468, 248, 222, 9468, 242, 98, 892, 13}},
	{"Ελληνικά κείμενο εδώ.", []int{138, 243, 34586, 34586, 42524, 34369, 30862, 68437, 75234, 72738, 31243, 55241, 44223, 31243, 34369, 28654, 60247, 86486, 139, 236, 13}},
	{"東京は日本の首都です。", []int{14276, 109, 47653, 15682, 9080, 22656, 16144, 61075, 72368, 38641, 1811}},
	{"func main() {\n\tfmt.Println(\"hi\")\n}\n", []int{2900, 1925, 368, 341, 11254, 12701, 446, 6151, 1158, 534}},
	{"<|endoftext|>", []int{27, 91, 8862, 728, 428, 91, 29}},
}

func TestPinnedIDs(t *testing.T) {
	for _, tc := range pinnedIDs {
		got, err := Cl100k.Encode(tc.input)
		if err != nil {
			t.Fatalf("Cl100k.Encode(%q): %v", tc.input, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("Cl100k.Encode(%q) = %v, want %v", tc.input, got, tc.want)
		}
		if back := Cl100k.DecodeBytes(got); string(back) != tc.input {
			t.Errorf("DecodeBytes round trip for %q = %q", tc.input, back)
		}
	}
}

// TestEncoderBuiltOnce checks the encoder is cached. Rebuilding it per call
// made chunk 65 times slower than tok.
func TestEncoderBuiltOnce(t *testing.T) {
	a, err := Cl100k.get()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Cl100k.get()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("get returned a new encoder on the second call")
	}
}

// TestDecodeBytesPartialRune checks DecodeBytes returns the raw bytes of a
// token that holds only part of a multi-byte character, so callers can test
// utf8.Valid before converting.
func TestDecodeBytesPartialRune(t *testing.T) {
	got := Cl100k.DecodeBytes([]int{138})
	if utf8.Valid(got) {
		t.Errorf("Cl100k.DecodeBytes([138]) = %q, want an invalid partial rune", got)
	}
	if Cl100k.DecodeBytes(nil) != nil {
		t.Error("Cl100k.DecodeBytes(nil) should be nil")
	}
}

// parseTiktokenFile reads the upstream base64 vocab: "<base64 token> <rank>" per line.
func parseTiktokenFile(t *testing.T, path string) map[string]int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ranks := map[string]int{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		tok, rank, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("bad vocab line %q", line)
		}
		b, err := base64.StdEncoding.DecodeString(tok)
		if err != nil {
			t.Fatal(err)
		}
		r, err := strconv.Atoi(rank)
		if err != nil {
			t.Fatal(err)
		}
		ranks[string(b)] = r
	}
	return ranks
}

// TestBinaryVocabMatchesSource checks the generated binary vocab holds exactly
// the upstream entries with the same ranks. If it fails, run
// `go generate ./internal/shared/tokcount`.
func TestBinaryVocabMatchesSource(t *testing.T) {
	for _, e := range []*Encoding{Cl100k, O200k} {
		t.Run(e.Name, func(t *testing.T) { checkBinaryVocab(t, e) })
	}
}

func checkBinaryVocab(t *testing.T, e *Encoding) {
	want := parseTiktokenFile(t, "data/"+e.Name+".tiktoken")
	if len(want) != e.size {
		t.Fatalf("source vocab has %d entries, want %d", len(want), e.size)
	}
	got, tokens, err := loadBinaryVocab(e.bin, e.size)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != len(want) {
		t.Fatalf("binary token table has %d entries, want %d", len(tokens), len(want))
	}
	if len(got) != len(want) {
		t.Fatalf("binary vocab has %d entries, source has %d", len(got), len(want))
	}
	for tok, rank := range want {
		if got[tok] != rank {
			t.Fatalf("token %q: binary rank %d, source rank %d", tok, got[tok], rank)
		}
		if string(tokens[rank]) != tok {
			t.Fatalf("rank %d: table has %q, source has %q", rank, tokens[rank], tok)
		}
	}
}

func TestLoadBinaryVocabTruncated(t *testing.T) {
	if _, _, err := loadBinaryVocab([]byte{5, 'a', 'b'}, 1); err == nil {
		t.Error("want an error for a truncated vocab")
	}
}

// tiktokenGo is the reference encoder: tiktoken-go loading the same upstream
// vocab files. Built once per encoding for all differential tests.
var tiktokenGo = sync.OnceValue(func() map[string]*tiktoken.Tiktoken {
	tiktoken.SetBpeLoader(fileLoader{})
	refs := map[string]*tiktoken.Tiktoken{}
	for _, name := range []string{"cl100k_base", "o200k_base"} {
		enc, err := tiktoken.GetEncoding(name)
		if err != nil {
			panic(err)
		}
		refs[name] = enc
	}
	return refs
})

type fileLoader struct{}

// LoadTiktokenBpe reads data/<name>.tiktoken for the upstream URL tiktoken-go asks for.
func (fileLoader) LoadTiktokenBpe(url string) (map[string]int, error) {
	data, err := os.ReadFile("data/" + path.Base(url))
	if err != nil {
		return nil, err
	}
	ranks := map[string]int{}
	for _, line := range strings.Split(string(data), "\n") {
		tok, rank, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(tok)
		if err != nil {
			return nil, err
		}
		r, err := strconv.Atoi(rank)
		if err != nil {
			return nil, err
		}
		ranks[string(b)] = r
	}
	return ranks, nil
}

func assertSameAsTiktokenGo(t *testing.T, e *Encoding, input string) {
	t.Helper()
	ref := tiktokenGo()[e.Name]
	want := ref.Encode(input, nil, nil)
	got, err := e.Encode(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 && len(got) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s: Encode(%q) = %v, tiktoken-go gives %v", e.Name, input, got, want)
	}
	if g, w := e.DecodeBytes(got), []byte(ref.Decode(want)); string(g) != string(w) {
		t.Fatalf("%s: DecodeBytes differs for %q: %q vs %q", e.Name, input, g, w)
	}
}

// TestMatchesTiktokenGo is a differential property test: for any input, token
// IDs equal tiktoken-go's in both encodings. Covers every script, invalid
// UTF-8, special-token text, digits, whitespace runs, contractions, case
// changes and long repeated runs.
func TestMatchesTiktokenGo(t *testing.T) {
	fixed := []string{
		"", " ", "\n\n\n", "  \t  x", "it's we'll THEY'RE", "12345678901234567890",
		"\xff\xfe\x00bad\xe0\xa4", "<|endoftext|><|fim_prefix|>", strings.Repeat("x", 3000),
		strings.Repeat("ab", 1500), strings.Repeat("🎉", 300), strings.Repeat(" ", 2000) + "a",
		"https://example.com/a?b=c&d=e#f", "naïve café Straße ﬁ",
		"HTTPServer parseJSON iPhone McDonald's DON'T", "a/b/c\n/usr/local/bin/\r\n",
	}
	for _, tc := range pinnedIDs {
		fixed = append(fixed, tc.input)
	}
	rng := rand.New(rand.NewSource(1))
	alphabet := []rune("aZ09 _-.,'/\n\t\r🎉🚀東京日本語यहएक्षΕλληνικά한국어éßÉǅʰ\u200d\u0301")
	for _, e := range []*Encoding{Cl100k, O200k} {
		t.Run(e.Name, func(t *testing.T) {
			for _, s := range fixed {
				assertSameAsTiktokenGo(t, e, s)
			}
			for i := 0; i < 400; i++ {
				var sb strings.Builder
				for j := rng.Intn(300); j > 0; j-- {
					if rng.Intn(20) == 0 {
						sb.WriteByte(byte(rng.Intn(256)))
						continue
					}
					sb.WriteRune(alphabet[rng.Intn(len(alphabet))])
				}
				assertSameAsTiktokenGo(t, e, sb.String())
			}
		})
	}
}

// FuzzMatchesTiktokenGo extends the differential test to fuzzed input.
func FuzzMatchesTiktokenGo(f *testing.F) {
	for _, tc := range pinnedIDs {
		f.Add(tc.input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 2048 {
			return
		}
		assertSameAsTiktokenGo(t, Cl100k, input)
		assertSameAsTiktokenGo(t, O200k, input)
	})
}

// TestLongRepeatedRunIsFast guards the heap-based merge: 200 KB of one
// character used to take ~30 s with the quadratic merge.
func TestLongRepeatedRunIsFast(t *testing.T) {
	start := time.Now()
	n, err := Cl100k.Count(strings.Repeat("x", 200000))
	if err != nil {
		t.Fatal(err)
	}
	if n != 25000 {
		t.Fatalf("got %d tokens, want 25000", n)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("took %v, want well under 5s", d)
	}
}

func TestForModel(t *testing.T) {
	cases := map[string]*Encoding{
		"": O200k, "o200k_base": O200k, "gpt-5": O200k, "gpt-5.5": O200k, "GPT-4o": O200k,
		"gpt-4o-mini": O200k, "gpt-4.1": O200k, "o1": O200k, "o3-mini": O200k, "o4-mini": O200k,
		"chatgpt-4o-latest": O200k, "gpt-6.1": O200k,
		"cl100k_base": Cl100k, "gpt-4": Cl100k, "gpt-4-turbo": Cl100k, "gpt-3.5": Cl100k,
		"gpt-3.5-turbo": Cl100k, "gpt-35-turbo": Cl100k, "text-embedding-3-small": Cl100k,
	}
	for model, want := range cases {
		got, err := ForModel(model)
		if err != nil || got != want {
			t.Errorf("ForModel(%q) = %v, %v; want %s", model, got, err, want.Name)
		}
	}
	for _, bad := range []string{"claude-sonnet-4", "llama3", "o", "p50k_base"} {
		if _, err := ForModel(bad); err == nil {
			t.Errorf("ForModel(%q): want an error", bad)
		}
	}
}

// TestDefaultIsO200k pins the default and a few o200k IDs, so a change to
// either shows up here first.
func TestDefaultIsO200k(t *testing.T) {
	if Default != O200k {
		t.Fatalf("Default = %s, want o200k_base", Default.Name)
	}
	n, err := CountTokens("hello world")
	if err != nil || n != 2 {
		t.Fatalf("CountTokens(hello world) = %d, %v; want 2", n, err)
	}
	ids, err := O200k.Encode("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int{24912, 2375}) {
		t.Errorf("o200k Encode(hello world) = %v, want [24912 2375]", ids)
	}
}
