## Shell completions

Bash:

    vrk completions bash > ~/.bash_completion.d/vrk
    source ~/.bash_completion.d/vrk
    # Or system-wide:
    vrk completions bash > /etc/bash_completion.d/vrk

Zsh:

    vrk completions zsh > "${fpath[1]}/_vrk"

Fish:

    vrk completions fish > ~/.config/fish/completions/vrk.fish
