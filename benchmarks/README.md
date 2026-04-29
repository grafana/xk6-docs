# Agent token cost benchmarks

Compares three ways agents look up k6 docs: xk6-docs CLI, xk6-docs file reading, and mcp-k6 MCP server.

## Setup

### Build binaries

```sh
cd /path/to/xk6-docs

# Original CLI mode (from main branch, before changes)
git stash  # stash local changes if any
go build -o /tmp/xk6-docs-test/k6o ./internal/clitest
git stash pop

# New file-reading mode (with changes)
go build -o /tmp/xk6-docs-test/k6d ./internal/clitest
```

Verify they behave differently:

```sh
# Original: prints TOC
/tmp/xk6-docs-test/k6o x docs 2>/dev/null | head -3

# New: prints markdown directory path
/tmp/xk6-docs-test/k6d x docs 2>/dev/null | head -3
```

### Install mcp-k6

```sh
brew tap grafana/grafana
brew install mcp-k6
```

Configure it for Claude Code by creating `/tmp/xk6-docs-test/.mcp.json`:

```json
{
  "mcpServers": {
    "k6": {
      "command": "mcp-k6",
      "args": ["-transport", "stdio"]
    }
  }
}
```

## Run benchmarks

All commands run from `/tmp/xk6-docs-test/`.

### Prompts

Same task list for all three, differing only in how they access docs:

**xk6-docs CLI** (`prompt-original-multi.md`):
```
Complete these tasks in order. Use `./k6o x docs` to look up k6 documentation. Do not load any skills.

1. Write a k6 browser test script that uses frame locators.
2. Write a k6 gRPC load test script that streams data.
3. Show me how to use environment variables and secrets in k6.
4. Apply browser testing best practices to the script from task 1.
5. Write a k6 script using WebSocket with custom metrics and thresholds.

For each task, write the complete script.
```

**xk6-docs files** (`prompt-new-multi.md`):
Same but `./k6d x docs` instead of `./k6o x docs`.

**mcp-k6** (`prompt-mcp-multi.md`):
```
Complete these tasks in order. Use the k6 MCP tools (list_sections, get_documentation) to look up k6 documentation. Do not load any skills. Do not use xk6-docs or any CLI binary.

1. Write a k6 browser test script that uses frame locators.
2. Write a k6 gRPC load test script that streams data.
3. Show me how to use environment variables and secrets in k6.
4. Apply browser testing best practices to the script from task 1.
5. Write a k6 script using WebSocket with custom metrics and thresholds.

For each task, write the complete script.
```

### Execute

Run all three in parallel (or sequentially):

```sh
cd /tmp/xk6-docs-test

# xk6-docs CLI
claude --dangerously-skip-permissions \
  -p "$(cat prompt-original-multi.md)" \
  --output-format json > result-original-multi.json

# xk6-docs files
claude --dangerously-skip-permissions \
  -p "$(cat prompt-new-multi.md)" \
  --output-format json > result-new-multi.json

# mcp-k6 (needs .mcp.json in cwd)
claude --dangerously-skip-permissions \
  -p "$(cat prompt-mcp-multi.md)" \
  --output-format json > result-mcp-multi.json
```

## Analyze results

```sh
python3 analyze.py result-original-multi.json result-new-multi.json result-mcp-multi.json
```

## Results (2025-04-06, Claude Sonnet)

5-task run (frame locators, gRPC, env vars, browser best practices, WebSocket):

```
                         xk6-docs     xk6-docs       mcp-k6
                              CLI    md files          MCP
------------------------------------------------------------
Duration                   151.5s       124.0s       155.1s
Turns                          40           30           25
Tool calls                     39           29           24
Total tokens            1,484,229    1,039,238    1,701,417
vs CLI tokens             baseline         -30%         +15%
```

Fair comparison (same prompt, path given directly, no CLI discovery overhead):

```
                       md files    json files   json + jq
----------------------------------------------------------
Duration                  96.9s        343.3s       131.2s
Tool calls                   20           100           46
Total tokens            644,805     4,835,633    1,346,053
vs markdown              baseline        +650%        +109%
```

Markdown file reading is the cheapest approach by a wide margin.

- **JSON (raw)**: catastrophically worse. Minified JSON can't be skimmed, so the agent reads every file individually (100 tool calls vs 20).
- **JSON + jq**: much better than raw JSON, but still 2x the calls of markdown. Each jq query returns a slice, forcing multiple passes per file (list headings → get code → get content). With markdown, one `cat` returns everything scannable.
- **MCP**: fewer tool calls than the CLI but higher total token cost due to protocol overhead.
