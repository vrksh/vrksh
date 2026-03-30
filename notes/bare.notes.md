## How it works

### Create symlinks

```bash
$ vrk bare
Created 26 symlinks in /usr/local/bin/
```

Each tool gets a symlink: `tok -> vrk`, `jwt -> vrk`, `epoch -> vrk`, etc. The multicall binary detects which name it was invoked as and dispatches to the right tool.

### Preview before creating (--dry-run)

```bash
$ vrk bare --dry-run
Would create: /usr/local/bin/tok -> /usr/local/bin/vrk
Would create: /usr/local/bin/jwt -> /usr/local/bin/vrk
Would create: /usr/local/bin/epoch -> /usr/local/bin/vrk
...
```

### Check existing symlinks (--list)

```bash
$ vrk bare --list
tok -> /usr/local/bin/vrk
jwt -> /usr/local/bin/vrk
epoch -> /usr/local/bin/vrk
```

### Remove symlinks (--remove)

```bash
$ vrk bare --remove
Removed 26 symlinks
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
