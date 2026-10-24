# Recipe tests

Every recipe page `hugo/content/recipes/<slug>.md` (except `_index.md`) needs a
directory `testdata/recipes/<slug>/` with fixtures and an `expect.sh`. A recipe
without one fails `make recipecheck`.

How it runs (`internal/recipecheck`, build tag `recipes`):

1. The dir is copied to a temp dir and the harness `cd`s into it.
2. All front-matter `steps:` entries that are scripts run in order, in ONE bash
   process (bare tool names like `- "kv"` are skipped). stdout+stderr of the
   steps go to the file named by `$OUT`. Stdin is closed.
3. `bash expect.sh` runs in the same dir with the same env. Exit 0 means pass.

Environment for both:

- `PATH` has the built `vrk` first.
- `HOME` and `VRK_KV_PATH` point inside the temp dir.
- `VRK_LLM_URL=http://127.0.0.1:<port>/v1` is a mockllm (`internal/mockllm`).
- `OPENAI_API_KEY` and `VRK_LLM_KEY` are dummy values; `ANTHROPIC_API_KEY` is unset.
- `VRK_DEFAULT_MODEL=mock`, because `prompt` requires a model with a custom endpoint.
- `MOCK_LOG` is mockllm's request log (JSONL: `path`, `auth`, `body`). It is
  emptied before each recipe.
- `OUT` is the steps output file.

Extras for `expect.sh` (not part of the lead's minimal contract):

- `STEPS` is the script that was run (re-run it with `bash "$STEPS"`), and
  `STEPS_RC` is its exit status. Steps do not stop on error.
- `SITE_URL` is a local HTTP server serving the temp dir, for recipes that fetch URLs.
- If the dir has a file `env`, it is sourced (with auto-export) before the
  steps. Use it for variables the recipe assumes (`TOKEN=...`) and for seeding
  state (`vrk kv set ...`). It is not available to `expect.sh` unless it re-sources it.

mockllm replies: JSON with the right keys if a system message contains a
`{"key":"type"}` object, otherwise `MOCK(<n> chars): <first 50 chars of the last user message>`.

`expect.sh` must encode the CORRECT behaviour of the recipe, not what the code
does today.
- `MOCKLLM` is the mockllm binary, for recipes that need a second mock with
  different flags (for example `--fail-first 1` to test retries). Start it with
  `--port 0` and read the address from its `listening on` stderr line.
