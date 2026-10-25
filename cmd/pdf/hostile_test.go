package pdf

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// hostile lists the adversarial fixtures from the 2026-10 security review.
// Each one used to crash, hang or eat gigabytes.
var hostile = []string{
	"objstm_cycle.pdf", "objstm_lenself.pdf", "objstm_hugeN.pdf",
	"xs_index_huge.pdf", "xref_start_1000000000.pdf", "xs_size_1099511627776.pdf",
}

// runHostile runs vrk pdf on a fixture with a 2s deadline and reports how
// many bytes were allocated meanwhile.
func runHostile(t *testing.T, name string, args ...string) (stdout, stderr string, code int, alloc uint64) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "pdf", "fixtures", name)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	done := make(chan struct{})
	go func() {
		stdout, stderr, code = runPdf(t, append(args, path), nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: hung", name)
	}
	runtime.ReadMemStats(&after)
	return stdout, stderr, code, after.TotalAlloc - before.TotalAlloc
}

// Every hostile fixture finishes fast, allocates little and fails cleanly:
// exit 0 or 1, the --json contract kept, no Go runtime text leaked.
func TestHostileFixtures(t *testing.T) {
	for _, name := range hostile {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code, alloc := runHostile(t, name, "--json")
			if code != 0 && code != 1 {
				t.Errorf("exit %d", code)
			}
			if stderr != "" {
				t.Errorf("stderr not empty with --json: %.200q", stderr)
			}
			for _, l := range strings.Split(strings.TrimSpace(stdout), "\n") {
				if !json.Valid([]byte(l)) {
					t.Errorf("not JSON: %.200q", l)
				}
			}
			if strings.Contains(stdout, "runtime error") {
				t.Errorf("runtime error leaked: %.200q", stdout)
			}
			if alloc > 64<<20 {
				t.Errorf("allocated %d MB", alloc>>20)
			}
			t.Logf("exit %d, %d KB allocated, %.120q", code, alloc>>10, stdout)
		})
	}
}

// An object stream that lives in itself overflowed the stack: exit 2 and a
// goroutine dump on stderr.
func TestObjStmCycle(t *testing.T) {
	for _, name := range []string{"objstm_cycle.pdf", "objstm_lenself.pdf"} {
		stdout, stderr, code, _ := runHostile(t, name, "--json")
		assertJSONError(t, stdout, stderr, code, 1, "pdf: ")
	}
}
