## How it works

### Create symlinks

<!-- doccheck: skip (creates symlinks next to the binary) -->
```bash
$ vrk bare
Linking assert   → /usr/local/bin/assert  ✓
Linking base     → /usr/local/bin/base  ✓
...
26 linked.
```

Each tool gets a symlink: `tok -> vrk`, `jwt -> vrk`, `epoch -> vrk`, etc. The multicall binary detects which name it was invoked as and dispatches to the right tool.

### Preview before creating (--dry-run)

<!-- doccheck: skip (creates symlinks next to the binary) -->
```bash
$ vrk bare --dry-run
Linking assert   → /usr/local/bin/assert
Linking base     → /usr/local/bin/base
...
```

### Check existing symlinks (--list)

<!-- doccheck: skip (depends on which symlinks exist) -->
```bash
$ vrk bare --list
assert    /usr/local/bin/assert
base      /usr/local/bin/base
...
```

### Remove symlinks (--remove)

<!-- doccheck: skip (removes symlinks next to the binary) -->
```bash
$ vrk bare --remove
Removed assert
Removed base
...
26 removed.
```

Only removes symlinks that point to the vrk binary. Other files with the same names are left untouched.

### Overwrite conflicts (--force)

If a file already exists at a symlink path, `--force` replaces it:

```bash
vrk bare --force
```

Without `--force`, existing files are skipped with a warning.

## After setup

```bash
# Before: vrk prefix required
vrk tok --check 8000 < prompt.txt

# After: direct invocation
tok --check 8000 < prompt.txt
grab https://example.com | prompt --system 'Summarize'
```

All flags and piping work identically.
