package grab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

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

// errInternalAddress is returned by the SSRF-safe dialer when the resolved
// address falls within a private, loopback, or link-local range.
var errInternalAddress = errors.New("grab: request to internal network address is blocked (use --allow-internal with VRK_ALLOW_INTERNAL=1 to override)")

// defaultMaxSize is the default response body size limit (10MB).
const defaultMaxSize = 10 * 1024 * 1024

// blockedPrefixes are the special-purpose ranges (RFC 6890 and friends) on top
// of what IsGlobalUnicast already rules out: private, CGNAT (100.64/10, where
// Alibaba metadata and Tailscale peers live), benchmarking, documentation,
// reserved, IPv4-compatible, NAT64 local-use, discard, Teredo and ULA.
var blockedPrefixes = func() []netip.Prefix {
	var ps []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
		"::/96", "64:ff9b:1::/48", "100::/64", "2001::/32", "2001:db8::/32", "fc00::/7",
		"fe80::/10", "ff00::/8",
	} {
		ps = append(ps, netip.MustParsePrefix(s))
	}
	return ps
}()

var (
	nat64  = netip.MustParsePrefix("64:ff9b::/96")
	sixTo4 = netip.MustParsePrefix("2002::/16")
)

// isInternalIP returns true if ip is loopback, private, link-local or any other
// address that should never be reached from an untrusted URL.
func isInternalIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	// NAT64 and 6to4 addresses carry an IPv4 address inside: 64:ff9b::7f00:1
	// reaches 127.0.0.1 through a NAT64 gateway. Check the embedded one rather
	// than blocking the prefix, since on IPv6-only networks DNS64 puts every
	// public IPv4 site under 64:ff9b::/96.
	b := a.As16()
	switch {
	case nat64.Contains(a):
		return isInternalIP(net.IP(b[12:16]))
	case sixTo4.Contains(a):
		return isInternalIP(net.IP(b[2:6]))
	}
	return false
}

// ssrfSafeDialer returns a DialContext function that resolves the target address
// and rejects connections to internal/private IP ranges. This check runs on
// every connection attempt, including redirect hops, because the HTTP client
// calls DialContext for each new TCP connection.
func ssrfSafeDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ipAddr := range ips {
			if isInternalIP(ipAddr.IP) {
				return nil, errInternalAddress
			}
		}
		// Connect to the first resolved address directly.
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

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

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "grab",
		Short: "URL fetcher - clean markdown, plain text, or raw HTML",
		Flags: []shared.FlagMeta{
			{Name: "text", Shorthand: "t", Usage: "plain prose output, no markdown syntax"},
			{Name: "raw", Usage: "raw HTML, no processing"},
			{Name: "json", Shorthand: "j", Usage: "emit JSON envelope with metadata"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
			{Name: "max-size", Usage: "max response body size in bytes (default 10MB)"},
			{Name: "allow-internal", Usage: "allow requests to private/loopback addresses (needs VRK_ALLOW_INTERNAL=1)"},
		},
	})
}

