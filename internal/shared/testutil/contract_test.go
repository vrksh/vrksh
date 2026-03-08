package testutil

import (
	"fmt"
	"os"
	"testing"

	"github.com/vrksh/vrksh/internal/shared"
)

func dummyRun() int {
	_, _ = fmt.Fprint(os.Stdout, "hello\n")
	return 0
}

func dummyRunExit1() int {
	return shared.Errorf("test error")
}

func dummyRunExit2() int {
	return shared.UsageErrorf("missing argument")
}

func dummyRunWithStdin() int {
	input, err := shared.ReadInput(os.Args[1:])
	if err != nil {
		return shared.UsageErrorf("%v", err)
	}
	_, _ = fmt.Fprintln(os.Stdout, input)
	return 0
}

func TestRunContractTests(t *testing.T) {
	t.Run("exit 0 with expected stdout", func(t *testing.T) {
		RunContractTests(t, dummyRun, []ContractCase{
			{Name: "hello", WantOut: "hello\n", WantExit: 0},
		})
	})

	t.Run("exit 1 returned directly", func(t *testing.T) {
		RunContractTests(t, dummyRunExit1, []ContractCase{
			{Name: "error", WantErr: "test error", WantExit: 1},
		})
	})

	t.Run("exit 2 returned directly", func(t *testing.T) {
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
