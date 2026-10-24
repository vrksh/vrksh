# Known-good examples: every one of these must pass

```bash
$ echo hello
hello
$ echo $?
0
$ false
$ echo $?
1
```

State carries over inside a block, and `...` works:

```bash
$ X=42
$ echo "x is $X"
x is 42
$ printf 'a\nb\nc\n'
a
...
c
$ echo prefix-12345-suffix
prefix-...-suffix
# comments in the expected text are ignored
$ printf 'one \\\n'
one \
```

Continued commands and stderr:

```bash
$ echo a \
    b
a b
$ echo oops >&2
oops
```

<!-- doccheck: skip (needs network) -->
```bash
$ this-command-does-not-exist
never compared
```
