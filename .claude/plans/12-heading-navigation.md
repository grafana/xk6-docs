# Navigate to specific headings within documentation pages

This Plan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.claude/PLANS.md`.


## Purpose / Big Picture

After this change, users can jump directly to a specific heading within a documentation page. Today, `k6 x docs http expected-statuses` prints the entire page. After this change, `k6 x docs http expected-statuses example` prints only the "### Example" section — from that heading to the next heading of equal or higher level (or end of file). Tab completion also suggests heading names when a user has navigated to a leaf page and presses Tab.

To see it working: type `k6 x docs http expected-statuses example` and see only the Example section. Type `k6 x docs http expected-statuses <TAB>` and see heading slugs like `returns`, `example`. For pages with duplicate headings (like `testing-guides/automated-performance-testing` which has two `### Example plan` headings), the first gets slug `example-plan` and the second gets `example-plan-1`, following GitHub's disambiguation convention.


## Progress

- [ ] Milestone 1: Heading slug generation and content extraction (`heading.go` + `heading_test.go`)
- [ ] Milestone 2: Integrate heading filter into `showDocs` (`docs.go` changes + test)
- [ ] Milestone 3: Heading completions for leaf sections (`completion.go` changes + test)
- [ ] Milestone 4: Integration tests (`testdata/scripts/heading.txtar`)
- [ ] Milestone 5: Update AGENTS.md


## Surprises & Discoveries

(None yet — to be filled during implementation.)


## Decision Log

- Decision: Use GitHub-style heading slug generation (lowercase, spaces to dashes, strip non-alphanumeric except dashes, append `-1`/`-2` for duplicates).
  Rationale: This is the most widely understood heading anchor convention. Users familiar with GitHub README anchors will find it intuitive. It handles the duplicate heading problem cleanly — first occurrence gets the base slug, second gets `-1`, third gets `-2`, etc.
  Date/Author: 2026-03-25

- Decision: Resolve heading filter by peeling the last arg when full resolution fails.
  Rationale: The current flow is: `ResolveWithLookup(args, exists)` returns a slug or fails. For heading navigation, if the full args don't resolve, we try `ResolveWithLookup(args[:len(args)-1], exists)`. If the shorter slug resolves, the peeled-off last arg is treated as a heading filter. This naturally gives section children priority over headings — if `example` is both a child section and a heading, the child wins because the full args resolve first.
  Date/Author: 2026-03-25

- Decision: Heading completions only appear for leaf sections (sections with no children in the index).
  Rationale: Sections with children already show child completions via `completionDeeper`. Mixing heading slugs with child slugs would be confusing — the user wouldn't know which are navigable sections vs. in-page anchors. Leaf sections have no children, so heading completions fill a gap that currently returns nothing.
  Date/Author: 2026-03-25

- Decision: Put heading logic in a new file `heading.go` rather than adding to `docs.go` or `transform.go`.
  Rationale: Heading parsing, slug generation, and content extraction are a distinct concern. Keeping them in their own file makes testing easier and avoids bloating `docs.go` (display logic) or `transform.go` (markdown cleanup). The test file `heading_test.go` can test the heading functions in isolation.
  Date/Author: 2026-03-25

- Decision: When displaying a heading-filtered section, do NOT append the subtopics footer.
  Rationale: The subtopics footer shows children of the section. When a user has drilled into a specific heading, they want just that content. The footer would be noise — it lists the same children they already navigated past.
  Date/Author: 2026-03-25

- Decision: The heading content extraction includes the heading line itself.
  Rationale: Printing `### Example` followed by the content under it gives the user context about what they're reading. Stripping the heading line would leave orphaned content with no title.
  Date/Author: 2026-03-25

- Decision: `ParseHeadings` must skip lines inside fenced code blocks (triple backticks).
  Rationale: Real k6 docs contain 114 heading-like lines inside code fences (shell comments like `# install the k6 types`, etc.). Without fence-awareness, heading lists would be polluted with false positives and heading navigation could extract the wrong content.
  Date/Author: 2026-03-25

- Decision: Headings that slugify to empty string are silently excluded.
  Rationale: Headings like `## ---` produce an empty slug after stripping non-alphanumeric characters. Including them would allow matching on "" which is nonsensical. Silently skipping is cleaner than erroring.
  Date/Author: 2026-03-25


## Edge Cases

