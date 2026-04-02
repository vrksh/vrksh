---
title: "Environment Variables"
meta_title: "Environment Variables - vrksh runtime configuration"
description: "Environment variables read by vrk at runtime. No config file required."
noindex: false
---

Most vrk tools need zero configuration. Only two of 26 tools read environment variables at all.

## vrk prompt

`vrk prompt` is the only tool that requires an environment variable. Set one API key to get started.

| Variable | Required | What it does | Default |
|----------|----------|-------------|---------|
| `ANTHROPIC_API_KEY` | Required for Anthropic models | Authenticates requests to the Anthropic API | none |
| `OPENAI_API_KEY` | Required for OpenAI models | Authenticates requests to the OpenAI API | none |
| `VRK_LLM_KEY` | Optional | Generic Bearer token for OpenAI-compatible endpoints that don't use the standard key names | none |
| `VRK_DEFAULT_MODEL` | Optional | Default model when `--model` is not specified | `claude-sonnet-4-6` or `gpt-4o-mini` |
| `VRK_LLM_URL` | Optional | Base URL for an OpenAI-compatible API endpoint | Provider default |

At least one of `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` is required. Provider selection is automatic: `claude-*` models use Anthropic, `gpt-*` models use OpenAI. If only one key is set, that provider is used regardless of model name.

## vrk kv

`vrk kv` works out of the box. The only env var changes where the database lives.

| Variable | Required | What it does | Default |
|----------|----------|-------------|---------|
| `VRK_KV_PATH` | Optional | Path to the SQLite database file | `~/.vrk.db` |

## Everything else

The other 24 tools (tok, chunk, grab, mask, validate, jwt, epoch, uuid, sse, coax, links, plain, jsonl, emit, assert, sip, throttle, digest, base, recase, slug, moniker, pct, urlinfo) need no environment variables. They work with zero setup.

## Flag precedence

When both a flag and an environment variable are set, the flag wins:

| Flag | Overrides |
|------|-----------|
| `--model` | `VRK_DEFAULT_MODEL` |
| `--endpoint` | `VRK_LLM_URL` |

`VRK_KV_PATH` has no flag equivalent. Set it in your shell profile or per-pipeline with `export`.
