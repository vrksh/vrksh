package pdf

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	lpdf "github.com/vrksh/vrksh/internal/pdfread"
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

// Per-page budgets. Depth alone doesn't stop a form that draws itself ten
// times at each of eight levels (10^8 runs from a 1 KB file). Real pages stay
// far below both.
const (
	maxGlyphs = 1_000_000
	maxForms  = 10_000
	maxSaves  = 10_000 // q nesting
)

// errBudget stops a page's walk when it goes over a budget.
var errBudget = errors.New("exceeds the per-page glyph or form budget; text after that is lost")

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
	bold             bool
}

// font is what the walker needs from a font dictionary.
type font struct {
	enc     lpdf.TextEncoding // nil: bytes are used as-is
	twoByte bool              // Type0 fonts use 2-byte codes (Identity-H and friends)
	widths  map[int]float64   // per-code advance, in 1/1000 em
	dw      float64           // advance for codes missing from widths
	bold    bool
}

func loadFont(f lpdf.Font) *font {
	v := f.V
	ft := &font{enc: f.Encoder(), widths: map[int]float64{}}
	ft.bold = isBold(v.Key("BaseFont").Name(), v.Key("FontDescriptor"))
	if v.Key("Subtype").Name() == "Type0" {
		ft.twoByte = true
		desc := v.Key("DescendantFonts").Index(0)
		ft.bold = ft.bold || isBold("", desc.Key("FontDescriptor"))
		ft.dw = 1000
		if dw := desc.Key("DW"); !dw.IsNull() {
			ft.dw = dw.Float64()
		}
		// /W is a mix of "c [w1 w2 ...]" and "cFirst cLast w". Codes are
		// two bytes, so more than 65,536 writes only repeat codes; stop
		// there so a few hundred full ranges can't cost millions.
		w := desc.Key("W")
		n := 0
		for i := 0; i+1 < w.Len() && n <= 0xffff; {
			first := int(w.Index(i).Int64())
			if next := w.Index(i + 1); next.Kind() == lpdf.Array {
				for j := 0; j < next.Len() && n <= 0xffff; j, n = j+1, n+1 {
					ft.widths[first+j] = next.Index(j).Float64()
				}
				i += 2
				continue
			}
			last, wv := int(w.Index(i+1).Int64()), w.Index(i+2).Float64()
			for c := first; c <= last && n <= 0xffff; c, n = c+1, n+1 {
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

// isBold reads weight off the font name, the way most PDFs say it, or off the
// descriptor's FontWeight or ForceBold flag. Nimbus calls its bold "Medi" and
// TeX's bold roman is CMBX.
func isBold(base string, desc lpdf.Value) bool {
	if i := strings.IndexByte(base, '+'); i == 6 {
		base = base[7:]
	}
	for _, w := range []string{"Bold", "Black", "Heavy", "Demi", "Semibold", "-Medi"} {
		if strings.Contains(base, w) {
			return true
		}
	}
	return strings.HasPrefix(base, "CMBX") || desc.Key("FontWeight").Float64() >= 600 ||
		desc.Key("Flags").Int64()&(1<<18) != 0
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
	forms int        // Form XObjects run so far
	box   [4]float64 // llx lly urx ury
	clip  bool
	fonts map[fontKey]*font
}

// fontKey names a font within a page walk. Font names are local to a
// resource dictionary, so the name alone isn't enough; the object holding
// the font tells dictionaries apart. A form drawn 10,000 times then loads
// its fonts once instead of once per run.
type fontKey struct {
	id   uint32
	gen  uint16
	name string
}

// pageGlyphs returns a page's glyphs in drawing order. The interpreter can
// panic on a broken stream; that comes back as err, along with the glyphs
// read before it.
func pageGlyphs(p lpdf.Page) (gs []glyph, err error) {
	w := &walker{st: textState{ctm: identity, tm: identity, tlm: identity, th: 1}, fonts: map[fontKey]*font{}}
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok && e == errBudget {
				gs, err = w.out, errBudget
				return
			}
			gs, err = w.out, fmt.Errorf("%v", r)
		}
	}()
	box := inherited(p.V, "CropBox")
	if box.Len() != 4 {
		box = inherited(p.V, "MediaBox")
	}
	if box.Len() == 4 {
		x0, y0, x1, y1 := box.Index(0).Float64(), box.Index(1).Float64(), box.Index(2).Float64(), box.Index(3).Float64()
		w.box = [4]float64{math.Min(x0, x1), math.Min(y0, y1), math.Max(x0, x1), math.Max(y0, y1)}
		w.clip = w.box[2] > w.box[0] && w.box[3] > w.box[1]
	}
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
			g := glyph{s: norm.NFKC.String(f.decode(raw)), x: x, y: y, endX: end[4], endY: end[5], ux: 1, bold: f.bold}
			g.size = math.Hypot(start[2], start[3])
			if n := math.Hypot(start[0], start[1]); n > 0 {
				g.ux, g.uy = start[0]/n, start[1]/n
			}
			if len(w.out) >= maxGlyphs {
				panic(errBudget)
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
		case op == "q" && len(w.stack) < maxSaves:
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
			fv := res.Key("Font").Key(name)
			id, gen := fv.ObjectID()
			k := fontKey{id, gen, name}
			f, ok := w.fonts[k]
			if !ok && !fv.IsNull() {
				f = loadFont(lpdf.Font{V: fv})
			}
			w.fonts[k] = f
			st.font, st.tfs = f, num(1)
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
			if w.forms++; w.forms > maxForms {
				panic(errBudget)
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
	size    float64   // average font size of its characters
	brk     bool      // a paragraph break comes before it
	upright bool      // runs left to right, not rotated
	bold    bool      // every letter in it is bold
	cells   []string  // text split at gaps wider than two ems, nil if none
	xs      []float64 // where each cell starts
	y       float64   // baseline of its first glyph, in page space
}

// columns reorders a page written row by row across two columns, left
// fragment then right fragment on each baseline, so the left column reads
// first. It needs a gutter that no glyph or word gap crosses and at least
// five lines that jump over it. Pages in stream order come back unchanged.
// ponytail: one gutter, whole page. A full-width title above row-ordered
// columns crosses the gutter and leaves the page alone; split by bands if
// that shows up.
func columns(gs []glyph) []glyph {
	type span struct{ a, b float64 }
	var jumps, joins []span
	var prev *glyph
	for i := range gs {
		g := &gs[i]
		if g.s == "" || g.ux < 0.9 {
			continue
		}
		joins = append(joins, span{g.x, g.endX})
		if prev != nil {
			size := math.Max(math.Max(g.size, prev.size), 1)
			if math.Abs(g.y-prev.endY) < size*0.5 {
				switch gap := g.x - prev.endX; {
				case gap > size*2:
					jumps = append(jumps, span{prev.endX, g.x})
				case gap > 0:
					joins = append(joins, span{prev.endX, g.x})
				}
			}
		}
		prev = g
	}
	if len(jumps) < 5 {
		return gs
	}
	mids := make([]float64, len(jumps))
	for i, j := range jumps {
		mids[i] = (j.a + j.b) / 2
	}
	sort.Float64s(mids)
	gutter := mids[len(mids)/2]
	over := 0
	for _, j := range jumps {
		if j.a < gutter && gutter < j.b {
			over++
		}
	}
	if over < 5 || over*2 < len(jumps) {
		return gs
	}
	for _, j := range joins {
		if j.a < gutter && gutter < j.b {
			return gs
		}
	}
	out := make([]glyph, 0, len(gs))
	for _, g := range gs {
		if g.x < gutter {
			out = append(out, g)
		}
	}
	for _, g := range gs {
		if g.x >= gutter {
			out = append(out, g)
		}
	}
	return out
}

// layout joins glyphs into lines. A gap wider than a fraction of the font
// size becomes a space; a jump off the baseline starts a new line, and a jump
// well past the page's usual line pitch is a paragraph break.
func layout(gs []glyph) []line {
	var lines []line
	var downs []float64 // per line: distance below the line before, 0 if not below
	var b strings.Builder
	var sizeSum, y float64
	var chars, letters, boldLetters int
	var cuts []int // builder offsets where a wide gap starts a new cell
	var xs []float64
	var prev *glyph // last glyph placed, so the last of the line inside end
	end := func(down float64) {
		size := 0.0
		if chars > 0 {
			size = sizeSum / float64(chars)
		}
		text := b.String()
		l := line{text: strings.TrimRight(text, " "), size: size, upright: prev.ux > 0.9,
			bold: letters > 0 && boldLetters == letters, y: y}
		if len(cuts) > 1 {
			cuts = append(cuts, len(text))
			for k := 1; k < len(cuts); k++ {
				l.cells = append(l.cells, strings.TrimSpace(text[cuts[k-1]:cuts[k]]))
			}
			l.xs = xs
		}
		lines = append(lines, l)
		downs = append(downs, down)
		b.Reset()
		sizeSum, chars, letters, boldLetters, cuts, xs = 0, 0, 0, 0, nil, nil
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
				if along > size*2 {
					cuts = append(cuts, b.Len())
					xs = append(xs, g.x)
				}
			}
		}
		if b.Len() == 0 {
			y = g.y
			cuts, xs = []int{0}, []float64{g.x}
		}
		b.WriteString(g.s)
		sizeSum += g.size * float64(utf8.RuneCountInString(g.s))
		chars += utf8.RuneCountInString(g.s)
		if strings.IndexFunc(g.s, unicode.IsLetter) >= 0 {
			letters++
			if g.bold {
				boldLetters++
			}
		}
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

// stripRunning drops running headers, footers and page numbers: lines among
// the top or bottom two of a page whose text, with digit runs as #, shows up
// there on at least three pages. Long documents change footers by chapter,
// so there's no share-of-pages rule. Only the pages being read count, so a
// single page keeps its header, and pages of four lines or fewer are left
// alone.
func stripRunning(pages [][]line) [][]line {
	if len(pages) < 3 {
		return pages
	}
	edge := make([]map[int]string, len(pages)) // per page: line index -> key
	seen := map[string]int{}
	for p, lines := range pages {
		idx := make([]int, 0, len(lines))
		for i, l := range lines {
			if l.upright && strings.TrimSpace(l.text) != "" {
				idx = append(idx, i)
			}
		}
		edge[p] = map[int]string{}
		if len(idx) <= 4 {
			continue // all edge and no body: a slide or a label, not a header
		}
		sort.SliceStable(idx, func(a, b int) bool { return lines[idx[a]].y > lines[idx[b]].y })
		idx = append(idx[:2], idx[len(idx)-2:]...)
		keys := map[string]bool{}
		for _, i := range idx {
			k := runningKey(lines[i].text)
			edge[p][i] = k
			if !keys[k] {
				keys[k] = true
				seen[k]++
			}
		}
	}
	out := make([][]line, len(pages))
	kept := map[string]bool{}
	for p, lines := range pages {
		for i, l := range lines {
			if k, ok := edge[p][i]; ok && seen[k] >= 3 {
				// The first one stays, so a table header repeated on every
				// page isn't lost. Page numbers go everywhere.
				if kept[k] || strings.IndexFunc(k, unicode.IsLetter) < 0 {
					continue
				}
				kept[k] = true
			}
			out[p] = append(out[p], l)
		}
	}
	return out
}

// joinHyphens re-joins words split across lines with a hyphen: "exist-" and
// "ing best" become "existing" and "best". Whether the hyphen stays is
// decided by the document's own words: "selfattention" never appears but
// "self-attention" does, and "sequence" and "aligned" are both words on their
// own, and "non" is hyphenated elsewhere as in "non-zero". TeX never breaks
// a word that already has a hyphen, so "stop-the-" and "English-" before
// "to-German" keep theirs. Otherwise the hyphen goes.
func joinHyphens(pages [][]line) {
	vocab, prefix := map[string]int{}, map[string]bool{}
	for _, p := range pages {
		for _, l := range p {
			for _, w := range strings.Fields(l.text) {
				w = strings.ToLower(strings.TrimFunc(w, notLetter))
				vocab[w]++
				if before, _, ok := strings.Cut(w, "-"); ok && !strings.HasSuffix(w, "-") {
					prefix[before] = true
				}
			}
		}
	}
	for _, p := range pages {
		for i := 0; i+1 < len(p); i++ {
			cur, next := &p[i], &p[i+1]
			head, ok := strings.CutSuffix(cur.text, "-")
			soft := false
			if !ok {
				head, soft = strings.CutSuffix(cur.text, "\u00ad")
			}
			r, _ := utf8.DecodeLastRuneInString(head)
			n, _ := utf8.DecodeRuneInString(next.text)
			if !ok && !soft || !unicode.IsLetter(r) || !unicode.IsLower(n) || next.brk || !next.upright ||
				cur.cells != nil || next.cells != nil {
				continue
			}
			tok, rest, _ := strings.Cut(next.text, " ")
			word := head[strings.LastIndexByte(head, ' ')+1:]
			w, f := strings.ToLower(word), strings.ToLower(strings.TrimFunc(tok, notLetter))
			// f always appears once, at the start of its own line.
			compound := vocab[w+"-"+f] > 0 || vocab[w] > 0 && vocab[f] > 1 || prefix[w] ||
				strings.Contains(w, "-") || strings.Contains(f, "-")
			hyphen := !soft && vocab[w+f] == 0 && compound
			if hyphen {
				head += "-"
			}
			cur.text, next.text = head+tok, strings.TrimLeft(rest, " ")
		}
	}
}

func notLetter(r rune) bool { return !unicode.IsLetter(r) && r != '-' }

// runningKey folds a line so "Page 3 of 10" and "Page 12 of 10" match.
func runningKey(s string) string {
	var b strings.Builder
	digit := false
	for _, r := range strings.ToLower(strings.Join(strings.Fields(s), " ")) {
		if unicode.IsDigit(r) {
			if !digit {
				b.WriteByte('#')
			}
			digit = true
			continue
		}
		digit = false
		b.WriteRune(r)
	}
	return b.String()
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

// headingRun is lines [start, end) of one page that read as a heading:
// horizontal, no paragraph break inside, short, and followed by body text.
// Either larger than body type and followed by smaller text, or body-sized,
// all bold, and followed by text that isn't. A long run of large type is a
// notice or a pull quote, not a heading.
type headingRun struct {
	start, end int
	size       float64
	bold       bool // body-sized bold, not large type
}

func headingRuns(lines []line, body float64) []headingRun {
	var runs []headingRun
	for i := 0; i < len(lines); {
		s := sizeKey(lines[i].size)
		big := s >= body*1.15
		bold := !big && lines[i].bold && math.Abs(s-body) <= 0.5
		if !lines[i].upright || !big && !bold {
			i++
			continue
		}
		j, n := i+1, utf8.RuneCountInString(lines[i].text)
		for j < len(lines) && !lines[j].brk && lines[j].upright && sizeKey(lines[j].size) == s && (big || lines[j].bold) {
			n += utf8.RuneCountInString(lines[j].text)
			j++
		}
		text := ""
		for _, l := range lines[i:j] {
			text += l.text
		}
		// A heading leads into smaller text. Labels inside figures are
		// followed by more labels, rotated axis text, or nothing.
		next := j < len(lines) && lines[j].upright
		var leads bool
		if big {
			leads = next && sizeKey(lines[j].size) < s && n <= 120
		} else {
			// Bold body text is a heading only when it's short and a full
			// line of plain text follows: bold names over affiliations are
			// followed by short lines. A trailing period or hyphen means a
			// sentence that carries on.
			t := strings.TrimSpace(text)
			// Wide gaps mean table cells, dot leaders a table of contents,
			// and a lowercase start on the next line a sentence that wraps.
			var nt []rune
			if next {
				nt = []rune(strings.TrimSpace(lines[j].text))
			}
			leads = next && !lines[j].bold && len(nt) >= 40 && !unicode.IsLower(nt[0]) &&
				n <= 80 && j-i <= 2 && !strings.HasSuffix(t, ".") && !strings.HasSuffix(t, "-") &&
				!strings.Contains(t, "..") && !strings.Contains(t, ". .") && !danglingWord(t)
			for _, l := range lines[i:j] {
				leads = leads && l.cells == nil
			}
		}
		if leads && strings.IndexFunc(text, unicode.IsLetter) >= 0 {
			runs = append(runs, headingRun{i, j, s, bold})
		}
		i = j
	}
	return runs
}

// danglingWord reports whether t ends on a word that can't end a title, so
// the heading must wrap onto a line that isn't all bold.
func danglingWord(t string) bool {
	switch t[strings.LastIndexByte(t, ' ')+1:] {
	case "a", "an", "and", "as", "at", "by", "for", "from", "in", "of", "on", "or", "the", "to", "under", "with":
		return true
	}
	return false
}

// headings is the document's body size and heading levels by size: the
// largest heading size is #, the next ##, the rest ###. Bold body-sized
// headings go one level below the smallest sized one, at most ####.
type headings struct {
	body   float64
	levels map[float64]int
	bold   int
}

func findHeadings(pages [][]line) headings {
	h := headings{body: bodySize(pages), levels: map[float64]int{}}
	var sizes []float64
	for _, p := range pages {
		for _, r := range headingRuns(p, h.body) {
			if _, ok := h.levels[r.size]; !ok && !r.bold {
				h.levels[r.size] = 0
				sizes = append(sizes, r.size)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sizes)))
	for i, s := range sizes {
		h.levels[s] = min(i+1, 3)
	}
	h.bold = min(len(sizes)+1, 4)
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
		for len(runs) > 0 && runs[0].start < i {
			runs = runs[1:]
		}
		if j := tableEnd(lines, i); j > i {
			flush()
			blocks = append(blocks, mdTable(lines[i:j]))
			i = j
			continue
		}
		if len(runs) > 0 && runs[0].start == i {
			r := runs[0]
			runs = runs[1:]
			flush()
			parts := make([]string, 0, r.end-r.start)
			for _, l := range lines[r.start:r.end] {
				parts = append(parts, strings.TrimSpace(l.text))
			}
			level := h.levels[r.size]
			if r.bold {
				level = h.bold
			}
			blocks = append(blocks, strings.Repeat("#", level)+" "+strings.Join(parts, " "))
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
// tableEnd returns the end of a table starting at lines[i], or i if none: at
// least two upright rows with the same number of cells, each cell starting
// within two ems of the one above.
func tableEnd(lines []line, i int) int {
	first := lines[i]
	if first.cells == nil || !first.upright {
		return i
	}
	j := i + 1
	for ; j < len(lines); j++ {
		l := lines[j]
		if !l.upright || len(l.cells) != len(first.cells) {
			break
		}
		aligned := true
		for k, x := range l.xs {
			aligned = aligned && math.Abs(x-lines[j-1].xs[k]) <= math.Max(l.size, 1)*2
		}
		if !aligned {
			break
		}
	}
	if j-i < 2 {
		return i
	}
	return j
}

// mdTable renders rows of cells as a Markdown table, the first row as header.
func mdTable(rows []line) string {
	esc := strings.NewReplacer("|", `\|`)
	var b strings.Builder
	for r, l := range rows {
		for _, c := range l.cells {
			b.WriteString("| " + esc.Replace(c) + " ")
		}
		b.WriteString("|\n")
		if r == 0 {
			b.WriteString(strings.Repeat("| --- ", len(l.cells)) + "|\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

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
