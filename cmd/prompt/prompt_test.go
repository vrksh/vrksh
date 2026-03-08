package prompt

import "testing"

func TestRun(t *testing.T) {
	got := Run()
	if got != 0 {
		t.Errorf("Run() = %d, want 0", got)
	}
}
