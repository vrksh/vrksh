package pdf

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	lpdf "github.com/ledongthuc/pdf"
	"golang.org/x/text/unicode/norm"
)

// The library's own text extraction ignores glyph positions, so PDFs that
// space words with kerning instead of space characters (everything LaTeX
// makes) come out as one run-on word. This walks the content stream itself,
// tracks where each glyph lands, and puts spaces and line breaks back from
// the gaps between them.

// maxFormDepth caps Form XObject nesting. Real files nest a level or two; a
// form that draws itself would otherwise recurse forever.
const maxFormDepth = 8

// matrix is a PDF transform [a b c d e f], applied to row vectors.
type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

func (m matrix) mul(n matrix) matrix {
	return matrix{
		m[0]*n[0] + m[1]*n[2], m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2], m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4], m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

func translate(x, y float64) matrix { return matrix{1, 0, 0, 1, x, y} }

// glyph is one decoded character in device space.
type glyph struct {
	s                string
	x, y, endX, endY float64 // where the glyph starts and where the pen ends up
	ux, uy           float64 // unit vector along the baseline
	size             float64 // rendered font size
}

// font is what the walker needs from a font dictionary.
type font struct {
	enc     lpdf.TextEncoding // nil: bytes are used as-is
	twoByte bool              // Type0 fonts use 2-byte codes (Identity-H and friends)
	widths  map[int]float64   // per-code advance, in 1/1000 em
	dw      float64           // advance for codes missing from widths
}

func loadFont(f lpdf.Font) *font {
	v := f.V
	ft := &font{enc: f.Encoder(), widths: map[int]float64{}}
	if v.Key("Subtype").Name() == "Type0" {
		ft.twoByte = true
		desc := v.Key("DescendantFonts").Index(0)
		ft.dw = 1000
		if dw := desc.Key("DW"); !dw.IsNull() {
			ft.dw = dw.Float64()
		}
		// /W is a mix of "c [w1 w2 ...]" and "cFirst cLast w".
		w := desc.Key("W")
		for i := 0; i+1 < w.Len(); {
			first := int(w.Index(i).Int64())
			if next := w.Index(i + 1); next.Kind() == lpdf.Array {
				for j := 0; j < next.Len(); j++ {
					ft.widths[first+j] = next.Index(j).Float64()
				}
				i += 2
				continue
			}
			last, wv := int(w.Index(i+1).Int64()), w.Index(i+2).Float64()
			for c := first; c <= last && c-first <= 0xffff; c++ {
				ft.widths[c] = wv
			}
			i += 3
		}
		return ft
	}
	first, ws := int(v.Key("FirstChar").Int64()), v.Key("Widths")
	for i := 0; i < ws.Len(); i++ {
		ft.widths[first+i] = ws.Index(i).Float64()
	}
	ft.dw = 500
	if mw := v.Key("FontDescriptor").Key("MissingWidth"); mw.Float64() > 0 {
		ft.dw = mw.Float64()
	}
	if ws.Len() == 0 {
		// The standard 14 fonts ship without /Widths.
		// ponytail: the table assumes WinAnsiEncoding; a font with its own
		// /Differences gets WinAnsi widths, close enough for gap detection.
		name := stdFontName(v.Key("BaseFont").Name())
		if name == "Courier" {
			ft.dw = 600
		} else if tbl, ok := std14Widths[name]; ok {
			for i, w := range tbl {
				if w > 0 {
					ft.widths[32+i] = float64(w)
				}
			}
		}
	}
	return ft
}

