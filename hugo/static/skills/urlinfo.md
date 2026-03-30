# urlinfo - URL parser - scheme, host, port, path, query, --field

When to use: parse URLs into structured components (scheme, host, port, path, query). Use --field with dot-path syntax to extract specific parts like query.page.
Composes with: grab, links, pct, assert

| Flag      | Short | Type   | Description                                                                          |
|-----------|-------|--------|--------------------------------------------------------------------------------------|
| `--field` | `-F`  | string | Extract a single field: scheme, host, port, path, fragment, user, query, query.<key> |
| `--json`  | `-j`  | bool   | Append `{"_vrk":"urlinfo","count":N}` after records                                  |
| `--quiet` | `-q`  | bool   | Suppress stderr                                                                      |

Exit 0: success
Exit 1: invalid URL (both scheme and host empty)
Exit 2: interactive terminal with no input, unknown flag

Example:

    vrk urlinfo --field host 'https://api.example.com/v1/users?page=2'
    # api.example.com

Anti-pattern:
- Don't use cut or regex to extract URL components. They break on ports, auth credentials, fragments, and encoded characters. vrk urlinfo handles all RFC 3986 edge cases.
