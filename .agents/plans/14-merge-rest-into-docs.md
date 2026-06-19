# Merge the xk6-rest OpenAPI reference into the docs tree

This Plan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/plans.md` (symlinked as `.claude/plans.md`).


## Purpose / Big Picture

Today there are two separate k6 extensions maintained in two repositories. This repository (`github.com/grafana/xk6-docs`) provides `k6 x docs`, an offline browser for k6 documentation. A sibling repository (`github.com/grafana/xk6-rest`) provides `k6 x rest`, a terminal browser for the Grafana Cloud k6 REST API, rendered from embedded OpenAPI specifications (an OpenAPI specification is a machine-readable description of an HTTP API: its endpoints, parameters, request/response shapes).

After this change, the REST API reference is no longer a separate command. Instead it becomes ordinary documentation inside the existing docs tree, served by the exact same machinery as every other page. A user can run:

    k6 x docs cloud-rest-api

to see the table of contents of all REST endpoints, and:

    k6 x docs cloud-rest-api/v6/load_tests_list

to read one endpoint rendered as markdown (authentication, parameters, request/response schemas, a curl example). The same `k6 x docs search <term>` now finds REST endpoints alongside prose docs, shell completion completes endpoint slugs, and AI agents reading the cached `markdown/` directory get the endpoint pages as files for free. The `k6 x rest` command ceases to exist.

Why this matters: it removes a whole second repository (its `go.mod`, `go.sum`, CI, release pipeline, and Renovate dependency-update stream) while adding zero new dependencies to this one, and it gives users a single, consistent reference surface instead of two commands with two different grammars.

How you will see it working: after implementation, building a bundle (via `cmd/prepare`) produces `markdown/cloud-rest-api/**` files and `cloud-rest-api` entries in `sections.json`; running `k6 x docs cloud-rest-api/v6/<operationId>` prints the rendered endpoint; and a new end-to-end test script asserts this.

Iteration-1 boundaries (descoped deliberately — see Decision Log): both v5 and v6 are included; v6 is fetched fresh at build time with the embedded spec as fallback, while v5 ships as a static embedded reference; there is NO runtime network (the whole xk6-rest fetch/cache layer is dropped); `--source` previews do NOT include the REST section; and scope hints are not carried over.


## Progress

- [x] (2026-06-17) Design completed; iteration-1 scope descoped and locked; this plan authored. Implementation not yet started.
- [ ] M1: Port `spec.go` + `render.go` into `internal/bundle/restdoc` (NOT `scope.go`); reword the two "SKILL.md" lines; drop cobra/runtime-coupled code; port unit tests.
- [ ] M2: Embed v5 (static) + v6 (fallback) specs; add `restdoc.Generate(afs, markdownDir, v6Spec)` producing `[]docs.Section` + markdown (3 index pages + v5/v6 leaves); unit test on fixture specs.
- [ ] M3: Add a `WithExtraSections` injection option to `internal/bundle.Build` (bundle does NOT import restdoc); `--source` passes no option so it omits REST; unit test the hook with a trivial generator.
- [ ] M4: Wire `cmd/prepare` to fetch v6 fresh at build time (embedded fallback) and inject `restdoc.Generate` via `WithExtraSections`; prepare test asserts `cloud-rest-api/v6/<op>` and the embedded-fallback path.
- [ ] M5: End-to-end serving via a new `internal/clitest/testdata/scripts/cloud_rest_api.txtar`: TOC, leaf render, search, completion.
- [ ] M6: Docs and cleanup — update `AGENTS.md`, `.agents/features.md`, the agent skill; confirm no `k6 x rest` is registered and `go mod tidy` adds no new dependency.


## Surprises & Discoveries

- Observation: Removing the `gopkg.in/yaml.v3` dependency (the original motivation, mirroring xk6-rest PR #5) is not achievable here and would not help.
  Evidence: `go mod graph` shows `go.k6.io/k6/v2` and `github.com/go-git/go-git/v5` (both direct dependencies) require `yaml.v3` transitively, so it stays in `go.mod`/`go.sum` and is linked into the shipped k6 binary regardless. `k6 x docs` Renovate history shows zero PRs ever for `yaml.v3`.

- Observation: Folding xk6-rest in adds zero new dependencies.
  Evidence: `go list -m` on xk6-rest `main` shows its only direct requires are `github.com/spf13/cobra`, `go.k6.io/k6/v2`, and `gopkg.in/yaml.v3` — all already direct requires of this repo; its indirect set is a strict subset of this repo's.

- Observation: xk6-rest's renderer (`internal/cli/render.go`) hardcodes references to a "SKILL.md" file that does not exist in the docs context.
  Evidence: `formatParamLine` emits "(see SKILL.md > Common headers)" for the `X-Stack-Id` header, and `renderEndpointResponses` emits "(schema in SKILL.md)" for error responses. These must be reworded during the port (see Decision Log).

- Observation: Descoping the rendered *output* is counterproductive; the savings come from dropping the acquisition layer, not from trimming pages.
  Evidence: `render.go`/`spec.go` already exist and are tested, so porting them verbatim is copy + repackage + reword two lines. Removing endpoint sections would mean editing battle-tested code and its tests — more effort and more risk than keeping them. So iteration 1 keeps the renderer whole and cuts only the surrounding fetch/cache/runtime layer and `--source` support.

- Observation: The Bundle Sync staleness check is coupled to k6-docs git activity, not to the OpenAPI spec.
  Evidence: `.github/scripts/sync.sh` (lines 36-49) rebuilds a version's bundle only when it is missing or when the last commit date of `docs/sources/k6/<version>` in the k6-docs repo is newer than the bundle asset's upload date. It never inspects the live v6 OpenAPI document. CONSEQUENCE: a build-time-fetched v6 spec only refreshes when the bundle is rebuilt for k6-docs reasons (or via manual `workflow_dispatch`); if the API changes while a version's k6-docs folder is quiet, that version's baked v6 snapshot stays stale until the next rebuild trigger.


## Decision Log

- Decision: Do not port xk6-rest PR #5 (YAML-to-JSON spec conversion) and do not attempt to drop `yaml.v3`.
  Rationale: The dependency cannot be removed (required transitively by k6 and go-git) and generates no Renovate noise; a hand-rolled frontmatter/spec parser would only add risk. The consolidation itself delivers the maintenance win.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Adopt UX model "C": the REST reference becomes content inside the docs tree, not a subcommand. `k6 x rest` is removed entirely with no deprecation shim.
  Rationale: The REST reference output is already markdown; making it real docs gives one search, one navigation, one rendering path, and agent-as-files for free. A nested `k6 x docs rest …` was rejected as illusory unification (two grammars under one command). xk6-rest is new/low-adoption so a clean break is acceptable.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Bake the REST content into bundles at build time. Runtime stays purely offline.
  Rationale: Consistent with the existing per-version bundle model; no runtime network, cache writes, or index merging. Freshness becomes the bundle rebuild cadence (each release plus the ~3-hourly sync), which is ample for an API reference.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Place the generated section at top-level slug `cloud-rest-api/`, with `cloud-rest-api/v5/<operationId>` and `cloud-rest-api/v6/<operationId>` beneath it.
  Rationale: We fully own this slug (verified absent from the live docs tree), so there is no coupling to the upstream, narrative-only `grafana-cloud-k6` section. The v5/v6 split mirrors the API's own versioning and keeps slugs collision-free.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Generation lives in `cmd/prepare` ONLY, injected into `internal/bundle.Build` via a `WithExtraSections` functional option; `internal/bundle` does not import `restdoc`. `--source` does not generate the REST section in iteration 1.
  Rationale: Keeps the embedded specs (~6,700 lines v6 + ~1,800 v5 YAML) out of the shipped k6 extension binary, because `restdoc` (and its embeds) is reachable only from the build tool. `--source` authors edit the k6-docs tree, not the OpenAPI spec, so omitting REST there is acceptable. The injection hook avoids duplicating `sections.json` logic in `cmd/prepare`. This supersedes the earlier idea of generating inside the shared `Build` and resolves the binary-size tradeoff in favour of a lean extension.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: During the port, reword the two "SKILL.md" references in the renderer.
  Rationale: They are artifacts of xk6-rest's origin (a Python skill generator) and are meaningless in docs. Replace the `X-Stack-Id` note with "Numeric ID of the Grafana stack." and drop the "(schema in SKILL.md)" suffix on error responses (keep "Body: `<schema>`").
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: External coordination (the k6-extension-registry change so `k6 x rest`/`cloud-rest-api` resolves to this module, and retiring/archiving the xk6-rest repo) is out of scope for this plan.
  Rationale: The user will handle registry and repo retirement. This plan is the in-repo code migration only. NOTE: until the registry advertises that this module provides the REST content, users provisioning via auto-extension-resolution will not get it automatically; this is a release-coordination follow-up, recorded here so it is not forgotten.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Drop xk6-rest's runtime acquisition layer entirely — do not port `fetch.go`, `cache.go`, `source.go`, or `runtime.go`. Iteration 1 performs no network access at runtime.
  Rationale: User descope. The runtime fetch/cache/embedded-fallback chain (and `XK6_REST_CACHE_DIR`, TTLs, provenance) is exactly the complexity we want to shed; the bundle model already handles distribution.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Include both v5 and v6. Fetch the latest v6 spec at build time in `cmd/prepare` (short timeout, embedded fallback on any error); ship v5 as a static embedded reference file.
  Rationale: User decision — v6 evolves so a build-time refresh keeps it current without runtime cost; v5 is deprecated and will not change, so a static embed is sufficient and needs no fetch path.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: v6 freshness in iteration 1 is "opportunistic + manual": accept that the baked v6 refreshes only when a bundle is rebuilt (driven by k6-docs commits per `sync.sh`) or via a manual `workflow_dispatch` of the sync workflow when we know the API changed. Do NOT modify `sync.sh` to watch the OpenAPI spec in iteration 1.
  Rationale: User decision. Active versions get frequent k6-docs commits so v6 refreshes often in practice; the manual dispatch lever covers urgent API changes; teaching `sync.sh` to diff the live spec (ETag/hash) is a worthwhile but separable later enhancement, kept out of the first iteration to limit scope (it would also touch the bash test `sync_test.sh`).
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Port the endpoint renderer verbatim — keep every section (header, Auth, Tags, Parameters, Request body with expanded schema tree + JSON example, Responses with schema + example + Errors, Invocation curl). Keep the Python-compatibility helpers as-is even though the byte-identical constraint is gone.
  Rationale: The renderer already exists and is tested; copying it whole is less work and less risk than surgically trimming sections, and full pages are the value of an API reference. The Python helpers are harmless; rewriting them would be pointless churn.
  Date/Author: 2026-06-17 / ankur22 + agent.

- Decision: Drop scope hints for iteration 1 — do not port `scope.go`.
  Rationale: `ScopeHint` was consumed only by xk6-rest's `list`/`search` one-line index format, which model C replaces with the docs TOC and `k6 x docs search` (keyed off `Title`/`Description`/slug). With no consumer, carrying it adds surface for no benefit; it can be folded into endpoint descriptions later if pages prove ambiguous.
  Date/Author: 2026-06-17 / ankur22 + agent.


## Outcomes & Retrospective

To be completed at milestone boundaries and at the end. Compare the result against the Purpose: can a user run `k6 x docs cloud-rest-api/v6/<operationId>` and read a correctly rendered endpoint, find it via `k6 x docs search`, and complete it via the shell, with no `k6 x rest` command and no new dependency?


## Context and Orientation

This repository builds a k6 extension. A k6 extension is Go code compiled into the `k6` binary that adds a subcommand; this one adds `k6 x docs`. The entry points are `register.go` (calls `subcommand.RegisterExtension("docs", newCmd)`) and `adapter.go` (bridges k6's global state to the internal CLI). The user-facing logic is in `internal/cli/`.

Documentation is not embedded. A separate build-time tool, `cmd/prepare/`, transforms the upstream Hugo documentation repository (`github.com/grafana/k6-docs`) into a "bundle": a directory containing `markdown/**/*.md` (cleaned markdown files) and `sections.json` (a structured index). Bundles are published as compressed release assets and downloaded/cached on first use. The transform pipeline is the package `internal/bundle` (function `bundle.Build`); it is shared by `cmd/prepare` and by the `--source` local-preview feature in `internal/cli/bundle.go`.

When does `cmd/prepare` actually run ("build time")? Only in the Bundle Sync GitHub Actions workflow (`.github/workflows/sync.yml`, cron `'3 */3 * * *'` = every 3 hours, plus manual `workflow_dispatch`) and in local/manual runs. It does NOT run during the k6 binary release (`.github/workflows/release.yml` builds the binary only — the binary contains no docs) nor at CLI runtime (the user downloads the already-built bundle). So the v6 fetch and REST generation in this plan happen inside the sync job, not when the extension is compiled or run.

Key types, by full path:

- `docs/types.go` defines `Section` (fields: `Slug`, `RelPath`, `Title`, `Description`, `Weight`, `Category`, `Children []string`, `IsIndex bool`, `Aliases`) and `Index` (`Version string`, `Sections []Section`). A "slug" is the navigation path (for example `javascript-api/k6-http/get`). A "RelPath" is the path of the markdown file under `markdown/` (for example `javascript-api/k6-http/get.md`). An `_index.md` page represents a section that has children; its slug is the directory.

- `internal/bundle/bundle.go` is the pipeline. `Build(k6Version, k6DocsPath, outputDir string, afs docs.FS, w io.Writer) error` orchestrates: `buildSharedContentMap` then `walkAndProcess` (walks the docs tree, writes one transformed markdown file per page and returns `[]docs.Section`) then `populateChildren` (fills each index section's `Children` by slug prefix, sorted by `Weight`) then `writeSectionsJSON` then `writeBestPractices`. The function `populateChildren` operates purely on the `[]docs.Section` slice, so any sections we append before it runs are wired into the tree automatically. Iteration 1 adds a `WithExtraSections` option so a caller (only `cmd/prepare`) can inject extra sections at exactly this point; `internal/bundle` itself does not import the REST generator, which is what keeps the embedded specs out of the extension binary.

- `docs/catalog.go` serves content at runtime: `Catalog.Read(ctx, version, slug)` looks up the `Section` by slug and reads its `RelPath` from `{version}/markdown/`. `internal/cli/ui.go` `printSection` renders that markdown (glamour for TTY) and, if the section has children, appends a subtopics tree. Search (`internal/cli/`, `searchResults`) and completion (`internal/cli/completion.go`) both iterate `Index.Sections`. CONSEQUENCE: a `Section` we synthesize plus a markdown file we write under `markdown/` is indistinguishable from a real page to serving, search, and completion. We do not need to touch the serving, search, or completion code.

The code to be ported lives in the xk6-rest working tree at `/Users/ankuragarwal/go/src/github.com/grafana/xk6-rest` on branch `main` (NOT the PR #5 branch). The relevant files are:

- `internal/cli/spec.go`: parses OpenAPI YAML bytes into a normalized model. Key exports: `type Spec` (has `Operations []Operation`, `BaseURL`, `Schemas`, `SecuritySchemes`), `type Operation` (has `OperationID`, `Method`, `Path`, `Summary`, `Description`, `Tags`, `Parameters`, `Responses`, request-body fields), `type OrderedMap` (insertion-order-preserving map used so output ordering is stable), `LoadSpecFromBytes([]byte) (*Spec, error)`, and `(*Spec).PrefixOperationIDs(prefix string)`. It imports `gopkg.in/yaml.v3` (already a dependency here).

- `internal/cli/render.go`: `RenderEndpoint(spec *Spec, op *Operation) string` returns the full markdown for one endpoint (header, Auth, Tags, Parameters, Request body, Responses, Invocation). Pure function; output is standard markdown safe for glamour. Contains the two "SKILL.md" references to reword.

- `internal/cli/scope.go`: `ScopeHint(op *Operation) string` returns a short disambiguation phrase (for example "org-wide", "single load_test by id"). NOT ported in iteration 1 — scope hints are dropped (see Decision Log); listed here only so a future contributor knows where it came from.

The bundle test harness: `cmd/prepare/main_test.go` builds from a fixture docs tree under `cmd/prepare/testdata/mockdocs/`. The end-to-end CLI harness: `internal/clitest/docs_test.go` builds the real binary and runs `.txtar` scripts from `internal/clitest/testdata/scripts/`, copying a fixture bundle from `internal/clitest/testdata/cache/` into each test's working directory. Existing scripts `render.txtar` and `toc.txtar` are the patterns to copy for M5.


## Plan of Work

The work proceeds additively in six milestones, each independently buildable and testable, so the tree compiles and tests pass at every commit.

Milestone 1 (port the OpenAPI machinery). Create a new package `internal/bundle/restdoc` (package name `restdoc`). Copy `spec.go` and `render.go` from xk6-rest's `internal/cli/` into it, changing the package clause to `restdoc` and removing the parts not needed for generation: the cobra-coupled `CmdShow`/`reportBareID`, the runtime `SpecBytes`/`SpecBytesV5` package globals, and `LoadSpec` (we pass bytes explicitly). Do NOT port `scope.go` — scope hints are dropped for iteration 1. Keep `LoadSpecFromBytes`, the model types, `RenderEndpoint`, and `PrefixOperationIDs`. Apply the two "SKILL.md" rewordings from the Decision Log. Copy the corresponding unit tests (`spec_test.go` and the render tests), dropping any scope-hint assertions, and adjust them to the new package and to passing bytes directly. At the end of M1, `go test ./internal/bundle/restdoc/...` passes and nothing else references the package yet.

Milestone 2 (generate sections and markdown). In `internal/bundle/restdoc`, embed `openapi-v5.yaml` (static v5) and `openapi.yaml` (v6 fallback) via `//go:embed`, copied from the xk6-rest module root. Add `Generate(afs docs.FS, markdownDir string, v6Spec []byte) ([]docs.Section, error)`. It parses the embedded v5 spec and either the supplied `v6Spec` (when non-nil) or the embedded v6 fallback, calls `PrefixOperationIDs("v5")` and `PrefixOperationIDs("v6")` so operation IDs carry their version, then for each operation writes `markdownDir/cloud-rest-api/<v>/<bareOperationId>.md` containing the verbatim `RenderEndpoint(...)` output, and appends a leaf `docs.Section{Slug: "cloud-rest-api/<v>/<bareOperationId>", RelPath: same+".md", Title: bareOperationId, Description: op.Summary, Weight: <document-order index>, Category: "cloud-rest-api", IsIndex: false}`. It also writes three `_index.md` files and appends three `IsIndex: true` index sections: `cloud-rest-api` (title "Grafana Cloud k6 REST API"), `cloud-rest-api/v5` (title "v5 — Metrics API"), and `cloud-rest-api/v6` (title "v6 — General-purpose API"). Unit test: call `Generate` with `nil` v6 (so the embedded fallback is used) against tiny in-repo fixture specs and assert the returned sections and the written files for both versions.