// stdFontName maps a BaseFont to its key in std14Widths, or "Courier".
// Arial and Times New Roman are metric-compatible stand-ins.
func stdFontName(base string) string {
	if i := strings.IndexByte(base, '+'); i == 6 {
		base = base[7:] // subset prefix like ABCDEF+
	}
	bold := strings.Contains(base, "Bold")
	italic := strings.Contains(base, "Italic") || strings.Contains(base, "Oblique")
	switch {
	case strings.HasPrefix(base, "Courier"):
		return "Courier"
	case strings.HasPrefix(base, "Helvetica"), strings.HasPrefix(base, "Arial"):
		if bold {
			return "Helvetica-Bold"
		}
		return "Helvetica"
	case strings.HasPrefix(base, "Times"):
		switch {
		case bold && italic:
			return "Times-BoldItalic"
		case bold:
			return "Times-Bold"
		case italic:
			return "Times-Italic"
		}
		return "Times-Roman"
	}
	return ""
}

func (f *font) decode(raw string) string {
	if f.enc == nil {
		return raw
	}
	return f.enc.Decode(raw)
}

// textState is the part of the graphics state that moves glyphs.
type textState struct {
	ctm, tm, tlm               matrix
	font                       *font
	tfs, th, tc, tw, tl, trise float64
}

// walker interprets content streams and collects the glyphs that land
// inside the page box.
type walker struct {
	st    textState
	stack []textState
	out   []glyph
	box   [4]float64 // llx lly urx ury
	clip  bool
}

// pageGlyphs returns a page's glyphs in drawing order. The interpreter can
// panic on a broken stream; that comes back as err, along with the glyphs
// read before it.
func pageGlyphs(p lpdf.Page) (gs []glyph, err error) {
	w := &walker{st: textState{ctm: identity, tm: identity, tlm: identity, th: 1}}
	box := inherited(p.V, "CropBox")
	if box.Len() != 4 {
		box = inherited(p.V, "MediaBox")
	}
	if box.Len() == 4 {
		x0, y0, x1, y1 := box.Index(0).Float64(), box.Index(1).Float64(), box.Index(2).Float64(), box.Index(3).Float64()
		w.box = [4]float64{math.Min(x0, x1), math.Min(y0, y1), math.Max(x0, x1), math.Max(y0, y1)}
		w.clip = w.box[2] > w.box[0] && w.box[3] > w.box[1]
	}
	defer func() {
		if r := recover(); r != nil {
			gs, err = w.out, fmt.Errorf("%v", r)
		}
	}()
	w.run(p.V.Key("Contents"), inherited(p.V, "Resources"), 0)
	return w.out, nil
}

// inherited looks key up on the page and then its ancestors, with a depth cap
// so a /Parent cycle can't hang.
func inherited(v lpdf.Value, key string) lpdf.Value {
	for i := 0; i < maxTreeDepth && !v.IsNull(); i++ {
		if r := v.Key(key); !r.IsNull() {
			return r
		}
		v = v.Key("Parent")
	}
	return lpdf.Value{}
}

