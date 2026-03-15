// Package digest implements vrk digest — a universal hasher.
// Hashes stdin or files. SHA-256 by default. Outputs algo:hash.
package digest

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // MD5 is available as a user-requested algorithm, not used for security
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// copyToHash is a var so tests can inject I/O errors on the stdin streaming path.
var copyToHash = func(h hash.Hash, r io.Reader) (int64, error) { return io.Copy(h, r) }

// Run is the entry point for vrk digest. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("digest", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		algo        string
		bareFlag    bool
		jsonFlag    bool
		quietFlag   bool
		files       []string
		compareFlag bool
		hmacFlag    bool
		keyFlag     string
		verifyFlag  string
	)

	fs.StringVarP(&algo, "algo", "a", "sha256", "hash algorithm: sha256, md5, sha512")
	fs.BoolVarP(&bareFlag, "bare", "b", false, "output hash only, no algo: prefix")
	fs.StringArrayVar(&files, "file", nil, "file to hash (repeatable)")
	fs.BoolVar(&compareFlag, "compare", false, "compare hashes of all --file inputs; exits 0 either way")
	fs.BoolVar(&hmacFlag, "hmac", false, "compute HMAC instead of plain hash")
	fs.StringVarP(&keyFlag, "key", "k", "", "HMAC secret key (required with --hmac)")
	fs.StringVar(&verifyFlag, "verify", "", "known HMAC hex; exits 0 if match, 1 if mismatch")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON object")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// --bare and --json are mutually exclusive. Since --json is set, use JSON error path.
	if bareFlag && jsonFlag {
		return shared.PrintJSONError(map[string]any{
			"error": "digest: --bare and --json are mutually exclusive",
			"code":  2,
		})
	}

	// Validate algorithm before any I/O.
	newHash, algoErr := hashFuncFor(algo)
	if algoErr != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("digest: %s", algoErr),
				"code":  2,
			})
		}
		return shared.UsageErrorf("digest: %s", algoErr)
	}

	// --hmac requires --key.
	if hmacFlag && keyFlag == "" {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "digest: --hmac requires --key",
				"code":  2,
			})
		}
		return shared.UsageErrorf("digest: --hmac requires --key")
	}

	// --verify requires --hmac.
	if verifyFlag != "" && !hmacFlag {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "digest: --verify requires --hmac",
				"code":  2,
			})
		}
		return shared.UsageErrorf("digest: --verify requires --hmac")
	}

	// --compare requires at least two --file values.
	if compareFlag && len(files) < 2 {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "digest: --compare requires at least 2 --file values",
				"code":  2,
			})
		}
		return shared.UsageErrorf("digest: --compare requires at least 2 --file values")
	}

	// Route: file mode takes priority over stdin/positional.
	if len(files) > 0 {
		return handleFiles(files, algo, newHash, keyFlag, hmacFlag, compareFlag, bareFlag, jsonFlag)
	}

	// Stdin / positional arg mode — initialise the hash before branching so the
	// stdin path can stream directly into it without buffering the entire input.
	var h hash.Hash
	if hmacFlag {
		h = hmac.New(newHash, []byte(keyFlag))
	} else {
		h = newHash()
	}

	var inputBytes int64

	if args := fs.Args(); len(args) > 0 {
		// Positional arg: hash the string as-is, no newline added.
		s := strings.Join(args, " ")
		_, _ = h.Write([]byte(s))
		inputBytes = int64(len(s))
	} else {
		// TTY guard: interactive terminal with no piped input and no --file → usage error.
		if isTerminal(int(os.Stdin.Fd())) {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": "digest: no input: pipe text to stdin or use --file",
					"code":  2,
				})
			}
			return shared.UsageErrorf("digest: no input: pipe text to stdin or use --file")
		}

		// Stream stdin directly into the hash — no intermediate buffer, no OOM risk.
		// Bytes are hashed verbatim, including any trailing newline from the pipe.
		n, err := copyToHash(h, os.Stdin)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("digest: reading stdin: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("digest: reading stdin: %v", err)
		}
		inputBytes = n
	}

	computed := h.Sum(nil)
	hexHash := hex.EncodeToString(computed)

	// --verify: constant-time comparison via hmac.Equal.
	if verifyFlag != "" {
		expected, err := hex.DecodeString(verifyFlag)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("digest: invalid --verify hex: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("digest: invalid --verify hex: %v", err)
		}
		if !hmac.Equal(computed, expected) {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": "digest: hmac mismatch",
					"code":  1,
				})
			}
			return shared.Errorf("digest: hmac mismatch")
		}
		// Match: exit 0, no output.
		return 0
	}

	// Format and write output.
	if jsonFlag {
		field := "hash"
		if hmacFlag {
			field = "hmac"
		}
		return writeJSONResult(map[string]any{
			"input_bytes": inputBytes,
			"algo":        algo,
			field:         hexHash,
		})
	}

	if bareFlag {
		if _, err := fmt.Fprintln(os.Stdout, hexHash); err != nil {
			return shared.Errorf("digest: writing output: %v", err)
		}
		return 0
	}

	if _, err := fmt.Fprintf(os.Stdout, "%s:%s\n", algo, hexHash); err != nil {
		return shared.Errorf("digest: writing output: %v", err)
	}
	return 0
}

