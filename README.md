[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/xk6-subcommand-docs)](https://goreportcard.com/report/github.com/grafana/xk6-subcommand-docs)
[![GitHub Actions](https://github.com/grafana/xk6-subcommand-docs/actions/workflows/ci.yml/badge.svg)](https://github.com/grafana/xk6-subcommand-docs/actions/workflows/ci.yml)

# xk6-subcommand-docs

**Look up any k6 doc instantly, right from your terminal.**

> [!NOTE]
> This extension is experimental and not officially supported by Grafana Labs.
>
> Star the repository to help us prioritize maintenance.

A [k6 extension](https://grafana.com/docs/k6/latest/extensions/) for developers and AI agents who want to stay in the terminal.

- Stay in the flow:  never leave the terminal to look something up
- Works offline: no network needed after first use
- Always the right version:  docs match your k6 build, not just "latest"
- Find what you need: search by any word
- Compose with the tools you love ([glow](https://github.com/charmbracelet/glow)) and render beautiful docs

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

## Build

To use the subcommand, compile a custom `k6`:

```bash
xk6 build --with github.com/grafana/xk6-subcommand-docs
# Emits a k6 in the current directory
```

Requires [xk6](https://github.com/grafana/xk6). See the [xk6 documentation](https://github.com/grafana/xk6) for more build options.

## Rendered output

For a nicer reading experience, configure a markdown renderer in `~/.config/k6/docs.yaml`:

```yaml
renderer: glow -p 200
```

## Teach your AI agent how to use k6 effectively

Spend less tokens and context (= less costs + better AI performance), and fast answers.

```bash
npx @anthropic-ai/claude-code skill install --url https://github.com/grafana/xk6-subcommand-docs
```

## Development

```
make test                                               # Run tests
make lint                                               # Run linter
make build                                              # Build k6 with this extension
make prepare K6_VERSION=v1.5.x K6_DOCS_PATH=~/k6-docs   # Prepare docs bundle locally
```

## Contribute

To report bugs or suggest features, [open an issue](https://github.com/grafana/xk6-subcommand-docs/issues).