// run interprets one content stream (a page's, or a form's) against res.
// Malformed operators are skipped.
func (w *walker) run(contents, res lpdf.Value, depth int) {
	if contents.IsNull() {
		return
	}
	// Font names are local to a resource dictionary, so each stream gets its
	// own cache.
	fonts := map[string]*font{}

	show := func(s string) {
		st := &w.st
		f := st.font
		if f == nil {
			return
		}
		step := 1
		if f.twoByte {
			step = 2
		}
		for i := 0; i+step <= len(s); i += step {
			raw := s[i : i+step]
			code := int(raw[0])
			if step == 2 {
				code = code<<8 | int(raw[1])
			}
			w0, ok := f.widths[code]
			if !ok || w0 == 0 {
				w0 = f.dw
			}
			scale := matrix{st.tfs * st.th, 0, 0, st.tfs, 0, st.trise}
			start := scale.mul(st.tm).mul(st.ctm)
			tx := w0/1000*st.tfs + st.tc
			if raw == " " {
				tx += st.tw
			}
			st.tm = translate(tx*st.th, 0).mul(st.tm)
			end := scale.mul(st.tm).mul(st.ctm)

			x, y := start[4], start[5]
			// Text outside the page box (printer's marks, proof slugs) isn't
			// on the page. A couple of points of slack for rounding.
			if w.clip && (x < w.box[0]-2 || x > w.box[2]+2 || y < w.box[1]-2 || y > w.box[3]+2) {
				continue
			}
			g := glyph{s: norm.NFKC.String(f.decode(raw)), x: x, y: y, endX: end[4], endY: end[5], ux: 1}
			g.size = math.Hypot(start[2], start[3])
			if n := math.Hypot(start[0], start[1]); n > 0 {
				g.ux, g.uy = start[0]/n, start[1]/n
			}
			w.out = append(w.out, g)
		}
	}
	nextLine := func(tx, ty float64) {
		w.st.tlm = translate(tx, ty).mul(w.st.tlm)
		w.st.tm = w.st.tlm
	}

	lpdf.Interpret(contents, func(stk *lpdf.Stack, op string) {
		args := make([]lpdf.Value, stk.Len())
		for i := len(args) - 1; i >= 0; i-- {
			args[i] = stk.Pop()
		}
		num := func(i int) float64 { return args[i].Float64() }
		mat := func() matrix {
			var m matrix
			for i := range m {
				m[i] = num(i)
			}
			return m
		}
		st := &w.st
		switch {
		case op == "q":
			w.stack = append(w.stack, *st)
		case op == "Q" && len(w.stack) > 0:
			*st = w.stack[len(w.stack)-1]
			w.stack = w.stack[:len(w.stack)-1]
		case op == "cm" && len(args) == 6:
			st.ctm = mat().mul(st.ctm)
		case op == "BT":
			st.tm, st.tlm = identity, identity
		case op == "Tm" && len(args) == 6:
			st.tm = mat()
			st.tlm = st.tm
		case op == "Td" && len(args) == 2:
			nextLine(num(0), num(1))
		case op == "TD" && len(args) == 2:
			st.tl = -num(1)
			nextLine(num(0), num(1))
		case op == "T*":
			nextLine(0, -st.tl)
		case op == "TL" && len(args) == 1:
			st.tl = num(0)
		case op == "Tc" && len(args) == 1:
			st.tc = num(0)
		case op == "Tw" && len(args) == 1:
			st.tw = num(0)
		case op == "Tz" && len(args) == 1:
			st.th = num(0) / 100
		case op == "Ts" && len(args) == 1:
			st.trise = num(0)
		case op == "Tf" && len(args) == 2:
			name := args[0].Name()
			if _, ok := fonts[name]; !ok {
				fonts[name] = nil
				if fv := res.Key("Font").Key(name); !fv.IsNull() {
					fonts[name] = loadFont(lpdf.Font{V: fv})
				}
			}
			st.font, st.tfs = fonts[name], num(1)
		case op == "Tj" && len(args) == 1:
			show(args[0].RawString())
		case op == "'" && len(args) == 1:
			nextLine(0, -st.tl)
			show(args[0].RawString())
		case op == `"` && len(args) == 3:
			st.tw, st.tc = num(0), num(1)
			nextLine(0, -st.tl)
			show(args[2].RawString())
		case op == "TJ" && len(args) == 1:
			a := args[0]
			for i := 0; i < a.Len(); i++ {
				if e := a.Index(i); e.Kind() == lpdf.String {
					show(e.RawString())
				} else {
					st.tm = translate(-e.Float64()/1000*st.tfs*st.th, 0).mul(st.tm)
				}
			}
		case op == "Do" && len(args) == 1 && depth < maxFormDepth:
			// Form XObjects are content streams of their own: figure labels,
			// callout boxes, and whole pages in some generators.
			xo := res.Key("XObject").Key(args[0].Name())
			if xo.Key("Subtype").Name() != "Form" {
				return
			}
			saved, savedStack := *st, w.stack
			m := identity
			if fm := xo.Key("Matrix"); fm.Len() == 6 {
				for i := range m {
					m[i] = fm.Index(i).Float64()
				}
			}
			st.ctm = m.mul(st.ctm)
			formRes := xo.Key("Resources")
			if formRes.IsNull() {
				formRes = res
			}
			w.stack = nil
			w.run(xo, formRes, depth+1)
			w.st, w.stack = saved, savedStack
		}
	})
}

