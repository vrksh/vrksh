# Known-bad examples: exactly 4 of these must fail

```bash
$ echo hello
goodbye
$ echo $?
5
$ echo a
a
$ printf 'a\nb\n'
a
...
c
$ echo fine
fine
$ echo extra
extra
more
```
