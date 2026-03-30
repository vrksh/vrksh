# assert - Pipeline condition check - jq conditions, --contains, --matches

When to use: check conditions on pipeline data and halt on failure. JSON mode evaluates gojq expressions; text mode checks substrings (--contains) or regex (--matches). Data passes through on success.
Composes with: prompt, validate, grab, kv

| Flag          | Short | Type   | Description                                       |
|---------------|-------|--------|---------------------------------------------------|
| `<condition>` |       | string | jq-compatible condition (positional, repeatable)  |
| `--contains`  |       | string | Assert stdin contains substring (plain text mode) |
| `--matches`   |       | string | Assert stdin matches Go regex (plain text mode)   |
| `--message`   | `-m`  | string | Custom failure message                            |
| `--json`      | `-j`  | bool   | Emit `{"passed":bool,...}` to stdout              |
| `--quiet`     | `-q`  | bool   | Suppress stderr on failure                        |

Exit 0: assertion passed, stdin passed through byte-for-byte
Exit 1: assertion failed, JSON parse error, I/O error
Exit 2: no condition, no stdin, mode conflict, invalid regex

Example:

    echo '{"score":0.9}' | vrk assert '.score > 0.8' | vrk kv set result

Anti-pattern:
- Don't use validate when you need value checks. Validate checks types (string, number); assert checks values (.confidence >= 0.8). They're complementary - validate the schema, then assert the content.
