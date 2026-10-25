package chunk

import (
	"encoding/json"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

	// stdin from a temp file so large inputs never block on a pipe buffer.
	in, _ := os.CreateTemp("", "chunk-stdin")
	defer func() { _ = os.Remove(in.Name()) }()
	_, _ = io.WriteString(in, input)
	_, _ = in.Seek(0, io.SeekStart)
	os.Stdin = in

	// stdout and stderr are drained concurrently so large outputs never block Run.
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr
	var bufOut, bufErr strings.Builder
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(&bufOut, rOut); done <- struct{}{} }()
	go func() { _, _ = io.Copy(&bufErr, rErr); done <- struct{}{} }()

	os.Args = append([]string{"vrk"}, args...)
	code = Run()

	_ = wOut.Close()
	_ = wErr.Close()
	<-done
	<-done
	_ = in.Close()
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
	// Build a deterministic input: " hello" is one token.
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
	ids0, err := tokcount.Default.Encode(records[0].Text)
	if err != nil {
		t.Fatalf("EncodeTokens chunk[0]: %v", err)
	}
	if len(ids0) < 100 {
		t.Fatalf("chunk[0] has %d tokens, need >= 100 to test overlap", len(ids0))
	}
	overlapText := tokcount.Default.Decode(ids0[len(ids0)-100:])
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

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on error.
// Exit code is unaffected. The --size flag is intentionally omitted to trigger
// the usage error that fires after the defer is registered.
func TestQuietSuppressesStderr(t *testing.T) {
	code, _, stderr := runChunk("", []string{"--quiet"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage: --size is required)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	code, stdout, stderr := runChunk("hello world this is a test", []string{"--size", "100", "--quiet"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if !strings.Contains(stdout, `"text"`) {
		t.Errorf("stdout = %q, want chunk JSONL record with text field", stdout)
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

// ── UTF-8 safety (multi-byte scripts) ────────────────────────────────────────

// multiByteSamples are sentences where tokens hold parts of characters.
var multiByteSamples = []string{
	"यह एक साधारण वाक्य है। ",
	"Emoji party 🎉🚀🔥 time. ",
	"Ελληνικά κείμενο εδώ. ",
	"東京は日本の首都です。",
}

// checkChunkInvariants asserts the chunk contract for any input:
//  1. every text is valid UTF-8 with no U+FFFD unless the input had one,
//  2. tokens is the real token count of text,
//  3. tokens never exceeds size by more than 3 (the one allowed overflow,
//     when a single character spans more tokens than fit).
func checkChunkInvariants(t *testing.T, input string, records []record, size int) {
	t.Helper()
	inputHasFFFD := strings.ContainsRune(input, utf8.RuneError)
	for _, r := range records {
		if !utf8.ValidString(r.Text) {
			t.Errorf("record[%d] text is not valid UTF-8: %q", r.Index, r.Text)
		}
		if !inputHasFFFD && strings.ContainsRune(r.Text, utf8.RuneError) {
			t.Errorf("record[%d] text has U+FFFD: %q", r.Index, r.Text)
		}
		n, err := tokcount.CountTokens(r.Text)
		if err != nil {
			t.Fatalf("CountTokens: %v", err)
		}
		if r.Tokens != n {
			t.Errorf("record[%d] tokens = %d, real count of text = %d (%q)", r.Index, r.Tokens, n, r.Text)
		}
		if r.Tokens > size+3 {
			t.Errorf("record[%d] tokens = %d, size = %d (overflow > 3)", r.Index, r.Tokens, size)
		}
	}
}

func joinTexts(records []record) string {
	var sb strings.Builder
	for _, r := range records {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

func TestMultiByteNoSplitCharacters(t *testing.T) {
	for _, s := range multiByteSamples {
		input := strings.Repeat(s, 60)
		for _, size := range []int{1, 2, 3, 5, 20, 37} {
			for _, overlap := range []int{0, 1, size / 2} {
				if overlap >= size {
					continue
				}
				code, stdout, stderr := runChunk(input, []string{"--size", itoa(size), "--overlap", itoa(overlap)})
				if code != 0 {
					t.Fatalf("%q size=%d overlap=%d: exit %d, stderr %q", s, size, overlap, code, stderr)
				}
				records := parseRecords(t, stdout)
				checkChunkInvariants(t, input, records, size)
				if overlap == 0 && joinTexts(records) != input {
					t.Errorf("%q size=%d: joined chunks differ from input", s, size)
				}
			}
		}
	}
}

func TestMultiByteByParagraph(t *testing.T) {
	var paras []string
	for _, s := range multiByteSamples {
		paras = append(paras, strings.Repeat(s, 3), strings.Repeat(s, 15))
	}
	input := strings.Join(paras, "\n\n")
	for _, size := range []int{5, 20, 60} {
		for _, overlap := range []int{0, 4} {
			code, stdout, stderr := runChunk(input, []string{"--size", itoa(size), "--overlap", itoa(overlap), "--by", "paragraph"})
			if code != 0 {
				t.Fatalf("size=%d: exit %d, stderr %q", size, code, stderr)
			}
			checkChunkInvariants(t, input, parseRecords(t, stdout), size)
		}
	}
}

// TestByParagraphKeepsSeparators checks packed paragraphs keep the blank line
// between them instead of being glued together.
func TestByParagraphKeepsSeparators(t *testing.T) {
	input := "First para.\n\nSecond para.\n\nThird."
	code, stdout, _ := runChunk(input, []string{"--size", "100", "--by", "paragraph"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	records := parseRecords(t, stdout)
	if len(records) != 1 || records[0].Text != input {
		t.Errorf("got %+v, want one chunk with text %q", records, input)
	}
	checkChunkInvariants(t, input, records, 100)
}

// randomMixedText builds text from a mix of scripts so token windows land
// inside multi-byte characters often.
func randomMixedText(rng *rand.Rand, n int) string {
	alphabet := []rune("ab z.\n🎉🚀🔥東京日本語यहएकΕλληνικά한국어é")
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteRune(alphabet[rng.Intn(len(alphabet))])
	}
	return sb.String()
}

// TestPropertyMultiByte: for random mixed-script input, any size and overlap,
// the chunk invariants hold and --overlap 0 rejoins to the input exactly.
func TestPropertyMultiByte(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 60; i++ {
		input := strings.TrimSuffix(randomMixedText(rng, 1+rng.Intn(400)), "\n")
		if input == "" {
			continue
		}
		size := 1 + rng.Intn(40)
		overlap := rng.Intn(size)
		args := []string{"--size", itoa(size), "--overlap", itoa(overlap)}
		if i%3 == 0 {
			args = append(args, "--by", "paragraph")
		}
		code, stdout, stderr := runChunk(input, args)
		if code != 0 {
			t.Fatalf("iteration %d: exit %d, stderr %q", i, code, stderr)
		}
		records := parseRecords(t, stdout)
		checkChunkInvariants(t, input, records, size)

		code, stdout, _ = runChunk(input, []string{"--size", itoa(size)})
		if code != 0 {
			t.Fatalf("iteration %d: exit %d", i, code)
		}
		if got := joinTexts(parseRecords(t, stdout)); got != input {
			t.Errorf("iteration %d size=%d: rejoin differs\n got %q\nwant %q", i, size, got, input)
		}
	}
}

// FuzzChunk: never panic, never hang, keep the invariants, finish fast.
func FuzzChunk(f *testing.F) {
	for _, s := range multiByteSamples {
		f.Add(strings.Repeat(s, 5), 3, 1)
	}
	f.Add("hello world", 1, 0)
	f.Add("a\n\nb\n\n\n\nc", 2, 1)
	f.Add("\xff\xfe broken \xe0\xa4", 2, 0)

	f.Fuzz(func(t *testing.T, input string, size, overlap int) {
		if len(input) > 4096 {
			return
		}
		size = 1 + abs(size)%50
		overlap = abs(overlap) % size
		start := time.Now()
		code, stdout, _ := runChunk(input, []string{"--size", itoa(size), "--overlap", itoa(overlap)})
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		records := parseRecords(t, stdout)
		if utf8.ValidString(input) {
			checkChunkInvariants(t, input, records, size)
		}
		if time.Since(start) > time.Second {
			t.Errorf("took %v, want under 1s", time.Since(start))
		}
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// TestModelFlag checks --model picks the encoding: the Hindi sentence is 24
// cl100k_base tokens and 9 o200k_base tokens.
func TestModelFlag(t *testing.T) {
	const hindi = "यह एक साधारण वाक्य है।"
	for args, want := range map[string]string{"": `"tokens":9`, "gpt-5": `"tokens":9`, "gpt-4": `"tokens":24`} {
		code, stdout, _ := runChunk(hindi, []string{"--size", "100", "--model", args})
		if code != 0 || !strings.Contains(stdout, want) {
			t.Errorf("--model %q: exit %d stdout %q, want %s", args, code, stdout, want)
		}
	}
	if code, stdout, _ := runChunk(hindi, []string{"--size", "100", "-m", "llama3"}); code != 2 || stdout != "" {
		t.Errorf("unknown model: exit %d stdout %q, want exit 2 and no output", code, stdout)
	}
}

// TestSplitStreamsRecords checks the splitters hand each record to emit as it
// is made and stop on an emit error, so output is never buffered in full
// (--size 1000 --overlap 999 on 200 KB used to hold 200 MB of records).
func TestSplitStreamsRecords(t *testing.T) {
	ids, err := tokcount.Default.Encode(strings.Repeat("alpha beta gamma delta ", 500))
	if err != nil {
		t.Fatal(err)
	}
	stop := io.ErrShortWrite
	for name, run := range map[string]func(emit func(string, int) error) error{
		"tokens": func(emit func(string, int) error) error {
			return splitByTokens(tokcount.Default, ids, 10, 9, emit)
		},
		"paragraph": func(emit func(string, int) error) error {
			return splitByParagraph(tokcount.Default, strings.Repeat("one two three\n\n", 200), 5, 0, emit)
		},
	} {
		calls := 0
		err := run(func(string, int) error { calls++; return stop })
		if err != stop || calls != 1 {
			t.Errorf("%s: err=%v calls=%d, want the emit error after 1 call", name, err, calls)
		}
	}
}
