package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/vrksh/vrksh/internal/mockllm/start"
)

// usage: go run ./internal/doccheck [file.md ...]   (run from the repo root;
// needs ./vrk built). With no files it checks hugo/content/{docs,recipes}/*.md.
func main() {
	root, _ := os.Getwd()
	files := os.Args[1:]
	if len(files) == 0 {
		for _, g := range []string{"hugo/content/docs/*.md", "hugo/content/recipes/*.md"} {
			m, _ := filepath.Glob(g)
			files = append(files, m...)
		}
		sort.Strings(files)
	}
	vrk := filepath.Join(root, "vrk")
	if _, err := os.Stat(vrk); err != nil {
		fmt.Fprintln(os.Stderr, "doccheck: ./vrk not found, run make build")
		os.Exit(2)
	}
	tmp, err := os.MkdirTemp("", "doccheck")
	if err != nil {
		fmt.Fprintln(os.Stderr, "doccheck:", err)
		os.Exit(2)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	mock, err := start.Start(root, tmp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "doccheck:", err)
		os.Exit(2)
	}
	defer mock.Stop()
	bin := filepath.Join(tmp, "bin")
	_ = os.MkdirAll(bin, 0o755)
	_ = os.Symlink(vrk, filepath.Join(bin, "vrk"))

	n := 0
	mk := func() (*env, func()) {
		n++
		dir := filepath.Join(tmp, fmt.Sprintf("w%d", n))
		_ = os.MkdirAll(dir, 0o755)
		_ = exec.Command("cp", "-R", filepath.Join(root, "testdata/doccheck/fixtures")+"/.", dir).Run()
		return &env{dir: dir, timeout: 60 * time.Second, environ: []string{
			"PATH=" + bin + ":" + os.Getenv("PATH"),
			"HOME=" + dir,
			"VRK_KV_PATH=" + filepath.Join(dir, "kv.db"),
			"VRK_LLM_URL=" + mock.URL,
			"VRK_LLM_KEY=dummy-key",
			"VRK_DEFAULT_MODEL=mock",
			"OPENAI_API_KEY=dummy-key",
			"LC_ALL=C.UTF-8",
		}}, func() {}
	}

	total, failed := 0, 0
	for _, f := range files {
		res, err := checkFile(f, mk)
		if err != nil {
			fmt.Fprintln(os.Stderr, "doccheck:", err)
			os.Exit(2)
		}
		for _, fl := range res.fails {
			fmt.Print(report(fl))
		}
		rel := f
		if len(res.fails) > 0 || res.skipped > 0 {
			fmt.Printf("%s: %d failed, %d passed, %d skipped\n", rel, len(res.fails), res.passed, res.skipped)
		}
		total += res.passed + len(res.fails)
		failed += len(res.fails)
	}
	fmt.Printf("doccheck: %d examples, %d failed\n", total, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
