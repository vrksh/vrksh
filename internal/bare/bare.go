package bare

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// OsExecutable is a var so tests can inject a fake binary path.
var OsExecutable = os.Executable

// Run implements `vrk --bare`. It receives the args after "--bare" and a
// sorted list of known tool names (from the tools map in main.go). It does NOT
// access the tools map directly — this keeps it testable with arbitrary names.
func Run(args []string, toolNames []string) int {
	fs := pflag.NewFlagSet("bare", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var force, remove, list, dryRun bool
	fs.BoolVar(&force, "force", false, "overwrite collisions")
	fs.BoolVar(&remove, "remove", false, "remove bare symlinks")
	fs.BoolVar(&list, "list", false, "list active bare symlinks")
	fs.BoolVar(&dryRun, "dry-run", false, "preview, no changes")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printBareUsage()
		}
		return shared.UsageErrorf("bare: %s", err.Error())
	}

	// Mutual exclusion.
	if list && remove {
		return shared.UsageErrorf("bare: --list and --remove cannot be combined")
	}
	if list && force {
		return shared.UsageErrorf("bare: --list and --force cannot be combined")
	}
	if list && dryRun {
		return shared.UsageErrorf("bare: --list and --dry-run cannot be combined")
	}
	if remove && force {
		return shared.UsageErrorf("bare: --remove and --force cannot be combined")
	}

	// Resolve the vrk binary's absolute path.
	vrkBin, err := resolveVrkBin()
	if err != nil {
		return shared.Errorf("bare: %v", err)
	}
	binDir := filepath.Dir(vrkBin)
	vrkName := filepath.Base(vrkBin)

	// Build a set of known tool names for validation.
	toolSet := make(map[string]bool, len(toolNames))
	for _, name := range toolNames {
		toolSet[name] = true
	}

	positional := fs.Args()

	switch {
	case list:
		return bareList(binDir, vrkBin)

	case remove:
		// --remove with positional args: remove those names.
		// --remove with no args: scan dir for all vrk symlinks.
		return bareRemove(binDir, vrkBin, positional, dryRun)

	default:
		// Link mode — validate positional args against known tools.
		for _, name := range positional {
			if !toolSet[name] {
				return shared.Errorf("unknown tool: %s", name)
			}
		}

		targets := toolNames
		if len(positional) > 0 {
			targets = positional
		}

		// Filter out the binary's own name (self-exclusion).
		var filtered []string
		for _, name := range targets {
			if name != vrkName {
				filtered = append(filtered, name)
			}
		}

		// Check write permission before making changes.
		if !dryRun {
			if err := checkBareWritable(binDir); err != nil {
				return shared.Errorf("cannot write to %s: permission denied\ntry: sudo vrk --bare", binDir)
			}
		}

		return bareLink(binDir, vrkBin, filtered, force, dryRun)
	}
}

