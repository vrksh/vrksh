package coax

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "coax",
		Short: "Retry wrapper - re-runs a command on failure with backoff",
		Flags: []shared.FlagMeta{
			{Name: "times", Usage: "number of retries (first attempt is free; total attempts = N+1)"},
			{Name: "backoff", Usage: "delay between retries: 100ms for fixed, exp:100ms for exponential"},
			{Name: "backoff-max", Usage: "cap for exponential backoff; 0 = uncapped"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress coax's own retry progress lines (subprocess stderr always passes through)"},
			{Name: "on", Usage: "retry only when exit code matches; repeatable: --on 1 --on 2 (default: any non-zero)"},
			{Name: "until", Usage: "shell command; retry until it exits 0"},
			{Name: "json", Shorthand: "j", Usage: "emit errors as JSON"},
		},
	})
}

// Run is the entry point for vrk coax.
func Run() int {
	fs := pflag.NewFlagSet("coax", pflag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress pflag's own error output; we handle it below
	times := fs.Int("times", 3, "number of retries (first attempt is free; total attempts = N+1)")
	backoffSpec := fs.String("backoff", "", "delay between retries: 100ms for fixed, exp:100ms for exponential")
	backoffMax := fs.Duration("backoff-max", 0, "cap for exponential backoff; 0 = uncapped")
	quiet := fs.BoolP("quiet", "q", false, "suppress coax's own retry progress lines (subprocess stderr always passes through)")
	var onCodes []int
	fs.IntSliceVar(&onCodes, "on", []int{}, "retry only when exit code matches; repeatable: --on 1 --on 2 (default: any non-zero)")
	untilCmd := fs.String("until", "", "shell command; retry until it exits 0")
	var jsonFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit errors as JSON")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printCoaxUsage(fs)
		}
		return shared.UsageErrorf("%s", err)
	}

	if *times < 1 {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "--times must be >= 1", "code": 2})
		}
		return shared.UsageErrorf("--times must be >= 1")
	}

	if *backoffMax < 0 {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "--backoff-max must be >= 0", "code": 2})
		}
		return shared.UsageErrorf("--backoff-max must be >= 0")
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "missing command: use -- to separate command from coax flags", "code": 2})
		}
		return shared.UsageErrorf("missing command: use -- to separate command from coax flags")
	}

	isExp, baseDelay, err := parseBackoff(*backoffSpec)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": fmt.Sprintf("--backoff: %v", err), "code": 2})
		}
		return shared.UsageErrorf("--backoff: %v", err)
	}

	// fail reports a runtime error as plain stderr or, with --json, on stdout.
	fail := func(msg string) int {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": msg, "code": 1})
		}
		fmt.Fprintln(os.Stderr, msg)
		return shared.ExitError
	}

	// Stdin is replayed lazily: nothing is read until a child asks for it, so an
	// open pipe that never closes cannot block a child that ignores stdin.
	// On a terminal there is nothing to replay; the child gets no input.
	var in *replayStdin
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		in = newReplayStdin(os.Stdin)
	}

	// Forward SIGINT and SIGTERM to the running child, and stop retrying once
	// coax itself has been told to stop.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	var signalled atomic.Bool

	maxAttempts := *times + 1
	var lastCode int

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 && in != nil && in.overflow {
			return fail("coax: stdin larger than 64 MiB cannot be replayed")
		}

		var startErr error
		lastCode, startErr = runSubcmd(cmdArgs, in, sigs, &signalled)
		if errors.Is(startErr, exec.ErrNotFound) {
			return fail("coax: command not found: " + cmdArgs[0])
		}
		if startErr != nil {
			return fail(fmt.Sprintf("coax: cannot run %s: %v", cmdArgs[0], startErr))
		}

		if lastCode == 0 || signalled.Load() {
			break
		}

		if !shouldRetry(lastCode, onCodes, *untilCmd) {
			if *untilCmd != "" {
				// --until condition passed despite command failing - treat as success
				return shared.ExitOK
			}
			break // --on exit code not in list; exit immediately
		}

		if attempt == maxAttempts-1 {
			if !*quiet {
				fmt.Fprintf(os.Stderr, "coax: all %d attempts failed (last exit %d)\n", maxAttempts, lastCode)
			}
			break // retries exhausted
		}

		retryNum := attempt + 1
		delay := computeDelay(isExp, baseDelay, *backoffMax, attempt)
		if !*quiet {
			if delay > 0 {
				fmt.Fprintf(os.Stderr, "coax: attempt %d failed (exit %d), retrying in %s (%d/%d)\n",
					attempt+1, lastCode, delay, retryNum, *times)
			} else {
				fmt.Fprintf(os.Stderr, "coax: attempt %d failed (exit %d), retrying (%d/%d)\n",
					attempt+1, lastCode, retryNum, *times)
			}
		}

		// Sleep in a select so a stop request ends the wait instead of being
		// buffered until after the next attempt has started.
		// A zero delay still polls, so a signal that landed during --until counts.
		timer := time.NewTimer(delay)
		select {
		case s := <-sigs:
			timer.Stop()
			signalled.Store(true)
			if sig, ok := s.(syscall.Signal); ok {
				return 128 + int(sig)
			}
			return lastCode
		case <-timer.C:
		}
	}

	return lastCode
}

