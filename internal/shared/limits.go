package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadAllLimited reads r to EOF and fails once it passes max bytes, so hostile
// input (a /dev/zero pipe, a multi-GB upload) can't grow memory without bound.
func ReadAllLimited(r io.Reader, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("input exceeds %s limit", byteSize(max))
	}
	return b, nil
}

// ReadFileLimited reads path with the ReadAllLimited cap. Only regular files are
// accepted, plus named pipes when allowPipe is set (for <(...) process
// substitution). Devices like /dev/zero and directories are refused before
// opening, so a FIFO with no writer can't block us either when pipes are off.
func ReadFileLimited(path string, max int64, allowPipe bool) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	m := fi.Mode()
	if !m.IsRegular() && (!allowPipe || m&os.ModeNamedPipe == 0) {
		return nil, fmt.Errorf("%s: not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := ReadAllLimited(f, max)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return b, nil
}

func byteSize(n int64) string {
	switch {
	case n >= 1<<30 && n%(1<<30) == 0:
		return fmt.Sprintf("%d GiB", n>>30)
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MiB", n>>20)
	}
	return fmt.Sprintf("%d byte", n)
}

// StripControls drops terminal control characters from text extracted out of a
// foreign format (PDF glyphs, web pages, echoed error bodies): C0 except \t and
// \n, DEL, C1, and the bidi overrides. Don't use it on byte-exact data tools.
func StripControls(s string) string {
	return strings.Map(func(r rune) rune {
		if isControl(r) {
			return -1
		}
		return r
	}, s)
}

func isControl(r rune) bool {
	switch {
	case r == '\t' || r == '\n':
		return false
	case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
		return true
	case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

// DecodeOne decodes exactly one JSON value from s (numbers kept as json.Number)
// and rejects anything but whitespace after it. A gate that checks the first
// value and forwards the whole line would pass whatever is appended.
func DecodeOne(s string, v any) error {
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	if err := d.Decode(v); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after JSON value")
	}
	return nil
}