// Run is the entry point for vrk grab. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("grab", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var textFlag, rawFlag, jsonFlag, quietFlag, allowInternal bool
	var maxSize int
	fs.BoolVarP(&textFlag, "text", "t", false, "plain prose output, no markdown syntax")
	fs.BoolVar(&rawFlag, "raw", false, "raw HTML, no processing")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON envelope with metadata")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")
	fs.IntVar(&maxSize, "max-size", defaultMaxSize, "max response body size in bytes")
	fs.BoolVar(&allowInternal, "allow-internal", false, "allow requests to private/loopback addresses (needs VRK_ALLOW_INTERNAL=1)")

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

	// The SSRF guard is there because a model picks the URL, and the same model
	// can pass --allow-internal. So the flag only counts when whoever runs vrk
	// has also set VRK_ALLOW_INTERNAL=1 in the environment.
	if allowInternal && os.Getenv("VRK_ALLOW_INTERNAL") != "1" {
		return emitError(jsonFlag, 2, "", 0, "grab: --allow-internal needs VRK_ALLOW_INTERNAL=1 in the environment")
	}

	// Validate URL — invalid format is a usage error (exit 2).
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme == "" && u.Host == "") {
		if err == nil {
			rawURL = u.Redacted()
		}
		return emitError(jsonFlag, 2, rawURL, 0, "grab: invalid URL %q: must be an absolute http or https URL", rawURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return emitError(jsonFlag, 2, u.Redacted(), 0, "grab: invalid URL %q: scheme must be http or https, got %q", u.Redacted(), u.Scheme)
	}

	// Strip fragment — it is client-only and not sent to the server.
	u.Fragment = ""
	cleanURL := u.String()
	// shownURL is what we print: the password in user:pass@ becomes ***.
	shownURL := u.Redacted()

	// Build a cookie-free HTTP client with redirect cap.
	// When --allow-internal is not set, use a custom transport with a DialContext
	// hook that rejects connections to private/loopback/link-local addresses.
	// This check runs on every TCP connection, including redirect hops.
	var transport http.RoundTripper
	if !allowInternal {
		transport = &http.Transport{DialContext: ssrfSafeDialer()}
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		Jar:       nil,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errTooManyRedirects
			}
			// Go sets Referer to the previous URL, which hands any ?token= in
			// it to whatever host the redirect points at.
			req.Header.Del("Referer")
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
			return emitError(jsonFlag, 1, shownURL, 0, "grab: too many redirects (> 5)")
		}
		return emitError(jsonFlag, 1, shownURL, 0, "grab: fetch failed: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return emitError(jsonFlag, 1, shownURL, resp.StatusCode,
			"grab: HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxSize)))
	if err != nil {
		return emitError(jsonFlag, 1, shownURL, 0, "grab: reading response body: %v", err)
	}
	if len(body) >= maxSize {
		return emitError(jsonFlag, 1, shownURL, resp.StatusCode, "grab: response body exceeded %d byte limit for %s", maxSize, shownURL)
	}

	fetchedAt := time.Now().Unix()
	// Resolve relative links against the final URL without its user:pass@,
	// or every link in the output would carry the credentials.
	base := *resp.Request.URL
	base.User = nil
	finalURL := resp.Request.URL.Redacted()

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
		// Non-HTML bodies stay byte-exact in a pipe, but a terminal must not
		// get the server's escape sequences (OSC 52 clipboard writes and such).
		if isTerminal(int(os.Stdout.Fd())) {
			body = []byte(shared.StripControls(string(body)))
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

	content, err := renderMarkdown(mainNode, &base, 4*max(maxSize, defaultMaxSize))
	if err != nil {
		return emitError(jsonFlag, 1, shownURL, resp.StatusCode, "grab: %v", err)
	}
	if textFlag {
		content = plaintext.StripMarkdown(content)
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

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("grab", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolP("text", "t", false, "plain prose output, no markdown syntax")
	fs.Bool("raw", false, "raw HTML, no processing")
	fs.BoolP("json", "j", false, "emit JSON envelope with metadata")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	fs.Int("max-size", defaultMaxSize, "max response body size in bytes")
	fs.Bool("allow-internal", false, "allow requests to private/loopback addresses (needs VRK_ALLOW_INTERNAL=1)")
	return fs
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
				title = strings.TrimSpace(shared.StripControls(n.FirstChild.Data))
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

// maxQuoteDepth caps the "> " prefix. x/net/html allows 512 open elements, and
// 512 levels of "> " on every line is how a small page became gigabytes.
const maxQuoteDepth = 8

// mdWriter collects the rendered markdown. Blockquote prefixes are written as
// each line starts, so nested quotes cost one pass instead of a re-render per
// level. Newlines are held back and collapsed to at most two (one blank line),
// and every piece of text goes through StripControls so a page can't put
// terminal escapes in the output.
type mdWriter struct {
	sb        strings.Builder
	depth     int // blockquote depth for the next line
	lineDepth int // depth of the last line written
	nl        int // newlines waiting to be written
	limit     int
	over      bool
}

func (w *mdWriter) WriteString(s string) {
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			w.text(s)
			return
		}
		w.text(s[:i])
		w.nl++
		s = s[i+1:]
	}
}

func quotePrefix(depth int) string {
	return strings.Repeat("> ", min(depth, maxQuoteDepth))
}

func (w *mdWriter) text(s string) {
	s = shared.StripControls(s)
	if s == "" || w.over {
		return
	}
	if w.nl > 0 || w.sb.Len() == 0 {
		if w.sb.Len() > 0 {
			w.sb.WriteByte('\n')
			if w.nl > 1 {
				// A blank line stays inside the quote only if both sides are.
				w.sb.WriteString(strings.TrimSuffix(quotePrefix(min(w.lineDepth, w.depth)), " "))
				w.sb.WriteByte('\n')
			}
		}
		w.sb.WriteString(quotePrefix(w.depth))
		w.lineDepth = w.depth
		w.nl = 0
	}
	w.sb.WriteString(s)
	if w.sb.Len() > w.limit {
		w.over = true
	}
}

// writeSpace adds one space unless the output is empty or already ends in whitespace.
func (w *mdWriter) writeSpace() {
	s := w.sb.String()
	if s == "" || w.nl > 0 {
		return
	}
	switch s[len(s)-1] {
	case ' ', '\t':
		return
	}
	w.sb.WriteString(" ")
}

// renderMarkdown converts an HTML subtree to Markdown, failing once the output
// passes limit bytes.
func renderMarkdown(root *html.Node, base *url.URL, limit int) (string, error) {
	w := &mdWriter{limit: limit}
	renderMdNode(root, w, false, base)
	if w.over {
		return "", fmt.Errorf("rendered output exceeds %d bytes", limit)
	}
	return strings.TrimSpace(w.sb.String()), nil
}

func renderMdNode(n *html.Node, sb *mdWriter, inPre bool, base *url.URL) {
	if n.Type == html.ElementNode && skipTags[n.DataAtom] {
		return
	}
	switch n.Type {
	case html.TextNode:
		if inPre {
			sb.WriteString(n.Data)
		} else {
			// Collapse whitespace to single spaces, but keep a space at either
			// edge where the HTML had one: "<b>bold</b> word" needs the space
			// between the closing ** and "word".
			fields := strings.Fields(n.Data)
			if len(fields) == 0 || strings.TrimLeftFunc(n.Data, unicode.IsSpace) != n.Data {
				sb.writeSpace()
			}
			if len(fields) > 0 {
				sb.WriteString(strings.Join(fields, " "))
				if strings.TrimRightFunc(n.Data, unicode.IsSpace) != n.Data {
					sb.WriteString(" ")
				}
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
		inner := &mdWriter{limit: sb.limit}
		renderChildrenMd(n, inner, false, base)
		// Collapse newlines/whitespace so link text never spans multiple lines.
		text := strings.Join(strings.Fields(inner.sb.String()), " ")
		if href != "" && text != "" {
			sb.WriteString(fmt.Sprintf("[%s](%s)", text, href))
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
		sb.WriteString("\n\n")
		sb.depth++
		renderChildrenMd(n, sb, false, base)
		sb.depth--
		sb.WriteString("\n\n")
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
			sb.WriteString(fmt.Sprintf("![%s](%s)", alt, src))
		}
	default:
		renderChildrenMd(n, sb, inPre, base)
	}
}

func renderChildrenMd(n *html.Node, sb *mdWriter, inPre bool, base *url.URL) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderMdNode(c, sb, inPre, base)
	}
}