These edge cases must be handled and tested. They are referenced in the milestones below where relevant.

**Code fences (CRITICAL):** `ParseHeadings` must skip lines inside fenced code blocks (triple backticks). Real k6 docs contain 114 heading-like lines inside code fences (e.g., `# create a package.json file` inside a bash snippet in `set-up/configure-your-code-editor`). Without this, heading lists would be polluted with false positives. Implementation: track a boolean `inFence` state. When a line starts with triple backticks, toggle it. Only match heading patterns when `inFence` is false.

**Empty slugs:** Headings like `## ---` or `## ***` slugify to an empty string (all characters stripped). `ParseHeadings` must skip headings whose slug is empty after slugification. Otherwise `HeadingSlugs` returns empty strings and `ExtractSection` could match on "".

**Title heading (h1):** Most pages have a single `# Title` heading at line 0. This heading IS included in `ParseHeadings` output and IS navigable. It's rarely useful but excluding it would add special-case logic for no gain. If a user types `k6 x docs http get get-url-params`, the h1 slug `get-url-params` is a valid target.

**Non-leaf sections with headings:** Sections with children (like `http/response` which has children `json`, `html`, etc.) also have headings in their own content (e.g., `### Example`). Heading navigation works for these via the args-peeling fallback — `k6 x docs http response example` fails full resolution, peels to `http response`, resolves, and uses `example` as heading filter. However, completions do NOT offer headings for non-leaf sections — only child names — because mixing the two would be confusing.

**Child name shadows heading slug:** If a section has a child whose name matches a heading slug in the parent's content (e.g., parent has child "json" AND heading "## JSON"), the child always wins. The full args resolve successfully, so the peeling fallback never triggers. This is the correct behavior — section navigation takes priority.

**Normalize args before peeling:** `showDocs` must call `normalizeArgs(args)` before peeling the last element. Otherwise `k6 x docs http/expected-statuses/example` passes `["http/expected-statuses/example"]` to `ResolveWithLookup` which normalizes internally, but the `len(args) >= 2` check fails because the raw args array has length 1. After normalization: `["http", "expected-statuses", "example"]` — peeling works correctly.

**Single-arg heading filter:** If the user types `k6 x docs example`, this is a single arg. After normalization it's still `["example"]`. Full resolution fails, but `len(args) >= 2` is false, so peeling is skipped. This falls through to the existing "topic not found" error. This is correct — heading navigation requires at least a section + a heading.

**Page with no subheadings:** Some leaf pages have only an h1 title and body text, no `##` or `###` headings. `HeadingSlugs` returns only the h1 slug (e.g., `["expected-statuses-statuses"]`). Completions would offer just that one slug, which is the title itself — not very useful but harmless. This is acceptable.

**Three or more duplicate headings:** The dedup suffix scheme handles any number of duplicates: first occurrence has no suffix, second gets `-1`, third gets `-2`, etc. This must be tested with at least 3 duplicates.

**Heading navigation on search command:** The search command (`k6 x docs search <query>`) is NOT affected by heading navigation. `runSearch` has its own flow that either shows exact-match docs or prints search results. No changes needed there. The heading filter only applies through `showDocs`.

**Slash in heading filter arg:** If a user types `k6 x docs using-k6 thresholds threshold-syntax/foo`, `normalizeArgs` splits this into `["using-k6", "thresholds", "threshold-syntax", "foo"]`. Peeling gives heading filter `"foo"`, which won't match any heading. The user gets a "heading not found" error. This is acceptable — headings never contain slashes.


## Outcomes & Retrospective

(To be filled at completion.)


## Context and Orientation

This is an extension for k6 (the load testing tool) that provides offline documentation browsing via `k6 x docs`. The extension lives in the `docs` Go package at the repository root.

A "section" is a documentation page stored in the bundle's `sections.json` index. Each section has a slug (like `javascript-api/k6-http/expected-statuses`), a title, optional children (sub-pages), and a markdown file on disk. A "heading" is a markdown heading line (`## Foo`, `### Bar`) within a section's markdown content. Headings are not in the index — they exist only in the raw markdown files.

A "slug" is a URL-friendly identifier. Section slugs come from the index. Heading slugs are generated at runtime from the heading text using GitHub-style rules (lowercase, spaces become dashes, special characters stripped, duplicates get `-1`/`-2` suffixes).

