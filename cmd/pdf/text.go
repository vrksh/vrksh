package pdf

import (
	"math"
	"strings"

	lpdf "github.com/ledongthuc/pdf"
)

// The library's own text extraction ignores glyph positions, so PDFs that
// space words with kerning instead of space characters (everything LaTeX
// makes) come out as one run-on word. This walks the content stream itself,
// tracks where each glyph lands, and puts spaces and line breaks back from
// the gaps between them.

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
	// ponytail: the standard 14 fonts ship without /Widths; half an em is a
	// guess. Embed their AFM widths if spacing on those PDFs looks off.
	ft.dw = 500
	if mw := v.Key("FontDescriptor").Key("MissingWidth"); mw.Float64() > 0 {
		ft.dw = mw.Float64()
	}
	return ft
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

// glyphs walks a page's content stream and returns its glyphs in drawing order.
// Malformed operators are skipped; the interpreter itself can panic on a broken
// stream, which extract recovers.
func glyphs(p lpdf.Page) []glyph {
	contents := p.V.Key("Contents")
	if contents.IsNull() {
		return nil
	}
	fonts := map[string]*font{}
	st := textState{ctm: identity, tm: identity, tlm: identity, th: 1}
	var stack []textState
	var out []glyph

	show := func(s string) {
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

			g := glyph{s: f.decode(raw), x: start[4], y: start[5], endX: end[4], endY: end[5], ux: 1}
			g.size = math.Hypot(start[2], start[3])
			if n := math.Hypot(start[0], start[1]); n > 0 {
				g.ux, g.uy = start[0]/n, start[1]/n
			}
			out = append(out, g)
		}
	}
	nextLine := func(tx, ty float64) {
		st.tlm = translate(tx, ty).mul(st.tlm)
		st.tm = st.tlm
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
		switch {
		case op == "q":
			stack = append(stack, st)
		case op == "Q" && len(stack) > 0:
			st = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
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
				if f := p.Font(name); !f.V.IsNull() {
					fonts[name] = loadFont(f)
				} else {
					fonts[name] = nil
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
		}
	})
	return out
}

// layout joins glyphs into text. A gap wider than a fraction of the font size
// becomes a space; a jump off the baseline becomes a newline, or a blank line
// when it is well over a line's height.
func layout(gs []glyph) string {
	var b strings.Builder
	var prev *glyph
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
				b.WriteByte('\n')
				if down > size*1.8 {
					b.WriteByte('\n')
				}
			case along > size*0.15 && !isSpace(prev.s) && !isSpace(g.s):
				b.WriteByte(' ')
			}
		}
		b.WriteString(g.s)
		prev = g
	}
	return b.String()
}

func isSpace(s string) bool { return strings.TrimSpace(s) == "" }
