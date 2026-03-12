// Package plaintext provides markdown-to-plain-text conversion using a
// goldmark AST walk. Formatting syntax is stripped; content is preserved.
package plaintext

import (
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// md is the goldmark instance with GFM table support enabled.
// Created once at package init so the parser is not rebuilt per call.
var md = goldmark.New(goldmark.WithExtensions(extension.Table))

// StripMarkdown parses s as CommonMark markdown (with GFM table extension)
// and returns plain prose with all syntax removed. Content is preserved —
// only formatting is stripped. Link text is kept; URLs are dropped. Code
// content is kept; fences and backticks are dropped. Table cells are
// space-separated; rows are newline-separated.
func StripMarkdown(s string) string {
	if s == "" {
		return ""
	}
	src := []byte(s)
	reader := text.NewReader(src)
	doc := md.Parser().Parse(reader)

	var sb strings.Builder
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		switch node := n.(type) {
		case *gast.Text:
			if entering {
				sb.Write(node.Value(src))
				// Soft line breaks (a single newline within a paragraph) are
				// rendered as spaces in HTML — treat them the same way here.
				if node.SoftLineBreak() {
					sb.WriteByte(' ')
				} else if node.HardLineBreak() {
					sb.WriteByte('\n')
				}
			}

		case *gast.String:
			if entering {
				sb.Write(node.Value)
			}

		case *gast.FencedCodeBlock:
			if entering {
				for i := 0; i < node.Lines().Len(); i++ {
					seg := node.Lines().At(i)
					sb.Write(seg.Value(src))
				}
				return gast.WalkSkipChildren, nil
			}

		case *gast.CodeBlock:
			if entering {
				for i := 0; i < node.Lines().Len(); i++ {
					seg := node.Lines().At(i)
					sb.Write(seg.Value(src))
				}
				return gast.WalkSkipChildren, nil
			}

		case *gast.HTMLBlock:
			// Drop block-level raw HTML — no text to extract.
			if entering {
				return gast.WalkSkipChildren, nil
			}

		case *gast.RawHTML:
			// Drop inline HTML tags.
			if entering {
				return gast.WalkSkipChildren, nil
			}

		case *gast.AutoLink:
			// Auto-links (<https://example.com>) — preserve the label as text.
			if entering {
				sb.Write(node.Label(src))
				return gast.WalkSkipChildren, nil
			}

		case *gast.Heading:
			if !entering {
				sb.WriteString("\n\n")
			}

		case *gast.Paragraph:
			if !entering {
				sb.WriteString("\n\n")
			}

		case *gast.List:
			if !entering {
				sb.WriteString("\n\n")
			}

		case *gast.ListItem:
			if entering {
				sb.WriteByte('\n')
			}

		// GFM table: emit cells space-separated, rows newline-separated.
		case *extast.TableCell:
			// Write a space before every cell that is not the first in its row.
			if entering && node.PreviousSibling() != nil {
				sb.WriteByte(' ')
			}

		case *extast.TableHeader:
			if !entering {
				sb.WriteByte('\n')
			}

		case *extast.TableRow:
			if !entering {
				sb.WriteByte('\n')
			}

		case *extast.Table:
			if !entering {
				sb.WriteByte('\n')
			}

			// Link, Image, Emphasis, CodeSpan, Blockquote, TextBlock, Document:
			// recurse into children — the Text leaf nodes carry the content.
		}
		return gast.WalkContinue, nil
	})

	return cleanup(sb.String())
}

// cleanup normalises whitespace in the collected plain text:
//   - Collapses runs of 3+ newlines to 2 (preserves paragraph breaks).
//   - Collapses multiple consecutive spaces on each line to one (removes
//     double-spaces that can arise from soft line breaks with trailing spaces).
//   - Trims leading and trailing whitespace from the result.
func cleanup(s string) string {
	// Collapse 3+ consecutive newlines to 2.
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	// Collapse multiple spaces within each line to one.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		for strings.Contains(line, "  ") {
			line = strings.ReplaceAll(line, "  ", " ")
		}
		lines[i] = line
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