// line is one line of a page's text.
type line struct {
	text    string
	size    float64 // average font size of its characters
	brk     bool    // a paragraph break comes before it
	upright bool    // runs left to right, not rotated
}

// layout joins glyphs into lines. A gap wider than a fraction of the font
// size becomes a space; a jump off the baseline starts a new line, and a jump
// well past the page's usual line pitch is a paragraph break.
func layout(gs []glyph) []line {
	var lines []line
	var downs []float64 // per line: distance below the line before, 0 if not below
	var b strings.Builder
	var sizeSum float64
	var chars int
	var prev *glyph // last glyph placed, so the last of the line inside end
	end := func(down float64) {
		size := 0.0
		if chars > 0 {
			size = sizeSum / float64(chars)
		}
		lines = append(lines, line{text: strings.TrimRight(b.String(), " "), size: size, upright: prev.ux > 0.9})
		downs = append(downs, down)
		b.Reset()
		sizeSum, chars = 0, 0
	}

	firstDown := 0.0
	for i := range gs {
		g := &gs[i]
		if g.s == "" {
			continue
		}
		if prev != nil {
			size := math.Max(math.Max(g.size, prev.size), 1)
			// Offset from where the pen stopped, along and across prev's baseline.
			dx, dy := g.x-prev.endX, g.y-prev.endY
			along := dx*prev.ux + dy*prev.uy
			down := dx*prev.uy - dy*prev.ux
			turned := g.ux*prev.ux+g.uy*prev.uy < 0.9
			switch {
			case turned || math.Abs(down) > size*0.5 || along < -size:
				end(firstDown)
				firstDown = 0
				if !turned && down > 0 {
					firstDown = down
				}
			case along > size*0.15 && !isSpace(prev.s) && !isSpace(g.s):
				b.WriteByte(' ')
			}
		}
		b.WriteString(g.s)
		sizeSum += g.size * float64(utf8.RuneCountInString(g.s))
		chars += utf8.RuneCountInString(g.s)
		prev = g
	}
	if prev != nil {
		end(firstDown)
	}

	// The lower quartile of line pitches is the page's normal line spacing,
	// so double-spaced text doesn't turn every line into a paragraph.
	var pitches []float64
	for _, d := range downs {
		if d > 0 {
			pitches = append(pitches, d)
		}
	}
	pitch := 0.0
	if len(pitches) > 0 {
		sort.Float64s(pitches)
		pitch = pitches[len(pitches)/4]
	}
	for i := 1; i < len(lines); i++ {
		size := math.Max(math.Max(lines[i].size, lines[i-1].size), 1)
		lines[i].brk = downs[i] > math.Max(size*1.8, pitch*1.4)
	}
	return lines
}

func isSpace(s string) bool { return strings.TrimSpace(s) == "" }

// plainText renders a page's lines as text, with a blank line at each
// paragraph break.
func plainText(lines []line) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
			if l.brk {
				b.WriteByte('\n')
			}
		}
		b.WriteString(l.text)
	}
	return strings.Trim(b.String(), "\r\n")
}

// sizeKey rounds a font size to half a point so 11.98 and 12 count as one.
func sizeKey(s float64) float64 { return math.Round(s*2) / 2 }

// bodySize is the font size most characters use.
func bodySize(pages [][]line) float64 {
	chars := map[float64]int{}
	for _, p := range pages {
		for _, l := range p {
			chars[sizeKey(l.size)] += utf8.RuneCountInString(l.text)
		}
	}
	body, most := 0.0, -1
	for s, n := range chars {
		if n > most || n == most && s < body {
			body, most = s, n
		}
	}
	return body
}

