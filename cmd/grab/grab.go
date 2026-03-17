package grab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/tokcount"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/vrksh/vrksh/internal/shared/plaintext"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

const userAgent = "vrk/0 (https://vrk.sh)"

// errTooManyRedirects is the sentinel returned by CheckRedirect after 5 hops.
var errTooManyRedirects = errors.New("too many redirects (> 5)")

// grabResult is the shape emitted by --json on success.
type grabResult struct {
	URL           string `json:"url"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	FetchedAt     int64  `json:"fetched_at"`
	Status        int    `json:"status"`
	TokenEstimate int    `json:"token_estimate"`
}

// grabErrorResult is the shape emitted by --json on runtime error.
type grabErrorResult struct {
	Error  string `json:"error"`
	URL    string `json:"url"`
	Status int    `json:"status"`
	Code   int    `json:"code"`
}

// emitError routes an error to stdout as JSON (when jsonFlag is true) or to
// stderr as plain text (otherwise). code must be 1 (runtime) or 2 (usage).
func emitError(jsonFlag bool, code int, rawURL string, status int, format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	if !jsonFlag {
		if code == 2 {
			return shared.UsageErrorf("%s", msg)
		}
		return shared.Errorf("%s", msg)
	}
	rec := &grabErrorResult{Error: msg, URL: rawURL, Status: status, Code: code}
	if err := json.NewEncoder(os.Stdout).Encode(rec); err != nil {
		// Last-ditch: can't write JSON, fall back to stderr so something surfaces.
		if code == 2 {
			return shared.UsageErrorf("%s", msg)
		}
		return shared.Errorf("%s", msg)
	}
	return code
}

// Run is the entry point for vrk grab. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("grab", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var textFlag, rawFlag, jsonFlag, quietFlag bool
	fs.BoolVarP(&textFlag, "text", "t", false, "plain prose output, no markdown syntax")
	fs.BoolVar(&rawFlag, "raw", false, "raw HTML, no processing")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON envelope with metadata")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// Exactly one output mode allowed.
	modes := 0
	if textFlag {
		modes++
	}
	if rawFlag {
		modes++
	}
	if jsonFlag {
		modes++
	}
	if modes > 1 {
		return shared.UsageErrorf("grab: --text, --raw, and --json are mutually exclusive")
	}

	// Resolve URL from positional arg or stdin.
	args := fs.Args()
	var rawURL string
	if len(args) > 0 {
		rawURL = args[0]
	} else {
		if isTerminal(int(os.Stdin.Fd())) {
			return shared.UsageErrorf("grab: no URL provided")
		}
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return shared.Errorf("grab: reading stdin: %v", err)
		}
		rawURL = strings.TrimSpace(string(b))
		if rawURL == "" {
			return shared.UsageErrorf("grab: no URL provided")
		}
	}

	// Validate URL — invalid format is a usage error (exit 2).
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme == "" && u.Host == "") {
		return emitError(jsonFlag, 2, rawURL, 0, "grab: invalid URL %q: must be an absolute http or https URL", rawURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return emitError(jsonFlag, 2, rawURL, 0, "grab: invalid URL %q: scheme must be http or https, got %q", rawURL, u.Scheme)
	}

	// Strip fragment — it is client-only and not sent to the server.
	u.Fragment = ""
	cleanURL := u.String()

	// Build a cookie-free HTTP client with redirect cap.
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     nil,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errTooManyRedirects
			}
			return nil
		},
	}

	req, err := http.NewRequest(http.MethodGet, cleanURL, nil)
	if err != nil {
		return shared.Errorf("grab: building request: %v", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		// Detect our sentinel so the message is human-readable, not Go internals.
		if errors.Is(err, errTooManyRedirects) {
			return emitError(jsonFlag, 1, cleanURL, 0, "grab: too many redirects (> 5)")
		}
		return emitError(jsonFlag, 1, cleanURL, 0, "grab: fetch failed: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return emitError(jsonFlag, 1, cleanURL, resp.StatusCode,
			"grab: HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return emitError(jsonFlag, 1, cleanURL, 0, "grab: reading response body: %v", err)
	}

	fetchedAt := time.Now().Unix()
	finalURL := resp.Request.URL.String()

	// --raw: emit bytes as-is regardless of content type.
	if rawFlag {
		if _, err := os.Stdout.Write(body); err != nil {
			return shared.Errorf("grab: writing output: %v", err)
		}
		return 0
	}

	// Determine whether the response is HTML.
	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	isHTML := ct == "text/html"

	if !isHTML {
		// Non-HTML: pass body through for all non-raw modes. --text is a no-op.
		if jsonFlag {
			content := string(body)
			toks, _ := tokcount.CountTokens(content)
			return emitJSON(&grabResult{
				URL:           finalURL,
				Title:         "",
				Content:       content,
				FetchedAt:     fetchedAt,
				Status:        resp.StatusCode,
				TokenEstimate: toks,
			})
		}
		if _, err := os.Stdout.Write(body); err != nil {
			return shared.Errorf("grab: writing output: %v", err)
		}
		return 0
	}

	// Parse HTML and extract content.
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return shared.Errorf("grab: parsing HTML: %v", err)
	}

	title := extractTitle(doc)
	mainNode := findMainContent(doc)

	base, _ := url.Parse(finalURL)

	var content string
	if textFlag {
		content = plaintext.StripMarkdown(renderMarkdown(mainNode, base))
	} else {
		content = renderMarkdown(mainNode, base)
	}

	if jsonFlag {
		toks, _ := tokcount.CountTokens(content)
		return emitJSON(&grabResult{
			URL:           finalURL,
			Title:         title,
			Content:       content,
			FetchedAt:     fetchedAt,
			Status:        resp.StatusCode,
			TokenEstimate: toks,
		})
	}

	if _, err := fmt.Fprint(os.Stdout, content); err != nil {
		return shared.Errorf("grab: writing output: %v", err)
	}
	// Ensure output ends with a newline.
	if !strings.HasSuffix(content, "\n") {
		if _, err := fmt.Fprintln(os.Stdout); err != nil {
			return shared.Errorf("grab: writing output: %v", err)
		}
	}
	return 0
}

// emitJSON writes v as a compact single-line JSON record to stdout.
func emitJSON(v any) int {
	if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
		return shared.Errorf("grab: encoding JSON: %v", err)
	}
	return 0
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: grab [flags] <url>",
		"       echo <url> | grab [flags]",
		"",
		"URL fetcher — fetches a URL and returns clean markdown (default),",
		"plain text (--text), or raw HTML (--raw). Applies Readability-style",
		"content extraction. JavaScript is not executed (static HTML only).",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("grab: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}

// --- HTML extraction ---

// skipTags lists elements whose entire subtree is stripped from extraction.
var skipTags = map[atom.Atom]bool{
	atom.Script: true,
	atom.Style:  true,
	atom.Nav:    true,
	atom.Header: true,
	atom.Footer: true,
	atom.Aside:  true,
	atom.Form:   true,
	atom.Iframe: true,
}

// extractTitle returns the text content of the first <title> element.
func extractTitle(doc *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Title {
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				title = strings.TrimSpace(n.FirstChild.Data)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return title
}

// findMainContent returns the most content-rich subtree. Prefers <main> then
// <article>, falls back to <body>, then the document root.
func findMainContent(doc *html.Node) *html.Node {
	var main, article, body *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom { //nolint:exhaustive // only care about these three tags
			case atom.Main:
				if main == nil {
					main = n
				}
			case atom.Article:
				if article == nil {
					article = n
				}
			case atom.Body:
				body = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	switch {
	case main != nil:
		return main
	case article != nil:
		return article
	case body != nil:
		return body
	default:
		return doc
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// resolveURL resolves href against base. Absolute URLs pass through unchanged.
func resolveURL(base *url.URL, href string) string {
	if base == nil || href == "" {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

// renderMarkdown converts an HTML subtree to Markdown.
func renderMarkdown(root *html.Node, base *url.URL) string {
	var sb strings.Builder
	renderMdNode(root, &sb, false, base)
	// Collapse runs of 3+ newlines to 2.
	result := collapseNewlines(sb.String())
	return strings.TrimSpace(result)
}

func renderMdNode(n *html.Node, sb *strings.Builder, inPre bool, base *url.URL) {
	if n.Type == html.ElementNode && skipTags[n.DataAtom] {
		return
	}
	switch n.Type {
	case html.TextNode:
		if inPre {
			sb.WriteString(n.Data)
		} else {
			// Normalize whitespace in non-pre context.
			normalized := strings.Join(strings.Fields(n.Data), " ")
			if normalized != "" {
				sb.WriteString(normalized)
				sb.WriteString(" ")
			}
		}
		return
	case html.ElementNode:
		// handled below
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderMdNode(c, sb, inPre, base)
		}
		return
	}

	switch n.DataAtom { //nolint:exhaustive // default: handles all unrecognized atoms
	case atom.H1:
		sb.WriteString("\n\n# ")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n\n")
	case atom.H2:
		sb.WriteString("\n\n## ")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n\n")
	case atom.H3:
		sb.WriteString("\n\n### ")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n\n")
	case atom.H4:
		sb.WriteString("\n\n#### ")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n\n")
	case atom.H5:
		sb.WriteString("\n\n##### ")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n\n")
	case atom.H6:
		sb.WriteString("\n\n###### ")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n\n")
	case atom.P:
		sb.WriteString("\n\n")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n\n")
	case atom.Br:
		sb.WriteString("\n")
	case atom.A:
		href := resolveURL(base, getAttr(n, "href"))
		var inner strings.Builder
		renderChildrenMd(n, &inner, false, base)
		// Collapse newlines/whitespace so link text never spans multiple lines.
		text := strings.Join(strings.Fields(inner.String()), " ")
		if href != "" && text != "" {
			fmt.Fprintf(sb, "[%s](%s)", text, href)
		} else if text != "" {
			sb.WriteString(text)
		}
	case atom.Strong, atom.B:
		sb.WriteString("**")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("**")
	case atom.Em, atom.I:
		sb.WriteString("*")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("*")
	case atom.Code:
		if !inPre {
			sb.WriteString("`")
			renderChildrenMd(n, sb, false, base)
			sb.WriteString("`")
		} else {
			renderChildrenMd(n, sb, true, base)
		}
	case atom.Pre:
		sb.WriteString("\n\n```\n")
		renderChildrenMd(n, sb, true, base)
		sb.WriteString("\n```\n\n")
	case atom.Blockquote:
		var inner strings.Builder
		renderChildrenMd(n, &inner, false, base)
		sb.WriteString("\n\n")
		for _, line := range strings.Split(strings.TrimSpace(inner.String()), "\n") {
			sb.WriteString("> ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	case atom.Ul, atom.Ol:
		sb.WriteString("\n\n")
		renderChildrenMd(n, sb, false, base)
		sb.WriteString("\n")
	case atom.Li:
		sb.WriteString("\n- ")
		renderChildrenMd(n, sb, false, base)
	case atom.Img:
		alt := getAttr(n, "alt")
		src := resolveURL(base, getAttr(n, "src"))
		if alt != "" {
			fmt.Fprintf(sb, "![%s](%s)", alt, src)
		}
	default:
		renderChildrenMd(n, sb, inPre, base)
	}
}

func renderChildrenMd(n *html.Node, sb *strings.Builder, inPre bool, base *url.URL) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderMdNode(c, sb, inPre, base)
	}
}

// reThreeNewlines matches runs of three or more consecutive newlines.
var reThreeNewlines = regexp.MustCompile(`\n{3,}`)

// collapseNewlines reduces runs of 3+ consecutive newlines to exactly 2.
func collapseNewlines(s string) string {
	return reThreeNewlines.ReplaceAllString(s, "\n\n")
}
