# xk6-docs user-affecting behaviors

Every user-observable behavior of the `k6 x docs` CLI.

## CLI commands and flags (cmd.go)

1. `k6 x docs` with no args: TTY shows TOC, non-TTY prints agent guide (markdown dir path + browsing tips)
2. `k6 x docs <topic> [subtopic...]` shows topic content
3. `k6 x docs search <term>` fuzzy-searches docs
4. `k6 x docs skill [dir]` installs agent skill or shows help
5. `--version` flag overrides k6 version
6. `--cache-dir` flag overrides cache directory
7. `--depth` flag controls subtopic nesting (default 1)
8. `-p/--pager` flag pipes through `$PAGER` (default `less -r`)
9. `-w/--width` flag controls text wrap (0 = auto-detect terminal, fallback 80)
10. `K6_DOCS_VERSION` env var used when no `--version`
11. `K6_DOCS_CACHE_DIR` env var used when no `--cache-dir`
12. TTY stdout → glamour ANSI rendering
13. Non-TTY stdout → raw markdown (for topic lookups); agent guide (for no-args)
14. `--no-color` flag disables rendering even on TTY
15. Debug log: "interactive mode" or "agent mode" on stderr
16. Auto-downloads docs on first run

## Documentation display (docs.go)

17. TOC header: `# k6 <version>`
18. TOC lists top-level categories as bullets, sorted by weight
19. TOC shows example hint at bottom
20. `best-practices` topic reads and displays best_practices.md
21. Slug resolution from args (space/slash separated)
22. "topic not found" error for unknown topics
23. Section with children shows subtopics footer with bullet list and example hint
24. Child names strip parent prefix (e.g. `child-a-clear` → `clear`)
25. Search "(no results)" for no matches
26. Single search group auto-navigates to deepest match
27. Multi-group search: sorted alphabetically, children listed, example hint
28. `javascript-api` slug excluded from search grouping
29. JS API modules grouped by second path segment
30. best_practices.md missing → error

## Shell completion (completion.go)

31. First-arg completion: top-level slugs + JS API modules (k6- stripped) + "best-practices"
32. Deeper-arg completion: child names relative to resolved slug
33. Prefix filtering (case-insensitive)
34. Deduplicates child names
35. NoFileComp directive always set
36. Nil completions if index fails to load

## Skill installation (skill.go)

37. No-arg skill: markdown table of 11 agents with skill directory paths
38. Skill with dir: installs SKILL.md + scripts to `<dir>/xk6-docs/`
39. Replaces `<binary>` placeholder with resolved k6 binary path
40. Reinstall removes stale files from previous install
41. Binary path resolved from PATH (bare name) or cwd (relative)
42. "Skill installed to ..." confirmation message
43. `.sh` files installed with executable permission (0750)

## Cache and download (cache.go)

44. Default cache: `~/.local/share/k6/docs/<version>/`
45. `HOME` env var determines home dir
46. `USERPROFILE` fallback when `HOME` unset
47. Error when neither `HOME` nor `USERPROFILE` set
48. Version validation: rejects path traversal (`..`, `/`, spaces, tabs)
49. Auto-download on first run from GitHub releases
50. `K6_DOCS_BUNDLE_URL` env var overrides download URL
51. Staleness check every 24h via ETag HEAD request
52. Same ETag → refresh timestamp only
53. Different ETag → re-download
54. Network/timeout error → serve stale cache silently
55. 10s timeout on staleness check and refresh
56. Oversized file rejection (>50MB per file)
57. Bundle size cap (100MB)
58. Path traversal rejection in tar entries (`../`, absolute paths)
59. Symlink entries silently skipped
60. Cache dir permissions: 0750
61. File permissions: 0640
62. Cleanup on extraction failure (removes partial cache dir)
63. `.last_check` / `.etag` metadata written after download
64. Corrupt `.last_check` self-heals (treated as stale)
65. Corrupt `.etag` self-heals (treated as stale)
66. HTTP error → download error message

## Slug resolution (resolve.go)

