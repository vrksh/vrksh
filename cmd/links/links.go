// Package links implements vrk links — a hyperlink extractor.
// Reads markdown, HTML, or plain text from stdin and writes one JSONL record
// per link to stdout: {"text":"...","url":"...","line":N}
package links

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

// Compiled regexes for link extraction.
var (
	// Markdown inline: [text](url)
	reMarkdownInline = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	// Markdown ref usage: [text][label]
	reMarkdownRef = regexp.MustCompile(`\[([^\]]*)\]\[([^\]]+)\]`)
	// Markdown ref definition at line start: [label]: url
	reRefDef = regexp.MustCompile(`^\s*\[([^\]]+)\]:\s*(\S+)`)
	// HTML <a href="url">text</a> — case-insensitive via (?i)
	reHTMLAnchor = regexp.MustCompile(`(?i)<a\s[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	// Bare URLs
	reBareURL = regexp.MustCompile(`https?://[^\s<>"')\]]+`)
	// Inner HTML tags (for stripping anchor text)
	reHTMLTag = regexp.MustCompile(`<[^>]+>`)
)

// linkRecord is one emitted JSONL record.
type linkRecord struct {
	Text string `json:"text"`
	URL  string `json:"url"`
	Line int    `json:"line"`
}

// metaRecord is the trailing --json envelope.
type metaRecord struct {
	VRK   string `json:"_vrk"`
	Count int    `json:"count"`
}

// foundLink pairs a record with its byte offset on the line for sort ordering.
type foundLink struct {
	pos int
	rec linkRecord
}

// Run is the entry point for vrk links. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("links", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var bareFlag, jsonFlag bool
	fs.BoolVarP(&bareFlag, "bare", "b", false, "output URLs only, one per line")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `append {"_vrk":"links","count":N} after all records`)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	var raw []byte
	if args := fs.Args(); len(args) > 0 {
		raw = []byte(strings.Join(args, " "))
	} else {
		// TTY guard: interactive terminal with no piped input → usage error.
		if isTerminal(int(os.Stdin.Fd())) {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": "links: no input: pipe text to stdin",
					"code":  2,
				})
			}
			return shared.UsageErrorf("links: no input: pipe text to stdin")
		}

		var err error
		raw, err = readAll(os.Stdin)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("links: reading stdin: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("links: reading stdin: %v", err)
		}
	}

	// Empty input is valid — exit 0, no output (plus metadata if --json).
	if len(raw) == 0 {
		if jsonFlag {
			if err := shared.PrintJSON(&metaRecord{VRK: "links", Count: 0}); err != nil {
				return shared.Errorf("links: %v", err)
			}
		}
		return 0
	}

	// Strip exactly one trailing newline — tools like echo append one; printf
	// and echo -n do not. Stripping exactly one keeps meaningful content intact.
	input := strings.TrimSuffix(string(raw), "\n")
	lines := strings.Split(input, "\n")

	// Two-pass: pass 1 collects ref definitions, pass 2 emits links.
	refMap := collectRefDefs(lines)
	records := extractLinks(lines, refMap)

	enc := json.NewEncoder(os.Stdout)
	count := 0
	for _, rec := range records {
		if bareFlag {
			if _, err := fmt.Fprintln(os.Stdout, rec.URL); err != nil {
				return shared.Errorf("links: writing output: %v", err)
			}
		} else {
			if err := enc.Encode(rec); err != nil {
				return shared.Errorf("links: writing output: %v", err)
			}
		}
		count++
	}

	if jsonFlag {
		if err := enc.Encode(&metaRecord{VRK: "links", Count: count}); err != nil {
			return shared.Errorf("links: writing output: %v", err)
		}
	}

	return 0
}

// collectRefDefs does pass 1: scans lines for [label]: url definitions and
// returns a map of lowercased label → url.
func collectRefDefs(lines []string) map[string]string {
	refs := make(map[string]string)
	for _, line := range lines {
		m := reRefDef.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// Lowercase so lookups in extractFromLine agree regardless of source casing.
		label := strings.ToLower(strings.TrimSpace(m[1]))
		refs[label] = m[2]
	}
	return refs
}

// extractLinks does pass 2: walks each line and emits links in source order.
func extractLinks(lines []string, refMap map[string]string) []linkRecord {
	var result []linkRecord
	for i, line := range lines {
		result = append(result, extractFromLine(line, i+1, refMap)...)
	}
	return result
}

