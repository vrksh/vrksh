## How it works

`vrk bare` creates a symlink for each vrk tool, so you can run `tok` instead of `vrk tok`.

### Create symlinks

<!-- doccheck: skip (creates symlinks next to the binary) -->
```bash
$ vrk bare
Linking assert   → /usr/local/bin/assert  ✓
Linking base     → /usr/local/bin/base  ✓
...
26 linked.
```

Each tool gets a symlink: `tok -> vrk`, `jwt -> vrk`, `epoch -> vrk`, and so on. The multicall binary checks the name it was started as and runs that tool.

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

`--remove` deletes only symlinks that point to the vrk binary. It does not touch other files with the same names.

### Overwrite conflicts (--force)

If a file already exists at a symlink path, `--force` replaces it:

```bash
vrk bare --force
```

If you do not use `--force`, bare skips existing files and prints a warning.

## After setup

```bash
# Before: vrk prefix required
vrk tok --check 8000 < prompt.txt

# After: direct invocation
tok --check 8000 < prompt.txt
grab https://example.com | prompt --system 'Summarize'
```

All flags and pipes work the same as before.
