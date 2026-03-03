# Plan 9: Respect --no-color global flag

## Problem

`k6 x docs` ignores k6's `--no-color` global flag. When a renderer (e.g. `glow`) is configured in `docs.yaml`, it produces colored output even when `--no-color` is set.

## Fix

In `cmd.go:92`, add `!gs.Flags.NoColor` to the renderer gate:

```go
// before:
if cfg.Renderer != "" && gs.Stdout.IsTTY {
// after:
if cfg.Renderer != "" && gs.Stdout.IsTTY && !gs.Flags.NoColor {
```

When `--no-color` is set, the renderer is skipped entirely and raw markdown is written to stdout — same as when no renderer is configured.

## Steps (TDD)

1. **Red**: Write a test that sets `gs.Flags.NoColor = true` with a renderer configured, asserts the renderer is NOT invoked (raw markdown output).
2. **Green**: Add `!gs.Flags.NoColor` to the condition in `prepareRun`.
3. **Refactor**: Check if anything needs cleanup.
4. **Lint**: Run `golangci-lint`.

## Files touched

- `cmd.go` — one-line condition change
- `cmd_test.go` (or `docs_test.go`) — new test
