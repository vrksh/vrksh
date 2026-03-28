// Package completions implements vrk completions — shell completion script generator.
// Emits completion scripts for bash, zsh, or fish to stdout.
package completions

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// Run is the entry point for vrk completions. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("completions", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var jsonFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit errors as JSON")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "completions: " + err.Error(), "code": 2})
		}
		return shared.UsageErrorf("completions: %s", err.Error())
	}

	args := fs.Args()
	if len(args) == 0 {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "completions: shell argument required (bash, zsh, fish)", "code": 2})
		}
		return shared.UsageErrorf("completions: shell argument required (bash, zsh, fish)")
	}

	shell := args[0]
	tools := shared.Registry()

	var out string
	switch shell {
	case "bash":
		out = genBash(tools)
	case "zsh":
		out = genZsh(tools)
	case "fish":
		out = genFish(tools)
	default:
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": fmt.Sprintf("completions: unknown shell %q", shell), "code": 1})
		}
		return shared.Errorf("completions: unknown shell %q (supported: bash, zsh, fish)", shell)
	}

	fmt.Print(out)
	return 0
}

func genBash(tools []shared.ToolMeta) string {
	var b strings.Builder
	b.WriteString("_vrk() {\n")
	b.WriteString("    local cur prev words cword\n")
	b.WriteString("    _init_completion || return\n")
	b.WriteString("\n")
	b.WriteString("    if [[ $cword -eq 1 ]]; then\n")

	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	fmt.Fprintf(&b, "        COMPREPLY=($(compgen -W %q -- \"$cur\"))\n", strings.Join(names, " "))
	b.WriteString("        return\n")
	b.WriteString("    fi\n")
	b.WriteString("\n")
	b.WriteString("    local tool=\"${words[1]}\"\n")
	b.WriteString("    case \"$tool\" in\n")

	for _, t := range tools {
		var flags []string
		for _, f := range t.Flags {
			flags = append(flags, "--"+f.Name)
			if f.Shorthand != "" {
				flags = append(flags, "-"+f.Shorthand)
			}
		}
		fmt.Fprintf(&b, "        %s)\n", t.Name)
		fmt.Fprintf(&b, "            COMPREPLY=($(compgen -W %q -- \"$cur\"))\n", strings.Join(flags, " "))
		b.WriteString("            ;;\n")
	}

	b.WriteString("    esac\n")
	b.WriteString("}\n")
	b.WriteString("complete -F _vrk vrk\n")

	return b.String()
}

func genZsh(tools []shared.ToolMeta) string {
	var b strings.Builder
	b.WriteString("#compdef vrk\n")
	b.WriteString("\n")
	b.WriteString("_vrk() {\n")
	b.WriteString("    local -a commands\n")
	b.WriteString("    commands=(\n")

	for _, t := range tools {
		desc := strings.ReplaceAll(t.Short, "'", "'\\''")
		fmt.Fprintf(&b, "        '%s:%s'\n", t.Name, desc)
	}

	b.WriteString("    )\n")
	b.WriteString("\n")
	b.WriteString("    _arguments -C \\\n")
	b.WriteString("        '1:command:->command' \\\n")
	b.WriteString("        '*::arg:->args'\n")
	b.WriteString("\n")
	b.WriteString("    case $state in\n")
	b.WriteString("    command)\n")
	b.WriteString("        _describe -t commands 'vrk tools' commands\n")
	b.WriteString("        ;;\n")
	b.WriteString("    args)\n")
	b.WriteString("        case $words[1] in\n")

	for _, t := range tools {
		fmt.Fprintf(&b, "        %s)\n", t.Name)

		var entries []string
		for _, f := range t.Flags {
			usage := escapeZshBrackets(f.Usage)
			entries = append(entries, fmt.Sprintf("'--%s[%s]'", f.Name, usage))
			if f.Shorthand != "" {
				entries = append(entries, fmt.Sprintf("'-%s[%s]'", f.Shorthand, usage))
			}
		}

		if len(entries) == 0 {
			// no flags — empty case
		} else if len(entries) == 1 {
			fmt.Fprintf(&b, "            _arguments %s\n", entries[0])
		} else {
			b.WriteString("            _arguments \\\n")
			for i, e := range entries {
				if i == len(entries)-1 {
					fmt.Fprintf(&b, "                %s\n", e)
				} else {
					fmt.Fprintf(&b, "                %s \\\n", e)
				}
			}
		}
		b.WriteString("            ;;\n")
	}

	b.WriteString("        esac\n")
	b.WriteString("        ;;\n")
	b.WriteString("    esac\n")
	b.WriteString("}\n")
	b.WriteString("\n")
	b.WriteString("_vrk \"$@\"\n")

	return b.String()
}

func genFish(tools []shared.ToolMeta) string {
	var b strings.Builder
	b.WriteString("complete -c vrk -f\n")

	for _, t := range tools {
		fmt.Fprintf(&b, "complete -c vrk -n '__fish_use_subcommand' -a '%s' -d '%s'\n",
			t.Name, escapeFishQuote(t.Short))
	}

	for _, t := range tools {
		for _, f := range t.Flags {
			if f.Shorthand != "" {
				fmt.Fprintf(&b, "complete -c vrk -n '__fish_seen_subcommand_from %s' -l %s -s %s -d '%s'\n",
					t.Name, f.Name, f.Shorthand, escapeFishQuote(f.Usage))
			} else {
				fmt.Fprintf(&b, "complete -c vrk -n '__fish_seen_subcommand_from %s' -l %s -d '%s'\n",
					t.Name, f.Name, escapeFishQuote(f.Usage))
			}
		}
	}

	return b.String()
}

func escapeZshBrackets(s string) string {
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}

func escapeFishQuote(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk completions <shell>",
		"       vrk completions bash",
		"       vrk completions zsh",
		"       vrk completions fish",
		"",
		"Generate shell completion scripts.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("completions: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
