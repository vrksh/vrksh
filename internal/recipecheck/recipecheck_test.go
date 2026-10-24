//go:build recipes

// Package recipecheck runs every recipe page's front-matter steps against
// the built vrk binary and a mock LLM, then runs the recipe's expect.sh.
// The contract is in testdata/recipes/README.md. Run: make recipecheck
package recipecheck

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vrksh/vrksh/internal/mockllm/start"
	"gopkg.in/yaml.v3"
)

const root = "../.."

var bareTool = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// stepScripts returns the front-matter steps that are scripts (not bare tool names).
func stepScripts(page string) ([]string, error) {
	data, err := os.ReadFile(page)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(data), "\n---", 3)
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "---") {
		return nil, fmt.Errorf("%s: no front matter", page)
	}
	var fm struct {
		Steps []string `yaml:"steps"`
	}
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(parts[0], "---")), &fm); err != nil {
		return nil, fmt.Errorf("%s: %v", page, err)
	}
	var out []string
	for _, s := range fm.Steps {
		if !bareTool.MatchString(strings.TrimSpace(s)) {
			out = append(out, s)
		}
	}
	return out, nil
}

func TestRecipes(t *testing.T) {
	absRoot, _ := filepath.Abs(root)
	vrk := filepath.Join(absRoot, "vrk")
	if _, err := os.Stat(vrk); err != nil {
		t.Fatal("./vrk not found, run make build")
	}
	pages, _ := filepath.Glob(filepath.Join(absRoot, "hugo/content/recipes/*.md"))
	shared := t.TempDir()
	mock, err := start.Start(absRoot, shared)
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Stop()
	bin := filepath.Join(shared, "bin")
	_ = os.MkdirAll(bin, 0o755)
	_ = os.Symlink(vrk, filepath.Join(bin, "vrk"))

	for _, page := range pages {
		slug := strings.TrimSuffix(filepath.Base(page), ".md")
		if slug == "_index" {
			continue
		}
		t.Run(slug, func(t *testing.T) {
			src := filepath.Join(absRoot, "testdata/recipes", slug)
			if _, err := os.Stat(filepath.Join(src, "expect.sh")); err != nil {
				t.Fatalf("missing testdata/recipes/%s/expect.sh: every recipe needs a test dir", slug)
			}
			steps, err := stepScripts(page)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if out, err := exec.Command("cp", "-R", src+"/.", dir).CombinedOutput(); err != nil {
				t.Fatalf("copy fixtures: %v\n%s", err, out)
			}
			// truncate the shared mock log so MOCK_LOG only shows this recipe
			if err := os.Truncate(mock.Log, 0); err != nil {
				t.Fatal(err)
			}
			site := httptest.NewServer(http.FileServer(http.Dir(dir)))
			defer site.Close()

			outFile := filepath.Join(dir, "steps.out")
			stepsFile := filepath.Join(dir, "steps.sh")
			script := "set -a\n[ -f ./env ] && . ./env\nset +a\n" + strings.Join(steps, "\n") + "\n"
			if err := os.WriteFile(stepsFile, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			env := []string{
				"PATH=" + bin + ":" + os.Getenv("PATH"),
				"HOME=" + dir,
				"VRK_KV_PATH=" + filepath.Join(dir, "kv.db"),
				"VRK_LLM_URL=" + mock.URL,
				"VRK_LLM_KEY=dummy-key",
				"OPENAI_API_KEY=dummy-key",
				"VRK_DEFAULT_MODEL=mock",
				"MOCK_LOG=" + mock.Log,
				"OUT=" + outFile,
				"STEPS=" + stepsFile,
				"SITE_URL=" + site.URL,
				"LC_ALL=C.UTF-8",
			}
			run := func(name string, args ...string) (int, string) {
				cmd := exec.Command(name, args...)
				cmd.Dir = dir
				cmd.Env = env
				cmd.WaitDelay = time.Second
				out, err := cmd.CombinedOutput()
				if ee, ok := err.(*exec.ExitError); ok {
					return ee.ExitCode(), string(out)
				}
				if err != nil {
					return -1, err.Error()
				}
				return 0, string(out)
			}
			// optional setup.sh prepares state the fixtures can't hold (git repos, servers)
			if _, err := os.Stat(filepath.Join(dir, "setup.sh")); err == nil {
				if rc, out := run("bash", "setup.sh"); rc != 0 {
					t.Fatalf("setup.sh exit %d\n%s", rc, out)
				}
			}
			// steps: stdout+stderr go to $OUT; stdin is closed.
			stepsRC := 0
			cmd := exec.Command("bash", "-c", `bash "$STEPS" >"$OUT" 2>&1 </dev/null`)
			cmd.Dir = dir
			cmd.Env = env
			cmd.WaitDelay = time.Second
			if err := cmd.Run(); err != nil {
				stepsRC = 1
				if ee, ok := err.(*exec.ExitError); ok {
					stepsRC = ee.ExitCode()
				}
			}
			env = append(env, fmt.Sprintf("STEPS_RC=%d", stepsRC))
			rc, out := run("bash", "expect.sh")
			if rc != 0 {
				steps, _ := os.ReadFile(outFile)
				t.Errorf("expect.sh exit %d (steps exit %d)\n--- expect.sh output ---\n%s--- steps output ---\n%s", rc, stepsRC, out, steps)
			}
		})
	}
}
