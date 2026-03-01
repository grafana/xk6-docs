---
name: k6-x-docs
description: Look up k6 documentation using the k6 x docs CLI extension
user_invocable: true
---

# k6 x docs — CLI documentation lookup

## Commands

| Command | Description |
|---------|-------------|
| `k6 x docs` | Show top-level categories (JavaScript API, using k6, etc.) |
| `k6 x docs <topic>` | Show documentation for a specific topic |
| `k6 x docs <topic> <subtopic>` | Show specific subtopic documentation |
| `k6 x docs search <term>` | Search documentation |
| `k6 x docs best-practices` | Show k6 best practices guide |

## Search

- Fuzzy matching: spaces and dashes are ignored ("close context" matches "closecontext")
- Searches titles, descriptions, and content

## JavaScript API navigation

The JavaScript API is the largest section. Key modules:

| Command | Module |
|---------|--------|
| `k6 x docs k6-http` | k6/http — HTTP requests |
| `k6 x docs k6-http get` | HTTP GET method |
| `k6 x docs browser` | Browser module |
| `k6 x docs browser newcontext` | Browser: newContext method |
| `k6 x docs k6-crypto` | k6/crypto (different from `crypto`) |
| `k6 x docs k6-metrics` | k6/metrics |
| `k6 x docs k6-ws` | k6/ws — WebSocket |

## Tips

- Use `search` for fuzzy finding when you don't know the exact path
- The `best-practices` command gives a comprehensive guide for writing k6 scripts
- Module names starting with `k6-` map to k6 JS modules (e.g., `k6-http` → `k6/http`)
