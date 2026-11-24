# Add machine-readable JSON output to `k6 x docs`

This Plan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.claude/PLANS.md`.


## Purpose / Big Picture

Today `k6 x docs` prints Markdown. A human reads it; an AI agent has to swallow the whole page into its context to answer a narrow question ("what is the default for `maxVUs`?", "give me the runnable example", "what sections does this page have?"). That is wasteful: the agent pays for the entire page to extract three lines.

After this change, `k6 x docs` can emit the same content as structured JSON, so an agent slices out exactly what it needs with `jq` before anything reaches its context. Three things become possible that were not before:

First, surgical extraction. Instead of reading a whole page, an agent runs a filter:

    k6 x docs http get --json | jq -r '.examples[] | select(.primary).code'

and gets only the runnable script.

Second, cheap navigation. An agent can ask "what is on this page?" without downloading the page body:

    k6 x docs using-k6/scenarios/executors/constant-arrival-rate --outline --json | jq -r '.outline[].title'
    # When to use
    # Options
    # Example
    # Observations

Third, precise fetch. Having seen the outline, the agent pulls one section:

    k6 x docs using-k6/scenarios/executors/constant-arrival-rate --section Options --json | jq -r '.section.tables[0].rows[] | select(.Default=="-").Option'
    # duration
    # rate
    # preAllocatedVUs

To see it working end to end: build the extension binary (see Concrete Steps), then run any of the commands above against a cached bundle and observe the JSON. The acceptance tests in `internal/clitest/testdata/scripts/json.txtar` fail before this change (the `--json` flag does not exist) and pass after.

This plan is deliberately scoped to be additive and low risk: all JSON structuring happens at read time inside the CLI layer. No bundle format changes, no `sections.json` schema change, so every already-published and already-cached documentation bundle keeps working with the new binary.


## Progress

- [ ] (2026-07-03) Milestone 1: `--json` flag and the "doc" surface for a single topic (`internal/cli/docjson.go` + `docjson_test.go`, flag wiring in `cmd.go`, dispatch in `ui.go`).
- [ ] (2026-07-03) Milestone 2: read-time content parser — sections, tables, fenced code, admonitions/notes, options table (`internal/cli/docjson.go` parser functions + unit tests).
- [ ] (2026-07-03) Milestone 3: `--outline` and `--section <name>` granularity selectors on the doc surface.
- [ ] (2026-07-03) Milestone 4: "toc" surface (`k6 x docs --json`) and "search" surface (`k6 x docs search <q> --json`) with per-result `outline`.
- [ ] (2026-07-03) Milestone 5: `references` extraction (from pre-Transform content, resolved to slugs with an `exists` flag) and the `error` envelope for unresolved topics.
- [ ] (2026-07-03) Milestone 6: end-to-end acceptance script `testdata/scripts/json.txtar`; update `CLAUDE.md`, `.agents/features.md`, `AGENTS.md`, and the agent skill `internal/cli/skills/xk6-docs/SKILL.md`.


## Surprises & Discoveries

The JSON schema in this plan was not guessed. It was derived empirically: a first pass converted eight representative k6 pages to a rich "superset" JSON, then eight independent agents that had never seen the schema were each given a realistic query task (filter, group, chain, navigate, aggregate) and asked to solve it with `jq` and report friction. The findings below are the evidence behind the schema decisions; keep them here so a future contributor understands why the shape is what it is.

- Observation: An outline (just heading titles) costs almost nothing, but if headings are embedded inside heavy section objects, extracting them forces a full-page parse.
  Evidence: For `ramping-arrival-rate`, the `sections` array measured 9,544 bytes; the heading titles alone measured 89 bytes. That is a 107x overhead to answer "what is on this page." This is why `outline` is a separate, lightweight field and why `--outline` exists as its own selector.

- Observation: A single free-text `default` field is unusable for filtering because it conflates three different things.
  Evidence: In the probe, `timeUnit`'s default serialized as the string `"\"1s\""` (a value with quote characters baked in), `maxVUs`'s default was the sentence `"If unset, same as preAllocatedVUs"` (prose, not a value), and required options used `null`. A query like "list each optional option with its default" would return garbage. This is why `default` is a typed value (string, number, boolean, or null) and human explanations move to a separate `default_note`.

- Observation: An array of option objects forces every consumer to bind the parent title into a variable and scan, where a name-keyed map would be a direct lookup.
  Evidence: Four of the eight agents independently rewrote `options` into a map or asked for one; the recurring `jq` contortion was `.title as $t | .options[] | select(.name==...)`. This is why `options` is a JSON object keyed by option name, so the query is `.options.timeUnit.default`.

- Observation: Agents could not tell which page was an "executor" without pattern-matching the slug string, because `category` is too coarse (every page under `using-k6/` shares one category).
  Evidence: Two agents fell back to `select(.slug|test("scenarios/executors/"))`. This is why each doc carries a `doc_type` discriminator derived once, up front.

- Observation: "The example" and "first example" were positional guesses.
  Evidence: Agents used `.code_examples[0]` with no signal that entry zero is the canonical runnable script rather than a fragment. This is why each example carries a `primary` boolean.

- Observation: Admonitions were being shredded line by line and Markdown markers leaked into the text.
  Evidence: A blockquote note came out as several array entries such as `"**Note:** **Iteration starts are spaced fractionally.**"` followed by `"Iterations **do not** start..."`. This is why the notes parser must treat one blockquote as one note and strip inline markers.


## Decision Log

- Decision: Do all JSON structuring at read time in the CLI layer; do not change the bundle format or `sections.json`.
  Rationale: The CLI already reads stored Markdown and applies `docs.Transform` at read time (`internal/cli/bundle.go`, method `readAndTransform`). Everything the JSON needs can be computed from that Markdown plus the existing `docs.Section` index metadata. Changing `sections.json` would force a bundle migration and risk breaking already-published and already-cached bundles for no benefit. Additive read-time parsing is the minimal change.
  Date/Author: 2026-07-03

- Decision: The top-level `kind` field names the response *surface* ("doc", "toc", "search", "outline", "section", "error"); the per-document category discriminator is a separate field named `doc_type`.
  Rationale: The probe wanted both a response-type discriminator and a document-type discriminator. Using one word for both collides. `kind` = what shape of response this is; `doc_type` = what kind of page this documents.
  Date/Author: 2026-07-03

- Decision: `options` is a JSON object keyed by option name, not an array.
  Rationale: Four independent probe agents converged on this. It turns a scan-and-bind into a direct lookup `.options.<name>`. Order is not semantically meaningful for k6 option tables, so losing array order is acceptable. If display order is ever needed, add a sibling `option_order` array; do not reintroduce the array of objects.
  Date/Author: 2026-07-03

- Decision: `default` holds a typed JSON value or `null`; prose explanations go in `default_note`.
  Rationale: See Surprises. A typed `default` makes `select(.default==null)` a reliable "is required" test and lets numeric defaults compare numerically. Never double-encode strings.
  Date/Author: 2026-07-03

- Decision: `references` are extracted at read time from the content *before* `docs.Transform` strips links, resolved to slugs, and each carries an `exists` boolean.
  Rationale: `docs.Transform` (in the `docs` module) strips internal doc links to plain text, so by the time content is rendered the links are gone. But the CLI reads the raw stored bytes first and only then calls `docs.Transform`; capturing links from those raw bytes recovers them without a build-time index. `exists` is computed by checking the resolved slug against the loaded index (`idx.Lookup`), so an agent can tell live cross-links from dangling ones.
  Date/Author: 2026-07-03

- Decision: `--json` output is always plain (no ANSI styling) and never paged.
  Rationale: JSON is for machines and pipes. The existing `newWithDocs` closure buffers output and runs it through the `glamour` Markdown renderer when stdout is a TTY, and optionally through a pager. JSON must bypass both and be written directly to stdout so `jq` and redirection work in every environment.
  Date/Author: 2026-07-03

- Decision: `doc_type` is derived from the slug shape and the index `is_index` flag, using a small fixed mapping.
  Rationale: No new data is needed; the slug already encodes the section. The mapping (see Interfaces and Dependencies) turns "is this an executor page?" into `select(.doc_type=="executor")` without slug string matching by the consumer.
  Date/Author: 2026-07-03

- Decision: This plan does not implement a cross-document inverted index (for example option-name to list-of-docs).
  Rationale: Two probe agents asked for one, but the CLI serves a single document or a single query per invocation. A global index is a different feature with its own storage and staleness concerns. Keep this plan single-document.
  Date/Author: 2026-07-03

- Decision: `--section <name>` matching is case-insensitive and matches either the heading text or its anchor slug.
  Rationale: Agents will pass the human title ("When to use") or a slugified anchor ("when-to-use"); both should work. Anchors follow the GitHub convention already chosen for this repo's heading work (lowercase, spaces to dashes, strip punctuation).
  Date/Author: 2026-07-03


## Outcomes & Retrospective

(To be written at completion. Compare the delivered surfaces and `jq` ergonomics against the Purpose. Record any schema field that turned out unused or any query that still needed contortions.)


## Context and Orientation

This section assumes no prior knowledge of the repository.

The repository `xk6-docs` builds a k6 extension. Running `k6 x docs ...` prints offline k6 documentation. Documentation is not embedded in the binary; on first run the extension downloads a compressed bundle for the detected k6 version and caches it under `~/.local/share/k6/docs/<version>/`. A bundle contains `sections.json` (an index of every page) and `markdown/**/*.md` (the page bodies).

Terms used in this plan:

A "slug" is a page's path-like identifier, for example `javascript-api/k6-http/get` or `using-k6/scenarios/executors/constant-arrival-rate`. Users can type shorter forms ("http get") that the resolver expands to a slug.

A "surface" is one of the shapes of output this feature produces: the table of contents ("toc"), a single document ("doc"), a search result list ("search"), a lightweight document map ("outline"), a single document section ("section"), or an error ("error"). The JSON always carries a top-level `kind` field naming the surface.

An "envelope" is the outermost JSON object for a surface. Every envelope has at least `version` (the docs version, for example `v2.0.x`) and `kind`.

The Go code lives in two modules. The root module (`go.mod` at the repository root) contains everything under `internal/`. The `docs` module (`docs/go.mod`) contains the reusable documentation types and transforms. The root module depends on the `docs` module.

Key files and functions a contributor will touch or call:

`docs/types.go` defines `docs.Section`, the per-page index record. Its fields are `Slug`, `RelPath`, `Title`, `Description`, `Weight`, `Category`, `Children` (a list of child slugs, as strings), `IsIndex`, and `Aliases`. It also defines `docs.Index` (all sections for a version) and `docs.Tree` (a depth-limited node). This plan does not modify these types.

`docs/index.go` defines methods on `*docs.Index`: `Lookup(slug)` returns the `*Section` and a found boolean; `Children(slug)` returns child sections; `TopLevel()` returns the root categories; `ByCategory(category)` groups sections; `Tree(rootSlug, depth)` yields a depth-limited tree; `Search(term, readContent)` fuzzy-searches, calling the supplied `readContent` to read page bodies for scoring.

`docs/transform.go` defines `docs.Transform(content, version)`, which strips shortcodes, converts admonitions to blockquotes, and strips internal doc links to plain text. Crucially for this plan, link stripping happens here, at read time, not in the stored file. It also defines `docs.SplitFrontmatter(content)` returning the YAML block and the body.

`internal/cli/bundle.go` defines the `docsEnv` struct (`cat *docs.Catalog`, `idx *docs.Index`, `version string`, `depth int`) and the method `readAndTransform(ctx, slug)`. That method reads the raw stored bytes via `env.cat.Read(ctx, env.version, slug)` and then returns `docs.Transform(string(data), env.version)`. The raw bytes still contain links; the transformed string does not. This plan needs both: the transformed string for `content` and `sections`, and the raw bytes for `references`.

`internal/cli/ui.go` defines the current renderers: `showDocs(ctx, env, w, idx, args)` prints the TOC or a topic; `searchResults(idx, args, readContent)` computes matches; `printSearch(ctx, env, w, idx, args)` prints them; `printTree(...)` is the shared recursive tree printer used by TOC and section footers.

`internal/cli/cmd.go` builds the cobra command tree. `docsOpts` holds the flags (`version`, `source`, `cacheDir`, `depth`, `pager`, `width`). `registerFlags` registers them. `newWithDocs` is a closure that runs setup, buffers output for TTY/pager, invokes the surface function, and then flushes through `glamour` (via `renderMarkdown`) or a pager. `buildDocsCmd`'s `RunE` calls `showDocs`; `buildSearchCmd`'s `RunE` calls `printSearch`.

`internal/cli/resolve.go` resolves user args to a slug. `showDocs` already uses it; the JSON doc surface reuses the same resolution so `--json http get` behaves identically to `http get`.

`internal/clitest/testdata/scripts/*.txtar` are the end-to-end acceptance tests. Each is a scripted CLI session with expected output. `render.txtar`, `agent_mode.txtar`, and `config_env.txtar` are good models for a new `json.txtar`.


## The JSON schema, surface by surface

All examples are indented (not fenced) per the plan formatting rules. Field names shown are the contract.

The "doc" surface, `k6 x docs <topic> --json`:

    {
      "version": "v2.0.x",
      "kind": "doc",
      "slug": "using-k6/scenarios/executors/constant-arrival-rate",
      "doc_type": "executor",
      "title": "Constant arrival rate",
      "summary": "k6 starts a fixed number of iterations over a specified period of time.",
      "category": "using-k6",
      "is_index": false,
      "outline": [
        { "title": "When to use", "level": 2, "anchor": "when-to-use" },
        { "title": "Options",     "level": 2, "anchor": "options" }
      ],
      "children": [],
      "references": [
        { "text": "Open and Closed models", "slug": "using-k6/scenarios/concepts/open-vs-closed", "exists": true }
      ],
      "options": {
        "duration":        { "type": "string",  "required": true,  "default": null,  "description": "Total scenario duration (excluding gracefulStop)." },
        "timeUnit":        { "type": "string",  "required": false, "default": "1s",  "description": "Period of time to apply the rate value." },
        "maxVUs":          { "type": "integer", "required": false, "default": null,  "default_note": "If unset, same as preAllocatedVUs", "description": "Maximum number of VUs to allow during the test run." }
      },
      "examples": [
        { "lang": "javascript", "section": "Example", "primary": true, "code": "import http from 'k6/http';\n..." }
      ],
      "notes": [
        { "type": "note", "text": "Iteration starts are spaced fractionally. At a rate of 10 with a timeUnit of 1s, each iteration starts about every 100ms." }
      ],
      "sections": [
        { "title": "Options", "level": 2, "path": ["Options"], "anchor": "options", "body": "| Option | Type | ...", "tables": [ { "headers": ["Option","Type","Description","Default"], "rows": [ { "Option": "duration", "Type": "string", "Description": "...", "Default": "-" } ] } ], "code": [] }
      ],
      "content": "# Constant arrival rate\n\n..."
    }

The "outline" surface, `k6 x docs <topic> --outline --json`, is the doc surface with the heavy fields omitted (`sections`, `content`, `options`, `examples`, `notes`). It keeps `slug`, `doc_type`, `title`, `summary`, `outline`, `children`, `references`. This is the cheap "what is on this page" answer.

The "section" surface, `k6 x docs <topic> --section <name> --json`, returns exactly one section:

    {
      "version": "v2.0.x",
      "kind": "section",
      "slug": "using-k6/scenarios/executors/constant-arrival-rate",
      "section": { "title": "Options", "level": 2, "path": ["Options"], "anchor": "options", "body": "...", "tables": [ ... ], "code": [] }
    }

The "toc" surface, `k6 x docs --json`:

    {
      "version": "v2.0.x",
      "kind": "toc",
      "depth": 1,
      "tree": [
        { "slug": "using-k6", "title": "Using k6", "description": "...", "children": [ { "slug": "using-k6/scenarios", "title": "Scenarios", "description": "..." } ] }
      ]
    }

The "search" surface, `k6 x docs search <q> --json`, embeds each hit's outline so an agent can jump straight to the right section of the right page:

    {
      "version": "v2.0.x",
      "kind": "search",
      "query": "arrival rate",
      "results": [
        { "slug": "using-k6/scenarios/executors/constant-arrival-rate", "title": "Constant arrival rate", "category": "using-k6", "doc_type": "executor", "outline": [ { "title": "Options", "level": 2, "anchor": "options" } ] }
      ]
    }

The "error" surface, printed to stdout with a non-zero exit code when a topic does not resolve:

    { "version": "v2.0.x", "kind": "error", "code": "not_found", "input": "htp get", "message": "no topic matches \"htp get\"" }


## Plan of Work

The work is additive. A new file `internal/cli/docjson.go` holds the JSON model types, the read-time Markdown parser, and the surface builders. A companion `internal/cli/docjson_test.go` holds unit tests for the parser (these are pure functions over a Markdown string, easy to test in isolation). The command wiring in `internal/cli/cmd.go` and dispatch in `internal/cli/ui.go` are small edits.

Milestone 1 adds the flag and the simplest surface so there is something to run. In `cmd.go`, extend `docsOpts` with `json bool`, `outline bool`, and `section string`, and register them in `registerFlags` (`--json`, `--outline`, `--section`). Then change `newWithDocs` so that when `opts.json` is set it does not create the TTY buffer and does not page: it writes whatever the surface function produced directly to `rt.Stdout`. Concretely, guard the `buf` creation and the flush block with `!opts.json`. In `buildDocsCmd`'s `RunE`, when `opts.json` is set, call a new `showDocsJSON(cmd.Context(), env, w, env.idx, args, opts)` instead of `showDocs`. For Milestone 1, `showDocsJSON` can resolve the slug (reuse the resolution `showDocs` uses from `resolve.go`), read the transformed content via `env.readAndTransform`, look up the section metadata via `idx.Lookup`, and emit a doc envelope whose `sections`, `options`, `examples`, and `notes` are still empty. That already proves the flag, the plain-output path, the envelope, `slug`, `doc_type`, `title`, `summary`, `children`, and `content` end to end.

Milestone 2 fills in the parser. Add to `docjson.go` a set of pure functions that take the transformed Markdown body and return the structured pieces: `parseSections(body)` splits on ATX headings (`##`, `###`, ...) into section records, tracking whether the current line is inside a fenced code block so heading-like lines inside code are ignored (real k6 docs contain many such lines); `parseTable(sectionBody)` turns a GitHub-style pipe table into `{headers, rows}` where each row is an object keyed by header; `parseCode(body)` collects fenced code blocks with their language and the nearest preceding heading as `section`, marking the first complete script as `primary`; `parseNotes(body)` collects each blockquote as one note, deriving `type` from a leading `**Note:**`/`**Warning:**`/`**Caution:**` label and stripping inline Markdown markers from the text; `parseOptions(sections)` finds a section titled "Options" (or "Parameters") whose table's first column is `Option` (or `Parameter`) and builds the `options` map, applying the default-typing rules below. The `summary` is the leading paragraph text after the H1 up to the first heading, table, code fence, or blockquote. Unit-test each function against small literal Markdown inputs and against one real page fetched into a testdata fixture.

The default-typing rules in `parseOptions`: a `Default` cell of `-`, empty, or an `Option` cell containing `(required)` means `required: true` and `default: null`; a cell that is a JSON-parseable number becomes that number; `[]` becomes an empty array; a bare token such as `1s` or `10m` becomes that string with any surrounding quotes removed; a cell that is a full sentence (contains a space and reads as prose, for example begins with a capitalized word and is not a known token) becomes `default: null` with the sentence stored in `default_note`.

Milestone 3 adds the granularity selectors. In `showDocsJSON`, if `opts.section != ""`, build only the matching section (case-insensitive match on heading text or anchor) and emit the "section" surface; if no section matches, emit an "error" envelope with `code: "section_not_found"`. If `opts.outline` is set, emit the "outline" surface (omit the heavy fields). Otherwise emit the full "doc" surface.

Milestone 4 adds the toc and search surfaces. For toc, when `opts.json` is set and no args are given, walk `idx.Tree(root, env.depth)` (the same tree `showDocs` prints) into `tocNode` records and emit the "toc" envelope. For search, add a `printSearchJSON` path in `buildSearchCmd`'s `RunE` (guarded by `opts.json`): reuse `searchResults(idx, args, readContent)`, then for each hit parse its content's headings into `outline` and emit the "search" envelope.

Milestone 5 adds `references`. In `showDocsJSON`, read the raw pre-Transform bytes (`env.cat.Read(ctx, env.version, slug)`), scan for Markdown links `[text](target)` where `target` is an internal doc reference (a site-relative or version-relative doc path, not an external URL and not an image), normalize each `target` to a slug, set `exists` by `idx.Lookup(slug)`, and attach the list as `references`. Add the "error" envelope for the top-level unresolved-topic case in `showDocsJSON` and give it exit code 1 (return an error from `RunE` after writing the JSON, or set the command's exit code explicitly; see Concrete Steps for how the existing code signals failure).

Milestone 6 writes the acceptance script and updates docs. Create `internal/clitest/testdata/scripts/json.txtar` exercising: a doc fetch piped through `jq`, an `--outline` fetch, a `--section` fetch, a toc fetch, a search fetch, and an unresolved-topic error with a non-zero exit. Update `CLAUDE.md` (a new "JSON output" subsection under Browsing), `.agents/features.md` (user-facing feature entry), `AGENTS.md`, and the agent skill `internal/cli/skills/xk6-docs/SKILL.md` so agents learn the `--json` workflow and the discover-then-slice pattern.


## Concrete Steps

All commands run from the repository checkout. This work lives on the branch `feat/json-output`; it was authored in a dedicated git worktree so the `main` checkout stays clean. List worktrees and enter the feature worktree (substitute your own paths):

    git worktree list          # shows main plus the feat/json-output worktree
    cd <path-to-feat/json-output-worktree>

Build the extension binary that bundles this extension into k6 (the repo uses xk6; the produced binary is `./k6`):

    make build
    # or, if there is no such target, the repository's documented build command; the result is a ./k6 that includes this extension.

Run the tests for the CLI package as you go:

    go test ./internal/cli/...

Run a single acceptance script while iterating:

    go test ./internal/clitest/ -run TestScripts/json

Manual end-to-end checks once Milestone 5 is in (expected output shown so success is unambiguous):

    ./k6 x docs http get --json | jq -r '.kind, .slug, (.examples | length)'
    # doc
    # javascript-api/k6-http/get
    # 1

    ./k6 x docs using-k6/scenarios/executors/constant-arrival-rate --json | jq -r '.options | keys[]'
    # duration
    # maxVUs
    # preAllocatedVUs
    # rate
    # timeUnit

    ./k6 x docs using-k6/scenarios/executors/constant-arrival-rate --json | jq -r '.options.timeUnit.default'
    # 1s

    ./k6 x docs using-k6/scenarios/executors/constant-arrival-rate --outline --json | jq -r '.outline[].title'
    # When to use
    # Options
    # Example
    # Observations

    ./k6 x docs using-k6/scenarios/executors/constant-arrival-rate --section Options --json | jq -r '.section.tables[0].rows[] | select(.Default=="-").Option'
    # duration
    # rate
    # preAllocatedVUs

    ./k6 x docs search "arrival rate" --json | jq -r '.results[] | .slug'
    # using-k6/scenarios/executors/constant-arrival-rate
    # using-k6/scenarios/executors/ramping-arrival-rate

    ./k6 x docs htp get --json; echo "exit=$?"
    # {"version":"v2.0.x","kind":"error","code":"not_found","input":"htp get","message":"no topic matches \"htp get\""}
    # exit=1

Run the linter before every commit (the repo requires a clean `golangci-lint`):

    golangci-lint run ./...

Commit frequently on the `feat/json-output` branch. Never commit to `main`.


## Validation and Acceptance

Acceptance is behavioral, not structural. Each item below names an observable behavior, the command that exercises it, and the expected result.

The `--json` flag produces valid JSON. Piping any `--json` invocation through `jq .` exits zero and reprints the object; a malformed object would make `jq` exit non-zero. The acceptance script asserts `jq .` succeeds on every surface.

The doc surface answers narrow questions without the whole page. `./k6 x docs http get --json | jq -r '.examples[] | select(.primary).code'` prints only the runnable script, with real newlines, and nothing else.

Typed defaults work as filters. `... constant-arrival-rate --json | jq -r '.options[] | select(.default==null) | . ' ` and the keyed lookup `.options.timeUnit.default == "1s"` both hold; no default value contains embedded quote characters and no default holds a prose sentence.

The outline is cheap and complete. `--outline --json` output has no `sections`, `content`, `options`, `examples`, or `notes` keys, but its `outline` lists every real heading in document order.

Section fetch is exact. `--section Options --json` returns one section whose `title` is "Options"; `--section when-to-use --json` (anchor form) returns the "When to use" section; `--section nonexistent --json` returns an error envelope with `code: "section_not_found"` and a non-zero exit.

Search embeds outlines. Every `results[]` entry has a non-empty `outline` for pages that have headings, and `slug`/`doc_type` are present.

References resolve and flag existence. In `http get --json`, `references[]` entries have a `slug` and an `exists` boolean; at least one resolves to a real page (`exists: true`), verified by `jq -e '.references[] | select(.exists)'` exiting zero.

Errors are machine-readable and fail loudly. An unresolved topic with `--json` prints an `error` envelope and exits non-zero, so a script can branch on the exit code.

The test file must fail before the change and pass after. Before implementing, `go test ./internal/clitest/ -run TestScripts/json` fails because `--json` is an unknown flag. After Milestone 6 it passes. Run the full suite `go test ./...` and expect it green, with the new `json.txtar` among the passing scripts.


## Idempotence and Recovery

Every step is additive and repeatable. The new file `internal/cli/docjson.go` can be rewritten wholesale without affecting other files. The flag registration and the `newWithDocs`/`RunE` edits are small and reversible; if a change misbehaves, revert those hunks and the Markdown path is unchanged because the JSON path is entered only when `--json` is set.

Because nothing in the bundle format or `sections.json` changes, there is no data migration and no cache to invalidate. A partially implemented feature simply means fewer fields are populated; the Markdown behavior is untouched throughout.

The work lives on the `feat/json-output` branch in a dedicated worktree. To abandon it safely without touching `main`, remove the worktree and delete the branch:

    git worktree remove <path-to-feat/json-output-worktree>
    git branch -D feat/json-output

To resume, re-add the worktree on the branch and continue.


## Artifacts and Notes

The schema was derived from an empirical probe rather than guessed. Several representative pages (a function page, an executor page, a module index, a concept page) were converted to a superset JSON, then fresh agents that had never seen the format were each handed a realistic query task and asked to solve it with `jq` and report where the shape fought them. Their friction produced the decisions in the Decision Log and the schema below. The probe dataset was throwaway scratch and is not part of the repository; the JSON shown throughout this plan is hand-written illustration, small and simplified to show the shape rather than scraped output.

An illustrative before/after that drove the separate `outline` field: an agent that only wants the section titles of a page should not have to load the section bodies. A heavy section entry looks like

    { "title": "Options", "level": 2, "body": "| Option | Type | ... (hundreds of bytes) ...", "tables": [ ... ], "code": [] }

while the outline entry for the same heading is just

    { "title": "Options", "level": 2, "anchor": "options" }

On a mid-sized page the full `sections` array is on the order of kilobytes while the heading titles alone are well under a hundred bytes, roughly a hundredfold difference. That gap is the whole justification for a separate lightweight `outline` field and the `--outline` selector.


## Interfaces and Dependencies

Use only the standard library plus what the repository already imports. JSON is produced with `encoding/json` (`json.NewEncoder(w).Encode(v)` writes directly to the plain output writer). The Markdown parser is line-based and fence-aware, consistent with the existing regex-based transforms in `docs/transform.go`; do not add a Markdown AST library. The root module already depends on `github.com/yuin/goldmark` transitively via `glamour`, but this plan does not use it, to keep the parser small and predictable and to avoid coupling the JSON shape to a third-party AST.

In `internal/cli/docjson.go`, define the model types (field tags shown are the JSON contract):

    type jsonDoc struct {
        Version    string                `json:"version"`
        Kind       string                `json:"kind"` // "doc"
        Slug       string                `json:"slug"`
        DocType    string                `json:"doc_type"`
        Title      string                `json:"title"`
        Summary    string                `json:"summary"`
        Category   string                `json:"category"`
        IsIndex    bool                  `json:"is_index"`
        Outline    []jsonHeading         `json:"outline"`
        Children   []jsonChild           `json:"children"`
        References []jsonReference       `json:"references"`
        Options    map[string]jsonOption `json:"options,omitempty"`
        Examples   []jsonExample         `json:"examples,omitempty"`
        Notes      []jsonNote            `json:"notes,omitempty"`
        Sections   []jsonSection         `json:"sections,omitempty"`
        Content    string                `json:"content,omitempty"`
    }

    type jsonHeading struct {
        Title  string `json:"title"`
        Level  int    `json:"level"`
        Anchor string `json:"anchor"`
    }

    type jsonChild struct {
        Slug        string `json:"slug"`
        Title       string `json:"title"`
        Description string `json:"description"`
    }

    type jsonReference struct {
        Text   string `json:"text"`
        Slug   string `json:"slug"`
        Exists bool   `json:"exists"`
    }

    type jsonOption struct {
        Type        string `json:"type"`
        Required    bool   `json:"required"`
        Default     any    `json:"default"` // string | float64 | bool | []any | nil
        DefaultNote string `json:"default_note,omitempty"`
        Description string `json:"description"`
    }

    type jsonExample struct {
        Lang    string `json:"lang"`
        Section string `json:"section"`
        Primary bool   `json:"primary"`
        Code    string `json:"code"`
    }

    type jsonNote struct {
        Type string `json:"type"`
        Text string `json:"text"`
    }

    type jsonTable struct {
        Headers []string            `json:"headers"`
        Rows    []map[string]string `json:"rows"`
    }

    type jsonSection struct {
        Title  string      `json:"title"`
        Level  int         `json:"level"`
        Path   []string    `json:"path"`
        Anchor string      `json:"anchor"`
        Body   string      `json:"body"`
        Tables []jsonTable `json:"tables"`
        Code   []jsonCode  `json:"code"`
    }

    type jsonCode struct {
        Lang string `json:"lang"`
        Code string `json:"code"`
    }

    type jsonOutline struct {
        Version    string          `json:"version"`
        Kind       string          `json:"kind"` // "outline"
        Slug       string          `json:"slug"`
        DocType    string          `json:"doc_type"`
        Title      string          `json:"title"`
        Summary    string          `json:"summary"`
        Outline    []jsonHeading   `json:"outline"`
        Children   []jsonChild     `json:"children"`
        References []jsonReference `json:"references"`
    }

    type jsonSectionResp struct {
        Version string      `json:"version"`
        Kind    string      `json:"kind"` // "section"
        Slug    string      `json:"slug"`
        Section jsonSection `json:"section"`
    }

    type jsonTOC struct {
        Version string        `json:"version"`
        Kind    string        `json:"kind"` // "toc"
        Depth   int           `json:"depth"`
        Tree    []jsonTOCNode `json:"tree"`
    }

    type jsonTOCNode struct {
        Slug        string        `json:"slug"`
        Title       string        `json:"title"`
        Description string        `json:"description"`
        Children    []jsonTOCNode `json:"children,omitempty"`
    }

    type jsonSearch struct {
        Version string          `json:"version"`
        Kind    string          `json:"kind"` // "search"
        Query   string          `json:"query"`
        Results []jsonSearchHit `json:"results"`
    }

    type jsonSearchHit struct {
        Slug     string        `json:"slug"`
        Title    string        `json:"title"`
        Category string        `json:"category"`
        DocType  string        `json:"doc_type"`
        Outline  []jsonHeading `json:"outline"`
    }

    type jsonError struct {
        Version string `json:"version"`
        Kind    string `json:"kind"` // "error"
        Code    string `json:"code"`
        Input   string `json:"input"`
        Message string `json:"message"`
    }

Surface builders and parser functions to define in the same file:

    // showDocsJSON is the JSON counterpart of showDocs. It dispatches to the
    // outline, section, or full doc surface based on opts, and to the toc
    // surface when args is empty. It writes JSON to w (the plain writer).
    func showDocsJSON(ctx context.Context, env *docsEnv, w io.Writer, idx *docs.Index, args []string, opts *docsOpts) error

    // printSearchJSON is the JSON counterpart of printSearch.
    func printSearchJSON(ctx context.Context, env *docsEnv, w io.Writer, idx *docs.Index, args []string) error

    // docTypeFor derives the doc_type discriminator from the slug and is_index.
    func docTypeFor(slug string, isIndex bool) string

    // parseSections splits transformed markdown into sections (fence-aware).
    func parseSections(body string) []jsonSection

    // parseTable parses a single GitHub pipe table into headers and header-keyed rows.
    func parseTable(md string) (jsonTable, bool)

    // parseCode collects fenced code blocks with language and nearest heading.
    func parseCode(body string) []jsonExample

    // parseNotes collects each blockquote as one note with a derived type.
    func parseNotes(body string) []jsonNote

    // parseOptions builds the options map from an "Options"/"Parameters" table.
    func parseOptions(sections []jsonSection) map[string]jsonOption

    // extractReferences scans pre-transform markdown for internal doc links,
    // resolves them to slugs, and flags existence against idx.
    func extractReferences(rawMarkdown string, idx *docs.Index) []jsonReference

    // anchorFor slugifies a heading title (GitHub convention: lowercase,
    // spaces to dashes, strip punctuation).
    func anchorFor(title string) string

The `docTypeFor` mapping is fixed: if `isIndex` then `"index"`; else if the slug contains `scenarios/executors/` then `"executor"`; else if it contains `scenarios/concepts/` then `"concept"`; else if it starts with `javascript-api/` then `"api"`; else if it starts with `examples/` then `"example"`; else if it starts with `testing-guides/` or `get-started` then `"guide"`; else `"page"`.


## Note on revisions

Initial draft, 2026-07-03. Establishes the read-time, CLI-only architecture (no bundle or `sections.json` change), the six surfaces (doc, outline, section, toc, search, error), and the empirically derived schema (options as a name-keyed map, typed `default` plus `default_note`, `doc_type` discriminator, separate lightweight `outline`, `primary` example flag, one-note-per-admonition, `references` with `exists`). The schema decisions trace to the blind-query probe recorded in Surprises & Discoveries; change them only with new evidence, and record that evidence here.
