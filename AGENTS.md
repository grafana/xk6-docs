`k6 x docs` — offline k6 documentation in the terminal. For humans and AI agents.

- The docs do not live in the binary. On first run the extension reads the k6 version it was built with, downloads the matching doc bundle from GitHub releases, and caches it under `~/.local/share/k6/docs/{version}/`.
- Later runs serve that cache and touch no network.
- A cached bundle is checked once a day and re-downloaded when it changed upstream.
- CI builds the bundles from the [k6 docs](https://github.com/grafana/k6-docs) repository and publishes them as assets of one `doc-bundles` release.

## RULES
1. Update this file concisely whenever features are added, removed, or changed.
2. Update the `.agents/features.md` file when user-facing features are added, removed, or changed.
3. Read `.agents/history.md` for past incidents and lessons learned.
4. Never take easy shortcuts or chase after easy wins. Execute what's asked for.
5. Make the minimal change that delivers the new feature or fixes the bug.
3. Use TDD: write a minimal test, fail it, write a minimal code to pass it. Repeat.
6. User facing features should be tested with test scripts in `testdata/scripts`.
7. Never skip linters (`//nolint` without proof), trick the linter, change `go.mod` k6 floor below v1.5.0, add global vars or `init()` (except `register.go`).
8. Plans: When writing complex features or significant refactors, use a Plan (as described in `.agents/PLANS.md`) from design to implementation. Store plans in `.claude/plans/` with incrementing numbers.
9. This file is not an implementation scratchpad. It tells an agent what the project is and how to operate in it, so keep the internals out: no function, method, type, or variable names, and no walk through the code. Behavior belongs in `.agents/features.md`, and the code is the only record of how it works.

## RELEASING
- Read the entire release skill: `https://github.com/grafana/k6-extension-registry/blob/main/.agents/skills/k6-extension-release/SKILL.md`. It shows you how to release this extension.
- Two Go modules live here. The root module is the extension. `docs/` is `github.com/grafana/xk6-docs/docs`, and `mcp-k6` imports it.
- The `docs` module carries its own tags, `docs/vX.Y.Z`, so its version does not follow the extension's `v0.0.x`. Tag it when `docs/` changes, then bump it in the root `go.mod` and in `mcp-k6`.

## CI/CD
- Every push and pull request to `main` runs the linters, the tests, and a build.
- A `vx.y.z` tag builds k6 with this extension for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, and windows/amd64, then publishes the binaries and their checksums as a GitHub release.
- A job runs every three hours, or on demand. It compares the k6 doc versions from v1.5.x up against the published bundles, builds the ones that are missing, and rebuilds the ones that went stale.
- To check that the sync works, compare the asset dates of the `doc-bundles` release against the commit dates of the k6 doc version folders. No version folder can be newer than its bundle.

## AGENT SKILL
- `k6 x docs skill <dir>` installs a skill that teaches an agent to use `k6 x docs`.
- `k6 x docs skill` with no directory lists the supported agents and where each one keeps its skills.
- When the skill cannot run the binary, it tells the user to install it again and stops.
- The skill holds navigation paths and the traps an agent hits, never docs content. Code examples and API descriptions are already in the docs that `k6 x docs` serves.
- Each reference file covers one module or one area.
- Run `k6 x docs` yourself before you change the skill, so you check every path you write.
- `skills/xk6-docs/scripts/validate-paths.sh ./k6` reports the broken paths and the modules that the skill does not cover.
