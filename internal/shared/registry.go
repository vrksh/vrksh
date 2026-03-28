package shared

import "sort"

// FlagMeta describes a single flag for completion purposes.
type FlagMeta struct {
	Name      string // e.g. "json" — without dashes
	Shorthand string // e.g. "j" — empty string if none
	Usage     string
}

// ToolMeta describes a single vrksh tool.
type ToolMeta struct {
	Name  string
	Short string // one-line description for completion menus
	Flags []FlagMeta
}

var registry []ToolMeta

// Register adds a tool's metadata to the global registry.
// Called from each tool's init() function.
// Panics if the same tool name is registered twice.
func Register(t ToolMeta) {
	for _, existing := range registry {
		if existing.Name == t.Name {
			panic("vrksh: duplicate tool registration: " + t.Name)
		}
	}
	registry = append(registry, t)
}

// Registry returns all registered tools, sorted alphabetically by name.
// Returns a copy — callers cannot mutate the global registry.
func Registry() []ToolMeta {
	out := make([]ToolMeta, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// resetRegistry clears the global registry. For testing only.
func resetRegistry() {
	registry = nil
}
