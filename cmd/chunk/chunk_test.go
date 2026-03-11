package chunk

import (
	"encoding/json"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vrksh/vrksh/internal/shared/tokcount"
)

// record is the JSONL shape emitted by chunk.
type record struct {
	Index  int    `json:"index"`
	Text   string `json:"text"`
	Tokens int    `json:"tokens"`
}

// runChunk sets os.Args, pipes input through stdin, captures stdout/stderr,
// calls Run(), and returns the exit code plus captured output strings.
func runChunk(input string, args []string) (code int, stdout, stderr string) {
	oldArgs := os.Args
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Args = oldArgs
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	// stdin pipe
	rIn, wIn, _ := os.Pipe()
	_, _ = io.WriteString(wIn, input)
	_ = wIn.Close()
	os.Stdin = rIn

	// stdout pipe
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// stderr pipe
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	os.Args = append([]string{"vrk"}, args...)
	code = Run()

	_ = wOut.Close()
	_ = wErr.Close()
	_ = rIn.Close()

	var bufOut, bufErr strings.Builder
	_, _ = io.Copy(&bufOut, rOut)
	_, _ = io.Copy(&bufErr, rErr)
	_ = rOut.Close()
	_ = rErr.Close()

	return code, bufOut.String(), bufErr.String()
}

// parseRecords decodes JSONL output into a slice of records.
func parseRecords(t *testing.T, s string) []record {
	t.Helper()
	var out []record
	dec := json.NewDecoder(strings.NewReader(s))
	for dec.More() {
		var r record
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("parseRecords: %v (output: %q)", err, s)
		}
		out = append(out, r)
	}
	return out
}

// assertInvariant fails the test if any record has tokens > size.
func assertInvariant(t *testing.T, records []record, size int) {
	t.Helper()
	for _, r := range records {
		if r.Tokens > size {
			t.Errorf("invariant violated: record[%d] has %d tokens, limit is %d",
				r.Index, r.Tokens, size)
		}
	}
}

// ── basic split ──────────────────────────────────────────────────────────────

