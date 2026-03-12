package coax

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// Run is the entry point for vrk coax.
func Run() int {
	fs := pflag.NewFlagSet("coax", pflag.ContinueOnError)
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
		if err == pflag.ErrHelp {
			return shared.ExitOK
		}
		return shared.UsageErrorf("%s", err)
	}

	if *times < 1 {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "--times must be >= 1", "code": 2})
		}
		return shared.UsageErrorf("--times must be >= 1")
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

	// Buffer stdin only when it's a pipe or regular file — not a TTY.
	// On a terminal, io.ReadAll blocks forever waiting for EOF.
	var stdinData []byte
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		stdinData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return shared.Errorf("reading stdin: %v", err)
		}
	}

	cmdStr := strings.Join(cmdArgs, " ")
	maxAttempts := *times + 1
	var lastCode int

	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastCode = runSubcmd(cmdStr, stdinData)

		if lastCode == 0 {
			break
		}

		if !shouldRetry(lastCode, onCodes, *untilCmd) {
			if *untilCmd != "" {
				// --until condition passed despite command failing — treat as success
				return shared.ExitOK
			}
			break // --on exit code not in list; exit immediately
		}

		if attempt == maxAttempts-1 {
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

		if delay > 0 {
			time.Sleep(delay)
		}
	}

	return lastCode
}

// runSubcmd runs a shell command string, re-supplying buffered stdin, and
// returns the exit code. stdout and stderr pass through to the process.
func runSubcmd(cmdStr string, stdinData []byte) int {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = bytes.NewReader(stdinData)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return shared.ExitError
	}
	return shared.ExitOK
}

// shouldRetry returns true if the loop should attempt another run.
// When --until is set, it runs the condition command and returns true while
// the condition fails. When --on is set, it returns true only for matching
// exit codes. Default: retry on any non-zero exit code.
func shouldRetry(cmdCode int, onCodes []int, untilCmd string) bool {
	if untilCmd != "" {
		cmd := exec.Command("sh", "-c", untilCmd)
		// suppress condition command's own output — it is a side-effect check only
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
// An empty spec returns (false, 0, nil) — no delay.
func parseBackoff(spec string) (isExp bool, base time.Duration, err error) {
	if spec == "" {
		return false, 0, nil
	}
	if strings.HasPrefix(spec, "exp:") {
		d, err := time.ParseDuration(strings.TrimPrefix(spec, "exp:"))
		if err != nil {
			return false, 0, fmt.Errorf("invalid duration in %q: %v", spec, err)
		}
		return true, d, nil
	}
	d, err := time.ParseDuration(spec)
	if err != nil {
		return false, 0, fmt.Errorf("invalid duration %q: %v", spec, err)
	}
	return false, d, nil
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
			d *= 2
		}
	} else {
		d = base
	}
	if max > 0 && d > max {
		return max
	}
	return d
}
