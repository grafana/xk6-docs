---
name: xk6-docs
description: Look up k6 documentation to write k6 load tests, browser tests, and performance scripts. Use when writing k6 test scripts, looking up k6 APIs (http, browser, websockets, grpc, metrics), or understanding k6 concepts (thresholds, checks, scenarios, executors).
---

# k6 docs

Read k6 docs via `<binary> x docs`.

If `<binary>` is not found: tell the user to re-run `k6 x docs skill <dir>` to update the skill, then stop.

## Commands

```
<binary> x docs                        # overview
<binary> x docs <path>                 # read a topic; shows content + subtopics at the bottom
<binary> x docs <path> --depth 2      # read a topic + 2 levels of subtopics in one call
<binary> x docs search <term>         # fuzzy search; returns matching paths
```

Paths use spaces or slashes interchangeably.

## Strategy

2 calls is the target.

Try a direct path first — wrong paths don't error, they return subtopics to guide your next call. Only use search or overview when you have no idea where to look.

Once you have a path (from a subtopics list or search), go to it directly — don't visit the parent to confirm first.

When a topic page shows a method table with descriptions, that is the complete API — no need to read individual method sub-pages.

## Rules

- Full parent path required: `using-k6 thresholds` works, `thresholds` alone fails.
- `k6-` prefix is auto-added on `javascript-api` paths where needed.
