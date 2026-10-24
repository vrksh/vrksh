// Command doccheck runs the `$ command` examples found in the docs and recipe
// pages and compares their output with what the page shows.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// example is one `$ cmd` line (with continuations) and the output under it.
type example struct {
	file     string
	line     int
	cmd      string
	expected string
}

// block is one fenced bash/sh block with at least one example.
type block struct {
	file     string
	line     int
	skipped  bool
	examples []example
}

var skipRe = regexp.MustCompile(`^<!--\s*doccheck:\s*skip\s*\((.+)\)\s*-->$`)

// parseBlocks extracts fenced bash/sh blocks from markdown text.
func parseBlocks(file, text string) []block {
	lines := strings.Split(text, "\n")
	var out []block
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t != "```bash" && t != "```sh" {
			continue
		}
		start := i + 1
		end := start
		for end < len(lines) && strings.TrimSpace(lines[end]) != "```" {
			end++
		}
		b := block{file: file, line: start + 1}
		// skip marker: nearest non-blank line before the fence
		for j := i - 1; j >= 0; j-- {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			b.skipped = strings.Contains(lines[j], "doccheck: skip")
			break
		}
		b.examples = parseExamples(file, start+1, lines[start:end])
		if len(b.examples) > 0 {
			out = append(out, b)
		}
		i = end
	}
	return out
}

func parseExamples(file string, firstLine int, body []string) []example {
	var exs []example
	for i := 0; i < len(body); i++ {
		if !strings.HasPrefix(body[i], "$ ") {
			continue
		}
		ex := example{file: file, line: firstLine + i}
		cmd := []string{body[i][2:]}
		for strings.HasSuffix(cmd[len(cmd)-1], `\`) && i+1 < len(body) {
			i++
			cmd = append(cmd, body[i])
		}
		ex.cmd = strings.Join(cmd, "\n")
		var exp []string
		for i+1 < len(body) && !strings.HasPrefix(body[i+1], "$ ") {
			i++
			if strings.HasPrefix(body[i], "#") {
				continue
			}
			exp = append(exp, body[i])
		}
		ex.expected = strings.Join(exp, "\n")
		exs = append(exs, ex)
	}
	return exs
}

func trimLines(s string) string {
	ls := strings.Split(s, "\n")
	for i := range ls {
		ls[i] = strings.TrimRight(ls[i], " \t\r")
	}
	return strings.TrimRight(strings.Join(ls, "\n"), "\n")
}

// matches compares expected to actual. A line that is only "..." matches any
// number of lines (including none); "..." inside a line matches any text.
func matches(expected, actual string) bool {
	exp, act := trimLines(expected), trimLines(actual)
	if act != "" {
		act += "\n"
	}
	var re strings.Builder
	re.WriteString(`^`)
	if exp != "" {
		for _, l := range strings.Split(exp, "\n") {
			if l == "..." {
				re.WriteString(`(?:[^\n]*\n)*`)
				continue
			}
			parts := strings.Split(l, "...")
			for i := range parts {
				parts[i] = regexp.QuoteMeta(parts[i])
			}
			re.WriteString(strings.Join(parts, `[^\n]*`) + `\n`)
		}
	}
	re.WriteString(`$`)
	ok, err := regexp.MatchString(re.String(), act)
	return err == nil && ok
}

type failure struct {
	ex       example
	expected string
	actual   string
}

type env struct {
	dir     string   // work dir for the file (fixtures copied in)
	environ []string // environment for commands
	timeout time.Duration
}

// runBlock runs all examples of a block in one bash process and returns failures.
func (e *env) runBlock(b block) []failure {
	var script strings.Builder
	out := filepath.Join(e.dir, ".doccheck")
	_ = os.MkdirAll(out, 0o755)
	for i, ex := range b.examples {
		if isExitEcho(ex.cmd) {
			continue
		}
		o := filepath.Join(out, fmt.Sprintf("out.%d", i))
		fmt.Fprintf(&script, "{\n%s\n} >%q 2>&1 </dev/null\necho $? >%q\n", ex.cmd, o, o+".rc")
	}
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", script.String())
	cmd.Dir = e.dir
	cmd.Env = e.environ
	cmd.WaitDelay = time.Second
	_ = cmd.Run()

	var fails []failure
	prevRC := ""
	for i, ex := range b.examples {
		o := filepath.Join(out, fmt.Sprintf("out.%d", i))
		var actual string
		if isExitEcho(ex.cmd) {
			actual = prevRC
		} else {
			data, err := os.ReadFile(o)
			actual = string(data)
			if err != nil {
				actual = "(did not run: timeout or earlier shell failure)"
			}
			rc, _ := os.ReadFile(o + ".rc")
			prevRC = strings.TrimSpace(string(rc))
		}
		if !matches(ex.expected, actual) {
			fails = append(fails, failure{ex, ex.expected, actual})
		}
	}
	return fails
}

func isExitEcho(cmd string) bool { return strings.TrimSpace(cmd) == "echo $?" }

type result struct {
	passed, skipped int
	fails           []failure
}

// checkFile runs every non-skipped block of one markdown file.
func checkFile(path string, mkEnv func() (*env, func())) (result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return result{}, err
	}
	var res result
	e, cleanup := mkEnv()
	defer cleanup()
	for _, b := range parseBlocks(path, string(data)) {
		if b.skipped {
			res.skipped += len(b.examples)
			continue
		}
		f := e.runBlock(b)
		res.fails = append(res.fails, f...)
		res.passed += len(b.examples) - len(f)
	}
	return res, nil
}

func report(f failure) string {
	return fmt.Sprintf("FAIL %s:%d\n  $ %s\n  expected: %s\n  actual:   %s\n",
		f.ex.file, f.ex.line, strings.ReplaceAll(f.ex.cmd, "\n", "\n    "),
		strconv.Quote(trimLines(f.expected)), strconv.Quote(trimLines(f.actual)))
}