// headingRun is lines [start, end) of one page that read as a heading: larger
// than body type, horizontal, no paragraph break inside, short, and followed
// by smaller text. A long run
// of large type is a notice or a pull quote, not a heading.
type headingRun struct {
	start, end int
	size       float64
}

func headingRuns(lines []line, body float64) []headingRun {
	var runs []headingRun
	for i := 0; i < len(lines); {
		s := sizeKey(lines[i].size)
		if s < body*1.15 || !lines[i].upright {
			i++
			continue
		}
		j, n := i+1, utf8.RuneCountInString(lines[i].text)
		for j < len(lines) && !lines[j].brk && lines[j].upright && sizeKey(lines[j].size) == s {
			n += utf8.RuneCountInString(lines[j].text)
			j++
		}
		text := ""
		for _, l := range lines[i:j] {
			text += l.text
		}
		// A heading leads into smaller text. Labels inside figures are
		// followed by more labels, rotated axis text, or nothing.
		leads := j < len(lines) && lines[j].upright && sizeKey(lines[j].size) < s
		if leads && n <= 120 && strings.IndexFunc(text, unicode.IsLetter) >= 0 {
			runs = append(runs, headingRun{i, j, s})
		}
		i = j
	}
	return runs
}

// headings is the document's body size and heading levels by size: the
// largest heading size is #, the next ##, the rest ###.
type headings struct {
	body   float64
	levels map[float64]int
}

func findHeadings(pages [][]line) headings {
	h := headings{body: bodySize(pages), levels: map[float64]int{}}
	var sizes []float64
	for _, p := range pages {
		for _, r := range headingRuns(p, h.body) {
			if _, ok := h.levels[r.size]; !ok {
				h.levels[r.size] = 0
				sizes = append(sizes, r.size)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sizes)))
	for i, s := range sizes {
		h.levels[s] = min(i+1, 3)
	}
	return h
}

// markdown renders a page's lines as Markdown: heading runs become headings,
// bullet glyphs become list items, paragraph breaks become blank lines.
func markdown(lines []line, h headings) string {
	var blocks, para []string
	flush := func() {
		if len(para) > 0 {
			blocks = append(blocks, strings.Join(para, "\n"))
			para = nil
		}
	}
	runs := headingRuns(lines, h.body)
	for i := 0; i < len(lines); {
		if len(runs) > 0 && runs[0].start == i {
			r := runs[0]
			runs = runs[1:]
			flush()
			parts := make([]string, 0, r.end-r.start)
			for _, l := range lines[r.start:r.end] {
				parts = append(parts, strings.TrimSpace(l.text))
			}
			blocks = append(blocks, strings.Repeat("#", h.levels[r.size])+" "+strings.Join(parts, " "))
			i = r.end
			continue
		}
		l := lines[i]
		i++
		t := strings.TrimSpace(l.text)
		if l.brk || t == "" {
			flush()
		}
		if t != "" {
			para = append(para, mdLine(t))
		}
	}
	flush()
	return strings.Join(blocks, "\n\n")
}

// mdLine turns a bullet glyph into a list marker and escapes line starts that
// Markdown would otherwise read as syntax.
func mdLine(t string) string {
	for _, bullet := range []string{"•", "◦", "▪", "‣", "●", "∙"} {
		if rest, ok := strings.CutPrefix(t, bullet); ok {
			return "- " + strings.TrimSpace(rest)
		}
	}
	switch {
	case strings.HasPrefix(t, "#"), strings.HasPrefix(t, ">"),
		strings.HasPrefix(t, "```"), strings.HasPrefix(t, "~~~"),
		strings.Trim(t, "-") == "", strings.Trim(t, "=") == "":
		return `\` + t
	}
	return t
}