Milestone 3 (injection hook in Build, keeping the extension binary lean). Add functional options to `internal/bundle.Build`: `Build(k6Version, k6DocsPath, outputDir string, afs docs.FS, w io.Writer, opts ...BuildOption)` with `type BuildOption func(*buildConfig)` and `func WithExtraSections(fn func(afs docs.FS, markdownDir string) ([]docs.Section, error)) BuildOption`. After `walkAndProcess` returns `sections`, if a generator `fn` is set, call it and append its sections before `populateChildren`. Crucially, `internal/bundle` does NOT import `restdoc`; the caller supplies the function, which is what keeps the embedded specs out of the extension binary. The `--source` path (`internal/cli/bundle.go`) keeps calling `Build` with no options, so `--source` previews omit the REST section. Add a `bundle` unit test that passes a trivial `WithExtraSections` returning one fake section and asserts it appears in `sections.json` and is wired by `populateChildren`.

Milestone 4 (wire cmd/prepare with build-time v6 fetch). In `cmd/prepare/main.go`, import `restdoc`. Before/around the `Build` call, attempt a single HTTP GET of `https://api.k6.io/cloud/v6/openapi` with a short timeout (plain `net/http`; no new dependency — an `Accept` header is unnecessary since `LoadSpecFromBytes` parses YAML and OpenAPI JSON is valid YAML). On success, capture the body as the v6 override; on any error, log to stderr and use `nil` (embedded fallback). Pass `bundle.WithExtraSections(func(afs docs.FS, md string) ([]docs.Section, error) { return restdoc.Generate(afs, md, v6Bytes) })`. Because only `cmd/prepare` imports `restdoc`, the embedded specs live only in the build tool. Extend `cmd/prepare/main_test.go` to assert that after a build the bundle contains `cloud-rest-api/v6/<op>` (markdown file + `sections.json` entry) and that a simulated fetch failure still succeeds via the embedded v6 spec (inject the fetch URL or an `http.Client` through a small seam so the test can force failure).

