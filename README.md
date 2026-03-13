[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/xk6-docs)](https://goreportcard.com/report/github.com/grafana/xk6-docs)
[![GitHub Actions](https://github.com/grafana/xk6-docs/actions/workflows/ci.yml/badge.svg)](https://github.com/grafana/xk6-docs/actions/workflows/ci.yml)

# xk6-docs

**Look up any k6 doc instantly, right from your terminal.**

An official [k6 extension](https://grafana.com/docs/k6/latest/extensions/) for developers and AI agents who want to stay in the terminal.

- Stay in the flow:  never leave the terminal to look something up
- Works offline: no network needed after first use
- Always the right version:  docs match your k6 build, not just "latest"
- Find what you need: search by any word
- Pretty output: built-in markdown rendering for terminals

## Usage

```
k6 x docs                              # See all available topics
k6 x docs http                         # Learn about the k6/http module
k6 x docs http get                     # Look up a specific function
k6 x docs browser page click           # Dig into nested topics
k6 x docs using-k6 scenarios           # Explore k6 concepts
k6 x docs search threshold             # Find docs by keyword
k6 x docs search "close context"       # Don't worry about exact names
k6 x docs best-practices               # Get best practices guidance
```

## Install

Download a pre-built binary from [releases](https://github.com/grafana/xk6-docs/releases) and use it as your `k6`. It's a drop-in replacement — everything k6 does, plus `k6 x docs`.

Or build it yourself with [xk6](https://github.com/grafana/xk6):

```bash
xk6 build --with github.com/grafana/xk6-docs
```

## Agent Skill

An [agent skill](https://agentskills.io) is included so AI coding agents can look up k6 docs efficiently — fewer commands, no guessing paths, no wasted tokens.

Works with Claude Code, Cursor, Codex, Gemini CLI, OpenCode, GitHub Copilot, and [35+ other agents](https://agentskills.io).

Install the skill directly from your k6 binary:

```bash
k6 x docs skill ~/.claude/skills    # Claude Code
k6 x docs skill ~/.agents/skills    # Cursor, Codex, Gemini CLI, etc.
```

Run `k6 x docs skill` without arguments to see all supported agents.

## Development

```
make test                                               # Run tests
make lint                                               # Run linter
make build                                              # Build k6 with this extension
make prepare K6_VERSION=v1.5.x K6_DOCS_PATH=~/k6-docs   # Prepare docs bundle locally
```

## Contribute

To report bugs or suggest features, [open an issue](https://github.com/grafana/xk6-docs/issues).