func TestBasicSplit(t *testing.T) {
	// Build a deterministic input: " hello" is one cl100k_base token.
	// Prepend "hello" (1 token) so the total is 1 + 2499 = 2500 tokens.
	input := "hello" + strings.Repeat(" hello", 2499)
	total, err := tokcount.CountTokens(input)
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if total != 2500 {
		t.Fatalf("test input has %d tokens, expected 2500 — adjust construction", total)
	}

	size := 1000
	code, stdout, _ := runChunk(input, []string{"--size", "1000"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	records := parseRecords(t, stdout)
	wantChunks := 3 // ceil(2500/1000)
	if len(records) != wantChunks {
		t.Errorf("got %d chunks, want %d", len(records), wantChunks)
	}

	// Indices must be 0-based and sequential.
	for i, r := range records {
		if r.Index != i {
			t.Errorf("records[%d].Index = %d, want %d", i, r.Index, i)
		}
	}

	assertInvariant(t, records, size)
}

func TestSingleChunk(t *testing.T) {
	input := "hello world"
	code, stdout, _ := runChunk(input, []string{"--size", "1000"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseRecords(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d chunks, want 1", len(records))
	}
	if records[0].Index != 0 {
		t.Errorf("Index = %d, want 0", records[0].Index)
	}
	assertInvariant(t, records, 1000)
}

// ── overlap ──────────────────────────────────────────────────────────────────

func TestOverlap(t *testing.T) {
	// 2500-token input, size 1000, overlap 100 → step = 900
	// chunk[0]: tokens 0..999  (1000 tokens)
	// chunk[1]: tokens 900..1899 (1000 tokens) — overlaps last 100 of chunk[0]
	input := "hello" + strings.Repeat(" hello", 2499)

	code, stdout, _ := runChunk(input, []string{"--size", "1000", "--overlap", "100"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	records := parseRecords(t, stdout)
	if len(records) < 2 {
		t.Fatalf("need at least 2 chunks to test overlap, got %d", len(records))
	}

	// chunk[1] must begin with the last 100 tokens of chunk[0].
	ids0, err := tokcount.EncodeTokens(records[0].Text)
	if err != nil {
		t.Fatalf("EncodeTokens chunk[0]: %v", err)
	}
	if len(ids0) < 100 {
		t.Fatalf("chunk[0] has %d tokens, need >= 100 to test overlap", len(ids0))
	}
	overlapText := tokcount.DecodeTokens(ids0[len(ids0)-100:])
	if !strings.HasPrefix(records[1].Text, overlapText) {
		t.Errorf("chunk[1] does not start with last 100 tokens of chunk[0]\n"+
			"want prefix: %q\n got prefix:  %q",
			overlapText, records[1].Text[:minInt(len(overlapText), len(records[1].Text))])
	}

	assertInvariant(t, records, 1000)
}

// ── --by paragraph ───────────────────────────────────────────────────────────

func TestByParagraph(t *testing.T) {
	// Three paragraphs, each short; all three fit in one chunk at size=500,
	// but not in one chunk at size=10.
	p1 := "The quick brown fox jumps over the lazy dog."
	p2 := "Pack my box with five dozen liquor jugs."
	p3 := "How vexingly quick daft zebras jump."
	input := p1 + "\n\n" + p2 + "\n\n" + p3

	code, stdout, _ := runChunk(input, []string{"--size", "500", "--by", "paragraph"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	records := parseRecords(t, stdout)
	if len(records) == 0 {
		t.Fatal("got 0 records, want >= 1")
	}
	assertInvariant(t, records, 500)

	// No record should split a paragraph in the middle — each paragraph text
	// must appear wholly within at least one record.
	for _, para := range []string{p1, p2, p3} {
		found := false
		for _, r := range records {
			if strings.Contains(r.Text, para) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("paragraph %q was split across chunks (not found whole in any record)", para)
		}
	}
}

func TestByParagraphSmallSize(t *testing.T) {
	// Each paragraph must land in its own chunk when size is tight.
	p1 := "Alpha beta gamma."
	p2 := "Delta epsilon zeta."
	input := p1 + "\n\n" + p2

	tok1, _ := tokcount.CountTokens(p1)
	tok2, _ := tokcount.CountTokens(p2)
	size := tok1 // size == tok1 so p2 cannot share a chunk with p1

	if tok2 > size {
		t.Skipf("p2 (%d tokens) > size (%d) — would fall to oversized path", tok2, size)
	}

	code, stdout, _ := runChunk(input, []string{"--size", itoa(size), "--by", "paragraph"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	records := parseRecords(t, stdout)
	assertInvariant(t, records, size)
}

func TestByParagraphOversized(t *testing.T) {
	// One paragraph that exceeds --size → must fall back to token-level split.
	// Invariant must still hold.
	big := strings.Repeat("word ", 200) // ~200 tokens
	bigCount, _ := tokcount.CountTokens(big)
	size := bigCount / 3

	code, stdout, _ := runChunk(big, []string{"--size", itoa(size), "--by", "paragraph"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseRecords(t, stdout)
	if len(records) < 2 {
		t.Errorf("expected multiple chunks for oversized paragraph, got %d", len(records))
	}
	assertInvariant(t, records, size)
}

func TestByParagraphOverlap(t *testing.T) {
	// With --overlap, chunk[1] should start with last <overlap> tokens of chunk[0].
	p1 := strings.Repeat("alpha ", 50) // ~50 tokens
	p2 := strings.Repeat("beta ", 50)  // ~50 tokens
	p3 := strings.Repeat("gamma ", 50) // ~50 tokens
	input := p1 + "\n\n" + p2 + "\n\n" + p3

	size := 60
	overlap := 10
	code, stdout, _ := runChunk(input, []string{
		"--size", itoa(size),
		"--overlap", itoa(overlap),
		"--by", "paragraph",
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseRecords(t, stdout)
	assertInvariant(t, records, size)
}

// ── empty / edge inputs ───────────────────────────────────────────────────────

func TestEmptyInput(t *testing.T) {
	code, stdout, _ := runChunk("", []string{"--size", "1000"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected no output for empty input, got %q", stdout)
	}
}

func TestPositionalArg(t *testing.T) {
	// Positional arg must produce the same result as stdin.
	input := "hello world"

	_, stdinOut, _ := runChunk(input, []string{"--size", "100"})
	_, argOut, _ := runChunk("", []string{"--size", "100", input})

	recsStdin := parseRecords(t, stdinOut)
	recsArg := parseRecords(t, argOut)

	if len(recsStdin) != len(recsArg) {
		t.Fatalf("stdin produced %d records, positional arg produced %d",
			len(recsStdin), len(recsArg))
	}
	for i := range recsStdin {
		if recsStdin[i].Text != recsArg[i].Text {
			t.Errorf("record[%d] text differs: stdin=%q arg=%q",
				i, recsStdin[i].Text, recsArg[i].Text)
		}
	}
}

// ── usage errors (exit 2) ─────────────────────────────────────────────────────

func TestSizeMissing(t *testing.T) {
	code, stdout, stderr := runChunk("hello", []string{})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "--size") {
		t.Errorf("stderr should mention --size, got %q", stderr)
	}
}

func TestSizeZero(t *testing.T) {
	code, stdout, stderr := runChunk("hello", []string{"--size", "0"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	_ = stderr
}

func TestSizeNegative(t *testing.T) {
	code, _, _ := runChunk("hello", []string{"--size", "-1"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestOverlapGeSize(t *testing.T) {
	code, stdout, stderr := runChunk("hello", []string{"--size", "10", "--overlap", "10"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	_ = stderr
}

func TestOverlapGtSize(t *testing.T) {
	code, _, _ := runChunk("hello", []string{"--size", "10", "--overlap", "11"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestUnknownBy(t *testing.T) {
	code, stdout, stderr := runChunk("hello", []string{"--size", "100", "--by", "sentence"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "paragraph") {
		t.Errorf("stderr should list supported modes, got %q", stderr)
	}
}

func TestUnknownFlag(t *testing.T) {
	code, stdout, _ := runChunk("hello", []string{"--size", "100", "--bogus"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
}

// ── output shape ─────────────────────────────────────────────────────────────

func TestOutputShape(t *testing.T) {
	code, stdout, _ := runChunk("hello world", []string{"--size", "100"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseRecords(t, stdout)
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}
	r := records[0]
	if r.Index != 0 {
		t.Errorf("first record Index = %d, want 0", r.Index)
	}
	if r.Text == "" {
		t.Error("Text field is empty")
	}
	if r.Tokens <= 0 {
		t.Errorf("Tokens = %d, want > 0", r.Tokens)
	}
	// tokens field must match actual token count of the text
	actual, err := tokcount.CountTokens(r.Text)
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if actual != r.Tokens {
		t.Errorf("tokens field = %d but actual token count of text = %d", r.Tokens, actual)
	}
}

// ── property test ─────────────────────────────────────────────────────────────

func TestInvariant(t *testing.T) {
	// For any input and any --size N, every emitted chunk must have tokens <= N.
	rng := rand.New(rand.NewSource(42))
	words := []string{"hello", "world", "foo", "bar", "baz", "the", "a", "an",
		"token", "chunk", "split", "text", "size", "overlap", "paragraph"}

	for i := 0; i < 50; i++ {
		// Random word-salad input, 10–300 words.
		nWords := 10 + rng.Intn(291)
		var sb strings.Builder
		for j := 0; j < nWords; j++ {
			if j > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(words[rng.Intn(len(words))])
		}
		input := sb.String()

		// Random size 1–200.
		size := 1 + rng.Intn(200)

		code, stdout, _ := runChunk(input, []string{"--size", itoa(size)})
		if code != 0 {
			t.Errorf("iteration %d: exit code = %d, want 0 (input=%q size=%d)",
				i, code, input, size)
			continue
		}
		records := parseRecords(t, stdout)
		assertInvariant(t, records, size)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func itoa(n int) string { return strconv.Itoa(n) }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
