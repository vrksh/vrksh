package testutil

import (
	"fmt"
	"os"
	"testing"

	"github.com/vrksh/vrksh/internal/shared"
)

func dummyRun() {
	_, _ = fmt.Fprint(os.Stdout, "hello\n")
}

func dummyRunExit1() {
	shared.Die("test error")
}

func dummyRunExit2() {
	shared.DieUsage("missing argument")
}

func dummyRunWithStdin() {
	input, err := shared.ReadInput(os.Args[1:])
	if err != nil {
		shared.DieUsage("%v", err)
	}
	_, _ = fmt.Fprintln(os.Stdout, input)
}

func TestRunContractTests(t *testing.T) {
	t.Run("exit 0 with expected stdout", func(t *testing.T) {
		RunContractTests(t, dummyRun, []ContractCase{
			{Name: "hello", WantOut: "hello\n", WantExit: 0},
		})
	})

	t.Run("exit 1 captured from Die", func(t *testing.T) {
		RunContractTests(t, dummyRunExit1, []ContractCase{
			{Name: "error", WantErr: "test error", WantExit: 1},
		})
	})

	t.Run("exit 2 captured from DieUsage", func(t *testing.T) {
		RunContractTests(t, dummyRunExit2, []ContractCase{
			{Name: "usage", WantErr: "missing argument", WantExit: 2},
		})
	})

	t.Run("stdin piped to tool", func(t *testing.T) {
		RunContractTests(t, dummyRunWithStdin, []ContractCase{
			{Name: "stdin", Stdin: "world\n", WantOut: "world\n", WantExit: 0},
		})
	})

	t.Run("args passed to tool", func(t *testing.T) {
		RunContractTests(t, dummyRunWithStdin, []ContractCase{
			{Name: "arg", Args: []string{"argval"}, WantOut: "argval\n", WantExit: 0},
		})
	})

	t.Run("empty stdin causes exit 2", func(t *testing.T) {
		RunContractTests(t, dummyRunWithStdin, []ContractCase{
			{Name: "no input", Stdin: "", WantExit: 2},
		})
	})
}