// resolveVrkBin returns the fully resolved absolute path of the running binary.
func resolveVrkBin() (string, error) {
	exe, err := OsExecutable()
	if err != nil {
		return "", fmt.Errorf("cannot determine binary path: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("cannot resolve binary path: %v", err)
	}
	return resolved, nil
}

// isVrkSymlink returns true if path is a symlink whose fully resolved target
// equals vrkBin. Both sides are resolved via filepath.EvalSymlinks so relative
// symlinks and chains are handled correctly. If the symlink is broken (target
// deleted), falls back to os.Readlink and manual resolution so --remove can
// still clean up dangling links from a previous installation.
func isVrkSymlink(path, vrkBin string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// Broken symlink — target doesn't exist. Read the raw link target
		// and resolve manually so --remove can clean up dangling links.
		target, readErr := os.Readlink(path)
		if readErr != nil {
			return false
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		target, _ = filepath.Abs(target)
		return target == vrkBin
	}
	return resolved == vrkBin
}

// checkBareWritable tests whether dir is writable by creating and removing a temp file.
func checkBareWritable(dir string) error {
	probe := filepath.Join(dir, ".vrk-bare-probe")
	f, err := os.Create(probe)
	if err != nil {
		return err
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return nil
}

// barePrintf writes to stdout, returning an error on write failure.
func barePrintf(format string, args ...any) error {
	_, err := fmt.Fprintf(os.Stdout, format, args...)
	return err
}

// bareLink creates symlinks for each tool in targets, pointing to vrkBin.
func bareLink(binDir, vrkBin string, targets []string, force, dryRun bool) int {
	maxName := 0
	for _, name := range targets {
		if len(name) > maxName {
			maxName = len(name)
		}
	}

	linked := 0
	skipped := 0
	var skippedNames []string

	for _, name := range targets {
		dest := filepath.Join(binDir, name)

		// Check current state at dest.
		if isVrkSymlink(dest, vrkBin) {
			if err := barePrintf("Linking %-*s → %s  ✓ (already linked)\n", maxName, name, dest); err != nil {
				return shared.Errorf("bare: %v", err)
			}
			linked++
			continue
		}

		_, statErr := os.Lstat(dest)
		exists := statErr == nil

		if exists && !force {
			if err := barePrintf("Linking %-*s → %s  ⚠ exists — skipped\n", maxName, name, dest); err != nil {
				return shared.Errorf("bare: %v", err)
			}
			skipped++
			skippedNames = append(skippedNames, name)
			continue
		}

		if dryRun {
			if exists && force {
				if err := barePrintf("Would overwrite %-*s → %s\n", maxName, name, dest); err != nil {
					return shared.Errorf("bare: %v", err)
				}
			} else {
				if err := barePrintf("Would link %-*s → %s\n", maxName, name, dest); err != nil {
					return shared.Errorf("bare: %v", err)
				}
			}
			linked++
			continue
		}

		// If force and exists, remove the existing file first.
		if exists && force {
			if err := os.Remove(dest); err != nil {
				return shared.Errorf("cannot remove %s: %v", dest, err)
			}
		}

		if err := os.Symlink(vrkBin, dest); err != nil {
			return shared.Errorf("cannot link %s: %v", dest, err)
		}

		if exists && force {
			if err := barePrintf("Linking %-*s → %s  ✓ (overwritten)\n", maxName, name, dest); err != nil {
				return shared.Errorf("bare: %v", err)
			}
		} else {
			if err := barePrintf("Linking %-*s → %s  ✓\n", maxName, name, dest); err != nil {
				return shared.Errorf("bare: %v", err)
			}
		}
		linked++
	}

	// Summary.
	if skipped > 0 {
		if err := barePrintf("%d linked, %d skipped.\n", linked, skipped); err != nil {
			return shared.Errorf("bare: %v", err)
		}
		if len(skippedNames) <= 5 {
			if err := barePrintf("Use 'vrk --bare --force %s' to overwrite, or 'vrk <tool>' directly.\n",
				strings.Join(skippedNames, " ")); err != nil {
				return shared.Errorf("bare: %v", err)
			}
		} else {
			if err := barePrintf("Use 'vrk --bare --force' to overwrite all, or 'vrk <tool>' directly.\n"); err != nil {
				return shared.Errorf("bare: %v", err)
			}
		}
	} else {
		if err := barePrintf("%d linked.\n", linked); err != nil {
			return shared.Errorf("bare: %v", err)
		}
	}

	return 0
}

// bareRemove removes symlinks pointing to vrkBin. If targets is empty, it scans
// the entire bin dir. If targets are specified, it removes only those names.
func bareRemove(binDir, vrkBin string, targets []string, dryRun bool) int {
	if len(targets) == 0 {
		entries, err := os.ReadDir(binDir)
		if err != nil {
			return shared.Errorf("bare: cannot read %s: %v", binDir, err)
		}
		for _, entry := range entries {
			path := filepath.Join(binDir, entry.Name())
			if isVrkSymlink(path, vrkBin) {
				targets = append(targets, entry.Name())
			}
		}
		sort.Strings(targets)
	}

	if !dryRun && len(targets) > 0 {
		if err := checkBareWritable(binDir); err != nil {
			return shared.Errorf("cannot write to %s: permission denied\ntry: sudo vrk --bare --remove", binDir)
		}
	}

	removed := 0
	for _, name := range targets {
		path := filepath.Join(binDir, name)

		if !isVrkSymlink(path, vrkBin) {
			if _, err := os.Lstat(path); err == nil {
				shared.Warn("%s is not a vrk symlink — skipped", path)
			}
			continue
		}

		if dryRun {
			if err := barePrintf("Would remove %s → %s\n", name, path); err != nil {
				return shared.Errorf("bare: %v", err)
			}
			removed++
			continue
		}

		if err := os.Remove(path); err != nil {
			return shared.Errorf("cannot remove %s: %v", path, err)
		}
		if err := barePrintf("Removed %s\n", name); err != nil {
			return shared.Errorf("bare: %v", err)
		}
		removed++
	}

	if len(targets) > 1 || removed > 1 {
		if err := barePrintf("%d removed.\n", removed); err != nil {
			return shared.Errorf("bare: %v", err)
		}
	}

	return 0
}

// bareList prints active vrk symlinks in two-column format.
func bareList(binDir, vrkBin string) int {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return shared.Errorf("bare: cannot read %s: %v", binDir, err)
	}

	type listEntry struct {
		name string
		path string
	}
	var active []listEntry

	for _, e := range entries {
		path := filepath.Join(binDir, e.Name())
		if isVrkSymlink(path, vrkBin) {
			active = append(active, listEntry{name: e.Name(), path: path})
		}
	}

	if len(active) == 0 {
		return 0
	}

	sort.Slice(active, func(i, j int) bool { return active[i].name < active[j].name })

	maxName := 0
	for _, a := range active {
		if len(a.name) > maxName {
			maxName = len(a.name)
		}
	}

	for _, a := range active {
		if err := barePrintf("%-*s  %s\n", maxName, a.name, a.path); err != nil {
			return shared.Errorf("bare: %v", err)
		}
	}

	return 0
}

func printBareUsage() int {
	lines := []string{
		"usage: vrk --bare [flags] [tools...]",
		"",
		"Create symlinks so vrksh tools can be called without the vrk prefix.",
		"Symlinks are created in the same directory as the vrk binary.",
		"",
		"flags:",
		"  --force      overwrite existing files at symlink paths",
		"  --remove     remove bare symlinks (only those pointing to vrk)",
		"  --list       list currently active bare symlinks",
		"  --dry-run    show what would happen, make no changes",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("bare: %v", err)
		}
	}
	return 0
}
