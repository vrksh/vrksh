package shared

import (
	"testing"
)

func TestRegisterAndRetrieve(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(ToolMeta{
		Name:  "tok",
		Short: "Token counter",
		Flags: []FlagMeta{
			{Name: "json", Shorthand: "j", Usage: "emit output as JSON"},
			{Name: "check", Usage: "pass input through if within N tokens; exit 1 if over"},
		},
	})
	Register(ToolMeta{
		Name:  "jwt",
		Short: "JWT inspector",
		Flags: []FlagMeta{
			{Name: "claim", Shorthand: "c", Usage: "extract a single claim"},
		},
	})

	got := Registry()
	if len(got) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(got))
	}

	// Sorted alphabetically: jwt before tok.
	if got[0].Name != "jwt" {
		t.Errorf("expected first tool to be jwt, got %s", got[0].Name)
	}
	if got[1].Name != "tok" {
		t.Errorf("expected second tool to be tok, got %s", got[1].Name)
	}

	// Verify flags carried through.
	if len(got[0].Flags) != 1 {
		t.Errorf("expected jwt to have 1 flag, got %d", len(got[0].Flags))
	}
	if len(got[1].Flags) != 2 {
		t.Errorf("expected tok to have 2 flags, got %d", len(got[1].Flags))
	}

	// Returned slice is a copy — mutating it must not affect the registry.
	got[0].Name = "MUTATED"
	fresh := Registry()
	if fresh[0].Name == "MUTATED" {
		t.Error("Registry() returned a reference to the internal slice, not a copy")
	}
}

func TestRegistryNoDuplicates(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(ToolMeta{Name: "epoch", Short: "Epoch converter"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected panic message to be a string, got %T", r)
		}
		if got := msg; got != "vrksh: duplicate tool registration: epoch" {
			t.Errorf("unexpected panic message: %s", got)
		}
	}()

	Register(ToolMeta{Name: "epoch", Short: "Epoch converter duplicate"})
}

func TestRegistryEmpty(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	got := Registry()
	if got == nil {
		t.Fatal("Registry() returned nil on empty registry, expected empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(got))
	}
}
