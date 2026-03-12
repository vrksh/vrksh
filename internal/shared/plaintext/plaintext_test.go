package plaintext

import (
	"testing"
	"testing/quick"
)

func TestStripMarkdown(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"bold_italic", "**hello** _world_", "hello world"},
		{"heading", "# Heading", "Heading"},
		{"link", "[link text](https://example.com)", "link text"},
		{"inline_code", "`code snippet`", "code snippet"},
		{"blockquote", "> blockquote text", "blockquote text"},
		{"unordered_list", "- item one\n- item two", "item one\nitem two"},
		{"ordered_list", "1. first\n2. second", "first\nsecond"},
		{"nested_emphasis", "normal **bold _nested_ emphasis** text", "normal bold nested emphasis text"},
		{"reference_link", "[text][ref]\n\n[ref]: https://example.com", "text"},
		{"fenced_code", "```\nfenced code\n```", "fenced code"},
		{"empty", "", ""},
		{"no_markdown", "no markdown here", "no markdown here"},
		// GFM table: pipes and dashes stripped, cells space-separated per row.
		{"table", "| col1 | col2 |\n|------|------|\n| val1 | val2 |", "col1 col2\nval1 val2"},
		// Space normalisation: double spaces produced by trailing-space soft breaks collapse.
		{"double_space_cleanup", "word  with  extra  spaces", "word with extra spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StripMarkdown(tc.input)
			if got != tc.want {
				t.Errorf("StripMarkdown(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestStripMarkdownNoPanic verifies that StripMarkdown never panics on arbitrary input.
func TestStripMarkdownNoPanic(t *testing.T) {
	f := func(s string) bool {
		_ = StripMarkdown(s)
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}

// TestStripMarkdownIdempotent verifies that applying StripMarkdown twice
// produces the same result as applying it once.
func TestStripMarkdownIdempotent(t *testing.T) {
	f := func(s string) bool {
		once := StripMarkdown(s)
		twice := StripMarkdown(once)
		return once == twice
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}