A "leaf section" is a section with no children in the index (its `Children` field is empty or nil). Non-leaf sections have child sections that are already navigable.

The "transform pipeline" is the function `Transform(content, version)` in `transform.go`. It cleans up raw markdown by stripping frontmatter, shortcodes, Hugo tags, and converting admonitions. After transformation, markdown headings (`## ...`, `### ...`) are preserved as plain markdown lines. Heading extraction must run AFTER the transform pipeline, because the raw markdown may contain Hugo shortcodes that interfere with heading detection.

Key files and their roles:

- `docs.go` — Display logic. `showDocs()` is the entry point. It resolves CLI args to a slug, looks up the section in the index, and calls `printSection()`. `printSection()` reads the markdown file from the cache, runs the transform pipeline, renders the content, and optionally appends a subtopics footer (a bullet tree of children). `printSection` signature: `func printSection(env *docsEnv, w io.Writer, idx *Index, sec Section)`.
- `resolve.go` — `ResolveWithLookup(args []string, exists func(string) bool) string` converts CLI args into a canonical slug. `normalizeArgs(args []string) []string` splits slash-separated args into individual segments. Fallbacks include `withK6Prefix` (inserts `k6-` on JS API slugs) and `withParentFallback` (retries `parent/child` as `parent/parent-child`).
- `completion.go` — `completionTopicArgs(idx *Index, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective)` dispatches to `completionFirstArg` (zero args) or `completionDeeper` (one or more args). `completionDeeper` resolves the slug from args, gets children, and returns their short names filtered by prefix. `setupForCompletion(gs *state.GlobalState, opts *docsOpts) *Index` is a lightweight setup that loads the index from cache without network I/O.
- `sections.go` — `Section` struct with fields `Slug`, `RelPath`, `Title`, `Description`, `Weight`, `Category`, `Children`, `IsIndex`. `Index` struct with methods `Lookup(slug) (Section, bool)`, `Children(slug) []Section`, `TopLevel() []Section`, `Search(query) []Section`.
- `cmd.go` — `newDocsCmd()` builds the cobra command tree. `runDocs()` calls `prepareRun()` then `showDocs()`. `prepareRun()` sets up `docsEnv` (version, cache dir, depth, color settings, buffered rendering).
- `transform.go` — `Transform(content string, version string) string` applies markdown cleanup.
- `render.go` — `renderMarkdown(content string, ...) string` uses glamour for TTY rendering.
- `completion_test.go` — Unit tests for completion logic. Uses in-memory index fixtures.
- `testdata/scripts/` — Integration tests using `testscript` `.txtar` format. `TestScripts` in `docs_test.go` drives them.

The `docsEnv` struct (defined in `cmd.go`) holds runtime settings including `Version`, `Depth`, `CacheDir`, color/TTY state, and a buffered writer. It is passed to display functions like `printSection`.

The cobra command tree:

    k6 (root)
      x (subcommand group)
        docs [topic] [subtopic...] [heading]    <- our extension
          search <term>
          skill [directory]

Testing uses two approaches: (1) `testscript` `.txtar` files in `testdata/scripts/` for CLI integration tests, driven by `TestScripts` in `docs_test.go`; (2) standard Go unit tests for logic functions. The project enforces TDD — write a failing test first, then implement.


## Plan of Work

The work is split into five milestones. Each milestone is independently verifiable.


### Milestone 1: Heading slug generation and content extraction

This milestone creates the `heading.go` file with pure functions for parsing headings from markdown, generating GitHub-style slugs, and extracting content for a specific heading. At the end of this milestone, `heading_test.go` passes all tests and the functions are ready for integration.

Create a new file `heading.go` in the repository root (the `docs` package). This file contains three exported functions and one unexported helper:

`Heading` is a struct representing a parsed heading:

    type Heading struct {
        Level int    // 1 for #, 2 for ##, 3 for ###, etc.
        Text  string // the heading text after the # marks and space
        Slug  string // the generated slug (GitHub-style, with dedup suffixes)
        Line  int    // 0-based line index in the content
    }

