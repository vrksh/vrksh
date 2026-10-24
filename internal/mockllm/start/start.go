// Package start builds and launches the mockllm server for tests and checkers.
package start

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Server is a running mockllm.
type Server struct {
	URL string // e.g. http://127.0.0.1:41234/v1
	Log string // path of the request log
	cmd *exec.Cmd
}

// Start builds ./internal/mockllm from repoRoot into dir and runs it on a free
// port, logging to dir/mock.jsonl. Extra args (e.g. "--reply", "x") are passed on.
func Start(repoRoot, dir string, extra ...string) (*Server, error) {
	bin := filepath.Join(dir, "mockllm")
	b := exec.Command("go", "build", "-o", bin, "./internal/mockllm")
	b.Dir = repoRoot
	if out, err := b.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("build mockllm: %v\n%s", err, out)
	}
	logPath := filepath.Join(dir, "mock.jsonl")
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, append([]string{"--port", "0", "--log", logPath}, extra...)...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	addr := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if a, ok := strings.CutPrefix(sc.Text(), "listening on "); ok {
				addr <- a
				break
			}
		}
		for sc.Scan() { // keep draining so the child never blocks
		}
	}()
	select {
	case a := <-addr:
		return &Server{URL: "http://" + a + "/v1", Log: logPath, cmd: cmd}, nil
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("mockllm did not start")
	}
}

// Stop kills the server.
func (s *Server) Stop() {
	_ = s.cmd.Process.Kill()
	_ = s.cmd.Wait()
}
