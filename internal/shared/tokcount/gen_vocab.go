//go:build ignore

// gen_vocab converts each data/<name>.tiktoken (base64 token, rank per line)
// into data/<name>.bin: for each rank in order, one length byte followed by
// the raw token bytes. Run with `go generate ./internal/shared/tokcount`.
package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	for _, name := range []string{"cl100k_base", "o200k_base"} {
		if err := run(name); err != nil {
			fmt.Fprintln(os.Stderr, "gen_vocab:", name+":", err)
			os.Exit(1)
		}
	}
}

func run(name string) error {
	src, err := os.ReadFile("data/" + name + ".tiktoken")
	if err != nil {
		return err
	}
	var out bytes.Buffer
	sc := bufio.NewScanner(bytes.NewReader(src))
	for want := 0; sc.Scan(); {
		line := sc.Text()
		if line == "" {
			continue
		}
		tok, rank, ok := strings.Cut(line, " ")
		if !ok {
			return fmt.Errorf("bad line %q", line)
		}
		b, err := base64.StdEncoding.DecodeString(tok)
		if err != nil {
			return err
		}
		r, err := strconv.Atoi(rank)
		if err != nil {
			return err
		}
		if r != want {
			return fmt.Errorf("rank %d out of order, want %d", r, want)
		}
		if len(b) == 0 || len(b) > 255 {
			return fmt.Errorf("token %d has length %d, want 1-255", r, len(b))
		}
		out.WriteByte(byte(len(b)))
		out.Write(b)
		want++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return os.WriteFile("data/"+name+".bin", out.Bytes(), 0o644)
}