`ParseHeadings(content string) []Heading` scans the transformed markdown content line by line. It tracks whether it is inside a fenced code block (lines starting with triple backticks toggle an `inFence` boolean). Lines inside code fences are skipped — see the "Code fences" edge case above. For each non-fenced line matching the pattern `^(#{1,6})\s+(.+)$`, it extracts the level (number of `#` characters) and the text (everything after the `# ` prefix, trimmed). It generates a slug for each heading using `slugify`. If the slug is empty after slugification (see "Empty slugs" edge case), the heading is skipped. It tracks duplicate slugs: the first occurrence gets the base slug, the second gets `-1` appended, the third gets `-2`, and so on. It returns the headings in document order.

`slugify(text string) string` is an unexported helper. It lowercases the text, replaces spaces with dashes, strips all characters that are not lowercase letters, digits, or dashes, and collapses consecutive dashes into one. It trims leading and trailing dashes.

`ExtractSection(content string, headingSlug string) (string, bool)` finds the heading matching `headingSlug` in the content. It calls `ParseHeadings` to get all headings with their slugs and line numbers. It finds the first heading whose slug matches. Then it extracts content starting from that heading's line (inclusive) up to (but not including) the next heading of the same level or higher (fewer or equal `#` marks), or end of content if no such heading follows. It returns the extracted text and `true`, or empty string and `false` if the slug was not found.

`HeadingSlugs(content string) []string` is a convenience function that calls `ParseHeadings` and returns just the slugs. This is used by the completion logic.

Create `heading_test.go` with the following test cases. Use TDD: write each test, see it fail, implement just enough to pass, repeat.

Test `TestSlugify`: verify the `slugify` helper converts heading text to slugs correctly.

    Input: "Returns"           -> Expected: "returns"
    Input: "Example"           -> Expected: "example"
    Input: "Some Long Title"   -> Expected: "some-long-title"
    Input: "request( method, url, [body], [params] )"
                               -> Expected: "request-method-url-body-params"
    Input: "A percentile of requests finishes in a specified duration"
                               -> Expected: "a-percentile-of-requests-finishes-in-a-specified-duration"
    Input: ""                  -> Expected: ""

Test `TestParseHeadings`: verify heading parsing with duplicate handling.

    Input (multi-line string):
        # Top
        ## Section One
        ### Example plan
        Some content here.
        ### Another
        ## Section Two
        ### Example plan

    Expected headings:
        {Level: 1, Text: "Top",          Slug: "top",            Line: 0}
        {Level: 2, Text: "Section One",  Slug: "section-one",    Line: 1}
        {Level: 3, Text: "Example plan", Slug: "example-plan",   Line: 2}
        {Level: 3, Text: "Another",      Slug: "another",        Line: 4}
        {Level: 2, Text: "Section Two",  Slug: "section-two",    Line: 5}
        {Level: 3, Text: "Example plan", Slug: "example-plan-1", Line: 6}

Test `TestParseHeadingsNoDuplicates`: verify that when all headings are unique, no suffixes are added.

