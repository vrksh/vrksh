# urlinfo - URL parser - scheme, host, port, path, query, --field

When to use: parse a URL into structured components without making a network call.
Composes with: grab, links, pct, assert

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--field` | `-F` | string | Extract a single field: scheme, host, port, path, fragment, user, query, query.<key> |
| `--json` | `-j` | bool | Append `{"_vrk":"urlinfo","count":N}` after records |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success
Exit 1: invalid URL (both scheme and host empty)
Exit 2: interactive terminal with no input, unknown flag

Example:

    vrk urlinfo --field host 'https://api.example.com/v1/users?page=2'
    # api.example.com

Anti-pattern:
- Don't use urlinfo to fetch content -- it only parses the string. Use grab for HTTP requests.