Milestone 5 (end-to-end serving). Add `internal/clitest/testdata/scripts/cloud_rest_api.txtar`, modeled on `render.txtar` and `toc.txtar`. It places a small fixture bundle containing a couple of `cloud-rest-api` sections and their markdown into the test cache (hand-authored fixture, independent of the real specs, to keep the test small and deterministic), then asserts: `k6 x docs cloud-rest-api` prints the section TOC including the v5 and v6 children; `k6 x docs cloud-rest-api/v6/<op>` prints the rendered endpoint (assert on a stable substring such as the `## Invocation` heading or the `curl` line); `k6 x docs search <endpoint-term>` lists the endpoint; and a completion request (the harness already exercises completion in `completion.txtar`) includes `cloud-rest-api`.

Milestone 6 (docs and cleanup). Update `AGENTS.md` (add a short "Cloud REST API reference" subsection explaining that `cloud-rest-api/**` is generated at build time from embedded OpenAPI specs, v6 refreshed at build time with embedded fallback). Update `.agents/features.md` (user-facing: `k6 x docs cloud-rest-api/...`). Update the agent skill under `skills/xk6-docs/` to mention the new section and a navigation recipe. Confirm no `subcommand.RegisterExtension("rest", ...)` exists anywhere. Run `go mod tidy` and confirm the diff adds no new module. Update this plan's Outcomes section.