Test `TestExtractSection`: verify content extraction.

    Input content (multi-line):
        # Title
        Intro text.
        ## First
        First content.
        More first content.
        ## Second
        Second content.
        ### Sub
        Sub content.
        ## Third
        Third content.

    Case 1: headingSlug="first" -> extracts lines from "## First" through "More first content." (stops before "## Second" because it's same level).
    Case 2: headingSlug="second" -> extracts "## Second", "Second content.", "### Sub", "Sub content." (includes the ### Sub because it's lower level; stops before "## Third").
    Case 3: headingSlug="sub" -> extracts "### Sub", "Sub content." (stops before "## Third" which is higher level).
    Case 4: headingSlug="third" -> extracts "## Third", "Third content." (runs to EOF).
    Case 5: headingSlug="nonexistent" -> returns "", false.

Test `TestExtractSectionDuplicateHeadings`: verify that `example-plan` gets the first occurrence and `example-plan-1` gets the second.

Test `TestHeadingSlugs`: verify it returns just the slug strings in order.

Test `TestParseHeadingsSkipsCodeFences`: verify that heading-like lines inside fenced code blocks are ignored. Input:

    ## Real heading
    ```bash
    # This is a comment, not a heading
    ## Also not a heading
    ```
    ## Another real heading

Expected: only two headings (`real-heading` and `another-real-heading`). The two lines inside the code fence must not appear.

Test `TestParseHeadingsSkipsEmptySlugs`: verify that headings like `## ---` or `## ***` (which slugify to empty string) are excluded from the results.

Test `TestParseHeadingsThreeDuplicates`: verify dedup with three identical headings. Input:

    ## Example
    ## Example
    ## Example

Expected slugs: `example`, `example-1`, `example-2`.

Test `TestExtractSectionTitleHeading`: verify that the h1 title heading is extractable. Input:

    # get( url, [params] )
    Some intro.
    ### Returns
    Return info.

headingSlug=`get-url-params` → extracts `# get( url, [params] )` and `Some intro.` (stops before `### Returns`).

Run tests from the repository root:

    go test -run TestSlugify -v ./...
    go test -run TestParseHeadings -v ./...
    go test -run TestExtractSection -v ./...
    go test -run TestHeadingSlugs -v ./...

All must pass. Run `golangci-lint run` and fix any issues.


### Milestone 2: Integrate heading filter into showDocs

This milestone modifies `showDocs()` in `docs.go` to support heading navigation. At the end of this milestone, running `./k6 x docs http expected-statuses example` prints only the Example section of that page.

The current flow in `showDocs()` is:

    1. ResolveWithLookup(args, exists) -> slug
    2. idx.Lookup(slug) -> sec
    3. printSection(env, w, idx, sec)

The new flow adds a fallback when resolution fails:

    1. ResolveWithLookup(args, exists) -> slug
    2. idx.Lookup(slug) -> if found, printSection(env, w, idx, sec) as before (no change)
    3. If NOT found AND len(args) >= 2:
       a. Try ResolveWithLookup(args[:len(args)-1], exists) -> slug
       b. idx.Lookup(slug) -> sec
       c. If found, treat args[len(args)-1] as the heading filter
       d. Read and transform the markdown content
       e. Call ExtractSection(transformedContent, headingSlug) to get the heading content
       f. If heading found, print just that content (rendered via renderMarkdown if TTY)
       g. If heading NOT found, return an error like: heading "example" not found in <slug>
    4. If still not found, return the existing "topic not found" error

The heading filter arg is the last element of `args` after `normalizeArgs` has run. Since `normalizeArgs` splits slashes, `k6 x docs http/expected-statuses/example` becomes `["http", "expected-statuses", "example"]` and the same peeling logic applies. IMPORTANT: `showDocs` must call `normalizeArgs(args)` before the peeling check, not rely on `ResolveWithLookup` normalizing internally. Otherwise `len(args) >= 2` fails for slash-joined input like `["http/expected-statuses/example"]` (length 1 before normalization). See the "Normalize args before peeling" edge case.

In `docs.go`, modify `showDocs` to implement this. The key change is between the current `idx.Lookup(slug)` failure and the error return. Add a block that peels the last arg, re-resolves, and if successful, reads the content, transforms it, extracts the heading section, and prints it.

For printing the heading-filtered content: do NOT call `printSection` (which appends the subtopics footer). Instead, read the markdown file, run `Transform`, call `ExtractSection`, then render and print. This is a simpler path — just the extracted text, rendered, with no footer.

Create a helper function `printHeadingSection` in `docs.go` to encapsulate the heading-filtered display:

    func printHeadingSection(env *docsEnv, w io.Writer, sec Section, headingSlug string) error

This function: reads the markdown file from the cache using `sec.RelPath` and `env.CacheDir`, runs `Transform(content, env.Version)`, calls `ExtractSection(transformed, headingSlug)`, returns an error if the heading is not found, otherwise renders the content (using `renderMarkdown` if TTY) and writes it to `w`.

Update `showDocs` to call `printHeadingSection` when the peeling logic identifies a heading filter.

Write tests in `docs_test.go` (or a separate test file). Test cases:

1. Happy path: args that don't fully resolve but whose prefix does resolve, last arg matches a heading slug → prints only that heading's content.
2. Heading not found: last arg doesn't match any heading → returns error mentioning the heading name.
3. Slash-joined args: `["topic/subtopic/heading-slug"]` → normalizeArgs splits, peeling works correctly.
4. Non-leaf section with heading: section has children AND headings. Full args (section + heading name that's NOT a child) fail resolution, peeling finds the section, heading filter extracts correctly.
5. Child shadows heading: section has a child named "example" AND a heading "## Example". Args resolve to the child (full resolution succeeds), heading filter is NOT triggered.

For integration testing (Milestone 4), we will use `.txtar` files with real markdown content. For this milestone, unit tests with in-memory fixtures are sufficient.

Run from the repo root:

    go test -run TestShowDocsHeading -v ./...
    golangci-lint run


### Milestone 3: Heading completions for leaf sections

This milestone modifies `completionDeeper` in `completion.go` to offer heading slugs when the resolved section has no children. At the end of this milestone, pressing Tab after typing a leaf section path suggests heading names.

The current logic in `completionDeeper`:

    1. Resolve args to slug
    2. Lookup section
    3. If section has no children -> return nil
    4. If section has children -> return child names filtered by prefix

The new logic:

    1. Resolve args to slug
    2. Lookup section
    3. If section HAS children -> return child names filtered by prefix (unchanged)
    4. If section has NO children -> parse headings from the section's markdown, return heading slugs filtered by prefix

For step 4, `completionDeeper` needs access to the cache directory to read the markdown file. Currently it only receives the `*Index`. There are two options: (a) pass the cache dir as an additional parameter, or (b) add the cache dir to a context object. Option (a) is simpler and keeps the function signature explicit.

Modify `completionDeeper` to accept a `cacheDir string` parameter:

    func completionDeeper(idx *Index, cacheDir string, version string, args []string, toComplete string) []cobra.Completion

When the resolved section has no children, `completionDeeper`:
1. Reads the section's markdown file from `filepath.Join(cacheDir, "markdown", sec.RelPath)`
2. Runs `Transform(content, version)` to get clean markdown
3. Calls `HeadingSlugs(transformed)` to get heading slugs
4. Filters by `toComplete` prefix (case-insensitive)
5. Returns the filtered slugs as completions

Update `completionTopicArgs` to pass the new parameters through to `completionDeeper`. This means `completionTopicArgs` also needs the cache dir and version. Trace the callers: `completionTopicArgs` is called from the closure in `newTopicCompletion`. The `newTopicCompletion` function already calls `setupForCompletion` which has access to the cache dir and version. Pass these through.

Update function signatures:

    func completionTopicArgs(idx *Index, cacheDir string, version string, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective)

Update `newTopicCompletion` to pass `cacheDir` and `version` from the setup results.

If the markdown file can't be read (e.g., file missing, permission error), silently return nil — completions should never fail loudly.

Add tests in `completion_test.go`:

Test `TestCompleteDeeperHeadings`: create a temp directory with a markdown file containing known headings. Create an `Index` with a leaf section (no children) whose `RelPath` points to that file. Call `completionDeeper` and verify it returns heading slugs. Also verify that when `toComplete` is set, only matching slugs are returned.

Test `TestCompleteDeeperChildrenWin`: create an `Index` with a non-leaf section (has children). Verify that `completionDeeper` returns child names, NOT heading slugs, even if the markdown has headings.

Run from the repo root:

    go test -run TestCompleteDeeper -v ./...
    golangci-lint run


### Milestone 4: Integration tests

This milestone adds a `.txtar` integration test that exercises heading navigation and completion end-to-end.

Create `testdata/scripts/heading.txtar`. This file uses the `testscript` framework. It needs a mock documentation bundle in the test archive with a markdown file that has multiple headings including duplicates.

The test archive should include:

1. A `sections.json` with at least one section entry (e.g., slug `testing-guides/my-topic`, no children, `rel_path` pointing to a markdown file in the archive).
2. A `markdown/testing-guides/my-topic.md` file with content like:

       ---
       title: My Topic
       ---
       # My Topic
       Intro paragraph.
       ## First Section
       First section content.
       ## Second Section
       Second section content.
       ### Subsection
       Subsection content.
       ## First Section
       Duplicate heading content.

After transform, the duplicate `## First Section` headings produce slugs `first-section` and `first-section-1`.

Test commands in the `.txtar`:

    # Show full page (existing behavior, should still work)
    exec k6-docs testing-guides my-topic
    stdout 'First section content'
    stdout 'Second section content'
    stdout 'Subsection content'

    # Navigate to a heading
    exec k6-docs testing-guides my-topic first-section
    stdout 'First section content'
    ! stdout 'Second section content'

    # Navigate to second section
    exec k6-docs testing-guides my-topic second-section
    stdout 'Second section content'
    stdout 'Subsection content'
    ! stdout 'First section content'

    # Navigate to duplicate heading (second occurrence)
    exec k6-docs testing-guides my-topic first-section-1
    stdout 'Duplicate heading content'
    ! stdout 'First section content'

    # Navigate to nonexistent heading -> error
    ! exec k6-docs testing-guides my-topic nonexistent
    stderr 'heading "nonexistent" not found'

    # Navigate using slash-joined form
    exec k6-docs testing-guides/my-topic/second-section
    stdout 'Second section content'
    ! stdout 'First section content'

    # Heading inside code fence should NOT be navigable
    ! exec k6-docs testing-guides my-topic this-is-a-comment
    stderr 'heading "this-is-a-comment" not found'

The markdown test file for the code-fence edge case should include a fenced code block with a heading-like line, e.g.:

    ## Second Section
    Second section content.
    ```bash
    # This is a comment
    echo hello
    ```
    ### Subsection
    Subsection content.

This verifies that `# This is a comment` inside the fence is not treated as a heading.

Look at existing `.txtar` files in `testdata/scripts/` to match the exact format, particularly how the mock bundle is set up (cache dir, sections.json placement, environment variables, the command name used — it may be `k6-docs` based on how `TestScripts` registers the command).

Run from the repo root:

    go test -run TestScripts/heading -v ./...


### Milestone 5: Update AGENTS.md

Add documentation about heading navigation to `AGENTS.md`. Under the Browsing section, add a bullet or paragraph explaining that an extra arg after a topic path filters to a specific heading within that page. Mention the GitHub-style slug rules and the `-1`/`-2` disambiguation for duplicate headings. Under Shell completions, mention that leaf sections offer heading completions.


## Concrete Steps

All commands run from the repository root: `/Users/inanc/grafana/xk6-docs-agent-ae0d0ec7`.

Milestone 1 — create `heading.go` and `heading_test.go`, run:

    go test -run TestSlugify -v ./...
    go test -run TestParseHeadings -v ./...
    go test -run TestExtractSection -v ./...
    go test -run TestHeadingSlugs -v ./...
    golangci-lint run

Expected: all tests pass, no lint errors.

Milestone 2 — modify `docs.go`, run:

    go test -run TestShowDocs -v ./...
    go test -v ./...
    golangci-lint run

Expected: existing tests still pass, new heading-filter tests pass.

Milestone 3 — modify `completion.go`, run:

    go test -run TestCompleteDeeper -v ./...
    go test -run TestCompleteTopicArgs -v ./...
    go test -v ./...
    golangci-lint run

Expected: existing completion tests still pass, new heading completion tests pass.

Milestone 4 — create `testdata/scripts/heading.txtar`, run:

    go test -run TestScripts/heading -v ./...

Expected: all txtar test assertions pass.

Milestone 5 — edit `AGENTS.md`, no command needed (documentation only).

Final verification:

    go test -v ./...
    golangci-lint run

Expected: all tests pass, no lint errors.

Manual verification (requires `xk6` installed):

    xk6 build --with xk6-docs=.
    ./k6 x docs http expected-statuses
    ./k6 x docs http expected-statuses example
    ./k6 x docs http expected-statuses returns
    ./k6 __completeNoDesc x docs http expected-statuses ""


## Validation and Acceptance

Unit tests verify the heading logic in isolation:

- `TestSlugify` — 6 cases covering normal text, parentheses, empty string
- `TestParseHeadings` — verifies level, text, slug, line number, and dedup suffix for duplicate headings
- `TestParseHeadingsNoDuplicates` — verifies no suffixes when headings are unique
- `TestParseHeadingsSkipsCodeFences` — verifies heading-like lines inside code fences are ignored
- `TestParseHeadingsSkipsEmptySlugs` — verifies headings that slugify to "" are excluded
- `TestParseHeadingsThreeDuplicates` — verifies `-1`, `-2` suffixes with 3 identical headings
- `TestExtractSection` — 5 cases: first heading, heading with sub-headings, sub-heading, last heading (to EOF), nonexistent
- `TestExtractSectionDuplicateHeadings` — verifies first vs second occurrence with dedup slugs
- `TestExtractSectionTitleHeading` — verifies h1 heading is extractable
- `TestHeadingSlugs` — verifies convenience function returns slug strings

Integration acceptance:

- `k6 x docs http expected-statuses example` prints only the Example section, not the full page. The output starts with `### Example` (or the rendered equivalent) and does not contain content from other sections.
- `k6 x docs http expected-statuses` continues to print the full page (no regression).
- `k6 x docs testing-guides automated-performance-testing example-plan` prints the first "Example plan" section.
- `k6 x docs testing-guides automated-performance-testing example-plan-1` prints the second "Example plan" section.
- `k6 x docs http expected-statuses nonexistent` returns an error mentioning the heading was not found.
- `k6 x docs http/expected-statuses/example` (slash-joined) works identically to the space-separated form.
- `k6 x docs http response example` works (non-leaf section, heading filter via peeling).
- `k6 __completeNoDesc x docs http expected-statuses ""` returns heading slugs for the leaf section.
- `k6 __completeNoDesc x docs http ""` continues to return children (get, post, etc.) — not headings.
- Headings inside code fences in real docs are not exposed as navigable headings or completions.

Run `go test -v ./...` from the repo root and expect all tests to pass. Run `golangci-lint run` and expect no errors.


## Idempotence and Recovery

All steps are safe to repeat. The new file `heading.go` is additive — if it already exists, the implementer can replace its contents. Tests are deterministic and use in-memory data or testscript sandboxes. No network calls. No mutations to the real filesystem outside the test sandbox.

If a milestone fails partway through, the implementer can re-run that milestone's tests without side effects. The changes in each milestone are backward-compatible — existing behavior is preserved at every step.


## Artifacts and Notes

Example of heading slug generation for a real page (`testing-guides/automated-performance-testing` after transform):

    ## Why automate performance tests     -> why-automate-performance-tests
    ### Example plan                       -> example-plan
    ## When to run performance tests       -> when-to-run-performance-tests
    ### Example plan                       -> example-plan-1

Example of the args-peeling resolution in `showDocs`:

    args = ["http", "expected-statuses", "example"]

    Step 1: ResolveWithLookup(["http", "expected-statuses", "example"], exists)
            -> tries "javascript-api/k6-http/expected-statuses/example"
            -> idx.Lookup fails (no such section)

    Step 2: len(args) >= 2, so peel last arg:
            ResolveWithLookup(["http", "expected-statuses"], exists)
            -> tries "javascript-api/k6-http/expected-statuses"
            -> idx.Lookup succeeds

    Step 3: headingSlug = "example" (the peeled arg)
            Read markdown, transform, ExtractSection(content, "example")
            -> finds ### Example heading, extracts content
            -> prints just that section

Example of completion flow for leaf section:

    User types: k6 x docs http expected-statuses <TAB>
    Cobra calls: completionTopicArgs(idx, cacheDir, version, ["http", "expected-statuses"], "")
    -> completionDeeper resolves to "javascript-api/k6-http/expected-statuses"
    -> section has no children (leaf)
    -> reads markdown, transforms, HeadingSlugs(content)
    -> returns ["returns", "example"] (or whatever headings exist)


## Interfaces and Dependencies

New file `heading.go` in the repository root (package `docs`):

    type Heading struct {
        Level int
        Text  string
        Slug  string
        Line  int
    }

    func ParseHeadings(content string) []Heading
    func ExtractSection(content string, headingSlug string) (string, bool)
    func HeadingSlugs(content string) []string

Unexported helper in `heading.go`:

    func slugify(text string) string

New function in `docs.go`:

    func printHeadingSection(env *docsEnv, w io.Writer, sec Section, headingSlug string) error

Modified function in `docs.go`:

    func showDocs(env *docsEnv, w io.Writer, idx *Index, args []string) error
    // Added: args-peeling fallback for heading filter

Modified functions in `completion.go`:

    func completionTopicArgs(idx *Index, cacheDir string, version string, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective)
    func completionDeeper(idx *Index, cacheDir string, version string, args []string, toComplete string) []cobra.Completion

New test file `heading_test.go` with: `TestSlugify`, `TestParseHeadings`, `TestParseHeadingsNoDuplicates`, `TestExtractSection`, `TestExtractSectionDuplicateHeadings`, `TestHeadingSlugs`.

New integration test `testdata/scripts/heading.txtar`.

Modified `AGENTS.md` — heading navigation and heading completions documented.

Dependencies: no new external dependencies. Uses only the standard library (`strings`, `regexp`, `path/filepath`, `os`) and existing package functions (`Transform`, `renderMarkdown`, `ResolveWithLookup`, `HeadingSlugs`).
