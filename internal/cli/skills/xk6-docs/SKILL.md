---
name: xk6-docs
description: Look up official k6 documentation with `<binary> x docs` when writing, debugging, or configuring a k6 test, or reaching for any k6 API, option, executor, or CLI flag. k6 syntax changes between versions, so consult it instead of recalling k6 from memory or searching the web.
---

# k6 docs

k6 ships its official documentation offline. Read it with `<binary> x docs`. It auto-detects the running k6 version, so the docs always match the k6 in use.

Start with [When to use it](#when-to-use-it). The efficient workflow is two batched commands, [Find the slugs](#find-the-slugs-one-command) then [Read the slugs](#read-the-slugs-one-command); [Batch to cut round trips](#batch-to-cut-round-trips) explains why one command beats many.

## When to use it

Do not infer or guess k6 APIs, options, or behavior from memory. The k6 API drifts between versions, so recalled syntax is often subtly wrong. Whenever you write or debug a k6 test, or reach for any k6 API, CLI option, or concept, read `<binary> x docs` first and base your answer on what it returns.

Do not search the web or fetch grafana.com for k6 questions. `<binary> x docs` needs no network and is matched to the user's exact k6 version, so it is both faster and more accurate than anything online. Make it your only k6 reference.

## Batch to cut round trips

The docs are a tree of topics. What costs you is round trips, not how many pages you read. So the one rule is: **put many `<binary> x docs` invocations in a single shell command**, joined with `&&`. Ten pages in one command is one round trip; ten separate commands is ten.

    <binary> x docs a/one && <binary> x docs b/two && <binary> x docs c/three

`&&` works on Linux, macOS, and Windows. If a slug is wrong it exits non-zero and stops the rest of the chain, so batch slugs you're confident in. Answering a question is two batched commands: find the slugs, then read them.

## Find the slugs (one command)

Every page has a `group/child` address. Read the address the tool prints; never build a slug from a topic's name, because a bare name almost never resolves. Two ways to get exact slugs, both batchable into a single command:

- **Map the area (reliable).** `<binary> x docs --depth 2` prints the category tree; add a path to go deeper into one branch:

      <binary> x docs <category> --depth 3

  Mapping, and reading a parent page (which lists its children), come straight from the doc tree, so they always show the real child slugs. Reach for this whenever you know roughly where a topic lives, or to browse. One map of the right branch beats mapping the whole tree, and beats mapping several branches one command at a time.
- **Search when you don't know where it lives (fallback).** `<binary> x docs search <query>` prints a `group` with its matching `child` lines indented under it; a match reads as the single argument `group/child`. Batch one search per topic:

      <binary> x docs search <query-a> && <binary> x docs search <query-b>

  Use a distinctive word: a common word matches many pages (noisy) or misses. If a search is noisy or empty, map the nearest category instead.

## Read the slugs (one command)

Chain the exact slugs that search or map printed:

    <binary> x docs group-a/child-x && <binary> x docs group-b/child-y

Reading a parent page prints its own content *and* lists its children, so one slot gives you the page and its sub-map together.

## Gotcha: read the slug the tool printed, don't invent it

A leaf topic usually lives under a category, so its own name alone often will not resolve ("topic not found"). Do not reguess a bare name from memory. Take the slug exactly as search or map printed it; if a lookup still misses, search or map again to get the real path rather than guessing another one.