// extractFromLine returns all links found on a single line, in source order.
// Patterns are applied in this fixed priority order, and each matched byte
// range is added to `consumed` before the next pattern runs:
//
//  1. Markdown inline  [text](url)
//  2. Markdown ref     [text][label]
//  3. HTML anchor      <a href="url">text</a>
//  4. Bare URLs        https://...
//
// The ordering is an invariant: reMarkdownRef uses \[…\]\[…\] which could
// technically match the ](url) tail of an inline link if the URL contains no
// ]. Running reMarkdownInline first and marking those spans consumed is what
// makes step 2 safe. Do not reorder these steps without updating the overlap
// check logic.
func extractFromLine(line string, lineNum int, refMap map[string]string) []linkRecord {
	var links []foundLink
	consumed := make([][2]int, 0, 4)

	// 1. Markdown inline: [text](url)
	for _, m := range reMarkdownInline.FindAllStringSubmatchIndex(line, -1) {
		// m[0]:m[1]=full, m[2]:m[3]=text, m[4]:m[5]=url
		links = append(links, foundLink{
			pos: m[0],
			rec: linkRecord{Text: line[m[2]:m[3]], URL: line[m[4]:m[5]], Line: lineNum},
		})
		consumed = append(consumed, [2]int{m[0], m[1]})
	}

	// 2. Markdown ref usage: [text][label] — resolve via refMap.
	for _, m := range reMarkdownRef.FindAllStringSubmatchIndex(line, -1) {
		if overlaps(m[0], m[1], consumed) {
			continue
		}
		// Lowercase to match the case-folded keys stored by collectRefDefs.
		label := strings.ToLower(strings.TrimSpace(line[m[4]:m[5]]))
		url, ok := refMap[label]
		// Always consume the span so bare URL extraction skips it.
		consumed = append(consumed, [2]int{m[0], m[1]})
		if !ok {
			continue
		}
		links = append(links, foundLink{
			pos: m[0],
			rec: linkRecord{Text: line[m[2]:m[3]], URL: url, Line: lineNum},
		})
	}

	// 3. HTML <a href="url">text</a> — case-insensitive.
	for _, m := range reHTMLAnchor.FindAllStringSubmatchIndex(line, -1) {
		if overlaps(m[0], m[1], consumed) {
			continue
		}
		url := line[m[2]:m[3]]
		rawText := line[m[4]:m[5]]
		text := reHTMLTag.ReplaceAllString(rawText, "")
		links = append(links, foundLink{
			pos: m[0],
			rec: linkRecord{Text: text, URL: url, Line: lineNum},
		})
		consumed = append(consumed, [2]int{m[0], m[1]})
	}

	// 4. Mark ref definition spans consumed so their URLs aren't re-extracted
	//    as bare URLs. (Definitions are not emitted — only usages are.)
	if m := reRefDef.FindStringIndex(line); m != nil {
		consumed = append(consumed, [2]int{m[0], m[1]})
	}

	// 5. Bare URLs — only at positions not already consumed.
	for _, m := range reBareURL.FindAllStringIndex(line, -1) {
		if overlaps(m[0], m[1], consumed) {
			continue
		}
		url := line[m[0]:m[1]]
		links = append(links, foundLink{
			pos: m[0],
			rec: linkRecord{Text: url, URL: url, Line: lineNum},
		})
		consumed = append(consumed, [2]int{m[0], m[1]})
	}

	// Sort by byte position so links on the same line emit in source order.
	// Guard on len > 1: sort.Slice has non-trivial overhead even for 0/1 elements.
	if len(links) > 1 {
		sort.Slice(links, func(i, j int) bool { return links[i].pos < links[j].pos })
	}

	result := make([]linkRecord, len(links))
	for i, fl := range links {
		result[i] = fl.rec
	}
	return result
}

// overlaps reports whether [start, end) overlaps any range in consumed.
// Two ranges [a,b) and [c,d) overlap when a < d && c < b.
func overlaps(start, end int, consumed [][2]int) bool {
	for _, r := range consumed {
		if r[0] < end && start < r[1] {
			return true
		}
	}
	return false
}

func printUsage(fs *pflag.FlagSet) int {
	usage := []string{
		"usage: vrk links [flags]",
		"       echo '# Doc' | vrk links",
		"       cat README.md | vrk links",
		"",
		"Hyperlink extractor — reads markdown, HTML, or plain text from stdin.",
		`Writes one JSONL record per link: {"text":"...","url":"...","line":N}`,
		"",
		"flags:",
	}
	for _, l := range usage {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("links: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