67. Space args joined as slug path (`mod-a fn-one` → `javascript-api/k6-mod-a/fn-one`)
68. Slash args flattened (`mod-a/fn-one` → same)
69. `k6-` prefix dedup (`k6-mod-a` → `mod-a` → `javascript-api/k6-mod-a`)
70. Case-insensitive resolution
71. Category + subtopic resolution (`alpha topic-one` → `alpha/topic-one`)
72. Parent fallback (`child-a clear` → `child-a/child-a-clear`)
73. Bare name prefers unprefixed when both exist (`lib-c` over `k6-lib-c`)

## Index and search (sections.go)

74. Index loaded from `sections.json`
75. Error on missing/malformed `sections.json`
76. Case-insensitive slug lookup
77. Search by title substring
78. Search by description substring
79. Search by body content (via readContent callback)
80. Fuzzy search: normalized matching ignoring spaces/dashes/slashes
81. Children sorted by weight
82. Missing child slugs gracefully skipped
83. TopLevel = sections where Category == Slug, sorted by weight

## Markdown transformation (transform.go)

84. Strips YAML frontmatter
85. Strips `{{< code >}}`/`{{< /code >}}` tags (keeps code content)
86. Converts `{{< admonition type="X" >}}` to `> **X:** ...` blockquotes
87. Strips `{{< section >}}` tags
88. Strips remaining Hugo shortcodes (youtube, card-grid, collapse, hero-simple)
89. Collapse content preserved (only tags stripped)
90. Replaces `<K6_VERSION>` with actual version
91. Strips markdown links to plain text
92. Strips markdown images to alt text
93. Strips HTML comments
94. Strips PascalCase component tags (`<Glossary>`, `<DescriptionList>`, etc.)
95. Strips `<br/>` / `<br />` / `<br>` tags
96. Normalizes 3+ consecutive newlines to 2
97. Resolves `{{< docs/shared >}}` shortcodes (build-time, via PrepareTransform)
98. Empty input → empty output

## Rendering (render.go)

99. Glamour auto dark/light style based on terminal background (COLORFGBG detection)
100. Width-aware text wrapping

## Version detection (version.go)

101. Auto-detects k6 version from `go.k6.io/k6` build dependency
102. Error if build info unavailable or k6 dep missing
103. `v1.5.0` → `v1.5.x` wildcard mapping
104. Pre-release stripped (`v1.5.0-rc.1` → `v1.5.x`)
105. Build metadata stripped (`v1.5.0+build` → `v1.5.x`)
106. No-v prefix normalized (`1.5.0` → `v1.5.x`)
107. Major.minor only → unchanged (`v1.5` stays `v1.5`)

## Extension registration (register.go)

108. Extension registered as `k6 x docs`

## Local source preview (`--source`)

109. `--source <k6-docs-path>` builds docs from a local k6-docs checkout instead of downloading; reflects edits on the next run
110. Source builds use the same transform as published bundles (shortcodes, admonitions, links)
111. With `--source`, `--version` defaults to `next` (in-development docs); explicit `--version`/`K6_DOCS_VERSION` overrides
112. Source bundle is built under the cache base (`.local/share/k6/docs/.sources/<hash>/`), isolated from downloaded bundles and version discovery, served local-only (no network)
113. Each distinct source path builds into its own subdir; different checkouts don't collide
114. Shell completion does not reflect `--source` (uses the normal version cache)
115. Rebuild is skipped when the source's markdown files are unchanged since the last build (stamp of path/size/mtime)
116. A rebuild prints `Building k6 <version> docs from <path>...` on stderr; a skipped (unchanged) run prints nothing

## Cloud REST API reference (`cloud-rest-api/`)

117. `k6 x docs cloud-rest-api` browses the Grafana Cloud k6 REST API reference (v5 + v6) like any other docs section
118. Each endpoint is a page at `cloud-rest-api/<v5|v6>/<operationId>` rendering auth, parameters, request/response schemas, and a curl example
119. The reference is searchable (`k6 x docs search`) and shell-completable like other topics
120. Content is baked into the bundle at build time and served offline (no runtime network); the v6 snapshot refreshes when bundles rebuild, v5 is static
121. Not produced under `--source`