## Concrete Steps

All commands run from the repository root `/Users/ankuragarwal/go/src/github.com/grafana/xk6-docs` unless stated.

Create the package directory and port files (M1). Source files are at `/Users/ankuragarwal/go/src/github.com/grafana/xk6-rest/internal/cli/`. After copying and editing, verify:

    go build ./...
    go test ./internal/bundle/restdoc/...

Expected: build succeeds; the ported unit tests pass.

Wire and run the bundle test (M3):

    go test ./cmd/prepare/... ./internal/bundle/...

Expected: a test named for the REST generation fails before M2/M3 wiring and passes after; it asserts `cloud-rest-api/v6/<op>` appears in `sections.json`.

End-to-end (M5):

    go test ./internal/clitest/...

Expected: the new `cloud_rest_api.txtar` passes. To preview by hand against the real specs, build a bundle and point the CLI at it:

    go run ./cmd/prepare --k6-version v1.6.x --output-dir /tmp/bundle
    ls /tmp/bundle/markdown/cloud-rest-api/v6 | head

Expected: a list of `<operationId>.md` files. Then, using the built test binary or an xk6 build, `k6 x docs cloud-rest-api/v6/<one-of-those>` prints a rendered endpoint.

Whole suite and tidy (M6):

    go test ./...
    go mod tidy && git diff --stat go.mod go.sum