// runSubcmd runs argv directly (no shell), feeding it replayed stdin, and
// returns the exit code. A child killed by a signal returns 128+signal.
// startErr is set when the child could not be started at all.
func runSubcmd(argv []string, in *replayStdin, sigs <-chan os.Signal, signalled *atomic.Bool) (code int, startErr error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	var pipe io.WriteCloser
	if in != nil {
		var err error
		if pipe, err = cmd.StdinPipe(); err != nil {
			return shared.ExitError, err
		}
	}
	if err := cmd.Start(); err != nil {
		return shared.ExitError, err
	}

	// The feeder is ours, not exec's, so Wait does not block on a stdin read
	// that may never return. Wait closes the pipe once the child exits, which
	// unblocks any pending write; done unblocks a pending read.
	done := make(chan struct{})
	fed := make(chan struct{})
	if in != nil {
		go func() {
			in.feed(pipe, done)
			close(fed)
		}()
	} else {
		close(fed)
	}

	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	var err error
loop:
	for {
		select {
		case s := <-sigs:
			signalled.Store(true)
			_ = cmd.Process.Signal(s)
		case err = <-waited:
			break loop
		}
	}
	close(done)
	<-fed

	if err == nil {
		return shared.ExitOK, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	return shared.ExitError, nil
}

// maxReplay caps how much stdin coax keeps for later attempts. A var so
// tests can lower it.
var maxReplay = 64 << 20

type chunk struct {
	b   []byte
	err error
}

// replayStdin hands stdin to one child at a time and remembers what it has
// seen, so a retry gets the same bytes. A single goroutine owns the reads
// and stays at most one chunk ahead of the children.
type replayStdin struct {
	chunks   chan chunk
	buf      []byte
	eof      bool
	overflow bool
}

func newReplayStdin(r io.Reader) *replayStdin {
	s := &replayStdin{chunks: make(chan chunk)}
	go func() {
		for {
			p := make([]byte, 32*1024)
			n, err := r.Read(p)
			if n > 0 {
				s.chunks <- chunk{b: p[:n]}
			}
			if err != nil {
				s.chunks <- chunk{err: err}
				return
			}
		}
	}()
	return s
}

// feed writes the replay buffer, then live stdin, to w until stdin ends, the
// child stops reading, or done closes. Only one feed runs at a time.
func (s *replayStdin) feed(w io.WriteCloser, done <-chan struct{}) {
	defer func() { _ = w.Close() }()
	if _, err := w.Write(s.buf); err != nil {
		return
	}
	for !s.eof {
		select {
		case <-done:
			return
		case c := <-s.chunks:
			if c.err != nil {
				s.eof = true
				return
			}
			if len(s.buf)+len(c.b) > maxReplay {
				s.overflow = true
			} else if !s.overflow {
				s.buf = append(s.buf, c.b...)
			}
			if _, err := w.Write(c.b); err != nil {
				return
			}
		}
	}
}

// shouldRetry returns true if the loop should attempt another run.
// When --until is set, it runs the condition command and returns true while
// the condition fails. When --on is set, it returns true only for matching
// exit codes. Default: retry on any non-zero exit code.
func shouldRetry(cmdCode int, onCodes []int, untilCmd string) bool {
	if untilCmd != "" {
		cmd := exec.Command("sh", "-c", untilCmd)
		// suppress condition command's own output - it is a side-effect check only
		return cmd.Run() != nil
	}
	if len(onCodes) > 0 {
		for _, c := range onCodes {
			if cmdCode == c {
				return true
			}
		}
		return false
	}
	return cmdCode != 0
}

// parseBackoff parses a backoff spec string.
// Returns isExp=true for "exp:<duration>", false for a plain duration.
// An empty spec returns (false, 0, nil) - no delay.
func parseBackoff(spec string) (isExp bool, base time.Duration, err error) {
	if spec == "" {
		return false, 0, nil
	}
	isExp = strings.HasPrefix(spec, "exp:")
	d, err := time.ParseDuration(strings.TrimPrefix(spec, "exp:"))
	if err != nil {
		return false, 0, fmt.Errorf("invalid duration %q: %v", spec, err)
	}
	if d < 0 {
		return false, 0, fmt.Errorf("duration must be >= 0, got %q", spec)
	}
	return isExp, d, nil
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing - Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("coax", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Int("times", 3, "number of retries (first attempt is free; total attempts = N+1)")
	fs.String("backoff", "", "delay between retries: 100ms for fixed, exp:100ms for exponential")
	fs.Duration("backoff-max", 0, "cap for exponential backoff; 0 = uncapped")
	fs.BoolP("quiet", "q", false, "suppress coax's own retry progress lines (subprocess stderr always passes through)")
	fs.IntSlice("on", []int{}, "retry only when exit code matches; repeatable: --on 1 --on 2 (default: any non-zero)")
	fs.String("until", "", "shell command; retry until it exits 0")
	fs.BoolP("json", "j", false, "emit errors as JSON")
	return fs
}

// printCoaxUsage writes usage to stdout and returns 0. Called when --help is passed.
func printCoaxUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk coax [flags] -- <command> [args...]",
		"       vrk coax --times 5 --backoff exp:100ms -- curl https://example.com",
		"",
		"Retry wrapper - re-runs a command on failure with optional backoff.",
		"",
		"The command runs directly, not through a shell. For pipes or redirects,",
		"use: vrk coax -- sh -c 'cat doc.txt | vrk prompt'",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("coax: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}

// computeDelay returns the delay before the retry that follows attempt index
// `attempt` (0-indexed). For exponential backoff: base * 2^attempt.
// For fixed backoff: always base. Capped at max when max > 0.
func computeDelay(isExp bool, base, max time.Duration, attempt int) time.Duration {
	if base == 0 {
		return 0
	}
	var d time.Duration
	if isExp {
		d = base
		for i := 0; i < attempt; i++ {
			if d > math.MaxInt64/2 {
				d = math.MaxInt64
				break
			}
			d *= 2
			if max > 0 && d >= max {
				break
			}
		}
	} else {
		d = base
	}
	if max > 0 && d > max {
		return max
	}
	return d
}