// handleFiles hashes one or more files, with optional --compare and --hmac support.
func handleFiles(files []string, algo string, newHash func() hash.Hash, key string, useHMAC, compare, bare, useJSON bool) int {
	if compare {
		hashes := make([]string, len(files))
		for i, f := range files {
			h, err := hashFile(f, newHash, useHMAC, key)
			if err != nil {
				if useJSON {
					return shared.PrintJSONError(map[string]any{
						"error": fmt.Sprintf("digest: %s: %v", f, err),
						"code":  1,
					})
				}
				return shared.Errorf("digest: %s: %v", f, err)
			}
			hashes[i] = h
		}

		match := true
		for i := 1; i < len(hashes); i++ {
			if hashes[i] != hashes[0] {
				match = false
				break
			}
		}

		if useJSON {
			return writeJSONResult(map[string]any{
				"files":  files,
				"algo":   algo,
				"hashes": hashes,
				"match":  match,
			})
		}
		var matchStr string
		if match {
			matchStr = "match: true"
		} else {
			matchStr = "match: false"
		}
		if _, err := fmt.Fprintln(os.Stdout, matchStr); err != nil {
			return shared.Errorf("digest: writing output: %v", err)
		}
		return 0
	}

	// Hash each file individually.
	for _, f := range files {
		hexHash, err := hashFile(f, newHash, useHMAC, key)
		if err != nil {
			if useJSON {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("digest: %s: %v", f, err),
					"code":  1,
				})
			}
			return shared.Errorf("digest: %s: %v", f, err)
		}

		if useJSON {
			if code := writeJSONResult(map[string]any{
				"file": f,
				"algo": algo,
				"hash": hexHash,
			}); code != 0 {
				return code
			}
			continue
		}
		if bare {
			if _, err := fmt.Fprintln(os.Stdout, hexHash); err != nil {
				return shared.Errorf("digest: writing output: %v", err)
			}
			continue
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s:%s\n", algo, hexHash); err != nil {
			return shared.Errorf("digest: writing output: %v", err)
		}
	}
	return 0
}

// hashFile streams a file through the hash (or HMAC) and returns the hex digest.
func hashFile(path string, newHash func() hash.Hash, useHMAC bool, key string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	var h hash.Hash
	if useHMAC {
		h = hmac.New(newHash, []byte(key))
	} else {
		h = newHash()
	}
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFuncFor returns the hash constructor for the given algorithm name.
func hashFuncFor(algo string) (func() hash.Hash, error) {
	switch algo {
	case "sha256":
		return sha256.New, nil
	case "md5":
		return md5.New, nil //nolint:gosec
	case "sha512":
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("unknown algorithm %q: want sha256, md5, sha512", algo)
	}
}

// writeJSONResult encodes v as JSON to stdout and returns 0 on success.
func writeJSONResult(v map[string]any) int {
	if err := shared.PrintJSON(v); err != nil {
		return shared.Errorf("digest: writing output: %v", err)
	}
	return 0
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk digest [flags]",
		"       echo 'data' | vrk digest",
		"       vrk digest 'data'",
		"       vrk digest --file /path/to/file",
		"",
		"Universal hasher — SHA-256 by default. Outputs algo:hash.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("digest: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