Expected: all tests pass; `go.mod`/`go.sum` show no added module (only possibly formatting).


## Validation and Acceptance

Acceptance is behavioral, verifiable by a novice:

After `go run ./cmd/prepare --k6-version v1.6.x --output-dir /tmp/bundle`, the directory `/tmp/bundle/markdown/cloud-rest-api/` exists and contains `_index.md`, `v5/`, and `v6/` with one `.md` per endpoint, and `/tmp/bundle/sections.json` contains objects whose `slug` begins with `cloud-rest-api/`.

Running the docs CLI (via the clitest binary or an xk6-built k6) against such a bundle: `k6 x docs cloud-rest-api` lists v5 and v6 as subtopics; `k6 x docs cloud-rest-api/v6/<operationId>` prints markdown containing the method/path line and a `## Invocation` curl block; `k6 x docs search <a term from an endpoint summary>` includes that endpoint in results.

`k6 x rest` does not exist (no extension registers it).

`go test ./...` passes, and `go mod tidy` adds no new dependency.

The new tests must fail before their corresponding implementation and pass after (state this explicitly in each test's commit).


## Idempotence and Recovery

All steps are additive and repeatable. `cmd/prepare` writes into a clean `--output-dir`; re-running overwrites deterministically (operations are emitted in document order with stable weights, so `sections.json` is stable across runs given the same specs). If the build-time v6 fetch fails, the build still succeeds via the embedded fallback, so a network outage cannot break a release build. Porting is reversible: `internal/bundle/restdoc` is a new directory; deleting it and reverting the `Build` signature restores prior behavior. No destructive operations on the working tree are required.


## Artifacts and Notes

Slug and file layout produced by `Generate`:

    markdown/cloud-rest-api/_index.md
    markdown/cloud-rest-api/v5/_index.md
    markdown/cloud-rest-api/v6/_index.md
    markdown/cloud-rest-api/v6/load_tests_list.md
    ...

Corresponding sections.json entries (shape, not literal):

    { "slug": "cloud-rest-api", "rel_path": "cloud-rest-api/_index.md", "is_index": true, "children": ["cloud-rest-api/v5", "cloud-rest-api/v6"], ... }
    { "slug": "cloud-rest-api/v6/load_tests_list", "rel_path": "cloud-rest-api/v6/load_tests_list.md", "title": "load_tests_list", "description": "<summary>", "category": "cloud-rest-api" }

The two renderer rewordings (in the ported `render.go`):

    X-Stack-Id line: replace "...(see SKILL.md > Common headers)." with "...Numeric ID of the Grafana stack."
    error response suffix: replace ' Body: `X` (schema in SKILL.md).' with ' Body: `X`.'


## Interfaces and Dependencies

No new modules. Uses existing `gopkg.in/yaml.v3` (parsing), `encoding/json` (already used by `bundle`), and `net/http` (build-time v6 fetch in `cmd/prepare`).

In `internal/bundle/restdoc` (imported ONLY by `cmd/prepare`, so its embedded specs never reach the extension binary), define and export:

    func LoadSpecFromBytes(specBytes []byte) (*Spec, error)
    func RenderEndpoint(spec *Spec, op *Operation) string
    func (s *Spec) PrefixOperationIDs(prefix string)
    func Generate(afs docs.FS, markdownDir string, v6Spec []byte) ([]docs.Section, error)

`Generate` always uses the embedded v5 spec; it uses `v6Spec` when non-nil, otherwise the embedded v6 fallback. `internal/bundle/restdoc` embeds, via `//go:embed`, `openapi.yaml` (v6 fallback) and `openapi-v5.yaml`, copied from the xk6-rest module root. `scope.go`/`ScopeHint` is NOT ported in iteration 1.

In `internal/bundle`, change `Build` and add (note: `bundle` does NOT import `restdoc` — the generator is injected by the caller):

    type BuildOption func(*buildConfig)
    func WithExtraSections(fn func(afs docs.FS, markdownDir string) ([]docs.Section, error)) BuildOption
    func Build(k6Version, k6DocsPath, outputDir string, afs docs.FS, w io.Writer, opts ...BuildOption) error

In `cmd/prepare/main.go`: fetch v6 (embedded fallback) then call `Build(..., bundle.WithExtraSections(func(afs docs.FS, md string) ([]docs.Section, error) { return restdoc.Generate(afs, md, v6Bytes) }))`.


## Revision note

2026-06-17: Initial authoring. Captures the decisions reached in discussion (model C, build-time bake, top-level `cloud-rest-api/`, clean removal of `k6 x rest`, generation inside shared `Build` with a v6 override from `cmd/prepare`). The one open tradeoff flagged for the user is the few-hundred-KB growth of the extension binary from embedding the specs (reachable via `--source`); the alternative (generation in `cmd/prepare` only) is recorded in the Decision Log.

2026-06-17 (descope pass): Iteration 1 narrowed per user direction. (1) Generation moves to `cmd/prepare` ONLY, injected via a new `WithExtraSections` option, so the embedded specs stay out of the extension binary and `--source` previews omit REST — this resolves the open binary-size tradeoff in favour of a lean extension. (2) The entire xk6-rest runtime acquisition layer (`fetch.go`/`cache.go`/`source.go`/`runtime.go`) is dropped; no runtime network. (3) v5 + v6 both included; v6 is fetched fresh at build time with embedded fallback, v5 is a static embedded reference. (4) The endpoint renderer is ported verbatim (all sections) — trimming it was judged more work and risk than keeping it. (5) Scope hints are dropped; `scope.go` is not ported. Milestones, Interfaces (`WithV6Spec` → `WithExtraSections`), Progress, Surprises, and the Decision Log were updated to match. Implementation has not started; awaiting the user's go-ahead.
