## Setup

### Bash

```bash
vrk completions bash > ~/.bash_completion.d/vrk
source ~/.bash_completion.d/vrk
```

### Zsh

```bash
vrk completions zsh > ~/.zsh/completions/_vrk
# Add to ~/.zshrc if not already there:
# fpath=(~/.zsh/completions $fpath)
# autoload -Uz compinit && compinit
```

### Fish

```bash
vrk completions fish > ~/.config/fish/completions/vrk.fish
```

## What you get

After installing completions:

```
$ vrk <tab>
assert   base   chunk   coax   completions   digest   emit   epoch   grab   ...

$ vrk tok --<tab>
--check   --json   --model   --quiet
```

Tool names and flags are completed from the binary itself, so they always match the installed version.

## When it fails

Unknown shell:

```bash
$ vrk completions powershell
error: completions: unsupported shell "powershell"
$ echo $?
1
```

No shell specified:

```bash
$ vrk completions
usage error: completions: shell argument required (bash, zsh, fish)
$ echo $?
2
```
