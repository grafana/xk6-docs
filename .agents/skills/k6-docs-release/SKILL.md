---
name: k6-docs-release
description: Release a new version of xk6-docs. Use when the user says "release", "tag", "new version", "publish", "cut a release" in the xk6-docs repo. Covers the full flow — push, tag, GitHub release, extension registry PR, and Slack announcement.
---

# xk6-docs Release

This skill handles the full release process for `xk6-docs`. It is NOT a k6 release — do not load the `k6-release` skill.

Read `AGENTS.md` and `history.md` before every release. History contains burned versions and lessons.

When any step fails (Slack, GitHub API, proxy check, etc.), retry up to 5 times before giving up. External services are flaky — persistence usually wins.

## Pre-flight

1. All tests and lint must pass locally before pushing.
2. Determine the next version: check `git tag --sort=-v:refname | head -1` and increment the patch.
3. Collect commits since last tag: `git log <last-tag>..HEAD --oneline`.

## Step 1: Push and tag

    git push origin main
    # Wait for CI to pass — check with: gh run list --limit 1
    git tag v<VERSION>
    git push origin v<VERSION>

This triggers the release workflow which builds binaries and creates a draft GitHub release.

IMPORTANT: Never move a tag after pushing. The Go module proxy cache is immutable — a faulty tag cannot be replaced, only superseded by a new version. See `history.md` for burned versions.

## Step 2: Verify the release

Wait for the release workflow to complete:

    gh run list --limit 1 --workflow release.yml

Then verify the release exists and has assets:

    gh release view v<VERSION>

## Step 3: Write the GitHub release description

Edit the release with a description. Always wrap technical terms in backticks. Group changes by new features, bug fixes, and maintenance. Each item links its commit hash. Omit sections that have no entries.

    gh release edit v<VERSION> --notes "$(cat <<'EOF'
    <release body here>
    EOF
    )"

Example from `v0.0.5`:

    xk6-docs `v0.0.5` is here!
    - Tab-complete topics as you type with shell completions
    - Scroll long pages with `-p`/`--pager`
    - Control line width with `-w`/`--width`
    - Search auto-navigates when it finds a single match
    - Fix topic resolution when k6 provisions the extension automatically

    ## New features

    ### Shell completions [243f99f](https://github.com/grafana/xk6-docs/commit/243f99f)

    Press Tab to complete topic names as you type. Works with zsh, bash, fish, and PowerShell after a one-time `k6 completion` setup.

    > [!NOTE]
    > Due to grafana/k6#5757, `k6 x docs` won't enable completions. However, using `xk6` to build `xk6-docs` enables them.

    ### Pager support [2da4247](https://github.com/grafana/xk6-docs/commit/2da4247)

    Scroll long pages in your pager with `-p`/`--pager`. Defaults to `less -r`, or honors `$PAGER`.

    ### Word-wrap control [4878d6f](https://github.com/grafana/xk6-docs/commit/4878d6f)

    Set the line width with `-w`/`--width`. Defaults to your terminal width, or 80 columns.

    ### Auto-navigate single search matches [047806a](https://github.com/grafana/xk6-docs/commit/047806a)

    When search finds results in a single group, you see the best match directly instead of a result list.

    ## Bug fixes

    - [12b54ac](https://github.com/grafana/xk6-docs/commit/12b54ac) Fix topic resolution when k6 provisions the extension automatically.

    ## Maintenance and internal improvements

    - [a23e957](https://github.com/grafana/xk6-docs/commit/a23e957) Replace three separate workflows with a single bundle sync that detects new k6 releases and stale bundles every 3 hours.
    - [b38b63f](https://github.com/grafana/xk6-docs/commit/b38b63f) Convert all tests to black-box via real k6 binary.
    - [c52c00a](https://github.com/grafana/xk6-docs/commit/c52c00a) Update Go to 1.25 and refresh dependencies.

## Step 4: Verify Go module proxy

    curl -s https://proxy.golang.org/github.com/grafana/xk6-docs/@v/v<VERSION>.info

If this returns JSON with the version and timestamp, the proxy has it. If not, wait and retry — it can take a few minutes.

## Step 5: Open the extension registry PR

Clone or checkout `grafana/k6-extension-registry`. Add the new version to the `versions` list under the `github.com/grafana/xk6-docs` entry in `registry.yaml`.

Create a PR on a feature branch:

    gh pr create --repo grafana/k6-extension-registry \
      --base main --head add-xk6-docs-v<VERSION> \
      --title "Add xk6-docs v<VERSION>" \
      --body "Add [v<VERSION>](https://github.com/grafana/xk6-docs/releases/tag/v<VERSION>) of \`xk6-docs\` to the registry."

The registry now requires review before merge — direct merge is no longer allowed by repo policy. Leave the PR open and ping a reviewer (or wait for an existing approver). The release is still usable via `xk6 build` immediately; provisioning via `k6 x docs` only updates after the PR is merged.

Example from `v0.0.5` (PR #191):

    Title: Add xk6-docs v0.0.5
    Body: Add [v0.0.5](https://github.com/grafana/xk6-docs/releases/tag/v0.0.5) of `xk6-docs` to the registry.

## Step 6: Post to Slack

Post to `#k6-changelog` using the slack skill. Only list features, not bug fixes or maintenance. Always wrap technical terms in backticks. Link each commit hash.

Treat architectural changes that expand who can use the project as features. New shared modules, new public APIs, new integration points — these are headline news, not "internal improvements." If `mcp-k6` or another consumer can do something new because of this release, lead with it.

Use plain Markdown — `[text](url)` for links, `*` for bullets. Do NOT use Slack's older `<url|text>` mrkdwn syntax or `-` bullets; the channel renders Markdown.

When passing the message to `reply.py`, use a single-quoted heredoc to avoid shell-escaping backticks. Backslash-escaped backticks (`` \` ``) leak through verbatim into the message body and look like garbage.

Format:

    :k6_party: *xk6-docs <VERSION> is released*

    > CLI k6 docs for AI agents and users.

    * Feature description ([short-hash](commit-url))
    * Feature description ([short-hash](commit-url))

    See the [complete release notes](https://github.com/grafana/xk6-docs/releases/tag/v<VERSION>).

Example from `v0.0.5`:

    :k6_party: *xk6-docs 0.0.5 is released*

    > CLI k6 docs for AI agents and users.

    * Press `Tab` to complete topic names as you type. Works with `zsh`, `bash`, `fish`, and PowerShell ([243f99f](https://github.com/grafana/xk6-docs/commit/243f99f))
    * Scroll long pages in your pager with `-p`/`--pager`. Defaults to `less -r`, or honors `$PAGER` ([2da4247](https://github.com/grafana/xk6-docs/commit/2da4247))
    * Set the line width with `-w`/`--width`. Defaults to your terminal width, or 80 columns ([4878d6f](https://github.com/grafana/xk6-docs/commit/4878d6f))
    * When search finds results in a single group, you see the best match directly instead of a result list ([047806a](https://github.com/grafana/xk6-docs/commit/047806a))

    See the [complete release notes](https://github.com/grafana/xk6-docs/releases/tag/v0.0.5).

## Checklist

Before telling the user the release is done, verify:

- [ ] CI passed on main
- [ ] Tag pushed and release workflow completed
- [ ] GitHub release has binaries + checksums + description
- [ ] Go module proxy has the version
- [ ] Registry PR opened (merge handled separately by a reviewer)
- [ ] Slack message posted to `#k6-changelog`
